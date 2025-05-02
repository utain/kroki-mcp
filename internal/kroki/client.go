package kroki

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/utain/kroki-mcp/internal/model"
)

type KrokiClient struct {
	Host string
}

type KrokiResult struct {
	ImageContent []byte `json:"-"` // The image content in base64 format
	MIMEType     string `json:"-"`
}

// krokiRequest represents the request body for the Kroki API.
type krokiRequest struct {
	DiagramSource  string            `json:"diagram_source"`
	DiagramType    string            `json:"diagram_type"`
	OutputFormat   string            `json:"output_format"`
	DiagramOptions map[string]string `json:"diagram_options"`
}

func NewKrokiClient(host string) *KrokiClient {
	return &KrokiClient{
		Host: host,
	}
}

// encodeDiagram compresses and base64-url encodes the diagram source for GET URLs.
func (kc *KrokiClient) encodeDiagram(diagramSource string) (string, error) {
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	_, err := w.Write([]byte(diagramSource))
	if err != nil {
		return "", err
	}
	w.Close()
	encoded := base64.StdEncoding.EncodeToString(b.Bytes())
	// Kroki expects URL-safe base64 (replace + with -, / with _, remove =)
	encoded = strings.ReplaceAll(encoded, "+", "-")
	encoded = strings.ReplaceAll(encoded, "/", "_")
	encoded = strings.TrimRight(encoded, "=")
	return encoded, nil
}

// RenderDiagram sends diagram code to the Kroki server and returns both image base64 and a direct URL.
func (kc *KrokiClient) RenderDiagram(diagramType, diagramSource string, format model.OutputFormat) (*KrokiResult, error) {
	u, err := url.Parse(kc.Host)
	if err != nil {
		slog.Error("Invalid Kroki host URL", "host", kc.Host, "error", err)
		return nil, err
	}

	var buf bytes.Buffer
	err = json.NewEncoder(&buf).Encode(&krokiRequest{
		DiagramSource:  diagramSource,
		DiagramType:    diagramType,
		OutputFormat:   string(format),
		DiagramOptions: map[string]string{},
	})
	if err != nil {
		slog.Error("Failed to encode Kroki request", "error", err)
		return nil, err
	}

	// POST to get image content
	req, err := http.NewRequest("POST", u.String(), &buf)
	if err != nil {
		slog.Error("Failed to create Kroki request", "error", err)
		return nil, err
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("Failed to send Kroki request", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		slog.Error("Kroki request failed", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("kroki error: %s", string(body))
	}
	imageContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &KrokiResult{
		ImageContent: imageContent,
		MIMEType:     format.MIMEType(),
	}, nil
}

// GetDiagramURL generates a URL for the Kroki API to fetch the diagram.
// It encodes the diagram source and appends it to the Kroki host URL.
func (kc *KrokiClient) GetDiagramURL(diagramType, diagramSource string, format model.OutputFormat) (string, error) {
	encoded, err := kc.encodeDiagram(diagramSource)
	if err != nil {
		slog.Error("Failed to encode diagram source", "error", err)
		return "", err
	}
	u, err := url.Parse(kc.Host)
	if err != nil {
		slog.Error("Invalid Kroki host URL", "host", kc.Host, "error", err)
		return "", err
	}
	u.Path = fmt.Sprintf("/%s/%s/%s", diagramType, string(format), encoded)
	return u.String(), nil
}
