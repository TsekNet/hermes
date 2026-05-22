//go:build windows

package tray

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
)

// encodeIcon wraps a PNG-encoded image in an ICO container.
// Windows Vista+ supports PNG-compressed ICO entries.
func encodeIcon(img *image.RGBA) ([]byte, error) {
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return nil, err
	}
	pngData := pngBuf.Bytes()

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	bw := byte(w)
	bh := byte(h)
	if w >= 256 {
		bw = 0
	}
	if h >= 256 {
		bh = 0
	}

	var buf bytes.Buffer
	// ICONDIR header: reserved(2) + type(2) + count(2)
	binary.Write(&buf, binary.LittleEndian, uint16(0))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	// ICONDIRENTRY: width, height, colors, reserved, planes, bpp, size, offset
	buf.WriteByte(bw)
	buf.WriteByte(bh)
	buf.WriteByte(0) // color count (0 = no palette)
	buf.WriteByte(0) // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1))             // planes
	binary.Write(&buf, binary.LittleEndian, uint16(32))            // bits per pixel
	binary.Write(&buf, binary.LittleEndian, uint32(len(pngData)))  // image size
	binary.Write(&buf, binary.LittleEndian, uint32(6+16))          // offset (header=6 + one entry=16)
	buf.Write(pngData)

	return buf.Bytes(), nil
}
