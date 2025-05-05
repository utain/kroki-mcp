package svgconv

import (
	"fmt"
	"image/jpeg"
	"io"
	"strings"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers"
)

type OutputFormat string

const (
	PNG  OutputFormat = "png"
	JPEG OutputFormat = "jpeg"
)

type Options struct {
	Format OutputFormat
	DPI    float64
}

func Convert(out io.Writer, svg string, opt Options) error {
	c, err := canvas.ParseSVG(strings.NewReader(svg))
	if err != nil {
		return err
	}

	opts := []any{canvas.DPI(opt.DPI)}
	switch opt.Format {
	case PNG:
		writer := renderers.PNG(opts...)
		writer(out, c)
		return nil
	case JPEG:
		ctx := canvas.NewContext(c)
		// Set the background color to white
		ctx.SetZIndex(-100)
		rect := canvas.RectFromSize(0, 0, c.W, c.H).ToPath()
		ctx.SetFillColor(canvas.Pink)
		ctx.DrawPath(0, 0, rect)
		ctx.Fill()
		ctx.FillStroke()
		ctx.Close()

		opts = append(opts, &jpeg.Options{Quality: 90})
		writer := renderers.JPEG(opts...)
		writer(out, c)
		return nil
	default:
		return fmt.Errorf("unsupported format: %s", opt.Format)
	}
}
