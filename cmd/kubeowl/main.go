package main

import (
	"log"
	"net/http"

	"kubeowl/internal/api"
	"kubeowl/internal/k8s"
)

func main() {
	// Inicializa o cliente do Kubernetes
	if err := k8s.InitClient(); err != nil {
		log.Printf("Aviso: Falha ao inicializar completamente o cliente K8s: %v", err)
	}

	// Define a rota da API
	http.HandleFunc("/api/realtime", api.RealtimeHandler)

	// Cria um file server para servir os arquivos estáticos do frontend
	// O servidor automaticamente encontrará o index.html para a rota "/"
	fs := http.FileServer(http.Dir("./web/static"))
	http.Handle("/", fs)

	log.Println("Iniciando o servidor KubeOwl na porta :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Falha ao iniciar o servidor: %v", err)
	}
}
