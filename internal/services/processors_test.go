package services

import (
	"io"
	"log"
	"os"
	"testing"
	"time"

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

// TestGetPodStatus valida a lógica de determinação de status e contagem de restarts.
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
			name: "Status Completed",
			pod: v1.Pod{Status: v1.PodStatus{Phase: v1.PodSucceeded, ContainerStatuses: []v1.ContainerStatus{
				{State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{Reason: "Completed"}}, RestartCount: 0},
			}}},
			expectedStatus:   "Completed",
			expectedRestarts: 0,
		},
		{
			name:             "Status Pending com base na Phase",
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

// TestProcessNodeInfo valida o processamento de informações dos nós.
func TestProcessNodeInfo(t *testing.T) {
	nodes := &v1.NodeList{Items: []v1.Node{{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: map[string]string{"node-role.kubernetes.io/control-plane": "true"}},
		Status:     v1.NodeStatus{Allocatable: v1.ResourceList{v1.ResourceCPU: *resource.NewMilliQuantity(2000, resource.DecimalSI)}},
	}}}
	nodeMetrics := &metricsv1beta1.NodeMetricsList{Items: []metricsv1beta1.NodeMetrics{{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Usage:      v1.ResourceList{v1.ResourceCPU: *resource.NewMilliQuantity(500, resource.DecimalSI)},
	}}}
	pods := &v1.PodList{Items: []v1.Pod{{Spec: v1.PodSpec{NodeName: "node-1"}}}}

	nodeInfo := processNodeInfo(nodes, pods, nodeMetrics)
	assert.Len(t, nodeInfo, 1)
	assert.Equal(t, "Control-Plane", nodeInfo[0].Role)
	assert.Equal(t, 1, nodeInfo[0].PodCount)
	assert.InDelta(t, 25.0, nodeInfo[0].CPUUsagePercentage, 0.01)
}

// TestProcessNamespacesForSelector valida a filtragem e ordenação dos namespaces para o seletor.
func TestProcessNamespacesForSelector(t *testing.T) {
	namespaces := &v1.NamespaceList{
		Items: []v1.Namespace{
			{ObjectMeta: metav1.ObjectMeta{Name: "zeta-app"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "alpha-app"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}}, // Namespace de sistema
		},
	}
	nsInfo := processNamespacesForSelector(namespaces)
	assert.Len(t, nsInfo, 2, "Deveria retornar apenas namespaces do usuário")
	assert.Equal(t, "alpha-app", nsInfo[0].Name, "A lista de namespaces deveria estar ordenada alfabeticamente")
	assert.Equal(t, "zeta-app", nsInfo[1].Name, "A lista de namespaces deveria estar ordenada alfabeticamente")
}

// TestFilteringByUserNamespaces valida se os recursos de sistema são corretamente filtrados.
func TestFilteringByUserNamespaces(t *testing.T) {
	userNamespaces := map[string]bool{"app-ns": true}

	t.Run("processServiceInfo filters system namespaces", func(t *testing.T) {
		services := &v1.ServiceList{Items: []v1.Service{
			{ObjectMeta: metav1.ObjectMeta{Name: "user-service", Namespace: "app-ns"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "system-service", Namespace: "kube-system"}},
		}}
		serviceInfo := processServiceInfo(services, userNamespaces)
		assert.Len(t, serviceInfo, 1)
		assert.Equal(t, "user-service", serviceInfo[0].Name)
	})

	t.Run("processPodInfo filters system namespaces", func(t *testing.T) {
		pods := &v1.PodList{Items: []v1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "user-pod", Namespace: "app-ns", UID: "uid-1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "system-pod", Namespace: "kube-system", UID: "uid-2"}},
		}}
		podInfo := processPodInfo(pods, nil, userNamespaces)
		assert.Len(t, podInfo, 1)
		assert.Equal(t, "user-pod", podInfo[0].Name)
	})

	t.Run("processIngressInfo filters system namespaces", func(t *testing.T) {
		ingresses := &networkingv1.IngressList{Items: []networkingv1.Ingress{
			{ObjectMeta: metav1.ObjectMeta{Name: "user-ingress", Namespace: "app-ns"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "system-ingress", Namespace: "kube-system"}},
		}}
		ingressInfo := processIngressInfo(ingresses, userNamespaces)
		assert.Len(t, ingressInfo, 1)
		assert.Equal(t, "user-ingress", ingressInfo[0].Name)
	})

	t.Run("processPvcs filters system namespaces", func(t *testing.T) {
		pvcs := &v1.PersistentVolumeClaimList{Items: []v1.PersistentVolumeClaim{
			{ObjectMeta: metav1.ObjectMeta{Name: "user-pvc", Namespace: "app-ns"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "system-pvc", Namespace: "kube-system"}},
		}}
		pvcInfo := processPvcs(pvcs, userNamespaces)
		assert.Len(t, pvcInfo, 1)
		assert.Equal(t, "user-pvc", pvcInfo[0].Name)
	})

	t.Run("processEvents filters system namespaces", func(t *testing.T) {
		now := metav1.NewTime(time.Now())
		events := &v1.EventList{Items: []v1.Event{
			{
				ObjectMeta:     metav1.ObjectMeta{Name: "user-event", Namespace: "app-ns"},
				LastTimestamp:  metav1.Time{Time: now.Add(1 * time.Second)},
				InvolvedObject: v1.ObjectReference{Kind: "Pod", Name: "pod-1", Namespace: "app-ns"},
			},
			{
				ObjectMeta:     metav1.ObjectMeta{Name: "system-event", Namespace: "kube-system"},
				LastTimestamp:  now,
				InvolvedObject: v1.ObjectReference{Kind: "Pod", Name: "pod-2", Namespace: "kube-system"},
			},
			{
				ObjectMeta:     metav1.ObjectMeta{Name: "global-event", Namespace: ""}, // Evento de cluster
				LastTimestamp:  now,
				InvolvedObject: v1.ObjectReference{Kind: "Node", Name: "node-1"},
			},
		}}

		eventInfo := processEvents(events, userNamespaces)
		assert.Len(t, eventInfo, 2, "Deveria processar eventos de namespaces de usuário e de cluster")
		assert.Equal(t, "Pod/pod-1", eventInfo[0].Object, "O evento mais recente deve vir primeiro")
		assert.Equal(t, "Node/node-1", eventInfo[1].Object)
	})
}

// TestProcessFunctions_NilInput garante que as funções não causem pânico com entradas nulas.
func TestProcessFunctions_NilInput(t *testing.T) {
	assert.NotPanics(t, func() { processNodeInfo(nil, nil, nil) }, "processNodeInfo não deve causar pânico com nil")
	assert.NotPanics(t, func() { processPodInfo(nil, nil, nil) }, "processPodInfo não deve causar pânico com nil")
	assert.NotPanics(t, func() { processServiceInfo(nil, nil) }, "processServiceInfo não deve causar pânico com nil")
	assert.NotPanics(t, func() { processIngressInfo(nil, nil) }, "processIngressInfo não deve causar pânico com nil")
	assert.NotPanics(t, func() { processPvcs(nil, nil) }, "processPvcs não deve causar pânico com nil")
	assert.NotPanics(t, func() { processEvents(nil, nil) }, "processEvents não deve causar pânico com nil")
	assert.NotPanics(t, func() { processNamespacesForSelector(nil) }, "processNamespacesForSelector não deve causar pânico com nil")
}
