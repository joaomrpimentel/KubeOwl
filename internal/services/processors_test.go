package services

import (
	"io"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// TestMain silencia a saída de log durante os testes deste pacote.
func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

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
			name:             "Status Pending",
			pod:              v1.Pod{Status: v1.PodStatus{Phase: v1.PodPending}},
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

func TestProcessClusterCapacity(t *testing.T) {
	nodes := &v1.NodeList{
		Items: []v1.Node{
			{Status: v1.NodeStatus{Allocatable: v1.ResourceList{
				v1.ResourceCPU:    *resource.NewMilliQuantity(2000, resource.DecimalSI),
				v1.ResourceMemory: *resource.NewQuantity(4*1024*1024*1024, resource.BinarySI),
			}}},
		},
	}
	nodeMetrics := &metricsv1beta1.NodeMetricsList{
		Items: []metricsv1beta1.NodeMetrics{
			{Usage: v1.ResourceList{
				v1.ResourceCPU:    *resource.NewMilliQuantity(500, resource.DecimalSI),
				v1.ResourceMemory: *resource.NewQuantity(1*1024*1024*1024, resource.BinarySI),
			}},
		},
	}

	capacity := processClusterCapacity(nodes, nodeMetrics)
	assert.Equal(t, int64(2000), capacity.TotalCPU)
	assert.Equal(t, int64(500), capacity.UsedCPU)
	assert.InDelta(t, 25.0, capacity.CPUUsagePercentage, 0.01)
}

func TestProcessNamespaces(t *testing.T) {
	namespaces := &v1.NamespaceList{
		Items: []v1.Namespace{
			{ObjectMeta: metav1.ObjectMeta{Name: "app-1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "cattle-monitoring-system"}},
		},
	}
	count, userNS := processNamespaces(namespaces)
	assert.Equal(t, 1, count)
	assert.True(t, userNS["app-1"])
	assert.False(t, userNS["kube-system"])
	assert.False(t, userNS["cattle-monitoring-system"])
}

func TestProcessServiceInfo(t *testing.T) {
	services := &v1.ServiceList{Items: []v1.Service{{
		ObjectMeta: metav1.ObjectMeta{Name: "my-service", Namespace: "app-ns"},
		Spec:       v1.ServiceSpec{Type: v1.ServiceTypeLoadBalancer, ClusterIP: "10.0.0.1"},
		Status:     v1.ServiceStatus{LoadBalancer: v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: "8.8.8.8"}}}},
	}}}
	userNamespaces := map[string]bool{"app-ns": true}

	serviceInfo := processServiceInfo(services, userNamespaces)
	assert.Len(t, serviceInfo, 1)
	assert.Equal(t, "8.8.8.8", serviceInfo[0].ExternalIP)
}

func TestProcessPodInfo(t *testing.T) {
	t.Run("WithMetrics", func(t *testing.T) {
		pods := &v1.PodList{Items: []v1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "app-ns"}}}}
		podMetrics := &metricsv1beta1.PodMetricsList{Items: []metricsv1beta1.PodMetrics{{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "app-ns"},
			Containers: []metricsv1beta1.ContainerMetrics{{Usage: v1.ResourceList{v1.ResourceCPU: *resource.NewMilliQuantity(100, resource.DecimalSI)}}},
		}}}
		userNamespaces := map[string]bool{"app-ns": true}

		podInfo := processPodInfo(pods, podMetrics, userNamespaces)
		assert.Len(t, podInfo, 1)
		assert.Equal(t, "100 m", podInfo[0].UsedCPU)
	})

	t.Run("WithoutMetrics", func(t *testing.T) {
		pods := &v1.PodList{Items: []v1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "app-ns"}}}}
		userNamespaces := map[string]bool{"app-ns": true}

		podInfo := processPodInfo(pods, nil, userNamespaces)
		assert.Len(t, podInfo, 1)
		assert.Empty(t, podInfo[0].UsedCPU, "UsedCPU should be empty when metrics are unavailable")
	})
}

func TestProcessIngressInfo(t *testing.T) {
	ingresses := &networkingv1.IngressList{Items: []networkingv1.Ingress{{
		ObjectMeta: metav1.ObjectMeta{Name: "my-ingress", Namespace: "app-ns"},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{Host: "test.com"}},
		},
	}}}
	userNamespaces := map[string]bool{"app-ns": true}

	ingressInfo := processIngressInfo(ingresses, userNamespaces)
	assert.Len(t, ingressInfo, 1)
	assert.Equal(t, "test.com", ingressInfo[0].Hosts)
}

func TestProcessNodeInfo(t *testing.T) {
	nodes := &v1.NodeList{Items: []v1.Node{{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""}},
		Status:     v1.NodeStatus{Allocatable: v1.ResourceList{v1.ResourceCPU: *resource.NewMilliQuantity(1000, resource.DecimalSI)}},
	}}}
	nodeMetrics := &metricsv1beta1.NodeMetricsList{Items: []metricsv1beta1.NodeMetrics{{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Usage:      v1.ResourceList{v1.ResourceCPU: *resource.NewMilliQuantity(250, resource.DecimalSI)},
	}}}

	nodeInfo := processNodeInfo(nodes, nil, nodeMetrics)
	assert.Len(t, nodeInfo, 1)
	assert.Equal(t, "Control-Plane", nodeInfo[0].Role)
	assert.InDelta(t, 25.0, nodeInfo[0].CPUUsagePercentage, 0.01)
}

// TestProcessFunctions_NilInput testa se as funções de processamento lidam com entradas nulas sem pânico.
func TestProcessFunctions_NilInput(t *testing.T) {
	assert.NotPanics(t, func() { processNodeInfo(nil, nil, nil) })
	assert.NotPanics(t, func() { processPodInfo(nil, nil, nil) })
	assert.NotPanics(t, func() { processServiceInfo(nil, nil) })
	assert.NotPanics(t, func() { processIngressInfo(nil, nil) })
	assert.NotPanics(t, func() { processPvcs(nil, nil) })
	assert.NotPanics(t, func() { processEvents(nil, nil) })
}
