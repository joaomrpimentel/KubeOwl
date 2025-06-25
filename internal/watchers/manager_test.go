package watchers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"kubeowl/internal/k8s"
	"kubeowl/internal/models"
	"kubeowl/internal/websocket"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// TestMain silencia a saída de log durante os testes deste pacote.
func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

func TestStart(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	fakeClient := fake.NewSimpleClientset()
	fakeWatcher := watch.NewFake()
	fakeClient.PrependWatchReactor("*", func(action k8stesting.Action) (handled bool, ret watch.Interface, err error) {
		return true, fakeWatcher, nil
	})
	k8s.Clientset = fakeClient

	go func() {
		time.Sleep(200 * time.Millisecond)
		fakeWatcher.Stop()
	}()

	assert.NotPanics(t, func() {
		Start(hub)
	}, "Start não deve causar pânico")
}

func TestRunWatcher_ErrorHandling(t *testing.T) {
	hub := websocket.NewHub()
	errorCount := 0

	watchFunc := func(ctx context.Context) (watch.Interface, error) {
		if errorCount == 0 {
			errorCount++
			return nil, errors.New("erro forçado")
		}
		fakeWatcher := watch.NewFake()
		go fakeWatcher.Stop()
		return fakeWatcher, nil
	}

	go runWatcher(hub, "test-resource", watchFunc)

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, errorCount, "A função de watch deveria ter sido chamada novamente após o erro")
}

func TestProcessWatcherEvents(t *testing.T) {
	hub := websocket.NewHub()
	// Não executa hub.Run() para garantir que a mensagem permaneça no canal de broadcast.

	eventChan := make(chan watch.Event, 1)
	go processWatcherEvents(hub, eventChan, "pods")

	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod"}}
	event := watch.Event{Type: watch.Added, Object: pod}
	eventChan <- event
	close(eventChan)

	select {
	case msgBytes := <-hub.Broadcast:
		var msg models.WSMessage
		err := json.Unmarshal(msgBytes, &msg)
		assert.NoError(t, err)
		assert.Equal(t, "pods", msg.Type, "O tipo da mensagem deve ser 'pods'")

		var payloadData struct {
			Type   watch.EventType `json:"type"`
			Object json.RawMessage `json:"object"`
		}
		payloadBytes, err := json.Marshal(msg.Payload)
		assert.NoError(t, err)
		err = json.Unmarshal(payloadBytes, &payloadData)

		assert.NoError(t, err, "A decodificação do payload não deve falhar")
		assert.Equal(t, event.Type, payloadData.Type, "O tipo do evento no payload deve corresponder")

	case <-time.After(1 * time.Second):
		t.Fatal("Tempo esgotado esperando a mensagem no canal de broadcast")
	}
}
