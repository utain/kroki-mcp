package kroki

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/utain/kroki-mcp/internal/model"
)

func TestRenderDiagram_MockServer(t *testing.T) {
	// Mock Kroki server that returns a fixed image
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake-image-bytes"))
	}))
	defer ts.Close()

	client := NewKrokiClient(ts.URL)
	diagramType := "plantuml"
	diagramSource := "A -> B: test"
	result, err := client.RenderDiagram(diagramType, diagramSource, model.OutputFormat("svg"))
	if err != nil {
		t.Fatalf("RenderDiagram error: %v", err)
	}
	if string(result.ImageContent) != "fake-image-bytes" {
		t.Errorf("unexpected image content: %s", string(result.ImageContent))
	}
}
