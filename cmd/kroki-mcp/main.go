package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/pflag"
	"github.com/utain/kroki-mcp/internal/config"
	"github.com/utain/kroki-mcp/internal/kroki"
	"github.com/utain/kroki-mcp/internal/mcp"
)

func main() {
	var cfg config.Config

	pflag.StringVarP(&cfg.ServerHost, "host", "h", "localhost", "Server host")
	pflag.IntVarP(&cfg.ServerPort, "port", "p", 5090, "Server port")
	pflag.StringVarP(&cfg.ServerMode, "mode", "m", "stdio", "Operation mode: sse or stdio (default)")
	pflag.StringVarP(&cfg.OutputFormat, "format", "f", "png", "Output format: png, svg, jpeg, pdf")
	pflag.StringVar(&cfg.KrokiHost, "kroki-host", "https://kroki.io", "Kroki server host URL")
	pflag.StringVar(&cfg.LogLevel, "log-level", "info", "Log level: debug, info, warn, error")
	pflag.StringVar(&cfg.LogFormat, "log-format", "text", "Log format: text or json")

	pflag.Parse()

	logger := config.InitLogger(cfg.LogLevel, cfg.LogFormat)
	logger.Info("Kroki-MCP starting...",
		"mode", cfg.ServerMode,
		"format", cfg.OutputFormat,
		"krokiHost", cfg.KrokiHost,
		"logLevel", cfg.LogLevel,
		"logFormat", cfg.LogFormat,
		"serverHost", cfg.ServerHost,
		"serverPort", cfg.ServerPort,
	)

	krokiClient := kroki.NewKrokiClient(cfg.KrokiHost)
	kroki := mcp.NewKrokiMCPServer(&cfg, krokiClient)
	switch cfg.ServerMode {
	case "stdio":
		logger.Info("STDIO mode: reading diagram type and source from stdin")
		server.ServeStdio(kroki.Handler())
	default:
		logger.Info("SSE mode: starting SSE server")
		sseServer := server.NewSSEServer(kroki.Handler())
		logger.Info("SSE server started successfully", "host", cfg.ServerHost, "port", cfg.ServerPort)
		if err := sseServer.Start(fmt.Sprintf("%s:%d", cfg.ServerHost, cfg.ServerPort)); err != nil {
			logger.Error("Failed to start SSE server", "error", err)
			os.Exit(1)
		}

		os.Exit(0)
	}
}
