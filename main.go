// main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

// --- Data Structures for API Response ---

type NodeInfo struct {
	Name        string `json:"name"`
	PodCount    int    `json:"podCount"`
	TotalCPU    string `json:"totalCpu"`
	UsedCPU     string `json:"usedCpu"`
	TotalMemory string `json:"totalMemory"`
	UsedMemory  string `json:"usedMemory"`
}

type PodInfo struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	UsedCPU         string `json:"usedCpu"`
	UsedCPUMilli    int64  `json:"usedCpuMilli"`
	UsedMemory      string `json:"usedMemory"`
	UsedMemoryBytes int64  `json:"usedMemoryBytes"`
}

type MetricsResponse struct {
	IsRunningInCluster bool       `json:"isRunningInCluster"`
	Nodes              []NodeInfo `json:"nodes"`
	Pods               []PodInfo  `json:"pods"`
	DeploymentCount    int        `json:"deploymentCount"`
	ServiceCount       int        `json:"serviceCount"`
	NamespaceCount     int        `json:"namespaceCount"`
}

var clientset *kubernetes.Clientset
var metricsClientset *metricsclientset.Clientset

// --- Main Application ---

func main() {
	initKubeClient()

	// Serve the static index.html file at the root
	http.HandleFunc("/", serveFrontend)
	// Serve the metrics data at the /api/metrics endpoint
	http.HandleFunc("/api/metrics", apiMetricsHandler)

	log.Println("Starting server on :8080...")
	log.Println("Access the dashboard at http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// serveFrontend handles serving the static HTML frontend.
func serveFrontend(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

// apiMetricsHandler handles the API request for cluster metrics.
func apiMetricsHandler(w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup
	ctx := context.Background()

	var nodes *v1.NodeList
	var pods *v1.PodList
	var deployments *appsv1.DeploymentList
	var services *v1.ServiceList
	var namespaces *v1.NamespaceList
	var nodeMetrics *metricsv1beta1.NodeMetricsList
	var podMetrics *metricsv1beta1.PodMetricsList
	var apiError error

	fetch := func(fn func() error) {
		defer wg.Done()
		if err := fn(); err != nil && apiError == nil {
			apiError = err
		}
	}

	wg.Add(7)
	go fetch(func() error { var err error; nodes, err = clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); return err })
	go fetch(func() error { var err error; pods, err = clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{}); return err })
	go fetch(func() error { var err error; deployments, err = clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{}); return err })
	go fetch(func() error { var err error; services, err = clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{}); return err })
	go fetch(func() error { var err error; namespaces, err = clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{}); return err })
	go fetch(func() error { var err error; nodeMetrics, err = metricsClientset.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{}); return err })
	go fetch(func() error { var err error; podMetrics, err = metricsClientset.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{}); return err })
	
	wg.Wait()

	if apiError != nil {
		log.Printf("Error fetching cluster data: %v", apiError)
		http.Error(w, fmt.Sprintf("Error fetching cluster data: %v", apiError), http.StatusInternalServerError)
		return
	}

	// Process Data
	userNamespaceCount, userNamespaces := filterNamespaces(namespaces)
	nodeInfoList := processNodeInfo(nodes, pods, nodeMetrics)
	podInfoList := processPodInfo(podMetrics, userNamespaces)

	// Determine if running in-cluster
	_, inClusterErr := rest.InClusterConfig()
	isRunningInCluster := inClusterErr == nil

	// Create API response
	response := MetricsResponse{
		IsRunningInCluster: isRunningInCluster,
		Nodes:              nodeInfoList,
		Pods:               podInfoList,
		DeploymentCount:    len(deployments.Items),
		ServiceCount:       len(services.Items),
		NamespaceCount:     userNamespaceCount,
	}

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// --- Helper Functions ---

func initKubeClient() {
	var config *rest.Config
	var err error
	if config, err = rest.InClusterConfig(); err != nil {
		log.Println("Not in-cluster. Using local kubeconfig.")
		homeDir, _ := os.UserHomeDir()
		kubeconfig := filepath.Join(homeDir, ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Fatalf("Failed to create config: %v", err)
		}
	} else {
		log.Println("Running in-cluster.")
	}
	clientset, _ = kubernetes.NewForConfig(config)
	metricsClientset, _ = metricsclientset.NewForConfig(config)
}

