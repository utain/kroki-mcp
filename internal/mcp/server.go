package mcp

import (
	"github.com/mark3labs/mcp-go/server"
	"github.com/utain/kroki-mcp/internal/config"
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
	mcp *server.MCPServer
	cfg *config.Config
}

func NewKrokiMCPServer(cfg *config.Config) *KrokiMCPServer {
	server := server.NewMCPServer(
		"Kroki-MCP",
		"1.0.0",
	)

	return &KrokiMCPServer{mcp: server, cfg: cfg}
}

func (s *KrokiMCPServer) Handler() *server.MCPServer {
	// Register the diagram types and output formats resources
	s.RegisterDiagramTypesResource()
	s.RegisterOutputFormatsResource()

	// Register the diagram generation tool
	s.RegisterGenerateDiagramTool()
	return s.mcp
}
