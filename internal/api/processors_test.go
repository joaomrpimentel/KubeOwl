package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// --- Testes para getPodStatus ---
func TestGetPodStatus(t *testing.T) {
	testCases := []struct {
		name             string
		pod              v1.Pod
		expectedStatus   string
		expectedRestarts int32
	}{
		{
			name: "Status Running",
			pod: v1.Pod{Status: v1.PodStatus{Phase: v1.PodRunning, ContainerStatuses: []v1.ContainerStatus{
				{State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}, RestartCount: 2},
			}}},
			expectedStatus:   "Running",
			expectedRestarts: 2,
		},
		{
			name: "Status CrashLoopBackOff",
			pod: v1.Pod{Status: v1.PodStatus{ContainerStatuses: []v1.ContainerStatus{
				{State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}, RestartCount: 5},
			}}},
			expectedStatus:   "CrashLoopBackOff",
			expectedRestarts: 5,
		},
		{
			name: "Status Terminated",
			pod: v1.Pod{Status: v1.PodStatus{Phase: v1.PodSucceeded, ContainerStatuses: []v1.ContainerStatus{
				{State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{Reason: "Completed"}}, RestartCount: 0},
			}}},
			expectedStatus:   "Completed",
			expectedRestarts: 0,
		},
		{
			name:             "Status Pending (fallback to phase)",
			pod:              v1.Pod{Status: v1.PodStatus{Phase: v1.PodPending, ContainerStatuses: []v1.ContainerStatus{}}},
			expectedStatus:   "Pending",
			expectedRestarts: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			status, restarts := getPodStatus(tc.pod)
			assert.Equal(t, tc.expectedStatus, status)
			assert.Equal(t, tc.expectedRestarts, restarts)
		})
	}
}

// --- Testes para processClusterCapacity ---
func TestProcessClusterCapacity(t *testing.T) {
	nodes := &v1.NodeList{
		Items: []v1.Node{
			{
				Status: v1.NodeStatus{
					Allocatable: v1.ResourceList{
						v1.ResourceCPU:    *resource.NewMilliQuantity(2000, resource.DecimalSI),
						v1.ResourceMemory: *resource.NewQuantity(4*1024*1024*1024, resource.BinarySI),
					},
				},
			},
		},
	}
	nodeMetrics := &metricsv1beta1.NodeMetricsList{
		Items: []metricsv1beta1.NodeMetrics{
			{
				Usage: v1.ResourceList{
					v1.ResourceCPU:    *resource.NewMilliQuantity(500, resource.DecimalSI),
					v1.ResourceMemory: *resource.NewQuantity(1*1024*1024*1024, resource.BinarySI),
				},
			},
		},
	}

	capacity := processClusterCapacity(nodes, nodeMetrics)
	assert.Equal(t, int64(2000), capacity.TotalCPU)
	assert.Equal(t, int64(500), capacity.UsedCPU)
	assert.InDelta(t, 25.0, capacity.CPUUsagePercentage, 0.01)
}

// --- Testes para processNamespaces ---
func TestProcessNamespaces(t *testing.T) {
	namespaces := &v1.NamespaceList{
		Items: []v1.Namespace{
			{ObjectMeta: metav1.ObjectMeta{Name: "app-1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "app-2"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}}, // System namespace
			{ObjectMeta: metav1.ObjectMeta{Name: "default"}},     // System namespace
			{ObjectMeta: metav1.ObjectMeta{Name: "cattle-system"}}, // System namespace by prefix
		},
	}

	count, userNamespaces := processNamespaces(namespaces)
	assert.Equal(t, 2, count, "Deveria contar apenas 2 namespaces de usuário")
	assert.True(t, userNamespaces["app-1"])
	assert.True(t, userNamespaces["app-2"])
	assert.False(t, userNamespaces["kube-system"], "Não deveria incluir namespaces de sistema")
}

// --- Testes para processPodInfo ---
func TestProcessPodInfo_WithMetrics(t *testing.T) {
	pods := &v1.PodList{Items: []v1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "app-ns"},
			Spec:       v1.PodSpec{NodeName: "node-1"},
			Status:     v1.PodStatus{Phase: v1.PodRunning},
		},
	}}
	podMetrics := &metricsv1beta1.PodMetricsList{Items: []metricsv1beta1.PodMetrics{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "app-ns"},
			Containers: []metricsv1beta1.ContainerMetrics{
				{
					Name: "container-1",
					Usage: v1.ResourceList{
						v1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
						v1.ResourceMemory: *resource.NewQuantity(200*1024*1024, resource.BinarySI), // 200Mi
					},
				},
			},
		},
	}}
	userNamespaces := map[string]bool{"app-ns": true}

	podInfoList := processPodInfo(pods, podMetrics, userNamespaces)

	assert.Len(t, podInfoList, 1)
	podInfo := podInfoList[0]
	assert.Equal(t, "pod-1", podInfo.Name)
	assert.Equal(t, "100 m", podInfo.UsedCPU)
	assert.Equal(t, "200.00 Mi", podInfo.UsedMemory)
	assert.Equal(t, int64(100), podInfo.UsedCPUMilli)
	assert.Equal(t, int64(200*1024*1024), podInfo.UsedMemoryBytes)
}

