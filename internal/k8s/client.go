package k8s

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes" // Fornece a interface kubernetes.Interface
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/metrics/pkg/client/clientset/versioned" // Fornece a interface versioned.Interface
)

var (
	// Clientset permite interagir com os recursos principais do Kubernetes.
	Clientset kubernetes.Interface

	// MetricsClientset permite buscar métricas de uso de recursos.
	MetricsClientset versioned.Interface

	// InClusterConfigFunc é uma variável que armazena a função a ser usada para obter a configuração do cluster.
	InClusterConfigFunc = rest.InClusterConfig
)

// InitClient inicializa a conexão com o cluster Kubernetes.
func InitClient() error {
	var config *rest.Config
	var err error

	// Tenta usar a configuração de dentro do cluster através da nossa variável de função.
	if config, err = InClusterConfigFunc(); err != nil {
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

	// Cria o clientset principal.
	Clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("falha ao criar clientset do Kubernetes: %w", err)
	}

	// Cria o clientset para métricas.
	MetricsClientset, err = versioned.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("aviso: Falha ao criar clientset de métricas, dados de uso não estarão disponíveis: %w", err)
	}

	return nil
}
