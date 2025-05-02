package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/utain/kroki-mcp/internal/kroki"
	"github.com/utain/kroki-mcp/internal/model"
)

func (s *KrokiMCPServer) RegisterGenerateDiagramTool() {
	tool := mcp.NewTool("generate_diagram",
		mcp.WithDescription("Generate a diagram image and URL from textual code using Kroki."),
		mcp.WithString("diagramType",
			mcp.Required(),
			mcp.Description("The diagram code syntax type (e.g., plantuml, mermaid, graphviz)"),
			mcp.Enum(model.SupportedDiagramTypes...),
		),
		mcp.WithString("source",
			mcp.Required(),
			mcp.Description("The textual diagram source code"),
		),
		mcp.WithString("format",
			mcp.Required(),
			mcp.Description("Output media format: png, svg, text etc."),
			mcp.Enum(model.SupportedOutputFormats...),
			mcp.DefaultString("png"),
		),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		diagramType, _ := req.Params.Arguments["diagramType"].(string)
		if !slices.Contains(model.SupportedDiagramTypes, strings.ToLower(diagramType)) {
			slog.Error("Invalid diagramType value", "diagramType", diagramType)
			return mcp.NewToolResultError("diagramType is required and must be a non-empty string"), nil
		}

		source, ok := req.Params.Arguments["source"].(string)
		if !ok || source == "" {
			slog.Error("Invalid source value", "source", source)
			return mcp.NewToolResultError("source is required and must be a non-empty string"), nil
		}

		format, _ := req.Params.Arguments["format"].(string)
		if !slices.Contains(model.SupportedOutputFormats, strings.ToLower(format)) {
			slog.Error("Invalid format value", "format", format)
			return mcp.NewToolResultError("format is required and must be one of: png, svg, txt, utxt"), nil
		}

		client := kroki.NewKrokiClient(s.cfg.KrokiHost, model.OutputFormat(format))
		result, err := client.RenderDiagram(diagramType, source)
		if err != nil {
			slog.Error("Failed to render diagram", "error", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		b, err := json.Marshal(result)
		if err != nil {
			slog.Error("Failed to marshal result", "error", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		switch model.OutputFormat(strings.ToLower(format)) {
		case model.PNG, model.SVG:
			data := base64.StdEncoding.EncodeToString(result.ImageContent)
			return mcp.NewToolResultImage(string(b), data, result.MIMEType), nil

		case model.TXT, model.UTXT:
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: result.URL,
					},
					mcp.TextContent{
						Type: "text",
						Text: string(result.ImageContent),
					},
				},
			}, nil
		default:
			return mcp.NewToolResultError(fmt.Sprintf("Unsupported format: %s", format)), nil
		}
	})
}
