package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/utain/kroki-mcp/internal/model"
	"github.com/utain/kroki-mcp/internal/svgconv"
)

// DPI bounds for generate_png_diagram_with_custom_dpi. minDPI is not merely a
// stylistic floor: below roughly 72 the rasterizer panics with "raster size is
// zero, increase resolution" on small diagrams. Kept in one place so the JSON
// schema, the validation guard and the error message cannot drift apart.
const (
	defaultDPI = 150
	minDPI     = 72
	maxDPI     = 300
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
		diagramType := req.GetString("diagramType", "")
		if !slices.Contains(model.SupportedDiagramTypes, strings.ToLower(diagramType)) {
			slog.Error("Invalid diagramType value", "diagramType", diagramType)
			return mcp.NewToolResultError("diagramType is required and must be a non-empty string"), nil
		}

		source := req.GetString("source", "")
		if source == "" {
			slog.Error("Invalid source value", "source", source)
			return mcp.NewToolResultError("source is required and must be a non-empty string"), nil
		}

		format := req.GetString("format", "")
		if !slices.Contains(model.SupportedOutputFormats, strings.ToLower(format)) {
			slog.Error("Invalid format value", "format", format)
			return mcp.NewToolResultError("format is required and must be one of: png, svg, txt, utxt"), nil
		}

		result, err := s.krokiClient.RenderDiagram(diagramType, source, model.OutputFormat(format))
		if err != nil {
			slog.Error("Failed to render diagram", "error", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		switch model.OutputFormat(strings.ToLower(format)) {
		case model.PNG:
			data := base64.StdEncoding.EncodeToString(result.ImageContent)
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.ImageContent{
						Type:     "image",
						MIMEType: result.MIMEType,
						Data:     data,
					},
				},
			}, nil
		case model.SVG:
			minifiedSVG, err := svgconv.MinifySVG(string(result.ImageContent))
			if err != nil {
				minifiedSVG = string(result.ImageContent)
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.ImageContent{
						Type:     "image",
						Data:     base64.StdEncoding.EncodeToString([]byte(minifiedSVG)),
						MIMEType: result.MIMEType,
					},
				},
			}, nil
		default:
			return mcp.NewToolResultError(fmt.Sprintf("Unsupported format: %s", format)), nil
		}
	})
}

func (s *KrokiMCPServer) RegisterGetDiagramURLTool() {
	tool := mcp.NewTool("get_diagram_url",
		mcp.WithDescription("Get a URL for a diagram image from textual code using Kroki."),
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
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           "Generate diagram URL from source",
			ReadOnlyHint:    mcp.ToBoolPtr(true),
			DestructiveHint: mcp.ToBoolPtr(false),
			IdempotentHint:  mcp.ToBoolPtr(false),
			OpenWorldHint:   mcp.ToBoolPtr(true),
		}),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		diagramType := req.GetString("diagramType", "")
		if !slices.Contains(model.SupportedDiagramTypes, strings.ToLower(diagramType)) {
			slog.Error("Invalid diagramType value", "diagramType", diagramType)
			return mcp.NewToolResultError("diagramType is required and must be a non-empty string"), nil
		}

		source := req.GetString("source", "")
		if source == "" {
			slog.Error("Invalid source value", "source", source)
			return mcp.NewToolResultError("source is required and must be a non-empty string"), nil
		}

		format := req.GetString("format", "")
		if !slices.Contains(model.SupportedOutputFormats, strings.ToLower(format)) {
			slog.Error("Invalid format value", "format", format)
			return mcp.NewToolResultError("format is required and must be one of: png, svg, txt, utxt"), nil
		}

		rawURL, err := s.krokiClient.GetDiagramURL(diagramType, source, model.OutputFormat(format))
		if err != nil {
			slog.Error("Failed to get diagram URL", "error", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: string(rawURL),
				},
			},
		}, nil
	})
}

func (s *KrokiMCPServer) RegisterGeneratePNGDiagramWithCustomDPITool() {
	tool := mcp.NewTool("generate_png_diagram_with_custom_dpi",
		mcp.WithDescription("Generate a high-quality diagram (recommended: 150dpi for Claude Desktop) PNG image from textual code using Kroki."),
		mcp.WithString("diagramType",
			mcp.Required(),
			mcp.Description("The diagram code syntax type (e.g., plantuml, mermaid, graphviz)"),
			mcp.Enum(model.SupportedDiagramTypes...),
		),
		mcp.WithString("source",
			mcp.Required(),
			mcp.Description("The textual diagram source code"),
		),
		mcp.WithNumber("dpi",
			mcp.Description(fmt.Sprintf("Output dots per inch (DPI) for the PNG image from %d to %d", minDPI, maxDPI)),
			mcp.DefaultNumber(defaultDPI),
		),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		diagramType := req.GetString("diagramType", "")
		if !slices.Contains(model.SupportedDiagramTypes, strings.ToLower(diagramType)) {
			slog.Error("Invalid diagramType value", "diagramType", diagramType)
			return mcp.NewToolResultError("diagramType is required and must be a non-empty string"), nil
		}

		source := req.GetString("source", "")
		if source == "" {
			slog.Error("Invalid source value", "source", source)
			return mcp.NewToolResultError("source is required and must be a non-empty string"), nil
		}

		// GetFloat would silently fall back to the default for a wrong-typed
		// argument, so only default when dpi is genuinely absent and report a
		// malformed value instead of discarding it.
		dpi := float64(defaultDPI)
		if raw, ok := req.GetArguments()["dpi"]; ok && raw != nil {
			value, err := req.RequireFloat("dpi")
			if err != nil {
				slog.Error("Invalid DPI value", "dpi", raw, "error", err)
				return mcp.NewToolResultError(fmt.Sprintf("dpi must be a number between %d and %d", minDPI, maxDPI)), nil
			}
			dpi = value
		}
		if dpi < minDPI || dpi > maxDPI {
			slog.Error("Invalid DPI value", "dpi", dpi)
			return mcp.NewToolResultError(fmt.Sprintf("DPI must be between %d and %d", minDPI, maxDPI)), nil
		}

		result, err := s.krokiClient.RenderDiagram(diagramType, source, model.OutputFormat(model.SVG))
		if err != nil {
			slog.Error("Failed to render high-quality diagram", "error", err)
			return mcp.NewToolResultError(err.Error()), nil
		}
		buf := &bytes.Buffer{}
		err = svgconv.Convert(buf, string(result.ImageContent), svgconv.Options{
			Format: svgconv.PNG,
			DPI:    dpi,
		})
		if err != nil {
			slog.Error("Failed to convert SVG to PNG", "error", err)
			return mcp.NewToolResultError(err.Error()), nil
		}
		data := base64.StdEncoding.EncodeToString(buf.Bytes())
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.ImageContent{
					Type:     "image",
					MIMEType: model.PNG.MIMEType(),
					Data:     data,
				},
			},
		}, nil
	})
}
