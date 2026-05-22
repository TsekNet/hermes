package tray

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
)

//go:embed icon.png
var badgeBasePNG []byte

// BadgeIcon composites a red dot indicator onto the tray icon when
// notifications need attention. Returns bytes in the platform's native
// icon format (ICO on Windows, PNG elsewhere).
// Count 0 returns the original platform icon unchanged.
func BadgeIcon(base []byte, count int) []byte {
	if count <= 0 {
		return base
	}

	src, err := png.Decode(bytes.NewReader(badgeBasePNG))
	if err != nil {
		return base
	}

	bounds := src.Bounds()
	canvas := image.NewRGBA(bounds)
	draw.Draw(canvas, bounds, src, bounds.Min, draw.Src)

	drawDot(canvas, bounds)

	out, err := encodeIcon(canvas)
	if err != nil {
		return base
	}
	return out
}

func drawDot(canvas *image.RGBA, bounds image.Rectangle) {
	w := bounds.Dx()
	radius := int(math.Round(float64(w) * 0.22))
	cx := bounds.Max.X - radius - 1
	cy := bounds.Min.Y + radius + 1

	red := color.RGBA{R: 220, G: 50, B: 50, A: 255}
	for y := -radius; y <= radius; y++ {
		for x := -radius; x <= radius; x++ {
			if x*x+y*y <= radius*radius {
				canvas.Set(cx+x, cy+y, red)
			}
		}
	}
}
