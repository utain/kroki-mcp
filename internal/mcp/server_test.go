package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/utain/kroki-mcp/internal/config"
	"github.com/utain/kroki-mcp/internal/kroki"
)

// newGuardedKrokiHost returns a URL for a local httptest server that fails
// the test if it is ever hit. It is used as the configured Kroki host for
// tests that must not perform real network I/O: any accidental network call
// made by a handler under test (e.g. a validation bug that lets a request
// fall through to RenderDiagram) will be caught immediately instead of
// silently reaching the real kroki.io.
func newGuardedKrokiHost(t *testing.T) string {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected network call to kroki host: %s %s", r.Method, r.URL.String())
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(ts.Close)
	return ts.URL
}

// newTestServer builds a *server.MCPServer wired to a KrokiClient pointed at
// a network-guarded host, mirroring how cmd/kroki-mcp/main.go wires
// NewKrokiMCPServer(cfg, krokiClient) in production. It returns both the
// underlying MCP server and the host string so callers can assert on URLs
// returned by pure-encoding tools such as get_diagram_url.
func newTestServer(t *testing.T) (*server.MCPServer, string) {
	t.Helper()
	host := newGuardedKrokiHost(t)
	cfg := &config.Config{KrokiHost: host}
	krokiClient := kroki.NewKrokiClient(host)
	s := NewKrokiMCPServer(cfg, krokiClient)
	return s.Handler(), host
}

// newInitializedClient creates an in-process client for mcpServer, starts
// it, and completes the initialize handshake, returning both the client and
// the handshake result so callers can inspect advertised capabilities.
func newInitializedClient(t *testing.T, mcpServer *server.MCPServer) (*client.Client, *mcp.InitializeResult) {
	t.Helper()

	c, err := client.NewInProcessClient(mcpServer)
	if err != nil {
		t.Fatalf("NewInProcessClient: %v", err)
	}
	t.Cleanup(func() {
		_ = c.Close()
	})

	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("client.Start: %v", err)
	}

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}

	result, err := c.Initialize(context.Background(), initRequest)
	if err != nil {
		t.Fatalf("client.Initialize: %v", err)
	}

	return c, result
}

// firstTextContent extracts the text of the first content item of a
// CallToolResult, failing the test if it is missing or not TextContent.
func firstTextContent(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatalf("expected at least 1 content item, got 0")
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected content[0] to be mcp.TextContent, got %T", result.Content[0])
	}
	return text.Text
}

// 1. Initialize handshake succeeds and server capabilities advertise both
// tools and resources.
func TestInitialize_AdvertisesToolsAndResources(t *testing.T) {
	mcpServer, _ := newTestServer(t)
	_, result := newInitializedClient(t, mcpServer)

	if result.ServerInfo.Name != "Kroki MCP Server" {
		t.Errorf("ServerInfo.Name = %q, want %q", result.ServerInfo.Name, "Kroki MCP Server")
	}
	if result.ServerInfo.Version != "2.0.0" {
		t.Errorf("ServerInfo.Version = %q, want %q", result.ServerInfo.Version, "2.0.0")
	}
	if result.Capabilities.Tools == nil {
		t.Error("expected server capabilities to advertise tools, got nil")
	}
	if result.Capabilities.Resources == nil {
		t.Error("expected server capabilities to advertise resources, got nil")
	}
}

// 2. tools/list returns exactly the 3 expected tool names.
func TestListTools_ReturnsExpectedNames(t *testing.T) {
	mcpServer, _ := newTestServer(t)
	c, _ := newInitializedClient(t, mcpServer)

	result, err := c.ListTools(context.Background(), mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	got := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		got = append(got, tool.Name)
	}
	slices.Sort(got)

	want := []string{"generate_diagram", "generate_png_diagram_with_custom_dpi", "get_diagram_url"}
	slices.Sort(want)

	if !slices.Equal(got, want) {
		t.Errorf("tool names = %v, want %v", got, want)
	}
}

// 3. resources/list returns exactly the 3 expected URIs.
func TestListResources_ReturnsExpectedURIs(t *testing.T) {
	mcpServer, _ := newTestServer(t)
	c, _ := newInitializedClient(t, mcpServer)

	result, err := c.ListResources(context.Background(), mcp.ListResourcesRequest{})
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}

	got := make([]string, 0, len(result.Resources))
	for _, resource := range result.Resources {
		got = append(got, resource.URI)
	}
	slices.Sort(got)

	want := []string{"diagrams://types", "diagrams://formats", "diagrams://dpi-list"}
	slices.Sort(want)

	if !slices.Equal(got, want) {
		t.Errorf("resource URIs = %v, want %v", got, want)
	}
}

