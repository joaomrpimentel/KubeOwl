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

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
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

// TestOverviewHandler testa o endpoint de visão geral.
func TestOverviewHandler(t *testing.T) {
	// 1. Setup: Cria dados mocados.
	mockK8sObjects := []runtime.Object{
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "deployment-1", Namespace: "default"}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
		&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
	}
	setupFakeClients(mockK8sObjects, nil)

	req, _ := http.NewRequest("GET", "/api/overview", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(OverviewHandler)

	// 2. Execução
	handler.ServeHTTP(rr, req)

	// 3. Verificação
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("OverviewHandler retornou status code incorreto: obteve %v, esperava %v", status, http.StatusOK)
	}

	var response OverviewResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Não foi possível decodificar a resposta JSON: %v", err)
	}

	if response.DeploymentCount != 1 {
		t.Errorf("DeploymentCount incorreto: esperado 1, obteve %d", response.DeploymentCount)
	}
	if response.NamespaceCount != 1 { // Apenas 'app-ns', já que 'default' é filtrado.
		t.Errorf("NamespaceCount incorreto: esperado 1, obteve %d", response.NamespaceCount)
	}
	if response.NodeCount != 1 {
		t.Errorf("NodeCount incorreto: esperado 1, obteve %d", response.NodeCount)
	}
}

// TestNodesHandler testa o endpoint que lista os nós.
func TestNodesHandler(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
	}
	setupFakeClients(mockK8sObjects, nil)

	req, _ := http.NewRequest("GET", "/api/nodes", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(NodesHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("NodesHandler retornou status code incorreto: obteve %v, esperava %v", status, http.StatusOK)
	}

	var response []NodeInfo
	json.NewDecoder(rr.Body).Decode(&response)
	if len(response) != 1 {
		t.Errorf("Esperado 1 nó, obteve %d", len(response))
	}
	if response[0].Name != "node-1" {
		t.Errorf("Nome do nó incorreto: esperado 'node-1', obteve '%s'", response[0].Name)
	}
}

// TestPodsHandler testa o endpoint que lista os pods.
func TestPodsHandler(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "app-ns"}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
	}
	mockMetricsObjects := []runtime.Object{
		&metricsv1beta1.PodMetrics{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "app-ns"}},
	}
	setupFakeClients(mockK8sObjects, mockMetricsObjects)

	req, _ := http.NewRequest("GET", "/api/pods", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(PodsHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("PodsHandler retornou status code incorreto: obteve %v, esperava %v", status, http.StatusOK)
	}

	var response []PodInfo
	json.NewDecoder(rr.Body).Decode(&response)
	if len(response) != 1 {
		t.Errorf("Esperado 1 pod, obteve %d", len(response))
	}
}

// TestServicesHandler testa o endpoint que lista os serviços.
func TestServicesHandler(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "service-1", Namespace: "app-ns"}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
	}
	setupFakeClients(mockK8sObjects, nil)

	req, _ := http.NewRequest("GET", "/api/services", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ServicesHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("ServicesHandler retornou status code incorreto: obteve %v, esperava %v", status, http.StatusOK)
	}

	var response []ServiceInfo
	json.NewDecoder(rr.Body).Decode(&response)
	if len(response) != 1 {
		t.Errorf("Esperado 1 serviço, obteve %d", len(response))
	}
}

// TestIngressesHandler testa o endpoint que lista os ingresses.
func TestIngressesHandler(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "ingress-1", Namespace: "app-ns"}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
	}
	setupFakeClients(mockK8sObjects, nil)

	req, _ := http.NewRequest("GET", "/api/ingresses", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(IngressesHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("IngressesHandler retornou status code incorreto: obteve %v, esperava %v", status, http.StatusOK)
	}

	var response []IngressInfo
	json.NewDecoder(rr.Body).Decode(&response)
	if len(response) != 1 {
		t.Errorf("Esperado 1 ingress, obteve %d", len(response))
	}
}

// TestPvcsHandler testa o endpoint que lista os PVCs.
func TestPvcsHandler(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&v1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "pvc-1", Namespace: "app-ns"}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
	}
	setupFakeClients(mockK8sObjects, nil)

	req, _ := http.NewRequest("GET", "/api/pvcs", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(PvcsHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("PvcsHandler retornou status code incorreto: obteve %v, esperava %v", status, http.StatusOK)
	}

	var response []PvcInfo
	json.NewDecoder(rr.Body).Decode(&response)
	if len(response) != 1 {
		t.Errorf("Esperado 1 PVC, obteve %d", len(response))
	}
}

// TestEventsHandler testa o endpoint que lista os eventos.
func TestEventsHandler(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&v1.Event{ObjectMeta: metav1.ObjectMeta{Name: "event-1", Namespace: "app-ns"}, LastTimestamp: metav1.NewTime(time.Now())},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
	}
	setupFakeClients(mockK8sObjects, nil)

	req, _ := http.NewRequest("GET", "/api/events", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(EventsHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("EventsHandler retornou status code incorreto: obteve %v, esperava %v", status, http.StatusOK)
	}

	var response []EventInfo
	json.NewDecoder(rr.Body).Decode(&response)
	if len(response) != 1 {
		t.Errorf("Esperado 1 evento, obteve %d", len(response))
	}
}

// --- Testes de Casos de Borda e Erros ---

