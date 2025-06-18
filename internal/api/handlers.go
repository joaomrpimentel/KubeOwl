package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"kubeowl/internal/k8s"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// RealtimeHandler busca e retorna os dados em tempo real do cluster.
func RealtimeHandler(w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup
	ctx := context.Background()

	var nodes *v1.NodeList
	var pods *v1.PodList
	var deployments *appsv1.DeploymentList
	var services *v1.ServiceList
	var namespaces *v1.NamespaceList
	var events *v1.EventList
	var pvcs *v1.PersistentVolumeClaimList
	var nodeMetrics *metricsv1beta1.NodeMetricsList
	var podMetrics *metricsv1beta1.PodMetricsList
	var apiError error

	fetch := func(fn func() error) {
		defer wg.Done()
		if err := fn(); err != nil && apiError == nil {
			apiError = err
		}
	}

	// Busca os recursos principais em paralelo
	wg.Add(7)
	go fetch(func() (err error) { nodes, err = k8s.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); return })
	go fetch(func() (err error) { pods, err = k8s.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{}); return })
	go fetch(func() (err error) { deployments, err = k8s.Clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{}); return })
	go fetch(func() (err error) { services, err = k8s.Clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{}); return })
	go fetch(func() (err error) { namespaces, err = k8s.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{}); return })
	go fetch(func() (err error) { events, err = k8s.Clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{}); return })
	go fetch(func() (err error) { pvcs, err = k8s.Clientset.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{}); return })

	// Busca as métricas se o cliente estiver disponível
	if k8s.MetricsClientset != nil {
		wg.Add(2)
		go fetch(func() (err error) { nodeMetrics, err = k8s.MetricsClientset.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{}); return })
		go fetch(func() (err error) { podMetrics, err = k8s.MetricsClientset.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{}); return })
	}

	wg.Wait()

	if apiError != nil {
		log.Printf("Erro ao buscar dados do cluster: %v", apiError)
		http.Error(w, fmt.Sprintf("Erro ao buscar dados do cluster: %v", apiError), http.StatusInternalServerError)
		return
	}

	// Processa os dados coletados
	userNamespaceCount, userNamespaces := processNamespaces(namespaces)
	_, inClusterErr := rest.InClusterConfig()
	isRunningInCluster := inClusterErr == nil

	response := RealtimeMetricsResponse{
		IsRunningInCluster: isRunningInCluster,
		Nodes:              processNodeInfo(nodes, pods, nodeMetrics),
		Pods:               processPodInfo(pods, podMetrics, userNamespaces),
		Events:             processEvents(events, userNamespaces),
		Pvcs:               processPvcs(pvcs, userNamespaces),
		Capacity:           processClusterCapacity(nodes, nodeMetrics),
		DeploymentCount:    len(deployments.Items),
		ServiceCount:       len(services.Items),
		NamespaceCount:     userNamespaceCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
