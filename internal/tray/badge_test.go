package tray

import (
	"bytes"
	"image/png"
	"testing"
)

func TestBadgeIcon_ZeroReturnsBase(t *testing.T) {
	t.Parallel()
	base := IconBytes()
	got := BadgeIcon(base, 0)
	if !bytes.Equal(got, base) {
		t.Error("count=0 should return unmodified base icon")
	}
}

func TestBadgeIcon_NegativeReturnsBase(t *testing.T) {
	t.Parallel()
	base := IconBytes()
	got := BadgeIcon(base, -1)
	if !bytes.Equal(got, base) {
		t.Error("negative count should return unmodified base icon")
	}
}

func TestBadgeIcon_ValidPNG(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		count int
	}{
		{"one", 1},
		{"five", 5},
		{"ten", 10},
		{"over ten", 15},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			base := IconBytes()
			got := BadgeIcon(base, tt.count)
			if bytes.Equal(got, base) {
				t.Error("badged icon should differ from base")
			}
			img, err := png.Decode(bytes.NewReader(got))
			if err != nil {
				t.Fatalf("result is not valid PNG: %v", err)
			}
			bounds := img.Bounds()
			if bounds.Dx() != 64 || bounds.Dy() != 64 {
				t.Errorf("dimensions = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
			}
		})
	}
}

func TestBadgeIcon_InvalidBaseStillBadges(t *testing.T) {
	t.Parallel()
	garbage := []byte("not a png")
	got := BadgeIcon(garbage, 5)
	if bytes.Equal(got, garbage) {
		t.Error("count>0 should produce a badged icon even with invalid base")
	}
	if _, err := png.Decode(bytes.NewReader(got)); err != nil {
		t.Fatalf("result is not valid PNG: %v", err)
	}
}

func TestBadgeIcon_InvalidBaseZeroReturnsBase(t *testing.T) {
	t.Parallel()
	garbage := []byte("not a png")
	got := BadgeIcon(garbage, 0)
	if !bytes.Equal(got, garbage) {
		t.Error("count=0 should return base unchanged regardless of validity")
	}
}