// --- Testes para processIngressInfo ---
func TestProcessIngressInfo_WithRules(t *testing.T) {
	ingresses := &networkingv1.IngressList{Items: []networkingv1.Ingress{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "ingress-1", Namespace: "app-ns"},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{
					{
						Host: "example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path: "/",
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{Name: "my-service"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}}
	userNamespaces := map[string]bool{"app-ns": true}

	ingressInfoList := processIngressInfo(ingresses, userNamespaces)
	assert.Len(t, ingressInfoList, 1)
	ingressInfo := ingressInfoList[0]
	assert.Equal(t, "example.com", ingressInfo.Hosts)
	assert.Equal(t, "my-service", ingressInfo.Service)
}

// --- Testes para processPvcs ---
func TestProcessPvcs(t *testing.T) {
	pvcs := &v1.PersistentVolumeClaimList{Items: []v1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pvc-1", Namespace: "app-ns"},
			Spec: v1.PersistentVolumeClaimSpec{
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceStorage: resource.MustParse("5Gi")},
				},
			},
			Status: v1.PersistentVolumeClaimStatus{Phase: v1.ClaimBound},
		},
	}}
	userNamespaces := map[string]bool{"app-ns": true}

	pvcInfoList := processPvcs(pvcs, userNamespaces)
	assert.Len(t, pvcInfoList, 1)
	pvcInfo := pvcInfoList[0]
	assert.Equal(t, "pvc-1", pvcInfo.Name)
	assert.Equal(t, "Bound", pvcInfo.Status)
	assert.Equal(t, "5Gi", pvcInfo.Capacity)
}

// --- Testes para Casos de Borda e Entradas Nulas ---
func TestProcessFunctions_NilInput(t *testing.T) {
	assert.NotPanics(t, func() { processNodeInfo(nil, nil, nil) }, "processNodeInfo com nil")
	assert.NotPanics(t, func() { processPodInfo(nil, nil, nil) }, "processPodInfo com nil")
	assert.NotPanics(t, func() { processServiceInfo(nil, nil) }, "processServiceInfo com nil")
	assert.NotPanics(t, func() { processIngressInfo(nil, nil) }, "processIngressInfo com nil")
	assert.NotPanics(t, func() { processPvcs(nil, nil) }, "processPvcs com nil")
	assert.NotPanics(t, func() { processEvents(nil, nil) }, "processEvents com nil")
	assert.NotPanics(t, func() { countPodsOnNode("any-node", nil) }, "countPodsOnNode com nil")
	assert.NotPanics(t, func() { getNodeUsage("any-node", nil) }, "getNodeUsage com nil")
}

func TestGetNodeUsage_NoMatch(t *testing.T) {
	nodeMetrics := &metricsv1beta1.NodeMetricsList{
		Items: []metricsv1beta1.NodeMetrics{
			{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
		},
	}
	cpu, mem := getNodeUsage("node-2", nodeMetrics)
	assert.True(t, cpu.IsZero())
	assert.True(t, mem.IsZero())
}

func TestProcessEvents_NamespaceFilter(t *testing.T) {
	events := &v1.EventList{
		Items: []v1.Event{
			{
				ObjectMeta:     metav1.ObjectMeta{Name: "event-1", Namespace: "kube-system", CreationTimestamp: metav1.NewTime(time.Now())},
				LastTimestamp:  metav1.NewTime(time.Now()),
				InvolvedObject: v1.ObjectReference{Kind: "Pod", Name: "kube-proxy-abc"},
			},
			{
				ObjectMeta:     metav1.ObjectMeta{Name: "event-2", Namespace: "", CreationTimestamp: metav1.NewTime(time.Now())},
				LastTimestamp:  metav1.NewTime(time.Now()),
				InvolvedObject: v1.ObjectReference{Kind: "Node", Name: "node-1"},
			},
		},
	}
	userNamespaces := map[string]bool{}

	eventInfo := processEvents(events, userNamespaces)

	assert.Len(t, eventInfo, 1)
	assert.Equal(t, "Node/node-1", eventInfo[0].Object, "Apenas o evento do nó (sem namespace) deveria ser retornado")
}
