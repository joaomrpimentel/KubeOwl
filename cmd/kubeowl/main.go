package main

import (
	"log"
	"net/http"

	"kubeowl/internal/api"
	"kubeowl/internal/k8s"
)

func main() {
	if err := k8s.InitClient(); err != nil {
		log.Printf("Aviso: Falha ao inicializar completamente o cliente K8s: %v", err)
	}

	http.HandleFunc("/api/overview", api.OverviewHandler)
	http.HandleFunc("/api/nodes", api.NodesHandler)
	http.HandleFunc("/api/pods", api.PodsHandler)
	http.HandleFunc("/api/services", api.ServicesHandler)
	http.HandleFunc("/api/ingresses", api.IngressesHandler)
	http.HandleFunc("/api/pvcs", api.PvcsHandler)
	http.HandleFunc("/api/events", api.EventsHandler)

	fs := http.FileServer(http.Dir("./web/static"))
	http.Handle("/", fs)

	log.Println("Iniciando o servidor KubeOwl na porta :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Falha ao iniciar o servidor: %v", err)
	}
}
