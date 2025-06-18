# Estágio 1: Builder - Compila a aplicação Go
FROM golang:1.24-alpine AS builder

# Define o diretório de trabalho
WORKDIR /src

# Copia todo o contexto do projeto de uma só vez.
# Isso garante que o compilador Go veja a estrutura completa do módulo.
COPY . .

# Garante que as dependências estão consistentes.
RUN go mod tidy

# Compila a aplicação. O Go irá localizar o módulo 'kubeowl' no WORKDIR
# e resolver os pacotes 'internal' corretamente.
RUN CGO_ENABLED=0 go build -o /app/kubeowl ./cmd/kubeowl

# ---

# Estágio 2: Final - Cria a imagem de produção enxuta
FROM alpine:latest

# Define o diretório raiz para a aplicação
WORKDIR /root/

# Copia apenas o binário compilado do estágio 'builder'
COPY --from=builder /app/kubeowl .

# Copia os arquivos estáticos do frontend (html, js) do estágio 'builder'
COPY --from=builder /src/web/static ./web/static

# Expõe a porta que a aplicação vai usar
EXPOSE 8080

# Define o comando para executar a aplicação quando o container iniciar
CMD ["./kubeowl"]
