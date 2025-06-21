package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"kubeowl/internal/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// jsonResponse é uma função utilitária para enviar respostas JSON e tratar erros.
func jsonResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Erro ao codificar resposta JSON: %v", err)
	}
}

// OverviewHandler retorna dados agregados para a visão geral do dashboard.
func OverviewHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	deployments, err := k8s.Clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		jsonResponse(w, map[string]string{"error": "Falha ao buscar deployments"}, http.StatusInternalServerError)
		return
	}
	namespaces, err := k8s.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		jsonResponse(w, map[string]string{"error": "Falha ao buscar namespaces"}, http.StatusInternalServerError)
		return
	}
	nodes, err := k8s.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		jsonResponse(w, map[string]string{"error": "Falha ao buscar nós"}, http.StatusInternalServerError)
		return
	}

	nodeMetrics, _ := k8s.MetricsClientset.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})

	userNamespaceCount, _ := processNamespaces(namespaces)
	_, inClusterErr := rest.InClusterConfig()

	response := OverviewResponse{
		IsRunningInCluster: inClusterErr == nil,
		DeploymentCount:    len(deployments.Items),
		NamespaceCount:     userNamespaceCount,
		NodeCount:          len(nodes.Items),
		Capacity:           processClusterCapacity(nodes, nodeMetrics),
	}

	jsonResponse(w, response, http.StatusOK)
}

// NodesHandler retorna a lista de nós e suas métricas.
func NodesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	nodes, err := k8s.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		jsonResponse(w, []NodeInfo{}, http.StatusInternalServerError)
		return
	}
	pods, _ := k8s.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	nodeMetrics, _ := k8s.MetricsClientset.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})

	processedData := processNodeInfo(nodes, pods, nodeMetrics)
	jsonResponse(w, processedData, http.StatusOK)
}

// PodsHandler retorna a lista de pods e suas métricas.
func PodsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	pods, err := k8s.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		jsonResponse(w, []PodInfo{}, http.StatusInternalServerError)
		return
	}
	podMetrics, _ := k8s.MetricsClientset.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})

	namespaces, err := k8s.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		jsonResponse(w, []PodInfo{}, http.StatusInternalServerError)
		return
	}
	_, userNamespaces := processNamespaces(namespaces)

	processedData := processPodInfo(pods, podMetrics, userNamespaces)
	jsonResponse(w, processedData, http.StatusOK)
}

// ServicesHandler retorna a lista de serviços.
func ServicesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	services, err := k8s.Clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		jsonResponse(w, []ServiceInfo{}, http.StatusInternalServerError)
		return
	}
	namespaces, err := k8s.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		jsonResponse(w, []ServiceInfo{}, http.StatusInternalServerError)
		return
	}
	_, userNamespaces := processNamespaces(namespaces)

	processedData := processServiceInfo(services, userNamespaces)
	jsonResponse(w, processedData, http.StatusOK)
}

// IngressesHandler retorna a lista de ingresses.
func IngressesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	ingresses, err := k8s.Clientset.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		jsonResponse(w, []IngressInfo{}, http.StatusInternalServerError)
		return
	}
	namespaces, err := k8s.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		jsonResponse(w, []IngressInfo{}, http.StatusInternalServerError)
		return
	}
	_, userNamespaces := processNamespaces(namespaces)

	processedData := processIngressInfo(ingresses, userNamespaces)
	jsonResponse(w, processedData, http.StatusOK)
}

// PvcsHandler retorna a lista de PersistentVolumeClaims.
func PvcsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	pvcs, err := k8s.Clientset.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	if err != nil {
		jsonResponse(w, []PvcInfo{}, http.StatusInternalServerError)
		return
	}
	namespaces, err := k8s.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		jsonResponse(w, []PvcInfo{}, http.StatusInternalServerError)
		return
	}
	_, userNamespaces := processNamespaces(namespaces)

	processedData := processPvcs(pvcs, userNamespaces)
	jsonResponse(w, processedData, http.StatusOK)
}

// EventsHandler retorna a lista de eventos recentes.
func EventsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	events, err := k8s.Clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{})
	if err != nil {
		jsonResponse(w, []EventInfo{}, http.StatusInternalServerError)
		return
	}
	namespaces, err := k8s.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		jsonResponse(w, []EventInfo{}, http.StatusInternalServerError)
		return
	}
	_, userNamespaces := processNamespaces(namespaces)

	processedData := processEvents(events, userNamespaces)
	jsonResponse(w, processedData, http.StatusOK)
}