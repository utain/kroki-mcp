APP_NAME = kroki-mcp
CMD_PATH = ./cmd/kroki-mcp

.PHONY: all build run test lint docker-build clean mcphost mcp-inspector

all: build

build:
	go build -o ./dist/$(APP_NAME) $(CMD_PATH)

run: build
	./dist/$(APP_NAME)

test:
	go test ./...

lint:
	golangci-lint run || true

docker-build:
	docker build -t $(APP_NAME) .

mcphost:
	go run github.com/mark3labs/mcphost@latest

mcphost-ollama:
	go run github.com/mark3labs/mcphost@latest -m ollama:qwen2.5-coder:14b --config ./config/mcp.json --system-prompt ./config/system_prompt.json

mcp-inspector:
	npx -y @modelcontextprotocol/inspector

clean:
	rm -f $(APP_NAME)
