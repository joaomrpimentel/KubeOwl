package api

import (
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// --- Testes para processClusterCapacity ---

func TestProcessClusterCapacity(t *testing.T) {
	// ... (teste existente) ...
	nodes := &v1.NodeList{
		Items: []v1.Node{
			{
				Status: v1.NodeStatus{
					Allocatable: v1.ResourceList{
						v1.ResourceCPU:    *resource.NewMilliQuantity(2000, resource.DecimalSI),       // 2 Cores
						v1.ResourceMemory: *resource.NewQuantity(4*1024*1024*1024, resource.BinarySI), // 4 Gi
					},
				},
			},
		},
	}
	nodeMetrics := &metricsv1beta1.NodeMetricsList{
		Items: []metricsv1beta1.NodeMetrics{
			{
				Usage: v1.ResourceList{
					v1.ResourceCPU:    *resource.NewMilliQuantity(500, resource.DecimalSI),        // 0.5 Cores
					v1.ResourceMemory: *resource.NewQuantity(1*1024*1024*1024, resource.BinarySI), // 1 Gi
				},
			},
		},
	}

	capacity := processClusterCapacity(nodes, nodeMetrics)

	if capacity.TotalCPU != 2000 {
		t.Errorf("Esperado TotalCPU 2000, obteve %d", capacity.TotalCPU)
	}
	if capacity.UsedCPU != 500 {
		t.Errorf("Esperado UsedCPU 500, obteve %d", capacity.UsedCPU)
	}
	if capacity.CPUUsagePercentage != 25.0 {
		t.Errorf("Esperado CPUUsagePercentage 25.0, obteve %f", capacity.CPUUsagePercentage)
	}
	if capacity.TotalMemory != 4*1024*1024*1024 {
		t.Errorf("Esperado TotalMemory 4294967296, obteve %d", capacity.TotalMemory)
	}
	if capacity.UsedMemory != 1*1024*1024*1024 {
		t.Errorf("Esperado UsedMemory 1073741824, obteve %d", capacity.UsedMemory)
	}
	if capacity.MemoryUsagePercentage != 25.0 {
		t.Errorf("Esperado MemoryUsagePercentage 25.0, obteve %f", capacity.MemoryUsagePercentage)
	}
}

func TestProcessClusterCapacity_NilInput(t *testing.T) {
	// NOVO: Testa o caso onde os dados de entrada são nulos para evitar panics.
	capacity := processClusterCapacity(nil, nil)
	if capacity.TotalCPU != 0 || capacity.UsedCPU != 0 || capacity.TotalMemory != 0 || capacity.UsedMemory != 0 {
		t.Errorf("Esperado capacidade zero para entrada nula, obteve %+v", capacity)
	}
}

func TestProcessClusterCapacity_NoNodes(t *testing.T) {
	// NOVO: Testa o caso com um cluster vazio (sem nós).
	capacity := processClusterCapacity(&v1.NodeList{}, &metricsv1beta1.NodeMetricsList{})
	if capacity.TotalCPU != 0 || capacity.UsedCPU != 0 || capacity.TotalMemory != 0 || capacity.UsedMemory != 0 {
		t.Errorf("Esperado capacidade zero para cluster sem nós, obteve %+v", capacity)
	}
}

// --- Testes para processNamespaces ---

func TestProcessNamespaces(t *testing.T) {
	// ... (teste existente) ...
	namespaces := &v1.NamespaceList{
		Items: []v1.Namespace{
			{ObjectMeta: metav1.ObjectMeta{Name: "app-prod"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "app-dev"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}}, // Namespace do sistema
			{ObjectMeta: metav1.ObjectMeta{Name: "cattle-system"}}, // Namespace de sistema por prefixo
		},
	}

	userCount, userNamespaces := processNamespaces(namespaces)

	if userCount != 2 {
		t.Errorf("Esperado 2 namespaces de usuário, obteve %d", userCount)
	}
	if !userNamespaces["app-prod"] || !userNamespaces["app-dev"] {
		t.Error("Namespaces de usuário esperados não encontrados no mapa")
	}
	if userNamespaces["kube-system"] || userNamespaces["cattle-system"] {
		t.Error("Namespace do sistema não deveria estar no mapa de namespaces de usuário")
	}
}

func TestProcessNamespaces_NilInput(t *testing.T) {
    // NOVO: Testa o caso com entrada nula.
    count, _ := processNamespaces(nil)
    if count != 0 {
        t.Errorf("Esperado 0 para contagem de namespaces com entrada nula, obteve %d", count)
    }
}

// --- Testes para processNodeInfo ---

func TestProcessNodeInfo(t *testing.T) {
	// ... (teste existente) ...
	nodes := &v1.NodeList{
		Items: []v1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Status: v1.NodeStatus{
					Allocatable: v1.ResourceList{
						v1.ResourceCPU:    *resource.NewMilliQuantity(4000, resource.DecimalSI),
						v1.ResourceMemory: *resource.NewQuantity(8*1024*1024*1024, resource.BinarySI),
					},
				},
			},
		},
	}
	pods := &v1.PodList{
		Items: []v1.Pod{
			{Spec: v1.PodSpec{NodeName: "node-01"}, Status: v1.PodStatus{Phase: v1.PodRunning}},
		},
	}
	nodeMetrics := &metricsv1beta1.NodeMetricsList{
		Items: []metricsv1beta1.NodeMetrics{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node-01"},
				Usage: v1.ResourceList{
					v1.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
					v1.ResourceMemory: *resource.NewQuantity(2*1024*1024*1024, resource.BinarySI),
				},
			},
		},
	}

	nodeInfo := processNodeInfo(nodes, pods, nodeMetrics)
	if len(nodeInfo) != 1 {
		t.Fatalf("Esperado 1 nó processado, obteve %d", len(nodeInfo))
	}
	if nodeInfo[0].Name != "node-01" {
		t.Errorf("Esperado nome do nó 'node-01', obteve '%s'", nodeInfo[0].Name)
	}
}

