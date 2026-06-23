# Usa uma imagem oficial do Go para compilar
FROM golang:1.26-alpine AS builder

# Instala dependências de compilação
RUN apk add --no-cache git build-base

# Define o diretório de trabalho
WORKDIR /app

# Copia os arquivos de dependência e baixa os pacotes
COPY go.mod go.sum* ./
RUN go mod download

# Copia todo o código fonte
COPY . .

# Compila o executável apontando para a nova pasta cmd/bot/
RUN CGO_ENABLED=0 GOOS=linux go build -o bot-chamados ./cmd/bot/

# ==========================================
# IMAGEM FINAL (PRODUÇÃO)
# ==========================================
FROM alpine:latest

# Define o fuso horário correto para o bot (Horário de Brasília)
ENV TZ=America/Sao_Paulo

# Instala certificados e configura o fuso horário no sistema
RUN apk add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/$TZ /etc/localtime && \
    echo $TZ > /etc/timezone

WORKDIR /app/

# Expõe a porta do painel web para a rede do Docker
EXPOSE 33090

# Copia o binário e os arquivos de interface
COPY --from=builder /app/bot-chamados .
COPY --from=builder /app/web ./web
COPY --from=builder /app/static ./static

# Comando para rodar o bot
CMD ["./bot-chamados"]