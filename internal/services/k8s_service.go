package services

import (
	"context"
	"kubeowl/internal/models"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	versioned "k8s.io/metrics/pkg/client/clientset/versioned"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Service define a interface para interagir com o cluster.
type Service interface {
	GetOverviewData(ctx context.Context) (*models.OverviewResponse, error)
	GetNodeInfo(ctx context.Context) ([]models.NodeInfo, error)
	GetPodInfo(ctx context.Context) ([]models.PodInfo, error)
	GetServiceInfo(ctx context.Context) ([]models.ServiceInfo, error)
	GetIngressInfo(ctx context.Context) ([]models.IngressInfo, error)
	GetPvcInfo(ctx context.Context) ([]models.PvcInfo, error)
	GetEventInfo(ctx context.Context) ([]models.EventInfo, error)
}

// k8sService é a implementação concreta da interface Service.
type k8sService struct {
	clientset        kubernetes.Interface
	metricsClientset versioned.Interface
}

// NewK8sService cria uma nova instância do k8sService.
func NewK8sService(clientset kubernetes.Interface, metricsClientset versioned.Interface) Service {
	return &k8sService{
		clientset:        clientset,
		metricsClientset: metricsClientset,
	}
}

// GetOverviewData coleta e processa os dados para a visão geral.
func (s *k8sService) GetOverviewData(ctx context.Context) (*models.OverviewResponse, error) {
	deployments, err := s.clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	namespaces, err := s.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	nodes, err := s.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	nodeMetrics, _ := s.metricsClientset.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})

	userNamespaceCount, _ := processNamespaces(namespaces)
	_, inClusterErr := rest.InClusterConfig()

	response := &models.OverviewResponse{
		IsRunningInCluster: inClusterErr == nil,
		DeploymentCount:    len(deployments.Items),
		NamespaceCount:     userNamespaceCount,
		NodeCount:          len(nodes.Items),
		Capacity:           processClusterCapacity(nodes, nodeMetrics),
	}
	return response, nil
}

// GetNodeInfo coleta e processa informações dos nós.
func (s *k8sService) GetNodeInfo(ctx context.Context) ([]models.NodeInfo, error) {
	nodes, err := s.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	pods, _ := s.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	nodeMetrics, _ := s.metricsClientset.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})

	return processNodeInfo(nodes, pods, nodeMetrics), nil
}

// GetPodInfo coleta e processa informações dos pods.
func (s *k8sService) GetPodInfo(ctx context.Context) ([]models.PodInfo, error) {
	pods, err := s.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	podMetrics, _ := s.metricsClientset.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
	namespaces, err := s.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	_, userNamespaces := processNamespaces(namespaces)
	return processPodInfo(pods, podMetrics, userNamespaces), nil
}

// GetServiceInfo coleta e processa informações dos services.
func (s *k8sService) GetServiceInfo(ctx context.Context) ([]models.ServiceInfo, error) {
	services, err := s.clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	namespaces, err := s.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	_, userNamespaces := processNamespaces(namespaces)
	return processServiceInfo(services, userNamespaces), nil
}

// GetIngressInfo coleta e processa informações dos ingresses.
func (s *k8sService) GetIngressInfo(ctx context.Context) ([]models.IngressInfo, error) {
	ingresses, err := s.clientset.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	namespaces, err := s.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	_, userNamespaces := processNamespaces(namespaces)
	return processIngressInfo(ingresses, userNamespaces), nil
}

// GetPvcInfo coleta e processa informações dos PVCs.
func (s *k8sService) GetPvcInfo(ctx context.Context) ([]models.PvcInfo, error) {
	pvcs, err := s.clientset.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	namespaces, err := s.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	_, userNamespaces := processNamespaces(namespaces)
	return processPvcs(pvcs, userNamespaces), nil
}

// GetEventInfo coleta e processa informações dos eventos.
func (s *k8sService) GetEventInfo(ctx context.Context) ([]models.EventInfo, error) {
	events, err := s.clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	namespaces, err := s.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	_, userNamespaces := processNamespaces(namespaces)
	return processEvents(events, userNamespaces), nil
}
