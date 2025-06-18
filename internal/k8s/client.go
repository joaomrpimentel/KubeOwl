package k8s

import (
	"log"
	"os"
	"path/filepath"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

var (
	// Clientset permite interagir com os recursos principais do Kubernetes
	Clientset *kubernetes.Clientset
	// MetricsClientset permite buscar métricas de uso de recursos
	MetricsClientset *metricsclientset.Clientset
)

// InitClient inicializa a conexão com o cluster Kubernetes.
// Ele tenta a configuração in-cluster primeiro, e como fallback, usa o kubeconfig local.
func InitClient() error {
	var config *rest.Config
	var err error

	// Tenta usar a configuração de dentro do cluster
	if config, err = rest.InClusterConfig(); err != nil {
		log.Println("Não está em um cluster. Usando o kubeconfig local.")
		homeDir, errHome := os.UserHomeDir()
		if errHome != nil {
			return fmt.Errorf("falha ao encontrar o diretório home: %w", errHome)
		}
		kubeconfig := filepath.Join(homeDir, ".kube", "config")
		// Usa o kubeconfig local como fallback
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return fmt.Errorf("falha ao carregar kubeconfig: %w", err)
		}
	} else {
		log.Println("Rodando dentro do cluster.")
	}

	// Cria o clientset principal
	Clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("falha ao criar clientset do Kubernetes: %w", err)
	}

	// Cria o clientset para métricas
	MetricsClientset, err = metricsclientset.NewForConfig(config)
	if err != nil {
		// Isso não é um erro fatal, apenas um aviso
		return fmt.Errorf("aviso: Falha ao criar clientset de métricas, dados de uso não estarão disponíveis: %w", err)
	}

	return nil
}
