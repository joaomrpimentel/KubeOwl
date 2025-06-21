package api

import (
	"context"
	"encoding/json"
	"kubeowl/internal/k8s"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// setupTestServer inicia um Hub e um servidor de teste para a conexão WebSocket.
func setupTestServer(t *testing.T) (*Hub, *httptest.Server, *websocket.Conn) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	}))

	// Converte a URL http para ws
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err, "Falha ao conectar ao servidor de teste WebSocket")

	// Espera um pouco para garantir que o cliente foi registrado no hub
	time.Sleep(100 * time.Millisecond)

	return hub, server, conn
}

// TestHubRegistrationAndUnregistration testa se um cliente é registrado e desregistrado corretamente.
func TestHubRegistrationAndUnregistration(t *testing.T) {
	hub, server, conn := setupTestServer(t)
	defer server.Close()
	defer conn.Close()

	// Verifica se o cliente foi registrado
	assert.Equal(t, 1, len(hub.clients), "O cliente deveria ter sido registrado no hub")

	// Fecha a conexão para acionar o desregistro
	conn.Close()

	// Espera um pouco para o hub processar o desregistro
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, len(hub.clients), "O cliente deveria ter sido desregistrado do hub")
}

// TestHubBroadcast testa se uma mensagem enviada para o broadcast do hub é recebida pelo cliente.
func TestHubBroadcast(t *testing.T) {
	hub, server, conn := setupTestServer(t)
	defer server.Close()
	defer conn.Close()

	testMessage := []byte("hello world")
	hub.broadcast <- testMessage

	// Lê a mensagem da conexão WebSocket
	_, receivedMessage, err := conn.ReadMessage()
	assert.NoError(t, err, "Deveria ler a mensagem do WebSocket sem erro")
	assert.Equal(t, testMessage, receivedMessage, "A mensagem recebida não corresponde à mensagem enviada")
}

// TestRunWatcher testa a lógica do watcher e o envio da mensagem para o hub.
func TestRunWatcher(t *testing.T) {
	hub, server, conn := setupTestServer(t)
	defer server.Close()
	defer conn.Close()

	// Configura um cliente Kubernetes falso com um watcher falso
	fakeClient := fake.NewSimpleClientset()
	k8s.Clientset = fakeClient
	fakeWatcher := watch.NewFake()
	// Configura o cliente para retornar o watcher falso quando a função Watch for chamada
	fakeClient.PrependWatchReactor("pods", func(action k8stesting.Action) (handled bool, ret watch.Interface, err error) {
		return true, fakeWatcher, nil
	})

	// Inicia o watcher em uma goroutine
	go runWatcher(hub, "pods", func(ctx context.Context) (watch.Interface, error) {
		return k8s.Clientset.CoreV1().Pods("").Watch(ctx, metav1.ListOptions{})
	})
	time.Sleep(100 * time.Millisecond) // Dá tempo para o watcher iniciar

	// Simula um evento de pod adicionado
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"}}
	fakeWatcher.Add(pod)

	// Lê a mensagem do WebSocket para verificar se o evento foi recebido
	_, msgBytes, err := conn.ReadMessage()
	assert.NoError(t, err)

	// Etapa 1: Decodificar a mensagem externa (WSMessage)
	var receivedMsg WSMessage
	err = json.Unmarshal(msgBytes, &receivedMsg)
	assert.NoError(t, err, "A mensagem recebida deveria ser um JSON válido de WSMessage")

	assert.Equal(t, "pods", receivedMsg.Type, "O tipo da mensagem WebSocket deve ser 'pods'")

	// Etapa 2: Decodificar o payload interno (o evento do watcher)
	payloadBytes, err := json.Marshal(receivedMsg.Payload)
	assert.NoError(t, err)

	// Usamos uma struct auxiliar para decodificar, tratando o 'Object' como json.RawMessage
	var watchEvent struct {
		Type   watch.EventType `json:"type"`
		Object json.RawMessage `json:"object"`
	}
	err = json.Unmarshal(payloadBytes, &watchEvent)
	assert.NoError(t, err, "Não foi possível decodificar o payload do evento do watcher")

	assert.Equal(t, watch.Added, watchEvent.Type)

	// Etapa 3: Decodificar o objeto final para o tipo concreto (*v1.Pod)
	var receivedPod v1.Pod
	err = json.Unmarshal(watchEvent.Object, &receivedPod)
	assert.NoError(t, err, "O objeto do evento deveria ser decodificável como um Pod")
	assert.Equal(t, "test-pod", receivedPod.Name)
}
