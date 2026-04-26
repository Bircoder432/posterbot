# --- Сборка бинарника ---
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o posterbot

# --- Финальный образ ---
FROM alpine:latest

RUN apk add --no-cache sqlite ca-certificates

WORKDIR /app

COPY --from=builder /app/posterbot /app/posterbot
RUN chmod +x /app/posterbot

# Порт для webhook-сервера
EXPOSE 8080

CMD ["/app/posterbot"]
