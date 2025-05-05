package mcp

import (
	"github.com/mark3labs/mcp-go/server"
	"github.com/utain/kroki-mcp/internal/config"
	"github.com/utain/kroki-mcp/internal/kroki"
)

type DiagramRequest struct {
	DiagramType string  `json:"diagramType"`
	Source      string  `json:"source"`
	Format      string  `json:"format"`
	Quality     float64 `json:"quality"`
}

type DiagramResponse struct {
	ImageContent []byte `json:"imageContent"`
	URL          string `json:"url"`
}
type KrokiMCPServer struct {
	mcp         *server.MCPServer
	krokiClient *kroki.KrokiClient
	cfg         *config.Config
}

func NewKrokiMCPServer(cfg *config.Config, krokiClient *kroki.KrokiClient) *KrokiMCPServer {
	server := server.NewMCPServer(
		"Kroki MCP Server",
		"2.0.0",
	)

	return &KrokiMCPServer{mcp: server, cfg: cfg, krokiClient: krokiClient}
}

func (s *KrokiMCPServer) Handler() *server.MCPServer {
	// Register the diagram types and output formats resources
	s.RegisterDiagramTypesResource()
	s.RegisterOutputFormatsResource()
	s.RegisterRecommendedDPIList()

	// Register the diagram generation tool
	s.RegisterGenerateDiagramTool()
	s.RegisterGeneratePNGDiagramWithCustomDPITool()
	s.RegisterGetDiagramURLTool()
	return s.mcp
}
