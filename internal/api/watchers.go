package api

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"kubeowl/internal/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// WSMessage define a estrutura da mensagem enviada pelo WebSocket.
type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// StartWatchers inicia os watchers para os recursos do Kubernetes e os envia para o hub.
func StartWatchers(hub *Hub) {
	log.Println("Iniciando watchers do Kubernetes...")
	go runWatcher(hub, "pods", watchPods)
	go runWatcher(hub, "events", watchEvents)
	go runWatcher(hub, "nodes", watchNodes)
}

// runWatcher é uma função genérica para manter um watcher sempre em execução.
func runWatcher(hub *Hub, resourceType string, watchFunc func(context.Context) (watch.Interface, error)) {
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
		for event := range watcher.ResultChan() {
			msg := WSMessage{Type: resourceType, Payload: event}
			jsonMsg, err := json.Marshal(msg)
			if err != nil {
				log.Printf("Erro ao serializar mensagem do watcher de %s: %v", resourceType, err)
				continue
			}
			hub.broadcast <- jsonMsg
		}

		log.Printf("Watcher de %s encerrado. Reiniciando...", resourceType)
		watcher.Stop()
		cancel()
	}
}

// watchPods cria um watcher para Pods.
func watchPods(ctx context.Context) (watch.Interface, error) {
	return k8s.Clientset.CoreV1().Pods("").Watch(ctx, metav1.ListOptions{})
}

// watchEvents cria um watcher para Events.
func watchEvents(ctx context.Context) (watch.Interface, error) {
	// Apenas eventos recentes
	return k8s.Clientset.CoreV1().Events("").Watch(ctx, metav1.ListOptions{})
}

// watchNodes cria um watcher para Nodes.
func watchNodes(ctx context.Context) (watch.Interface, error) {
	return k8s.Clientset.CoreV1().Nodes().Watch(ctx, metav1.ListOptions{})
}
