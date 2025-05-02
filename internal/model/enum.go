package model

// Enum types for various diagram formats and output formats
// and their corresponding MIME types.
//
//	Must be one of png, svg, pdf, base64, txt or utxt.
type OutputFormat string

const (
	PNG  OutputFormat = "png"
	SVG  OutputFormat = "svg"
	TXT  OutputFormat = "txt"
	UTXT OutputFormat = "utxt"
)

var SupportedOutputFormats = []string{
	string(PNG),
	string(SVG),
	string(TXT),
	string(UTXT),
}

func (f OutputFormat) MIMEType() string {
	switch f {
	case PNG:
		return "image/png"
	case SVG:
		return "image/svg+xml"
	case TXT:
		return "text/plain"
	case UTXT:
		return "text/plain"
	default:
		return "text/plain"
	}
}

var SupportedDiagramTypes = []string{
	"blockdiag", "bpmn", "bytefield", "c4plantuml", "d2", "dbml", "ditaa", "erd",
	"excalidraw", "graphviz", "mermaid", "nomnoml", "nwdiag", "packetdiag",
	"pikchr", "plantuml", "rackdiag", "seqdiag", "structurizr", "svgbob", "umlet",
	"vega", "vegalite", "wavedrom",
}
