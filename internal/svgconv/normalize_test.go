package svgconv

import (
	"strings"
	"testing"
)

// krokiRootSVG mirrors the root tag PlantUML diagrams come back with from
// Kroki: fixed pixel sizing duplicated in the style attribute, a hardcoded
// white background, and preserveAspectRatio="none".
const krokiRootSVG = `<svg xmlns="http://www.w3.org/2000/svg" data-diagram-type="SEQUENCE" height="210" preserveAspectRatio="none" style="width:198px;height:210px;background:#FFFFFF;" viewBox="0 0 198 210" width="198" zoomAndPan="magnify"><g><rect fill="#E2E2F0" height="30" width="50" x="10" y="10"/><text fill="#181818" x="12" y="25">Alice</text></g></svg>`

func TestNormalizeForInline_KrokiRoot(t *testing.T) {
	got := NormalizeForInline(krokiRootSVG)

	if strings.Contains(got, "background") {
		t.Errorf("normalized SVG still has a background: %q", got)
	}
	if strings.Contains(got, "preserveAspectRatio") {
		t.Errorf("normalized SVG still has preserveAspectRatio: %q", got)
	}
	if !strings.Contains(got, `width="100%"`) {
		t.Errorf("normalized SVG missing width=\"100%%\": %q", got)
	}
	if !strings.Contains(got, `viewBox="0 0 198 210"`) {
		t.Errorf("normalized SVG lost its viewBox: %q", got)
	}
	if strings.Contains(got, `height="210"`) {
		t.Errorf("normalized SVG kept the fixed root height: %q", got)
	}
	// Shape-level colors must survive untouched.
	if !strings.Contains(got, `fill="#E2E2F0"`) || !strings.Contains(got, `fill="#181818"`) {
		t.Errorf("normalized SVG lost shape-level colors: %q", got)
	}
}

func TestNormalizeForInline_SynthesizesViewBox(t *testing.T) {
	got := NormalizeForInline(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="50"><rect width="100" height="50"/></svg>`)

	if !strings.Contains(got, `viewBox="0 0 100 50"`) {
		t.Errorf("expected synthesized viewBox, got: %q", got)
	}
	if !strings.Contains(got, `width="100%"`) {
		t.Errorf("expected width=\"100%%\", got: %q", got)
	}
	// The inner rect's sizing is content, not root sizing.
	if !strings.Contains(got, `<rect width="100" height="50"/>`) {
		t.Errorf("inner elements were modified: %q", got)
	}
}

func TestNormalizeForInline_UnscalableSizingKept(t *testing.T) {
	in := `<svg xmlns="http://www.w3.org/2000/svg" width="10em" height="5em"></svg>`
	got := NormalizeForInline(in)

	// No viewBox and no px dimensions to derive one from: sizing must be
	// left alone rather than made 100% with no aspect ratio to anchor it.
	if !strings.Contains(got, `width="10em"`) || !strings.Contains(got, `height="5em"`) {
		t.Errorf("unscalable sizing was rewritten: %q", got)
	}
}

func TestNormalizeForInline_NotSVGUnchanged(t *testing.T) {
	in := `{"error": "not an svg"}`
	if got := NormalizeForInline(in); got != in {
		t.Errorf("non-SVG input was modified: %q", got)
	}
}
