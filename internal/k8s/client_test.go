package k8s

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/client-go/rest"
)

// TestMain é executado antes de todos os outros testes neste pacote.
// Usamos isso para silenciar a saída do logger durante a execução dos testes.
func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// TestInitClient_InCluster simula o caminho de inicialização dentro de um cluster.
func TestInitClient_InCluster(t *testing.T) {
	// Guarda a função original para restaurá-la após o teste.
	originalInClusterConfig := InClusterConfigFunc
	defer func() { InClusterConfigFunc = originalInClusterConfig }()

	// Simula a função InClusterConfig para retornar uma configuração que vai falhar na criação do cliente.
	// Apontar para um arquivo de certificado que não existe é uma forma garantida de causar um erro.
	InClusterConfigFunc = func() (*rest.Config, error) {
		return &rest.Config{
			Host: "https://localhost:8080", // Host válido para evitar outros erros
			TLSClientConfig: rest.TLSClientConfig{
				CertFile: "/path/to/non/existent/cert.pem",
			},
		}, nil
	}

	// O erro esperado é sobre a falha ao tentar carregar o certificado,
	// o que prova que o caminho "in-cluster" foi seguido e o erro foi propagado corretamente.
	err := InitClient()
	if err == nil {
		t.Error("Esperado um erro ao inicializar com uma config in-cluster inválida, mas não ocorreu")
	}
}

// TestInitClient_LocalKubeconfig simula a inicialização bem-sucedida usando um kubeconfig local.
func TestInitClient_LocalKubeconfig(t *testing.T) {
	// Guarda a função original para restaurá-la após o teste.
	originalInClusterConfig := InClusterConfigFunc
	defer func() { InClusterConfigFunc = originalInClusterConfig }()

	// Força a falha da configuração in-cluster para testar o caminho de fallback.
	InClusterConfigFunc = func() (*rest.Config, error) {
		return nil, fmt.Errorf("forçado: não está em um cluster")
	}

	// Cria um arquivo kubeconfig falso para o teste.
	dir := t.TempDir() // Cria um diretório temporário que é limpo após o teste.
	kubeconfigFile := filepath.Join(dir, "config")
	// Conteúdo mínimo para um kubeconfig ser considerado válido pelo parser.
	kubeconfigData := []byte(`
apiVersion: v1
clusters:
- cluster:
    server: http://localhost:8080
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
kind: Config
preferences: {}
users:
- name: test-user
  user: {}
`)
	if err := os.WriteFile(kubeconfigFile, kubeconfigData, 0644); err != nil {
		t.Fatalf("Falha ao criar arquivo kubeconfig falso: %v", err)
	}

	// Define a variável de ambiente KUBECONFIG para apontar para o nosso arquivo falso.
	// O client-go usará isso em vez do padrão ~/.kube/config.
	t.Setenv("KUBECONFIG", kubeconfigFile)

	// A inicialização deve funcionar sem erros, pois agora encontra um kubeconfig válido.
	err := InitClient()
	if err != nil {
		t.Errorf("InitClient retornou um erro inesperado ao usar kubeconfig local: %v", err)
	}
}

// TestInitClient_NoConfigAvailable simula o caso onde nenhuma configuração está disponível.
func TestInitClient_NoConfigAvailable(t *testing.T) {
	// Guarda a função original.
	originalInClusterConfig := InClusterConfigFunc
	defer func() { InClusterConfigFunc = originalInClusterConfig }()

	// Força a falha da configuração in-cluster.
	InClusterConfigFunc = func() (*rest.Config, error) {
		return nil, fmt.Errorf("forçado: não está em um cluster")
	}

	// Garante que nenhum KUBECONFIG esteja definido e aponta o diretório home para um local vazio.
	t.Setenv("KUBECONFIG", "")
	t.Setenv("HOME", t.TempDir()) // Evita encontrar o kubeconfig real do usuário.

	err := InitClient()
	if err == nil {
		t.Error("Esperado um erro quando nenhuma configuração do kubernetes está disponível, mas não ocorreu")
	}
}