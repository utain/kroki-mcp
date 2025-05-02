# Kroki-MCP Dockerfile

FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY . .

RUN go mod download
RUN go build -o kroki-mcp ./cmd/kroki-mcp

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/kroki-mcp /usr/local/bin/kroki-mcp

ENTRYPOINT ["kroki-mcp"]