// 4. tools/call get_diagram_url with valid args returns a non-error result
// whose text content is a URL starting with the configured Kroki host.
// GetDiagramURL is pure local base64/deflate encoding (no network), so this
// is safe to exercise fully even though the host is network-guarded.
func TestCallTool_GetDiagramURL_ValidArgs(t *testing.T) {
	mcpServer, host := newTestServer(t)
	c, _ := newInitializedClient(t, mcpServer)

	req := mcp.CallToolRequest{}
	req.Params.Name = "get_diagram_url"
	req.Params.Arguments = map[string]any{
		"diagramType": "mermaid",
		"source":      "graph TD; A-->B;",
		"format":      "svg",
	}

	result, err := c.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", firstTextContent(t, result))
	}

	text := firstTextContent(t, result)
	if !strings.HasPrefix(text, host) {
		t.Errorf("URL %q does not start with configured host %q", text, host)
	}
}

// 5. Validation error paths for generate_diagram: invalid diagramType, empty
// source, invalid format. Each of these returns before RenderDiagram is
// called, so no network I/O occurs (also enforced by the guarded host).
func TestCallTool_GenerateDiagram_ValidationErrors(t *testing.T) {
	mcpServer, _ := newTestServer(t)
	c, _ := newInitializedClient(t, mcpServer)

	tests := []struct {
		name    string
		args    map[string]any
		wantMsg string
	}{
		{
			name: "invalid diagramType",
			args: map[string]any{
				"diagramType": "not-a-type",
				"source":      "graph TD; A-->B;",
				"format":      "svg",
			},
			wantMsg: "diagramType is required and must be a non-empty string",
		},
		{
			name: "empty source",
			args: map[string]any{
				"diagramType": "mermaid",
				"source":      "",
				"format":      "svg",
			},
			wantMsg: "source is required and must be a non-empty string",
		},
		{
			name: "invalid format",
			args: map[string]any{
				"diagramType": "mermaid",
				"source":      "graph TD; A-->B;",
				"format":      "not-a-format",
			},
			wantMsg: "format is required and must be one of: png, svg, txt, utxt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Name = "generate_diagram"
			req.Params.Arguments = tt.args

			result, err := c.CallTool(context.Background(), req)
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected IsError result, got success: %s", firstTextContent(t, result))
			}

			got := firstTextContent(t, result)
			if got != tt.wantMsg {
				t.Errorf("error message = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

// 5 (continued). Validation error path for
// generate_png_diagram_with_custom_dpi: dpi out of the allowed [1, 300]
// range. This returns before RenderDiagram is called, so no network I/O
// occurs.
func TestCallTool_GeneratePNGDiagramWithCustomDPI_DPIOutOfRange(t *testing.T) {
	mcpServer, _ := newTestServer(t)
	c, _ := newInitializedClient(t, mcpServer)

	req := mcp.CallToolRequest{}
	req.Params.Name = "generate_png_diagram_with_custom_dpi"
	req.Params.Arguments = map[string]any{
		"diagramType": "mermaid",
		"source":      "graph TD; A-->B;",
		"dpi":         500,
	}

	result, err := c.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected IsError result, got success: %s", firstTextContent(t, result))
	}

	want := "DPI must be between 72 and 300"
	if got := firstTextContent(t, result); got != want {
		t.Errorf("error message = %q, want %q", got, want)
	}
}

// 6. resources/read on diagrams://types returns JSON that unmarshals to a
// non-empty string array.
func TestReadResource_DiagramTypes(t *testing.T) {
	mcpServer, _ := newTestServer(t)
	c, _ := newInitializedClient(t, mcpServer)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "diagrams://types"

	result, err := c.ReadResource(context.Background(), req)
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("expected 1 resource content item, got %d", len(result.Contents))
	}

	textResource, ok := result.Contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("expected mcp.TextResourceContents, got %T", result.Contents[0])
	}

	var diagramTypes []string
	if err := json.Unmarshal([]byte(textResource.Text), &diagramTypes); err != nil {
		t.Fatalf("failed to unmarshal diagram types JSON: %v", err)
	}
	if len(diagramTypes) == 0 {
		t.Error("expected non-empty diagram types list")
	}
}
