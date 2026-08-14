package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/utain/kroki-mcp/internal/config"
	"github.com/utain/kroki-mcp/internal/kroki"
)

// stubSVG is a minimal but valid SVG document. It is what the stub Kroki
// host returns, and it is small enough to keep the SVG -> PNG rasterization
// used by generate_png_diagram_with_custom_dpi fast.
const stubSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><rect width="100" height="100" fill="red"/></svg>`

// krokiRequestBody mirrors the JSON body that kroki.KrokiClient.RenderDiagram
// POSTs to the Kroki host root. RenderDiagram never encodes anything in the
// URL path, so assertions about what the tools forward have to be made
// against this decoded body.
type krokiRequestBody struct {
	DiagramSource string `json:"diagram_source"`
	DiagramType   string `json:"diagram_type"`
	OutputFormat  string `json:"output_format"`
}

// stubKrokiRequest is one request observed by the stub Kroki host.
type stubKrokiRequest struct {
	Method string
	Body   krokiRequestBody
}

// stubKrokiRecorder collects the requests the stub Kroki host received.
type stubKrokiRecorder struct {
	mu       sync.Mutex
	requests []stubKrokiRequest
}

func (r *stubKrokiRecorder) record(req stubKrokiRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, req)
}

// only returns the single recorded request, failing the test if the stub was
// not hit exactly once.
func (r *stubKrokiRecorder) only(t *testing.T) stubKrokiRequest {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.requests) != 1 {
		t.Fatalf("expected exactly 1 request to the stub kroki host, got %d", len(r.requests))
	}
	return r.requests[0]
}

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

// newStubKrokiHost returns a URL for a local httptest server that stands in
// for a real Kroki backend: it answers every request with stubSVG and records
// the method and decoded JSON body so tests can assert on exactly what the
// tool handlers forwarded.
func newStubKrokiHost(t *testing.T) (string, *stubKrokiRecorder) {
	t.Helper()
	return newStubKrokiHostServing(t, stubSVG)
}

// newStubKrokiHostServing is newStubKrokiHost with a caller-chosen SVG body,
// for tests that need realistic Kroki output rather than the minimal stubSVG.
func newStubKrokiHostServing(t *testing.T, svgBody string) (string, *stubKrokiRecorder) {
	t.Helper()
	recorder := &stubKrokiRecorder{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body krokiRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode kroki request body: %v", err)
		}
		recorder.record(stubKrokiRequest{Method: r.Method, Body: body})

		w.Header().Set("Content-Type", "image/svg+xml")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(svgBody)); err != nil {
			t.Errorf("failed to write stub SVG response: %v", err)
		}
	}))
	t.Cleanup(ts.Close)
	return ts.URL, recorder
}

// newTestServerWithHost builds a *server.MCPServer wired to a KrokiClient
// pointed at host, mirroring how cmd/kroki-mcp/main.go wires
// NewKrokiMCPServer(cfg, krokiClient) in production. The kroki.NewKrokiClient
// argument is what actually routes traffic; cfg.KrokiHost is only carried
// along by KrokiMCPServer (it is never read by the tool handlers), so it is
// set here purely to mirror production wiring.
func newTestServerWithHost(t *testing.T, host string) *server.MCPServer {
	t.Helper()
	cfg := &config.Config{KrokiHost: host}
	krokiClient := kroki.NewKrokiClient(host)
	s := NewKrokiMCPServer(cfg, krokiClient)
	return s.Handler()
}

// newTestServer builds a *server.MCPServer wired to a KrokiClient pointed at
// a network-guarded host. It returns both the underlying MCP server and the
// host string so callers can assert on URLs returned by pure-encoding tools
// such as get_diagram_url.
func newTestServer(t *testing.T) (*server.MCPServer, string) {
	t.Helper()
	host := newGuardedKrokiHost(t)
	return newTestServerWithHost(t, host), host
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

// firstImageContent extracts the first content item of a CallToolResult as
// mcp.ImageContent together with its base64-decoded payload, failing the test
// if it is missing, of the wrong type, or not valid base64.
func firstImageContent(t *testing.T, result *mcp.CallToolResult) (mcp.ImageContent, []byte) {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatalf("expected at least 1 content item, got 0")
	}
	image, ok := result.Content[0].(mcp.ImageContent)
	if !ok {
		t.Fatalf("expected content[0] to be mcp.ImageContent, got %T", result.Content[0])
	}
	data, err := base64.StdEncoding.DecodeString(image.Data)
	if err != nil {
		t.Fatalf("failed to base64-decode image content: %v", err)
	}
	return image, data
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
			wantMsg: "format is required and must be one of: png, svg",
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
// generate_png_diagram_with_custom_dpi: dpi out of the allowed [72, 300]
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

// 7. generate_diagram happy path against a stub Kroki host. Besides asserting
// a non-error SVG text result, this locks the lowercase-normalization fix end
// to end: mixed-case diagramType/format arguments must reach Kroki lowercased.
func TestCallTool_GenerateDiagram_MixedCaseArgsForwardedLowercase(t *testing.T) {
	host, recorder := newStubKrokiHost(t)
	mcpServer := newTestServerWithHost(t, host)
	c, _ := newInitializedClient(t, mcpServer)

	req := mcp.CallToolRequest{}
	req.Params.Name = "generate_diagram"
	req.Params.Arguments = map[string]any{
		"diagramType": "Mermaid",
		"source":      "graph TD; A-->B;",
		"format":      "SVG",
	}

	result, err := c.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", firstTextContent(t, result))
	}

	svg := firstTextContent(t, result)
	if !strings.Contains(svg, "<svg") {
		t.Errorf("text content does not look like SVG: %q", svg)
	}

	got := recorder.only(t)
	if got.Method != http.MethodPost {
		t.Errorf("method = %q, want %q", got.Method, http.MethodPost)
	}
	if got.Body.DiagramType != "mermaid" {
		t.Errorf("forwarded diagram_type = %q, want %q", got.Body.DiagramType, "mermaid")
	}
	if got.Body.OutputFormat != "svg" {
		t.Errorf("forwarded output_format = %q, want %q", got.Body.OutputFormat, "svg")
	}
	if got.Body.DiagramSource != "graph TD; A-->B;" {
		t.Errorf("forwarded diagram_source = %q, want %q", got.Body.DiagramSource, "graph TD; A-->B;")
	}
}

// 8. get_diagram_url with mixed-case arguments must emit a lowercased
// /<diagramType>/<format>/<encoded> path. This is pure local encoding, so the
// network-guarded host is used.
func TestCallTool_GetDiagramURL_MixedCaseArgsLowercasedInPath(t *testing.T) {
	mcpServer, host := newTestServer(t)
	c, _ := newInitializedClient(t, mcpServer)

	req := mcp.CallToolRequest{}
	req.Params.Name = "get_diagram_url"
	req.Params.Arguments = map[string]any{
		"diagramType": "Mermaid",
		"source":      "graph TD; A-->B;",
		"format":      "SVG",
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
	if !strings.Contains(text, "/mermaid/svg/") {
		t.Errorf("URL %q does not contain lowercased path segment %q", text, "/mermaid/svg/")
	}
}

// 9. generate_png_diagram_with_custom_dpi with dpi omitted must fall back to
// the declared default (150) and succeed, returning real PNG bytes.
func TestCallTool_GeneratePNGDiagramWithCustomDPI_DefaultDPI(t *testing.T) {
	host, recorder := newStubKrokiHost(t)
	mcpServer := newTestServerWithHost(t, host)
	c, _ := newInitializedClient(t, mcpServer)

	req := mcp.CallToolRequest{}
	req.Params.Name = "generate_png_diagram_with_custom_dpi"
	req.Params.Arguments = map[string]any{
		"diagramType": "mermaid",
		"source":      "graph TD; A-->B;",
	}

	result, err := c.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", firstTextContent(t, result))
	}

	image, data := firstImageContent(t, result)
	if image.MIMEType != "image/png" {
		t.Errorf("MIMEType = %q, want %q", image.MIMEType, "image/png")
	}
	if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		t.Errorf("decoded content is not a PNG (magic bytes = % x)", data[:min(8, len(data))])
	}

	// The tool always asks Kroki for SVG and rasterizes locally.
	got := recorder.only(t)
	if got.Body.OutputFormat != "svg" {
		t.Errorf("forwarded output_format = %q, want %q", got.Body.OutputFormat, "svg")
	}
}

// 10. dpi range boundaries: 71 is rejected before any network call, 72 is
// accepted and rasterizes successfully against the stub host.
func TestCallTool_GeneratePNGDiagramWithCustomDPI_DPIBoundaries(t *testing.T) {
	t.Run("below minimum", func(t *testing.T) {
		mcpServer, _ := newTestServer(t)
		c, _ := newInitializedClient(t, mcpServer)

		req := mcp.CallToolRequest{}
		req.Params.Name = "generate_png_diagram_with_custom_dpi"
		req.Params.Arguments = map[string]any{
			"diagramType": "mermaid",
			"source":      "graph TD; A-->B;",
			"dpi":         71,
		}

		result, err := c.CallTool(context.Background(), req)
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if !result.IsError {
			t.Fatalf("expected IsError result, got success")
		}
		want := "DPI must be between 72 and 300"
		if got := firstTextContent(t, result); got != want {
			t.Errorf("error message = %q, want %q", got, want)
		}
	})

	t.Run("at minimum", func(t *testing.T) {
		host, _ := newStubKrokiHost(t)
		mcpServer := newTestServerWithHost(t, host)
		c, _ := newInitializedClient(t, mcpServer)

		req := mcp.CallToolRequest{}
		req.Params.Name = "generate_png_diagram_with_custom_dpi"
		req.Params.Arguments = map[string]any{
			"diagramType": "mermaid",
			"source":      "graph TD; A-->B;",
			"dpi":         72,
		}

		result, err := c.CallTool(context.Background(), req)
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected error result: %s", firstTextContent(t, result))
		}
		if _, data := firstImageContent(t, result); !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
			t.Errorf("decoded content is not a PNG (magic bytes = % x)", data[:min(8, len(data))])
		}
	})
}

// 11. A wrong-typed dpi argument must be rejected instead of silently falling
// back to the default. Numeric strings are intentionally not covered here:
// mcp-go v0.58.0 RequireFloat coerces them via strconv.ParseFloat, which is
// accepted behavior.
func TestCallTool_GeneratePNGDiagramWithCustomDPI_DPIWrongType(t *testing.T) {
	mcpServer, _ := newTestServer(t)
	c, _ := newInitializedClient(t, mcpServer)

	tests := []struct {
		name string
		dpi  any
	}{
		{name: "bool", dpi: true},
		{name: "object", dpi: map[string]any{}},
		{name: "non-numeric string", dpi: "high"},
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
				t.Fatalf("expected IsError result, got success")
			}
			want := "dpi must be a number"
			if got := firstTextContent(t, result); got != want {
				t.Errorf("error message = %q, want %q", got, want)
			}
		})
	}
}

// 12. generate_diagram with format=svg must return SVG normalized for inline
// chat rendering: no hardcoded background on the root element (it breaks dark
// mode), width="100%" plus a viewBox instead of fixed pixel sizing, and no
// preserveAspectRatio="none". The stub serves a root tag copied from real
// PlantUML output via Kroki, which has all three problems.
func TestCallTool_GenerateDiagram_SVGNormalizedForInline(t *testing.T) {
	const krokiStyledSVG = `<svg xmlns="http://www.w3.org/2000/svg" data-diagram-type="SEQUENCE" height="210" preserveAspectRatio="none" style="width:198px;height:210px;background:#FFFFFF;" viewBox="0 0 198 210" width="198" zoomAndPan="magnify"><g><rect fill="#E2E2F0" height="30" width="50" x="10" y="10"/><text fill="#181818" x="12" y="25">Alice</text></g></svg>`

	host, _ := newStubKrokiHostServing(t, krokiStyledSVG)
	mcpServer := newTestServerWithHost(t, host)
	c, _ := newInitializedClient(t, mcpServer)

	req := mcp.CallToolRequest{}
	req.Params.Name = "generate_diagram"
	req.Params.Arguments = map[string]any{
		"diagramType": "plantuml",
		"source":      "@startuml\nAlice -> Bob: hi\n@enduml",
		"format":      "svg",
	}

	result, err := c.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", firstTextContent(t, result))
	}

	svg := firstTextContent(t, result)
	tagStart := strings.Index(svg, "<svg")
	if tagStart < 0 {
		t.Fatalf("result does not look like SVG: %q", svg)
	}
	tagLen := strings.Index(svg[tagStart:], ">")
	if tagLen < 0 {
		t.Fatalf("unterminated root tag: %q", svg)
	}
	// The mechanism under test rewrites the root tag only, so the assertions
	// target it: body content may legitimately contain e.g. the word
	// "background", and a body-painted background would not be caught here.
	root := svg[tagStart : tagStart+tagLen+1]
	if strings.Contains(root, "background") {
		t.Errorf("root tag still has a hardcoded background: %q", root)
	}
	if !strings.Contains(root, `width="100%"`) {
		t.Errorf("root tag missing width=\"100%%\": %q", root)
	}
	if !strings.Contains(root, `viewBox="0 0 198 210"`) {
		t.Errorf("root tag missing its viewBox: %q", root)
	}
	if strings.Contains(root, `preserveAspectRatio="none"`) {
		t.Errorf("root tag still disables proportional scaling: %q", root)
	}
	// Shape-level colors pass through untouched.
	if !strings.Contains(svg, "#E2E2F0") || !strings.Contains(svg, "#181818") {
		t.Errorf("shape-level colors were modified: %q", svg)
	}
}
