package websocket

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

// TestMain é executado antes de todos os outros testes neste pacote.
// Usamos isso para silenciar a saída do logger durante a execução dos testes.
func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// setupTestServer inicia um Hub e um servidor de teste para a conexão WebSocket.
func setupTestServer(t *testing.T) (*Hub, *httptest.Server, *websocket.Conn) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	}))

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err, "Falha ao conectar ao servidor de teste WebSocket")

	time.Sleep(100 * time.Millisecond) // Garante que o cliente foi registrado

	return hub, server, conn
}

func TestHubRegistrationAndUnregistration(t *testing.T) {
	hub, server, conn := setupTestServer(t)
	defer server.Close()
	defer conn.Close()

	assert.Equal(t, 1, len(hub.clients), "O cliente deveria ter sido registrado")

	conn.Close()
	time.Sleep(100 * time.Millisecond) // Garante que o desregistro foi processado
	assert.Equal(t, 0, len(hub.clients), "O cliente deveria ter sido desregistrado")
}

func TestHubBroadcast(t *testing.T) {
	hub, server, conn := setupTestServer(t)
	defer server.Close()
	defer conn.Close()

	// Cria um segundo cliente para garantir que o broadcast funciona para múltiplos clientes
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)
	defer conn2.Close()
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 2, len(hub.clients), "Deveriam existir 2 clientes registrados")

	testMessage := []byte("hello broadcast")
	hub.Broadcast <- testMessage

	// Verifica a mensagem no primeiro cliente
	_, receivedMessage1, err := conn.ReadMessage()
	assert.NoError(t, err)
	assert.Equal(t, testMessage, receivedMessage1)

	// Verifica a mensagem no segundo cliente
	_, receivedMessage2, err := conn2.ReadMessage()
	assert.NoError(t, err)
	assert.Equal(t, testMessage, receivedMessage2)
}
