package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"kubeowl/internal/k8s"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	metricsvake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

// TestMain é executado antes de todos os outros testes neste pacote.
// Usamos isso para silenciar a saída do logger durante a execução dos testes.
func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// setupFakeClients configura os clientes falsos do Kubernetes com objetos mocados.
func setupFakeClients(k8sObjects []runtime.Object, metricsObjects []runtime.Object) {
	k8s.Clientset = fake.NewSimpleClientset(k8sObjects...)
	k8s.MetricsClientset = metricsvake.NewSimpleClientset(metricsObjects...)
}

// createTestRequest é uma função utilitária para criar um request e recorder.
func createTestRequest(t *testing.T, handlerFunc http.HandlerFunc, method, path string) *httptest.ResponseRecorder {
	req, err := http.NewRequest(method, path, nil)
	assert.NoError(t, err)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handlerFunc)
	handler.ServeHTTP(rr, req)
	return rr
}

// --- Testes para cada Handler ---

func TestOverviewHandler(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "deployment-1", Namespace: "default"}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
		&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
	}
	setupFakeClients(mockK8sObjects, nil)

	rr := createTestRequest(t, OverviewHandler, "GET", "/api/overview")

	assert.Equal(t, http.StatusOK, rr.Code)
	var response OverviewResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, 1, response.DeploymentCount)
	assert.Equal(t, 1, response.NamespaceCount)
	assert.Equal(t, 1, response.NodeCount)
}

func TestNodesHandler(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Status: v1.NodeStatus{
			Allocatable: v1.ResourceList{
				v1.ResourceCPU:    *resource.NewMilliQuantity(2000, resource.DecimalSI),
				v1.ResourceMemory: *resource.NewQuantity(4*1024*1024*1024, resource.BinarySI),
			},
		}},
	}
	setupFakeClients(mockK8sObjects, nil)

	rr := createTestRequest(t, NodesHandler, "GET", "/api/nodes")
	assert.Equal(t, http.StatusOK, rr.Code)

	var response []NodeInfo
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Len(t, response, 1)
	assert.Equal(t, "node-1", response[0].Name)
}

func TestPodsHandler(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "app-ns"}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
	}
	setupFakeClients(mockK8sObjects, nil)

	rr := createTestRequest(t, PodsHandler, "GET", "/api/pods")
	assert.Equal(t, http.StatusOK, rr.Code)

	var response []PodInfo
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Len(t, response, 1)
	assert.Equal(t, "pod-1", response[0].Name)
}

func TestServicesHandler(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "service-1", Namespace: "app-ns"}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
	}
	setupFakeClients(mockK8sObjects, nil)

	rr := createTestRequest(t, ServicesHandler, "GET", "/api/services")
	assert.Equal(t, http.StatusOK, rr.Code)

	var response []ServiceInfo
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Len(t, response, 1)
	assert.Equal(t, "service-1", response[0].Name)
}

func TestIngressesHandler(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "ingress-1", Namespace: "app-ns"}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
	}
	setupFakeClients(mockK8sObjects, nil)

	rr := createTestRequest(t, IngressesHandler, "GET", "/api/ingresses")
	assert.Equal(t, http.StatusOK, rr.Code)

	var response []IngressInfo
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Len(t, response, 1)
	assert.Equal(t, "ingress-1", response[0].Name)
}

func TestPvcsHandler(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "pvc-1", Namespace: "app-ns"},
			Spec: v1.PersistentVolumeClaimSpec{
				Resources: v1.VolumeResourceRequirements{ // Corrigido para o tipo esperado
					Requests: v1.ResourceList{v1.ResourceStorage: resource.MustParse("1Gi")},
				},
			},
		},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
	}
	setupFakeClients(mockK8sObjects, nil)

	rr := createTestRequest(t, PvcsHandler, "GET", "/api/pvcs")
	assert.Equal(t, http.StatusOK, rr.Code)

	var response []PvcInfo
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Len(t, response, 1)
	assert.Equal(t, "pvc-1", response[0].Name)
}