// TestOverviewHandler_PartialFailure testa o caso onde uma das chamadas internas do handler falha.
func TestOverviewHandler_PartialFailure(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "deployment-1"}},
	}
	fakecs := fake.NewSimpleClientset(mockK8sObjects...)
	// Simula erro apenas na chamada para listar namespaces.
	fakecs.PrependReactor("list", "namespaces", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("erro forçado ao listar namespaces")
	})
	k8s.Clientset = fakecs
	k8s.MetricsClientset = metricsvake.NewSimpleClientset()

	req, _ := http.NewRequest("GET", "/api/overview", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(OverviewHandler)

	handler.ServeHTTP(rr, req)

	// A falha em uma dependência (namespaces) deve resultar em um erro do servidor.
	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Handler retornou status incorreto para falha parcial: obteve %v, esperava %v", status, http.StatusInternalServerError)
	}
}


// TestNodesHandler_Empty testa o caso onde não há nós no cluster.
func TestNodesHandler_Empty(t *testing.T) {
	setupFakeClients(nil, nil) // Nenhum objeto k8s

	req, _ := http.NewRequest("GET", "/api/nodes", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(NodesHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler retornou status incorreto para estado vazio: obteve %v, esperava %v", status, http.StatusOK)
	}

	var response []NodeInfo
	json.NewDecoder(rr.Body).Decode(&response)
	if len(response) != 0 {
		t.Errorf("Esperada uma lista vazia, mas obteve %d nós", len(response))
	}
}

// TestNodesHandler_MetricsError testa o caso onde a API de métricas falha.
func TestNodesHandler_MetricsError(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
	}
	// Configura o cliente de métricas para retornar um erro.
	fakeMetricsClient := metricsvake.NewSimpleClientset()
	fakeMetricsClient.PrependReactor("list", "nodemetricses", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("erro forçado da API de métricas")
	})
	k8s.Clientset = fake.NewSimpleClientset(mockK8sObjects...)
	k8s.MetricsClientset = fakeMetricsClient

	req, _ := http.NewRequest("GET", "/api/nodes", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(NodesHandler)

	handler.ServeHTTP(rr, req)

	// O handler deve ter sucesso, apenas sem os dados de métricas.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler retornou status incorreto para falha de métricas: obteve %v, esperava %v", status, http.StatusOK)
	}

	var response []NodeInfo
	json.NewDecoder(rr.Body).Decode(&response)
	if len(response) == 0 {
		t.Fatal("Nenhum nó retornado apesar da falha de métricas")
	}
	// Verifica se os valores de métricas estão zerados.
	if response[0].CPUUsagePercentage != 0 || response[0].UsedCPU != "0.00" {
		t.Errorf("Dados de métricas de CPU não deveriam estar presentes, mas estavam: %+v", response[0])
	}
}

// TestPodsHandler_MetricsError testa a degradação graciosa do handler de pods.
func TestPodsHandler_MetricsError(t *testing.T) {
	mockK8sObjects := []runtime.Object{
		&v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "app-ns"}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
	}
	// Configura o cliente de métricas para retornar um erro.
	fakeMetricsClient := metricsvake.NewSimpleClientset()
	fakeMetricsClient.PrependReactor("list", "podmetricses", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("erro forçado da API de métricas")
	})
	k8s.Clientset = fake.NewSimpleClientset(mockK8sObjects...)
	k8s.MetricsClientset = fakeMetricsClient

	req, _ := http.NewRequest("GET", "/api/pods", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(PodsHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler retornou status incorreto para falha de métricas: obteve %v, esperava %v", status, http.StatusOK)
	}
	var response []PodInfo
	json.NewDecoder(rr.Body).Decode(&response)
	if len(response) == 0 {
		t.Fatal("Nenhum pod retornado apesar da falha de métricas")
	}
	if response[0].UsedCPU != "" || response[0].UsedMemory != "" {
		t.Errorf("Dados de métricas de Pod não deveriam estar presentes, mas estavam: %+v", response[0])
	}
}


// TestPodsHandler_NamespaceError testa o caso onde a chamada para listar namespaces falha.
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

	req, _ := http.NewRequest("GET", "/api/pods", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(PodsHandler)

	handler.ServeHTTP(rr, req)

	// A falha em uma dependência (namespaces) deve resultar em um erro do servidor.
	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Handler retornou status incorreto para falha de namespace: obteve %v, esperava %v", status, http.StatusInternalServerError)
	}
}


// TestHandler_ClientError testa o comportamento de um handler quando o cliente K8s retorna um erro.
func TestHandler_ClientError(t *testing.T) {
	// 1. Setup: Configura o cliente falso para retornar um erro na chamada principal.
	fakecs := fake.NewSimpleClientset()
	fakecs.PrependReactor("list", "nodes", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("erro forçado da API do kubernetes")
	})
	k8s.Clientset = fakecs
	k8s.MetricsClientset = metricsvake.NewSimpleClientset()

	// Escolhe um handler para testar o comportamento de erro (ex: NodesHandler).
	handler := http.HandlerFunc(NodesHandler)
	req, _ := http.NewRequest("GET", "/api/nodes", nil)
	rr := httptest.NewRecorder()

	// 2. Execução
	handler.ServeHTTP(rr, req)

	// 3. Verificação
	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Handler retornou status code incorreto para erro de cliente: obteve %v, esperava %v", status, http.StatusInternalServerError)
	}
}
