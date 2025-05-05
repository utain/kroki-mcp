package svgconv

import (
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/svg"
)

func MinifySVG(in string) (string, error) {
	m := minify.New()
	m.AddFunc("image/svg+xml", svg.Minify)

	out, err := m.String("image/svg+xml", in)
	if err != nil {
		return "", err
	}
	return out, nil
}
