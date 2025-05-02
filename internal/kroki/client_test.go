package kroki

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEncodeDiagram(t *testing.T) {
	input := "A -> B: test"
	encoded, err := encodeDiagram(input)
	if err != nil {
		t.Fatalf("encodeDiagram error: %v", err)
	}
	if encoded == "" {
		t.Error("encodeDiagram returned empty string")
	}
	// Should not contain +, /, or =
	if strings.ContainsAny(encoded, "+/=") {
		t.Errorf("encodeDiagram produced non-url-safe base64: %s", encoded)
	}
}

func TestRenderDiagram_URLOnly(t *testing.T) {
	client := NewKrokiClient("https://kroki.io", "svg")
	diagramType := "plantuml"
	diagramSource := "A -> B: test"
	result, err := client.RenderDiagram(diagramType, diagramSource)
	if err != nil {
		t.Fatalf("RenderDiagram error: %v", err)
	}
	if !strings.HasPrefix(result.URL, "https://kroki.io/plantuml/svg/") {
		t.Errorf("unexpected URL: %s", result.URL)
	}
	if !strings.Contains(result.URL, "scale=1.50") {
		t.Errorf("scale param missing or incorrect in URL: %s", result.URL)
	}
}

func TestRenderDiagram_MockServer(t *testing.T) {
	// Mock Kroki server that returns a fixed image
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake-image-bytes"))
	}))
	defer ts.Close()

	client := NewKrokiClient(ts.URL, "svg")
	diagramType := "plantuml"
	diagramSource := "A -> B: test"
	result, err := client.RenderDiagram(diagramType, diagramSource)
	if err != nil {
		t.Fatalf("RenderDiagram error: %v", err)
	}
	if string(result.ImageContent) != "fake-image-bytes" {
		t.Errorf("unexpected image content: %s", string(result.ImageContent))
	}
}
