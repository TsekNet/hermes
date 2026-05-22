package tray

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strconv"
)

//go:embed icon.png
var badgeBasePNG []byte

// BadgeIcon composites a notification count badge onto the tray icon.
// Always composites on the embedded PNG (ICO can't be decoded by image/png).
// Returns bytes in the platform's native icon format (ICO on Windows, PNG elsewhere).
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

	label := strconv.Itoa(count)
	if count > 10 {
		label = "10+"
	}

	drawBadge(canvas, bounds, label)

	out, err := encodeIcon(canvas)
	if err != nil {
		return base
	}
	return out
}

func drawBadge(canvas *image.RGBA, bounds image.Rectangle, label string) {
	w := bounds.Dx()

	radius := int(math.Round(float64(w) * 0.28))
	charW := int(math.Round(float64(w) * 0.10))
	padding := int(math.Round(float64(w) * 0.06))

	textW := len(label)*charW + padding*2
	if textW < radius*2 {
		textW = radius * 2
	}

	cx := bounds.Max.X - textW/2 - 1
	cy := bounds.Min.Y + radius + 1

	badgeColor := color.RGBA{R: 220, G: 50, B: 50, A: 255}
	fillCircle(canvas, cx, cy, radius, badgeColor)

	if textW > radius*2 {
		pillLeft := cx - textW/2
		pillRight := cx + textW/2
		for y := cy - radius; y <= cy+radius; y++ {
			for x := pillLeft + radius; x <= pillRight-radius; x++ {
				canvas.Set(x, y, badgeColor)
			}
		}
		fillCircle(canvas, pillLeft+radius, cy, radius, badgeColor)
		fillCircle(canvas, pillRight-radius, cy, radius, badgeColor)
	}

	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	drawText(canvas, cx, cy, label, charW, white)
}

func fillCircle(img *image.RGBA, cx, cy, r int, c color.Color) {
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			if x*x+y*y <= r*r {
				img.Set(cx+x, cy+y, c)
			}
		}
	}
}

func drawText(img *image.RGBA, cx, cy int, label string, charW int, c color.Color) {
	totalW := len(label) * charW
	startX := cx - totalW/2

	for i, ch := range label {
		x := startX + i*charW
		drawDigit(img, x, cy, ch, charW, c)
	}
}

func drawDigit(img *image.RGBA, x, cy int, ch rune, charW int, c color.Color) {
	h := int(math.Round(float64(charW) * 1.4))
	top := cy - h/2

	patterns := map[rune][]string{
		'0': {" ## ", "#  #", "#  #", "#  #", " ## "},
		'1': {" # ", "## ", " # ", " # ", "###"},
		'2': {" ## ", "#  #", "  # ", " #  ", "####"},
		'3': {" ## ", "#  #", "  # ", "#  #", " ## "},
		'4': {"#  #", "#  #", "####", "   #", "   #"},
		'5': {"####", "#   ", "### ", "   #", "### "},
		'6': {" ## ", "#   ", "### ", "#  #", " ## "},
		'7': {"####", "   #", "  # ", " #  ", " #  "},
		'8': {" ## ", "#  #", " ## ", "#  #", " ## "},
		'9': {" ## ", "#  #", " ###", "   #", " ## "},
		'+': {"    ", " #  ", "### ", " #  ", "    "},
	}

	pat, ok := patterns[ch]
	if !ok {
		return
	}

	rowH := h / len(pat)
	if rowH < 1 {
		rowH = 1
	}

	for row, line := range pat {
		colW := charW / len(line)
		if colW < 1 {
			colW = 1
		}
		for col, pixel := range line {
			if pixel == '#' {
				for dy := 0; dy < rowH; dy++ {
					for dx := 0; dx < colW; dx++ {
						img.Set(x+col*colW+dx, top+row*rowH+dy, c)
					}
				}
			}
		}
	}
}
