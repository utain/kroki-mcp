package svgconv

import (
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// NormalizeForInline rewrites the root <svg> tag of a Kroki-rendered diagram
// so it displays cleanly when embedded inline in a chat card:
//
//   - the root style attribute loses its background and fixed-size
//     declarations (width/height and the min-/max- variants Mermaid uses), so
//     they cannot paint over the page theme or override the scalable sizing
//     below;
//   - the fixed width attribute becomes width="100%" with a viewBox
//     (synthesized from the old width/height when Kroki omitted one) and a
//     style of height:auto, so the diagram scales to its container while the
//     retained height attribute keeps an intrinsic size for renderers that
//     ignore CSS;
//   - preserveAspectRatio is dropped, restoring the SVG default of
//     "xMidYMid meet" so scaling keeps the aspect ratio.
//
// Only the root tag is rewritten. That removes the background PlantUML-style
// output carries on the root element; backgrounds painted inside the body
// (Graphviz's canvas polygon, Mermaid's embedded <style> rules) are out of
// scope, as is re-theming shape-level fills and strokes — those are lossy
// transforms, while a proportionally scaled diagram is legible on both themes
// as-is. The rewrite is best-effort: input without a well-formed root <svg>
// tag is returned unchanged, and input without a valid viewBox or positive
// pixel dimensions to derive one from keeps its original sizing.
func NormalizeForInline(in string) string {
	start, end, name, rawAttrs, ok := locateRootSVGTag(in)
	if !ok {
		return in
	}
	selfClosing := strings.HasSuffix(strings.TrimSpace(in[start:end]), "/>")

	get := func(attrName string) (string, bool) {
		for _, a := range rawAttrs {
			if a.Name.Space == "" && strings.EqualFold(a.Name.Local, attrName) {
				return a.Value, true
			}
		}
		return "", false
	}

	viewBox, hasViewBoxAttr := get("viewBox")
	viewBoxOK := hasViewBoxAttr && validViewBox(viewBox)
	width, hasWidth := get("width")
	height, hasHeight := get("height")
	_, hasStyle := get("style")

	synthViewBox := ""
	if !viewBoxOK && hasWidth && hasHeight {
		w, wOK := parseSVGLength(width)
		h, hOK := parseSVGLength(height)
		if wOK && hOK {
			synthViewBox = fmt.Sprintf("0 0 %s %s", w, h)
		}
	}
	scalable := viewBoxOK || synthViewBox != ""

	out := make([]xml.Attr, 0, len(rawAttrs)+2)
	for _, a := range rawAttrs {
		if a.Name.Space != "" {
			out = append(out, a)
			continue
		}
		switch strings.ToLower(a.Name.Local) {
		case "style":
			v := cleanRootStyle(a.Value)
			if scalable {
				v = appendStyleDeclaration(v, "height:auto")
			}
			if v != "" {
				a.Value = v
				out = append(out, a)
			}
		case "preserveaspectratio":
			// Dropped: the SVG default is "xMidYMid meet".
		case "width":
			if scalable {
				a.Value = "100%"
			}
			out = append(out, a)
		case "viewbox":
			// An unusable viewBox (empty, malformed, non-positive size) is
			// replaced when width/height allow synthesizing one; otherwise
			// the sizing is not scalable and everything stays as it was.
			if !viewBoxOK && synthViewBox != "" {
				a.Value = synthViewBox
			}
			out = append(out, a)
		default:
			out = append(out, a)
		}
	}
	if synthViewBox != "" && !hasViewBoxAttr {
		out = append(out, xml.Attr{Name: xml.Name{Local: "viewBox"}, Value: synthViewBox})
	}
	if scalable && !hasWidth {
		out = append(out, xml.Attr{Name: xml.Name{Local: "width"}, Value: "100%"})
	}
	if scalable && !hasStyle {
		out = append(out, xml.Attr{Name: xml.Name{Local: "style"}, Value: "height:auto"})
	}

	var b strings.Builder
	b.WriteString("<")
	b.WriteString(qualifiedName(name))
	for _, a := range out {
		b.WriteString(" ")
		b.WriteString(qualifiedName(a.Name))
		b.WriteString(`="`)
		b.WriteString(escapeAttrValue(a.Value))
		b.WriteString(`"`)
	}
	if selfClosing {
		b.WriteString("/")
	}
	b.WriteString(">")
	return in[:start] + b.String() + in[end:]
}

// locateRootSVGTag finds the byte range [start, end) of the root <svg> start
// tag and returns its name and attributes as parsed by encoding/xml, so
// quoting and entities are handled correctly. The XML prolog, doctype,
// comments (Graphviz prefixes its output with several), and whitespace before
// the root element are skipped; anything other than a root <svg> element
// reports !ok.
func locateRootSVGTag(in string) (start, end int, name xml.Name, attrs []xml.Attr, ok bool) {
	d := xml.NewDecoder(strings.NewReader(in))
	for {
		tokenStart := d.InputOffset()
		tok, err := d.RawToken()
		if err != nil {
			return 0, 0, xml.Name{}, nil, false
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if !strings.EqualFold(t.Name.Local, "svg") {
				return 0, 0, xml.Name{}, nil, false
			}
			return int(tokenStart), int(d.InputOffset()), t.Name, t.Attr, true
		case xml.ProcInst, xml.Comment, xml.Directive, xml.CharData:
			// Prolog, doctype, comments, and inter-token whitespace.
		default:
			return 0, 0, xml.Name{}, nil, false
		}
	}
}

// qualifiedName renders an xml.Name the way RawToken read it: RawToken does
// not resolve namespaces, so Space holds the literal prefix (if any).
func qualifiedName(n xml.Name) string {
	if n.Space != "" {
		return n.Space + ":" + n.Local
	}
	return n.Local
}

// escapeAttrValue re-escapes an attribute value that the tokenizer has
// entity-decoded, for emission inside double quotes.
var attrEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", `"`, "&quot;")

func escapeAttrValue(v string) string {
	return attrEscaper.Replace(v)
}

// parseSVGLength accepts the plain-number and px forms Kroki emits for
// width/height ("198", "198px", "197.5"). The value must be a finite,
// strictly positive number: other units (%, em, pt) are not a safe basis for
// a synthesized viewBox, and NaN/Inf/non-positive values would make it
// invalid (SVG 1.1 §7.7: such an element must not be rendered).
func parseSVGLength(v string) (string, bool) {
	n := strings.TrimSuffix(strings.TrimSpace(v), "px")
	f, err := strconv.ParseFloat(n, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f <= 0 {
		return "", false
	}
	return n, true
}

// validViewBox reports whether v is a usable viewBox: four finite numbers
// (comma- or whitespace-separated) with a strictly positive width and height.
func validViewBox(v string) bool {
	fields := strings.FieldsFunc(v, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	if len(fields) != 4 {
		return false
	}
	nums := make([]float64, 4)
	for i, field := range fields {
		f, err := strconv.ParseFloat(field, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return false
		}
		nums[i] = f
	}
	return nums[2] > 0 && nums[3] > 0
}

// cleanRootStyle drops the background and fixed-size declarations Kroki puts
// on the root element's style attribute (PlantUML uses width/height, Mermaid
// max-width), keeping any other declarations.
func cleanRootStyle(style string) string {
	var kept []string
	for _, decl := range splitStyleDeclarations(style) {
		prop, _, found := strings.Cut(decl, ":")
		if !found {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(prop)) {
		case "background", "background-color", "background-image",
			"width", "height", "max-width", "min-width", "max-height", "min-height":
		default:
			kept = append(kept, strings.TrimSpace(decl))
		}
	}
	return strings.Join(kept, ";")
}

// splitStyleDeclarations splits a style attribute value on the ';' between
// declarations, leaving semicolons inside url(...) or quoted strings alone —
// a data: URI ("url(data:image/png;base64,...)") legally contains them.
func splitStyleDeclarations(style string) []string {
	var (
		decls []string
		start int
		depth int
		quote rune
	)
	for i, r := range style {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == '(':
			depth++
		case r == ')':
			if depth > 0 {
				depth--
			}
		case r == ';' && depth == 0:
			decls = append(decls, style[start:i])
			start = i + 1
		}
	}
	return append(decls, style[start:])
}

// appendStyleDeclaration appends decl to a (possibly empty) sequence of style
// declarations.
func appendStyleDeclaration(style, decl string) string {
	if style == "" {
		return decl
	}
	return style + ";" + decl
}
