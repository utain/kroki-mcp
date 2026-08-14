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
	return newTestServerWithHost(t, newGuardedKrokiHost(t))
}

// stubSVG is a minimal but real SVG with non-zero dimensions, enough for
// canvas.ParseSVG to produce a canvas that rasterizes without error.
const stubSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="60" viewBox="0 0 100 60">` +
	`<rect x="5" y="5" width="90" height="50" fill="none" stroke="black"/></svg>`

// newStubKrokiHost returns a URL for a local httptest server that answers
// every request with stubSVG. It is the counterpart to newGuardedKrokiHost,
// for tests that need to exercise a render path end to end rather than stop
// at validation.
func newStubKrokiHost(t *testing.T) string {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte(stubSVG))
	}))
	t.Cleanup(ts.Close)
	return ts.URL
}

// newTestServerWithHost is newTestServer with an explicit Kroki host.
func newTestServerWithHost(t *testing.T, host string) (*server.MCPServer, string) {
	t.Helper()
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

// 5 (continued). Rejected dpi arguments for
// generate_png_diagram_with_custom_dpi. Each returns before RenderDiagram is
// called, so no network I/O occurs. minDPI is a hard floor rather than a
// stylistic one: below it the rasterizer panics on small diagrams, so the
// dpi=71 case pins the bound that keeps that path unreachable. The
// wrong-typed cases pin that a malformed argument is reported instead of
// being silently replaced by the default.
func TestCallTool_GeneratePNGDiagramWithCustomDPI_RejectedDPI(t *testing.T) {
	mcpServer, _ := newTestServer(t)
	c, _ := newInitializedClient(t, mcpServer)

	tests := []struct {
		name    string
		dpi     any
		wantMsg string
	}{
		{name: "above maxDPI", dpi: 500, wantMsg: "DPI must be between 72 and 300"},
		{name: "below minDPI", dpi: 71, wantMsg: "DPI must be between 72 and 300"},
		{name: "unparsable string", dpi: "high", wantMsg: "dpi must be a number between 72 and 300"},
		{name: "boolean", dpi: true, wantMsg: "dpi must be a number between 72 and 300"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Name = "generate_png_diagram_with_custom_dpi"
			req.Params.Arguments = map[string]any{
				"diagramType": "mermaid",
				"source":      "graph TD; A-->B;",
				"dpi":         tt.dpi,
			}

			result, err := c.CallTool(context.Background(), req)
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected IsError result, got success: %s", firstTextContent(t, result))
			}

			if got := firstTextContent(t, result); got != tt.wantMsg {
				t.Errorf("error message = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

// 5 (continued). An omitted dpi must fall back to the declared default of
// 150 rather than erroring or rendering at some other resolution. Asserted
// by rendering twice against a stub Kroki host — once with dpi omitted, once
// with dpi=150 explicitly — and requiring byte-identical PNG output, which
// pins the effective default to the value the JSON schema advertises.
func TestCallTool_GeneratePNGDiagramWithCustomDPI_OmittedDPIUsesDefault(t *testing.T) {
	mcpServer, _ := newTestServerWithHost(t, newStubKrokiHost(t))
	c, _ := newInitializedClient(t, mcpServer)

	render := func(t *testing.T, args map[string]any) string {
		t.Helper()
		req := mcp.CallToolRequest{}
		req.Params.Name = "generate_png_diagram_with_custom_dpi"
		req.Params.Arguments = args

		result, err := c.CallTool(context.Background(), req)
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %s", firstTextContent(t, result))
		}
		if len(result.Content) == 0 {
			t.Fatalf("expected at least 1 content item, got 0")
		}
		image, ok := result.Content[0].(mcp.ImageContent)
		if !ok {
			t.Fatalf("expected content[0] to be mcp.ImageContent, got %T", result.Content[0])
		}
		if image.Data == "" {
			t.Fatal("expected non-empty PNG data")
		}
		return image.Data
	}

	omitted := render(t, map[string]any{
		"diagramType": "mermaid",
		"source":      "graph TD; A-->B;",
	})
	explicit := render(t, map[string]any{
		"diagramType": "mermaid",
		"source":      "graph TD; A-->B;",
		"dpi":         150,
	})

	if omitted != explicit {
		t.Error("omitted dpi did not render identically to an explicit dpi of 150")
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