func TestEventsHandler(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&v1.Event{
			ObjectMeta:     metav1.ObjectMeta{Name: "event-1", Namespace: "app-ns"},
			LastTimestamp:  metav1.NewTime(time.Now()),
			Reason:         "Scheduled",
			InvolvedObject: v1.ObjectReference{Kind: "Pod", Name: "my-pod"},
		},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
	}
	setupFakeClients(mockK8sObjects, nil)

	rr := createTestRequest(t, EventsHandler, "GET", "/api/events")
	assert.Equal(t, http.StatusOK, rr.Code)

	var response []EventInfo
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Len(t, response, 1)
	assert.Equal(t, "Scheduled", response[0].Reason)
}

// --- Testes de Casos de Borda e Erros ---

func TestHandler_ClientError(t *testing.T) {
	fakecs := fake.NewSimpleClientset()
	fakecs.PrependReactor("list", "nodes", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("erro forçado da API do kubernetes")
	})
	k8s.Clientset = fakecs

	rr := createTestRequest(t, NodesHandler, "GET", "/api/nodes")
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandler_EmptyList(t *testing.T) {
	setupFakeClients(nil, nil) // Nenhum objeto k8s

	rr := createTestRequest(t, PodsHandler, "GET", "/api/pods")
	assert.Equal(t, http.StatusOK, rr.Code)

	var response []PodInfo
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Empty(t, response, "A resposta deveria ser uma lista vazia")
}

func TestPodsHandler_NamespaceError(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "app-ns"}},
	}
	fakecs := fake.NewSimpleClientset(mockK8sObjects...)
	// Simula erro apenas na chamada para listar namespaces.
	fakecs.PrependReactor("list", "namespaces", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("erro forçado ao listar namespaces")
	})
	k8s.Clientset = fakecs
	k8s.MetricsClientset = metricsvake.NewSimpleClientset()

	rr := createTestRequest(t, PodsHandler, "GET", "/api/pods")
	assert.Equal(t, http.StatusInternalServerError, rr.Code, "Deveria falhar se não conseguir listar namespaces")
}

func TestNodesHandler_MetricsError(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Status: v1.NodeStatus{
			Allocatable: v1.ResourceList{v1.ResourceCPU: *resource.NewMilliQuantity(1000, resource.DecimalSI)},
		}},
	}
	fakeMetricsClient := metricsvake.NewSimpleClientset()
	fakeMetricsClient.PrependReactor("list", "nodemetricses", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("erro forçado da API de métricas")
	})
	k8s.Clientset = fake.NewSimpleClientset(mockK8sObjects...)
	k8s.MetricsClientset = fakeMetricsClient

	rr := createTestRequest(t, NodesHandler, "GET", "/api/nodes")
	assert.Equal(t, http.StatusOK, rr.Code, "Handler deveria funcionar mesmo com erro na API de métricas")

	var response []NodeInfo
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Len(t, response, 1)
	assert.Equal(t, 0.0, response[0].CPUUsagePercentage, "Uso de CPU deveria ser 0 quando as métricas falham")
}

func TestPodsHandler_MetricsError(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "app-ns"}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
	}
	fakeMetricsClient := metricsvake.NewSimpleClientset()
	fakeMetricsClient.PrependReactor("list", "podmetricses", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("erro forçado da API de métricas")
	})
	k8s.Clientset = fake.NewSimpleClientset(mockK8sObjects...)
	k8s.MetricsClientset = fakeMetricsClient

	rr := createTestRequest(t, PodsHandler, "GET", "/api/pods")
	assert.Equal(t, http.StatusOK, rr.Code)

	var response []PodInfo
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Len(t, response, 1)
	assert.Equal(t, "", response[0].UsedCPU, "UsedCPU deveria ser vazio quando métricas falham")
	assert.Equal(t, "", response[0].UsedMemory, "UsedMemory deveria ser vazio quando métricas falham")
}
