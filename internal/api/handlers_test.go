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

	"kubeowl/internal/k8s"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
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

// setupMockHandler cria um handler HTTP com um cliente Kubernetes e de métricas falso para testes.
func setupMockHandler(t *testing.T, k8sObjects []runtime.Object, metricsObjects []runtime.Object) http.HandlerFunc {
	// Guarda a função original para restaurá-la após o teste.
	originalInClusterConfig := k8s.InClusterConfigFunc
	t.Cleanup(func() {
		k8s.InClusterConfigFunc = originalInClusterConfig
	})

	// Substitui os clientes globais pelos clientes falsos.
	k8s.Clientset = fake.NewSimpleClientset(k8sObjects...)
	k8s.MetricsClientset = metricsvake.NewSimpleClientset(metricsObjects...)

	// Monkey patch a função InClusterConfig para sempre retornar um erro, forçando o uso do kubeconfig falso.
	k8s.InClusterConfigFunc = func() (*rest.Config, error) {
		return nil, fmt.Errorf("não está em um cluster")
	}

	return http.HandlerFunc(RealtimeHandler)
}

// TestRealtimeHandler_Success testa o caminho de sucesso do manipulador da API.
func TestRealtimeHandler_Success(t *testing.T) {
	// 1. Setup: Cria dados mocados.
	mockNodes := []runtime.Object{
		&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
	}
	mockMetrics := []runtime.Object{
		&metricsv1beta1.NodeMetrics{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
	}

	// Cria o handler com os dados mocados.
	handler := setupMockHandler(t, mockNodes, mockMetrics)
	req, _ := http.NewRequest("GET", "/api/realtime", nil)
	rr := httptest.NewRecorder()

	// 2. Execução: Chama o handler.
	handler.ServeHTTP(rr, req)

	// 3. Verificação
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler retornou status code incorreto: obteve %v, esperava %v", status, http.StatusOK)
	}

	var response RealtimeMetricsResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("não foi possível decodificar a resposta JSON: %v", err)
	}

	// Verifica se a resposta contém os dados esperados.
	if len(response.Nodes) != 1 {
		t.Errorf("esperado 1 nó na resposta, obteve %d", len(response.Nodes))
	}
	if response.Nodes[0].Name != "node-1" {
		t.Errorf("nome do nó incorreto: esperado 'node-1', obteve '%s'", response.Nodes[0].Name)
	}
	if response.IsRunningInCluster {
		t.Error("esperado IsRunningInCluster ser falso no teste")
	}
}

// TestRealtimeHandler_ClientError testa o comportamento do handler quando o cliente K8s retorna um erro.
func TestRealtimeHandler_ClientError(t *testing.T) {
	// 1. Setup: Configura o cliente falso para retornar um erro em qualquer chamada 'List'.
	fakecs := fake.NewSimpleClientset()
	fakecs.PrependReactor("list", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("erro forçado da API do kubernetes")
	})
	k8s.Clientset = fakecs
	k8s.MetricsClientset = metricsvake.NewSimpleClientset() // Cliente de métricas sem erro.

	// Monkey patch a função InClusterConfig
	originalInClusterConfig := k8s.InClusterConfigFunc
	t.Cleanup(func() { k8s.InClusterConfigFunc = originalInClusterConfig })
	k8s.InClusterConfigFunc = func() (*rest.Config, error) {
		return nil, fmt.Errorf("não está em um cluster")
	}

	handler := http.HandlerFunc(RealtimeHandler)
	req, _ := http.NewRequest("GET", "/api/realtime", nil)
	rr := httptest.NewRecorder()

	// 2. Execução
	handler.ServeHTTP(rr, req)

	// 3. Verificação
	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler retornou status code incorreto para erro de cliente: obteve %v, esperava %v", status, http.StatusInternalServerError)
	}
}