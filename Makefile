# Makefile para o projeto KubeOwl

# --- Variáveis ---
# Define o nome da imagem Docker para consistência.
IMAGE_NAME := kubeowl:latest
# Define o caminho para o ponto de entrada da aplicação.
APP_ENTRYPOINT := ./cmd/kubeowl/main.go

# --- Alvos Principais ---

.PHONY: help build run test run-dev clean

# O alvo padrão, executado quando você digita apenas 'make'.
default: help

# Mostra uma ajuda com os comandos disponíveis.
help:
	@echo "Comandos disponíveis:"
	@echo "  make build      -> Constrói a imagem Docker da aplicação."
	@echo "  make run        -> Executa a aplicação em um container Docker."
	@echo "  make run-dev    -> Executa a aplicação localmente para desenvolvimento (sem Docker)."
	@echo "  make test       -> Roda todos os testes do projeto com detalhes."
	@echo "  make clean      -> Para e remove qualquer container Docker 'kubeowl' em execução."
	@echo "  make help       -> Mostra esta mensagem de ajuda."

# Constrói a imagem Docker.
build:
	@echo "-> Construindo a imagem Docker: $(IMAGE_NAME)..."
	@docker build -t $(IMAGE_NAME) .
	@echo "-> Imagem construída com sucesso!"

# Executa a aplicação dentro de um container Docker.
run:
	@echo "-> Executando o container Docker..."
	@docker run --rm --name kubeowl --network="host" -v ~/.kube:/root/.kube:ro $(IMAGE_NAME)

# Roda os testes da aplicação.
test:
	@echo "-> Rodando testes..."
	@go test -v -cover ./...

# Executa a aplicação localmente para desenvolvimento, sem usar o Docker.
run-dev:
	@echo "-> Executando em modo de desenvolvimento local..."
	@go run $(APP_ENTRYPOINT)

# Limpa o ambiente, parando e removendo o container se estiver em execução.
clean:
	@echo "-> Limpando o ambiente..."
	@docker stop kubeowl || true
	@docker rm kubeowl || true

