package main

import (
	"log"
	"net/http"

	"kubeowl/internal/handlers"
	"kubeowl/internal/k8s"
	"kubeowl/internal/services"
	"kubeowl/internal/watchers"
	"kubeowl/internal/websocket"
)

func main() {
	if err := k8s.InitClient(); err != nil {
		log.Printf("Aviso: Falha ao inicializar completamente o cliente K8s: %v", err)
	}

	hub := websocket.NewHub()
	go hub.Run()

	go watchers.Start(hub)

	k8sService := services.NewK8sService(k8s.Clientset, k8s.MetricsClientset)

	router := handlers.NewRouter(hub, k8sService)
	router.RegisterRoutes()

	log.Println("Iniciando o servidor KubeOwl na porta :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Falha ao iniciar o servidor: %v", err)
	}
}
