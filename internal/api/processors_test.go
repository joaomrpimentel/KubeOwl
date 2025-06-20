package api

import (
	"testing"

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
			expectedStatus: "Running",
			expectedRestarts: 2,
		},
		{
			name: "Status CrashLoopBackOff",
			pod: v1.Pod{Status: v1.PodStatus{ContainerStatuses: []v1.ContainerStatus{
				{State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}, RestartCount: 5},
			}}},
			expectedStatus: "CrashLoopBackOff",
			expectedRestarts: 5,
		},
		{
			name: "Status Terminated",
			pod: v1.Pod{Status: v1.PodStatus{Phase: v1.PodSucceeded, ContainerStatuses: []v1.ContainerStatus{
				{State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{Reason: "Completed"}}, RestartCount: 0},
			}}},
			expectedStatus: "Completed",
			expectedRestarts: 0,
		},
		{
			name: "Status Pending (fallback to phase)",
			pod: v1.Pod{Status: v1.PodStatus{Phase: v1.PodPending, ContainerStatuses: []v1.ContainerStatus{}}},
			expectedStatus: "Pending",
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

// Testa o caso de divisão por zero se não houver CPUs alocáveis.
func TestProcessClusterCapacity_NoCPU(t *testing.T) {
	nodes := &v1.NodeList{Items: []v1.Node{{Status: v1.NodeStatus{Allocatable: v1.ResourceList{
		v1.ResourceCPU: *resource.NewMilliQuantity(0, resource.DecimalSI),
	}}}}}
	capacity := processClusterCapacity(nodes, nil)
	assert.Equal(t, float64(0), capacity.CPUUsagePercentage)
}


// --- Testes para processNodeInfo ---
func TestProcessNodeInfo_Role(t *testing.T) {
	testCases := []struct {
		name         string
		labels       map[string]string
		expectedRole string
	}{
		{"Role Control-Plane", map[string]string{"node-role.kubernetes.io/control-plane": ""}, "Control-Plane"},
		{"Role Master", map[string]string{"node-role.kubernetes.io/master": ""}, "Control-Plane"},
		{"Role Worker (no role label)", map[string]string{}, "Worker"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nodes := &v1.NodeList{
				Items: []v1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: tc.labels}}},
			}
			nodeInfo := processNodeInfo(nodes, nil, nil)
			assert.Equal(t, tc.expectedRole, nodeInfo[0].Role)
		})
	}
}

// --- Testes para processServiceInfo ---
func TestProcessServiceInfo(t *testing.T) {
	services := &v1.ServiceList{
		Items: []v1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "service-lb", Namespace: "app-ns"},
				Spec:       v1.ServiceSpec{Type: v1.ServiceTypeLoadBalancer, ClusterIP: "10.0.0.1"},
				Status:     v1.ServiceStatus{LoadBalancer: v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: "8.8.8.8"}}}},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "service-cip", Namespace: "app-ns"},
				Spec:       v1.ServiceSpec{Type: v1.ServiceTypeClusterIP, ClusterIP: "10.0.0.2"},
			},
		},
	}
	userNamespaces := map[string]bool{"app-ns": true}

	serviceInfo := processServiceInfo(services, userNamespaces)

	assert.Len(t, serviceInfo, 2)
	assert.Equal(t, "", serviceInfo[0].ExternalIP)
	assert.Equal(t, "service-cip", serviceInfo[0].Name)
	assert.Equal(t, "8.8.8.8", serviceInfo[1].ExternalIP)
	assert.Equal(t, "service-lb", serviceInfo[1].Name)
}

// --- Testes para Casos de Borda e Entradas Nulas ---

// Testa todas as funções de processamento com entrada nula para garantir que não quebrem.
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

// Testa o comportamento do getNodeUsage quando as métricas de um nó específico não são encontradas.
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

// Testa o comportamento do processPodInfo quando um pod não tem métricas.
func TestProcessPodInfo_NoMetrics(t *testing.T) {
	pods := &v1.PodList{
		Items: []v1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "app-ns"}},
		},
	}
	userNamespaces := map[string]bool{"app-ns": true}
	// Passa uma lista de métricas vazia.
	podInfo := processPodInfo(pods, &metricsv1beta1.PodMetricsList{}, userNamespaces)

	assert.Len(t, podInfo, 1)
	assert.Equal(t, "", podInfo[0].UsedCPU, "UsedCPU deve ser vazio")
	assert.Equal(t, "", podInfo[0].UsedMemory, "UsedMemory deve ser vazio")
}

// Testa o comportamento do processIngressInfo quando um Ingress não tem regras.
func TestProcessIngressInfo_NoRules(t *testing.T) {
	ingresses := &networkingv1.IngressList{
		Items: []networkingv1.Ingress{
			{ObjectMeta: metav1.ObjectMeta{Name: "ingress-1", Namespace: "app-ns"}},
		},
	}
	userNamespaces := map[string]bool{"app-ns": true}
	ingressInfo := processIngressInfo(ingresses, userNamespaces)

	assert.Len(t, ingressInfo, 1)
	assert.Equal(t, "", ingressInfo[0].Hosts)
	assert.Equal(t, "", ingressInfo[0].Service)
}

// Testa o filtro de namespace de sistema em processEvents.
func TestProcessEvents_NamespaceFilter(t *testing.T) {
	events := &v1.EventList{
		Items: []v1.Event{
			{ // Este evento deve ser filtrado por estar em um namespace de sistema.
				ObjectMeta:     metav1.ObjectMeta{Name: "event-1", Namespace: "kube-system"},
				InvolvedObject: v1.ObjectReference{Kind: "Pod", Name: "kube-proxy-abc"},
			},
			{ // Este evento de cluster (sem namespace) deve ser incluído.
				ObjectMeta:     metav1.ObjectMeta{Name: "event-2", Namespace: ""},
				InvolvedObject: v1.ObjectReference{Kind: "Node", Name: "node-1"},
			},
		},
	}
	userNamespaces := map[string]bool{} // Nenhum namespace de usuário

	eventInfo := processEvents(events, userNamespaces)

	assert.Len(t, eventInfo, 1)
	// A asserção correta verifica o objeto formatado "Kind/Name".
	assert.Equal(t, "Node/node-1", eventInfo[0].Object)
}
