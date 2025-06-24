package handlers

import (
	"kubeowl/internal/websocket"
	"net/http"
)

// ServeWs trata as solicitações de WebSocket.
func (r *Router) ServeWs(w http.ResponseWriter, req *http.Request) {
	websocket.ServeWs(r.hub, w, req)
}
