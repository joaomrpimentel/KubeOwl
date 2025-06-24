package handlers

import (
	"kubeowl/internal/services"
	"kubeowl/internal/websocket"
	"net/http"
)

// Router gerencia o roteamento da API.
type Router struct {
	hub        *websocket.Hub
	k8sService *services.K8sService
}

// NewRouter cria uma nova instância do Router.
func NewRouter(hub *websocket.Hub, k8sService *services.K8sService) *Router {
	return &Router{
		hub:        hub,
		k8sService: k8sService,
	}
}

// RegisterRoutes registra todos os handlers da aplicação.
func (r *Router) RegisterRoutes() {
	// Handlers da API REST
	http.HandleFunc("/api/overview", r.OverviewHandler)
	http.HandleFunc("/api/nodes", r.NodesHandler)
	http.HandleFunc("/api/pods", r.PodsHandler)
	http.HandleFunc("/api/services", r.ServicesHandler)
	http.HandleFunc("/api/ingresses", r.IngressesHandler)
	http.HandleFunc("/api/pvcs", r.PvcsHandler)
	http.HandleFunc("/api/events", r.EventsHandler)

	// Handler do WebSocket
	http.HandleFunc("/ws", r.ServeWs)

	// Servidor de arquivos estáticos
	fs := http.FileServer(http.Dir("./web/static"))
	http.Handle("/", fs)
}