func filterNamespaces(namespaces *v1.NamespaceList) (int, map[string]bool) {
	systemNamespaces := map[string]bool{
		"default": true, "kube-system": true, "kube-public": true, "kube-node-lease": true,
		"local": true, "cert-manager": true,
	}
	systemPrefixes := []string{"cattle-", "fleet-", "cluster-fleet-", "local-p-", "p-", "user-"}
	userNamespaceCount := 0
	userNamespaces := map[string]bool{}

	for _, ns := range namespaces.Items {
		isSystem := false
		if systemNamespaces[ns.Name] {
			isSystem = true
		} else {
			for _, prefix := range systemPrefixes {
				if strings.HasPrefix(ns.Name, prefix) {
					isSystem = true
					break
				}
			}
		}
		if !isSystem {
			userNamespaceCount++
			userNamespaces[ns.Name] = true
		}
	}
	return userNamespaceCount, userNamespaces
}

func processNodeInfo(nodes *v1.NodeList, pods *v1.PodList, nodeMetrics *metricsv1beta1.NodeMetricsList) []NodeInfo {
	nodeInfoList := []NodeInfo{}
	if nodes == nil {
		return nodeInfoList
	}
	for _, node := range nodes.Items {
		usedCPU, usedMemory := getNodeUsage(node.Name, nodeMetrics)
		podCount := countPodsOnNode(node.Name, pods)
		info := NodeInfo{
			Name:        node.Name,
			PodCount:    podCount,
			TotalCPU:    fmt.Sprintf("%.2f", float64(node.Status.Allocatable.Cpu().MilliValue())/1000.0),
			UsedCPU:     fmt.Sprintf("%.2f", float64(usedCPU.MilliValue())/1000.0),
			TotalMemory: fmt.Sprintf("%.2f Gi", float64(node.Status.Allocatable.Memory().Value())/(1024*1024*1024)),
			UsedMemory:  fmt.Sprintf("%.2f Gi", float64(usedMemory.Value())/(1024*1024*1024)),
		}
		nodeInfoList = append(nodeInfoList, info)
	}
	sort.Slice(nodeInfoList, func(i, j int) bool { return nodeInfoList[i].Name < nodeInfoList[j].Name })
	return nodeInfoList
}

func getNodeUsage(nodeName string, nodeMetrics *metricsv1beta1.NodeMetricsList) (*resource.Quantity, *resource.Quantity) {
	if nodeMetrics == nil {
		return resource.NewQuantity(0, resource.DecimalSI), resource.NewQuantity(0, resource.BinarySI)
	}
	for _, m := range nodeMetrics.Items {
		if m.Name == nodeName {
			return m.Usage.Cpu(), m.Usage.Memory()
		}
	}
	return resource.NewQuantity(0, resource.DecimalSI), resource.NewQuantity(0, resource.BinarySI)
}

func countPodsOnNode(nodeName string, pods *v1.PodList) int {
	if pods == nil {
		return 0
	}
	count := 0
	for _, pod := range pods.Items {
		if pod.Spec.NodeName == nodeName && pod.Status.Phase == v1.PodRunning {
			count++
		}
	}
	return count
}

func processPodInfo(podMetrics *metricsv1beta1.PodMetricsList, userNamespaces map[string]bool) []PodInfo {
	podInfoList := []PodInfo{}
	if podMetrics == nil {
		return podInfoList
	}
	for _, podMetric := range podMetrics.Items {
		if !userNamespaces[podMetric.Namespace] {
			continue
		}
		totalCPU := resource.NewQuantity(0, resource.DecimalSI)
		totalMemory := resource.NewQuantity(0, resource.BinarySI)
		for _, container := range podMetric.Containers {
			totalCPU.Add(*container.Usage.Cpu())
			totalMemory.Add(*container.Usage.Memory())
		}
		info := PodInfo{
			Name:            podMetric.Name,
			Namespace:       podMetric.Namespace,
			UsedCPUMilli:    totalCPU.MilliValue(),
			UsedCPU:         fmt.Sprintf("%d m", totalCPU.MilliValue()),
			UsedMemoryBytes: totalMemory.Value(),
			UsedMemory:      fmt.Sprintf("%.2f Mi", float64(totalMemory.Value())/(1024*1024)),
		}
		podInfoList = append(podInfoList, info)
	}
	return podInfoList
}
