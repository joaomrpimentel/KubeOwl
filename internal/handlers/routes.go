package handlers

import (
	"kubeowl/internal/services"
	"kubeowl/internal/websocket"
	"net/http"
)

// Router gerencia o roteamento da API.
type Router struct {
	hub     *websocket.Hub
	Service services.Service
}

// NewRouter cria uma nova instância do Router.
func NewRouter(hub *websocket.Hub, service services.Service) *Router {
	return &Router{
		hub:     hub,
		Service: service,
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
	http.HandleFunc("/api/namespaces", r.NamespacesHandler)

	// Handler do WebSocket
	http.HandleFunc("/ws", r.ServeWs)

	// Servidor de arquivos estáticos
	fs := http.FileServer(http.Dir("./web/static"))
	http.Handle("/", fs)
}