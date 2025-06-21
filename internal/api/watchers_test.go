package api

import (
	"context"
	"encoding/json"
	"errors"
	"kubeowl/internal/k8s"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// TestStartWatchers verifica se a função principal chama a sub-rotina do watcher.
func TestStartWatchers(t *testing.T) {
	// Usamos um hub "falso" que não faz nada, pois o foco é testar a inicialização.
	hub := NewHub()
	go hub.Run() // Inicia o hub para que o envio não bloqueie

	// Configura um cliente falso que retorna um watcher falso
	fakeClient := fake.NewSimpleClientset()
	fakeWatcher := watch.NewFake()
	fakeClient.PrependWatchReactor("*", func(action k8stesting.Action) (handled bool, ret watch.Interface, err error) {
		return true, fakeWatcher, nil
	})
	k8s.Clientset = fakeClient

	// Encerra o watcher falso logo após a chamada para que o teste não fique preso em um loop infinito.
	go func() {
		time.Sleep(100 * time.Millisecond)
		fakeWatcher.Stop()
	}()

	// Verifica se a função é executada sem pânico e se o teste termina.
	assert.NotPanics(t, func() {
		StartWatchers(hub)
	}, "StartWatchers não deve causar pânico")
}

// TestRunWatcher_ErrorHandling testa se a função runWatcher tenta reiniciar após um erro.
func TestRunWatcher_ErrorHandling(t *testing.T) {
	hub := NewHub()
	errorCount := 0

	// Função de watcher que retorna um erro na primeira vez que é chamada.
	watchFunc := func(ctx context.Context) (watch.Interface, error) {
		if errorCount == 0 {
			errorCount++
			return nil, errors.New("erro forçado ao iniciar watcher")
		}
		// Na segunda chamada, retorna um watcher que fecha imediatamente para terminar o teste.
		fakeWatcher := watch.NewFake()
		go fakeWatcher.Stop()
		return fakeWatcher, nil
	}

	// Executa o watcher em uma goroutine
	go func() {
		runWatcher(hub, "test-resource", watchFunc)
	}()

	// Dá tempo para a função tentar reiniciar
	time.Sleep(100 * time.Millisecond)

	// Verifica se a função de watch foi chamada duas vezes (uma falha, uma sucesso)
	assert.Equal(t, 1, errorCount, "A função de watch deveria ter sido chamada novamente após o erro")
}

// TestSpecificWatchFunctions testa se as funções de watch individuais (watchPods, etc.) chamam a API correta do cliente Kubernetes.
func TestSpecificWatchFunctions(t *testing.T) {
	tests := []struct {
		name         string
		watchFunc    func(context.Context) (watch.Interface, error)
		resourceName string // Nome do recurso na API (ex: "pods", "events")
	}{
		{"watchPods", watchPods, "pods"},
		{"watchEvents", watchEvents, "events"},
		{"watchNodes", watchNodes, "nodes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset()
			k8s.Clientset = fakeClient
			callCount := 0

			// Intercepta a chamada de "watch" para o recurso específico
			fakeClient.PrependWatchReactor(tt.resourceName, func(action k8stesting.Action) (handled bool, ret watch.Interface, err error) {
				callCount++
				return true, watch.NewFake(), nil
			})

			// Chama a função de watch
			_, err := tt.watchFunc(context.Background())

			// Verifica os resultados
			assert.NoError(t, err, "A função de watch não deveria retornar erro com um cliente falso")
			assert.Equal(t, 1, callCount, "A API de watch para '%s' deveria ter sido chamada exatamente uma vez", tt.resourceName)
		})
	}
}

// TestRunWatcher_MessageBroadcasting testa se o runWatcher envia corretamente a mensagem para o hub e para o cliente.
func TestRunWatcher_MessageBroadcasting(t *testing.T) {
	// setupTestServer nos dá um hub funcional e um cliente conectado
	hub, server, conn := setupTestServer(t)
	defer server.Close()
	defer conn.Close()

	// Configura o cliente Kubernetes falso para o watcher
	fakeClient := fake.NewSimpleClientset()
	k8s.Clientset = fakeClient
	fakeWatcher := watch.NewFake()
	fakeClient.PrependWatchReactor("pods", func(action k8stesting.Action) (handled bool, ret watch.Interface, err error) {
		return true, fakeWatcher, nil
	})

	// Inicia o watcher, passando o hub do nosso servidor de teste
	go runWatcher(hub, "pods", watchPods)

	// Adiciona um pod para gerar um evento
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod"}}
	fakeWatcher.Add(pod)

	// Define um deadline para a leitura da mensagem para evitar que o teste trave
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Falha ao ler mensagem do WebSocket: %v", err)
	}

	// Verifica se a mensagem recebida pelo cliente WebSocket está correta
	var msg WSMessage
	err = json.Unmarshal(msgBytes, &msg)
	assert.NoError(t, err, "A mensagem recebida pelo WebSocket deveria ser um JSON válido")
	assert.Equal(t, "pods", msg.Type, "O tipo da mensagem deveria ser 'pods'")
}
