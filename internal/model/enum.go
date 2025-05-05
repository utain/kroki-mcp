package model

// Enum types for various diagram formats and output formats
// and their corresponding MIME types.
//
//	Must be one of png, svg
type OutputFormat string

const (
	PNG OutputFormat = "png"
	SVG OutputFormat = "svg"
)

var SupportedOutputFormats = []string{
	string(PNG),
	string(SVG),
}

func (f OutputFormat) MIMEType() string {
	switch f {
	case SVG:
		return "image/svg+xml"
	case PNG:
		return "image/png"
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

var RecommendedDPIList = []float64{
	72, 84, 96, 120, 144, 150, 166, 180, 200, 220, 240,
}
