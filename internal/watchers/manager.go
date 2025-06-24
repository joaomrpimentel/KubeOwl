package watchers

import (
	"context"
	"encoding/json"
	"kubeowl/internal/k8s"
	"kubeowl/internal/models"
	"kubeowl/internal/websocket"
	"log"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// Start inicia os watchers para os recursos do Kubernetes.
func Start(hub *websocket.Hub) {
	log.Println("Iniciando watchers do Kubernetes...")
	go runWatcher(hub, "pods", watchPods)
	go runWatcher(hub, "events", watchEvents)
	go runWatcher(hub, "nodes", watchNodes)
}

func runWatcher(hub *websocket.Hub, resourceType string, watchFunc func(context.Context) (watch.Interface, error)) {
	for {
		ctx, cancel := context.WithCancel(context.Background())
		watcher, err := watchFunc(ctx)
		if err != nil {
			log.Printf("Erro ao iniciar watcher de %s: %v. Tentando novamente em 5s.", resourceType, err)
			cancel()
			time.Sleep(5 * time.Second)
			continue
		}

		log.Printf("Watcher de %s iniciado.", resourceType)
		processWatcherEvents(hub, watcher.ResultChan(), resourceType)
		watcher.Stop()
		cancel()
		log.Printf("Watcher de %s encerrado. Reiniciando...", resourceType)
	}
}

func processWatcherEvents(hub *websocket.Hub, events <-chan watch.Event, resourceType string) {
	for event := range events {
		msg := models.WSMessage{Type: resourceType, Payload: event}
		jsonMsg, err := json.Marshal(msg)
		if err != nil {
			log.Printf("Erro ao serializar mensagem do watcher de %s: %v", resourceType, err)
			continue
		}
		hub.Broadcast <- jsonMsg
	}
}

func watchPods(ctx context.Context) (watch.Interface, error) {
	return k8s.Clientset.CoreV1().Pods("").Watch(ctx, metav1.ListOptions{})
}

func watchEvents(ctx context.Context) (watch.Interface, error) {
	return k8s.Clientset.CoreV1().Events("").Watch(ctx, metav1.ListOptions{})
}

func watchNodes(ctx context.Context) (watch.Interface, error) {
	return k8s.Clientset.CoreV1().Nodes().Watch(ctx, metav1.ListOptions{})
}
