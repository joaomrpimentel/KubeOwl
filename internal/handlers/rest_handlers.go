package handlers

import (
	"encoding/json"
	"log"
	"net/http"
)

func (r *Router) OverviewHandler(w http.ResponseWriter, req *http.Request) {
	data, err := r.Service.GetOverviewData(req.Context())
	if err != nil {
		jsonErrorResponse(w, "Falha ao buscar dados da visão geral", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, data, http.StatusOK)
}

func (r *Router) NodesHandler(w http.ResponseWriter, req *http.Request) {
	data, err := r.Service.GetNodeInfo(req.Context())
	if err != nil {
		jsonErrorResponse(w, "Falha ao buscar dados dos nós", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, data, http.StatusOK)
}

func (r *Router) PodsHandler(w http.ResponseWriter, req *http.Request) {
	data, err := r.Service.GetPodInfo(req.Context())
	if err != nil {
		jsonErrorResponse(w, "Falha ao buscar dados dos pods", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, data, http.StatusOK)
}

func (r *Router) ServicesHandler(w http.ResponseWriter, req *http.Request) {
	data, err := r.Service.GetServiceInfo(req.Context())
	if err != nil {
		jsonErrorResponse(w, "Falha ao buscar dados dos services", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, data, http.StatusOK)
}

func (r *Router) IngressesHandler(w http.ResponseWriter, req *http.Request) {
	data, err := r.Service.GetIngressInfo(req.Context())
	if err != nil {
		jsonErrorResponse(w, "Falha ao buscar dados dos ingresses", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, data, http.StatusOK)
}

func (r *Router) PvcsHandler(w http.ResponseWriter, req *http.Request) {
	data, err := r.Service.GetPvcInfo(req.Context())
	if err != nil {
		jsonErrorResponse(w, "Falha ao buscar dados dos PVCs", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, data, http.StatusOK)
}

func (r *Router) EventsHandler(w http.ResponseWriter, req *http.Request) {
	data, err := r.Service.GetEventInfo(req.Context())
	if err != nil {
		jsonErrorResponse(w, "Falha ao buscar dados dos eventos", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, data, http.StatusOK)
}

func (r *Router) NamespacesHandler(w http.ResponseWriter, req *http.Request) {
	data, err := r.Service.GetNamespaces(req.Context())
	if err != nil {
		jsonErrorResponse(w, "Falha ao buscar dados dos namespaces", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, data, http.StatusOK)
}

// --- Funções Utilitárias de Resposta ---

func jsonResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Erro ao codificar resposta JSON: %v", err)
	}
}

func jsonErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	log.Println(message)
	jsonResponse(w, map[string]string{"error": message}, statusCode)
}
