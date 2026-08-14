package svgconv

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	rootSVGTagRe = regexp.MustCompile(`(?is)<svg\b[^>]*>`)
	svgAttrRe    = regexp.MustCompile(`([^\s=/>"']+)\s*=\s*("[^"]*"|'[^']*')`)
)

// NormalizeForInline rewrites the root <svg> tag of a Kroki-rendered diagram
// so it displays cleanly when embedded inline in a chat card:
//
//   - the style attribute loses its hardcoded background and fixed pixel
//     width/height, so the page background (light or dark) shows through and
//     cannot override the scalable sizing below;
//   - the fixed width/height attributes become width="100%" with a viewBox
//     (synthesized from the old width/height when Kroki omitted one), so the
//     diagram scales to its container;
//   - preserveAspectRatio is dropped, restoring the SVG default of
//     "xMidYMid meet" so scaling keeps the aspect ratio.
//
// Shape-level fills and strokes are deliberately left untouched: re-theming
// every element is a lossy transform, while a transparent, proportionally
// scaled diagram is legible on both themes as-is. The rewrite is best-effort
// and only touches the root tag; input without a recognizable root <svg> tag,
// or without both a viewBox and parseable dimensions to derive one from, keeps
// its original sizing.
func NormalizeForInline(in string) string {
	loc := rootSVGTagRe.FindStringIndex(in)
	if loc == nil {
		return in
	}
	tag := in[loc[0]:loc[1]]
	selfClosing := strings.HasSuffix(tag, "/>")

	type attr struct{ name, value string }
	var attrs []attr
	for _, m := range svgAttrRe.FindAllStringSubmatch(tag, -1) {
		attrs = append(attrs, attr{name: m[1], value: m[2][1 : len(m[2])-1]})
	}

	get := func(name string) (string, bool) {
		for _, a := range attrs {
			if strings.EqualFold(a.name, name) {
				return a.value, true
			}
		}
		return "", false
	}

	_, hasViewBox := get("viewBox")
	width, hasWidth := get("width")
	height, hasHeight := get("height")

	synthViewBox := ""
	if !hasViewBox && hasWidth && hasHeight {
		w, wOK := parseSVGLength(width)
		h, hOK := parseSVGLength(height)
		if wOK && hOK {
			synthViewBox = fmt.Sprintf("0 0 %s %s", w, h)
		}
	}
	scalable := hasViewBox || synthViewBox != ""

	out := make([]attr, 0, len(attrs)+1)
	for _, a := range attrs {
		switch strings.ToLower(a.name) {
		case "style":
			if v := cleanRootStyle(a.value); v != "" {
				out = append(out, attr{a.name, v})
			}
		case "preserveaspectratio":
			// Dropped: the SVG default is "xMidYMid meet".
		case "width":
			if scalable {
				a.value = "100%"
			}
			out = append(out, a)
		case "height":
			if !scalable {
				out = append(out, a)
			}
		default:
			out = append(out, a)
		}
	}
	if synthViewBox != "" {
		out = append(out, attr{"viewBox", synthViewBox})
	}
	if scalable && !hasWidth {
		out = append(out, attr{"width", "100%"})
	}

	var b strings.Builder
	b.WriteString("<svg")
	for _, a := range out {
		b.WriteString(" ")
		b.WriteString(a.name)
		b.WriteString(`="`)
		b.WriteString(strings.ReplaceAll(a.value, `"`, "&quot;"))
		b.WriteString(`"`)
	}
	if selfClosing {
		b.WriteString("/")
	}
	b.WriteString(">")
	return in[:loc[0]] + b.String() + in[loc[1]:]
}

// parseSVGLength accepts the plain-number and px forms Kroki emits for
// width/height ("198", "198px", "197.5"). Anything else (%, em, pt) is not a
// safe basis for a synthesized viewBox.
func parseSVGLength(v string) (string, bool) {
	n := strings.TrimSuffix(strings.TrimSpace(v), "px")
	if _, err := strconv.ParseFloat(n, 64); err != nil {
		return "", false
	}
	return n, true
}

// cleanRootStyle drops the background and fixed-size declarations Kroki puts
// on the root element's style attribute, keeping any other declarations.
func cleanRootStyle(style string) string {
	var kept []string
	for _, decl := range strings.Split(style, ";") {
		prop, _, found := strings.Cut(decl, ":")
		if !found {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(prop)) {
		case "background", "background-color", "width", "height":
		default:
			kept = append(kept, strings.TrimSpace(decl))
		}
	}
	return strings.Join(kept, ";")
}
