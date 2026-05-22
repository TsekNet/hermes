//go:build !windows

package tray

import (
	"bytes"
	"image"
	"image/png"
)

func encodeIcon(img *image.RGBA) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
