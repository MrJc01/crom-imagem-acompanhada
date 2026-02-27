# Build Stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Necessário para o go-sqlite3 compilar via CGO no Alpine
RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Compilando de forma enxuta com CGO habilitado
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o crom-vision ./cmd/server

# Runtime Stage
FROM alpine:latest

WORKDIR /app

# Instala certificados necessários para Go enviar requests e E-mail TLS
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/crom-vision .
COPY public /app/public

# Cria diretório de database que será montado por volume
RUN mkdir /app/data

ENV DATABASE_URL=/app/data/crom_vision.db?_journal_mode=WAL

EXPOSE 8080

CMD ["./crom-vision"]
