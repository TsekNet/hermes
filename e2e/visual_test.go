//go:build e2e

package e2e

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"strings"
	"testing"
)

func TestVisual_AllConfigs(t *testing.T) {
	update := os.Getenv("UPDATE_GOLDEN") == "1"
	ci := os.Getenv("CI") != ""

	configs := AllTestdataConfigs(t)
	if len(configs) == 0 {
		t.Fatal("no testdata configs found")
	}

	for _, name := range configs {
		t.Run(name, func(t *testing.T) {
			cfg := LoadConfig(t, name)

			var h *Harness
			if len(cfg.Images) > 0 {
				h = StartTall(t, cfg)
			} else {
				h = Start(t, cfg)
			}

			got := h.Screenshot()
			goldenName := strings.TrimSuffix(name, ".json") + ".png"

			if update {
				UpdateGolden(t, goldenName, got)
				t.Logf("updated golden: %s", goldenName)
				return
			}

			if ci {
				// CI font rendering differs from local; golden comparison
				// would false-positive. Verify screenshots can be taken
				// (smoke test) and save for artifact upload.
				UpdateGolden(t, goldenName+".ci.png", got)
				return
			}

			want, exists := ReadGolden(t, goldenName)
			if !exists {
				UpdateGolden(t, goldenName, got)
				t.Logf("created golden: %s (run again to validate)", goldenName)
				return
			}

			if !bytes.Equal(got, want) {
				actualName := strings.TrimSuffix(goldenName, ".png") + "-actual.png"
				UpdateGolden(t, actualName, got)

				gotImg := decodePNG(t, got)
				wantImg := decodePNG(t, want)

				diffPct := imageDiffPercent(gotImg, wantImg)
				if diffPct > 0.5 {
					t.Errorf("screenshot differs by %.1f%% (threshold 0.5%%), saved actual to %s", diffPct, actualName)
				}
			}
		})
	}
}

func decodePNG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	return img
}

func imageDiffPercent(a, b image.Image) float64 {
	boundsA := a.Bounds()
	boundsB := b.Bounds()

	// Different dimensions = 100% diff
	if boundsA.Dx() != boundsB.Dx() || boundsA.Dy() != boundsB.Dy() {
		return 100.0
	}

	total := boundsA.Dx() * boundsA.Dy()
	if total == 0 {
		return 0
	}

	diff := 0
	for y := boundsA.Min.Y; y < boundsA.Max.Y; y++ {
		for x := boundsA.Min.X; x < boundsA.Max.X; x++ {
			r1, g1, b1, a1 := a.At(x, y).RGBA()
			r2, g2, b2, a2 := b.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				diff++
			}
		}
	}

	return float64(diff) / float64(total) * 100
}
