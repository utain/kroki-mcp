package mcp

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/utain/kroki-mcp/internal/model"
)

func (s *KrokiMCPServer) RegisterDiagramTypesResource() {
	resource := mcp.NewResource(
		"diagrams://types",
		"Supported Diagram Types",
		mcp.WithResourceDescription("List of supported diagram types"),
		mcp.WithMIMEType("application/json"),
	)
	s.mcp.AddResource(resource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		data, err := json.Marshal(model.SupportedDiagramTypes)
		if err != nil {
			return nil, err
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "diagrams://types",
				MIMEType: "application/json",
				Text:     string(data),
			},
		}, nil
	})
}

func (s *KrokiMCPServer) RegisterOutputFormatsResource() {
	resource := mcp.NewResource(
		"diagrams://formats",
		"Supported Output Media Formats",
		mcp.WithResourceDescription("List of supported output formats"),
		mcp.WithMIMEType("application/json"),
	)
	s.mcp.AddResource(resource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		data, err := json.Marshal(model.SupportedOutputFormats)
		if err != nil {
			return nil, err
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "diagrams://formats",
				MIMEType: "application/json",
				Text:     string(data),
			},
		}, nil
	})
}

func (s *KrokiMCPServer) RegisterRecommendedDPIList() {
	resource := mcp.NewResource(
		"diagrams://dpi-list",
		"Recommended DPI list good for generate diagram image",
		mcp.WithResourceDescription("List recommended DPI for generate diagram image"),
		mcp.WithMIMEType("application/json"),
	)
	s.mcp.AddResource(resource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		data, err := json.Marshal(model.RecommendedDPIList)
		if err != nil {
			return nil, err
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "diagrams://dpi-list",
				MIMEType: "application/json",
				Text:     string(data),
			},
		}, nil
	})
}
