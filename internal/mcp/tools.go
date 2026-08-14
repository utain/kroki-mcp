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

// defaultDPI is the single source of truth for the DPI used by
// generate_png_diagram_with_custom_dpi when the caller omits the argument.
// The tool schema advertises the same value via mcp.DefaultNumber.
const defaultDPI = 150

// parseDiagramArgs validates the shared tool arguments and returns them
// normalized to lowercase (except source). A non-nil errResult must be
// returned to the client as-is.
func parseDiagramArgs(req mcp.CallToolRequest, needsFormat bool) (diagramType, source, format string, errResult *mcp.CallToolResult) {
	rawDiagramType := req.GetString("diagramType", "")
	diagramType = strings.ToLower(rawDiagramType)
	if !slices.Contains(model.SupportedDiagramTypes, diagramType) {
		slog.Error("Invalid diagramType value", "diagramType", rawDiagramType)
		return "", "", "", mcp.NewToolResultError("diagramType is required and must be a non-empty string")
	}

	source = req.GetString("source", "")
	if source == "" {
		slog.Error("Invalid source value", "source", source)
		return "", "", "", mcp.NewToolResultError("source is required and must be a non-empty string")
	}

	if needsFormat {
		rawFormat := req.GetString("format", "")
		format = strings.ToLower(rawFormat)
		if !slices.Contains(model.SupportedOutputFormats, format) {
			slog.Error("Invalid format value", "format", rawFormat)
			return "", "", "", mcp.NewToolResultError("format is required and must be one of: png, svg")
		}
	}

	return diagramType, source, format, nil
}

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
			mcp.Description("Output media format: svg, png, text etc."),
			mcp.Enum(model.SupportedOutputFormats...),
			mcp.DefaultString("svg"),
		),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           "Generate diagram image from source",
			ReadOnlyHint:    mcp.ToBoolPtr(true),
			DestructiveHint: mcp.ToBoolPtr(false),
			IdempotentHint:  mcp.ToBoolPtr(true),
			OpenWorldHint:   mcp.ToBoolPtr(true),
		}),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		diagramType, source, format, errResult := parseDiagramArgs(req, true)
		if errResult != nil {
			return errResult, nil
		}

		result, err := s.krokiClient.RenderDiagram(diagramType, source, model.OutputFormat(format))
		if err != nil {
			slog.Error("Failed to render diagram", "error", err)
			return mcp.NewToolResultError(err.Error()), nil
		}

		switch model.OutputFormat(format) {
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
			// Claude Desktop rejects image content blocks with image/svg+xml
			// (only raster formats are supported), so SVG goes back as text,
			// normalized so it renders inline on both light and dark themes.
			svgOut := svgconv.NormalizeForInline(string(result.ImageContent))
			if minified, err := svgconv.MinifySVG(svgOut); err == nil {
				svgOut = minified
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: svgOut,
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
			IdempotentHint:  mcp.ToBoolPtr(true),
			OpenWorldHint:   mcp.ToBoolPtr(true),
		}),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		diagramType, source, format, errResult := parseDiagramArgs(req, true)
		if errResult != nil {
			return errResult, nil
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
			mcp.Description("Output dots per inch (DPI) for the PNG image from 72 to 300"),
			mcp.DefaultNumber(defaultDPI),
		),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           "Generate high-DPI PNG diagram from source",
			ReadOnlyHint:    mcp.ToBoolPtr(true),
			DestructiveHint: mcp.ToBoolPtr(false),
			IdempotentHint:  mcp.ToBoolPtr(true),
			OpenWorldHint:   mcp.ToBoolPtr(true),
		}),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		diagramType, source, _, errResult := parseDiagramArgs(req, false)
		if errResult != nil {
			return errResult, nil
		}

		dpi := float64(defaultDPI)
		if _, present := req.GetArguments()["dpi"]; present {
			var err error
			dpi, err = req.RequireFloat("dpi")
			if err != nil {
				slog.Error("Invalid DPI value", "error", err)
				return mcp.NewToolResultError("dpi must be a number"), nil
			}
		}
		if dpi < 72 || dpi > 300 {
			slog.Error("Invalid DPI value", "dpi", dpi)
			return mcp.NewToolResultError("DPI must be between 72 and 300"), nil
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
