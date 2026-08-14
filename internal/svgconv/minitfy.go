package svgconv

import (
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/svg"
)

// minifier is shared across calls: minify.M is stateless after registration
// and safe for concurrent use.
var minifier = func() *minify.M {
	m := minify.New()
	m.AddFunc("image/svg+xml", svg.Minify)
	return m
}()

func MinifySVG(in string) (string, error) {
	return minifier.String("image/svg+xml", in)
}