func TestProcessNodeInfo_NilInput(t *testing.T) {
    // NOVO: Testa o caso com entrada nula.
    info := processNodeInfo(nil, nil, nil)
    if len(info) != 0 {
        t.Errorf("Esperado 0 nós para entrada nula, obteve %d", len(info))
    }
}


// --- Testes para processPodInfo ---

func TestProcessPodInfo(t *testing.T) {
	// ... (teste existente) ...
	pods := &v1.PodList{
		Items: []v1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "my-app-pod", Namespace: "production"},
				Spec:       v1.PodSpec{NodeName: "node-01"},
			},
		},
	}
	podMetrics := &metricsv1beta1.PodMetricsList{
		Items: []metricsv1beta1.PodMetrics{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "my-app-pod", Namespace: "production"},
				Containers: []metricsv1beta1.ContainerMetrics{
					{
						Name: "main-container",
						Usage: v1.ResourceList{
							v1.ResourceCPU:    *resource.NewMilliQuantity(150, resource.DecimalSI),
							v1.ResourceMemory: *resource.NewQuantity(100*1024*1024, resource.BinarySI),
						},
					},
				},
			},
		},
	}
	userNamespaces := map[string]bool{"production": true}

	podInfo := processPodInfo(pods, podMetrics, userNamespaces)
	if len(podInfo) != 1 {
		t.Fatalf("Esperado 1 pod processado, obteve %d", len(podInfo))
	}
}

func TestProcessPodInfo_NoMatch(t *testing.T) {
    // NOVO: Testa o caso onde as métricas de um pod não correspondem a um pod real.
    pods := &v1.PodList{ Items: []v1.Pod{} }
    podMetrics := &metricsv1beta1.PodMetricsList{
		Items: []metricsv1beta1.PodMetrics{
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-fantasma", Namespace: "production"}},
        },
    }
    userNamespaces := map[string]bool{"production": true}
    info := processPodInfo(pods, podMetrics, userNamespaces)
    if len(info) != 0 {
        t.Errorf("Esperado 0 pods quando as métricas não correspondem, obteve %d", len(info))
    }
}

// --- Testes para processEvents ---

func TestProcessEvents(t *testing.T) {
	// ... (teste existente) ...
	now := time.Now()
	events := &v1.EventList{
		Items: []v1.Event{
			{
				ObjectMeta:    metav1.ObjectMeta{Namespace: "app-prod"},
				LastTimestamp: metav1.NewTime(now.Add(-1 * time.Hour)),
				Reason:        "ScalingReplicaSet", Message: "Scaled up replica set", Type: "Normal",
			},
			{
				ObjectMeta:    metav1.ObjectMeta{Namespace: "app-dev"},
				LastTimestamp: metav1.NewTime(now),
				Reason:        "FailedScheduling", Message: "pod has unbound immediate PersistentVolumeClaims", Type: "Warning",
			},
		},
	}

	userNamespaces := map[string]bool{"app-prod": true, "app-dev": true}
	processedEvents := processEvents(events, userNamespaces)

	if len(processedEvents) != 2 {
		t.Fatalf("Esperado 2 eventos processados, obteve %d", len(processedEvents))
	}
}

// --- Testes para processPvcs ---

func TestProcessPvcs(t *testing.T) {
	// ... (teste existente) ...
	storageClassName := "fast-storage"
	pvcs := &v1.PersistentVolumeClaimList{
		Items: []v1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "data-pvc", Namespace: "database"},
				Spec: v1.PersistentVolumeClaimSpec{
					StorageClassName: &storageClassName,
					Resources: v1.VolumeResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceStorage: *resource.NewQuantity(10*1024*1024*1024, resource.BinarySI),
						},
					},
				},
				Status: v1.PersistentVolumeClaimStatus{Phase: v1.ClaimBound},
			},
		},
	}
	userNamespaces := map[string]bool{"database": true}
	pvcInfo := processPvcs(pvcs, userNamespaces)

	if len(pvcInfo) != 1 {
		t.Fatalf("Esperado 1 PVC processado, obteve %d", len(pvcInfo))
	}
}
