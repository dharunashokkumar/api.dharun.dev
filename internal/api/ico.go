package api

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
)

// decodeICO reads a Windows icon file and returns its largest image. Entries are PNG
// files or headerless bitmaps (DIB) whose height counts the transparency mask too.

var errNotICO = errors.New("not an icon file")

func isICO(data []byte) bool {
	return len(data) >= 6 && binary.LittleEndian.Uint16(data[0:]) == 0 && binary.LittleEndian.Uint16(data[2:]) == 1 && binary.LittleEndian.Uint16(data[4:]) > 0
}

func decodeICO(data []byte) (image.Image, error) {
	if !isICO(data) {
		return nil, errNotICO
	}
	count := int(binary.LittleEndian.Uint16(data[4:]))
	best, bestArea, bestDepth := -1, 0, 0
	for i := 0; i < count && 6+16*(i+1) <= len(data); i++ {
		entry := data[6+16*i:]
		width, height := int(entry[0]), int(entry[1])
		if width == 0 {
			width = 256
		}
		if height == 0 {
			height = 256
		}
		depth := int(binary.LittleEndian.Uint16(entry[6:]))
		if area := width * height; area > bestArea || (area == bestArea && depth > bestDepth) {
			best, bestArea, bestDepth = i, area, depth
		}
	}
	if best < 0 {
		return nil, errNotICO
	}
	entry := data[6+16*best:]
	size, offset := int(binary.LittleEndian.Uint32(entry[8:])), int(binary.LittleEndian.Uint32(entry[12:]))
	if offset < 0 || size <= 0 || offset+size > len(data) || offset+size < offset {
		return nil, errNotICO
	}
	body := data[offset : offset+size]
	if bytes.HasPrefix(body, []byte("\x89PNG\r\n\x1a\n")) {
		return png.Decode(bytes.NewReader(body))
	}
	return decodeDIB(body)
}

// decodeDIB decodes the bitmap inside an icon: 1, 4, 8, 24 or 32 bits a pixel, rows
// bottom up, followed by a one-bit mask that marks transparent pixels.
func decodeDIB(body []byte) (image.Image, error) {
	if len(body) < 40 {
		return nil, errNotICO
	}
	headerSize := int(binary.LittleEndian.Uint32(body[0:]))
	width := int(int32(binary.LittleEndian.Uint32(body[4:])))
	height := int(int32(binary.LittleEndian.Uint32(body[8:]))) / 2
	depth := int(binary.LittleEndian.Uint16(body[14:]))
	compression := binary.LittleEndian.Uint32(body[16:])
	colours := int(binary.LittleEndian.Uint32(body[32:]))
	if headerSize < 40 || headerSize > len(body) || width <= 0 || height <= 0 || width > 512 || height > 512 || compression != 0 {
		return nil, errNotICO
	}
	var palette []color.NRGBA
	if depth <= 8 {
		switch depth {
		case 1, 4, 8:
		default:
			return nil, errNotICO
		}
		if colours == 0 {
			colours = 1 << depth
		}
		start := headerSize
		if start+4*colours > len(body) {
			return nil, errNotICO
		}
		for i := 0; i < colours; i++ {
			p := body[start+4*i:]
			palette = append(palette, color.NRGBA{R: p[2], G: p[1], B: p[0], A: 255})
		}
	} else if depth != 24 && depth != 32 {
		return nil, errNotICO
	}
	pixels := body[headerSize+4*len(palette):]
	stride := ((width*depth + 31) / 32) * 4
	maskStride := ((width + 31) / 32) * 4
	if len(pixels) < stride*height {
		return nil, errNotICO
	}
	mask := pixels[stride*height:]
	hasMask := len(mask) >= maskStride*height
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	anyAlpha := false
	for y := 0; y < height; y++ {
		row := pixels[(height-1-y)*stride:]
		for x := 0; x < width; x++ {
			var c color.NRGBA
			switch depth {
			case 32:
				p := row[4*x:]
				c = color.NRGBA{R: p[2], G: p[1], B: p[0], A: p[3]}
				anyAlpha = anyAlpha || p[3] != 0
			case 24:
				p := row[3*x:]
				c = color.NRGBA{R: p[2], G: p[1], B: p[0], A: 255}
			default:
				bit := x * depth
				index := int(row[bit/8]>>(8-depth-bit%8)) & (1<<depth - 1)
				if index < len(palette) {
					c = palette[index]
				}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	// The mask applies to bitmaps without their own alpha channel.
	if hasMask && !anyAlpha {
		for y := 0; y < height; y++ {
			row := mask[(height-1-y)*maskStride:]
			for x := 0; x < width; x++ {
				if row[x/8]&(0x80>>(x%8)) != 0 {
					img.SetNRGBA(x, y, color.NRGBA{})
				} else if depth == 32 {
					c := img.NRGBAAt(x, y)
					c.A = 255
					img.SetNRGBA(x, y, c)
				}
			}
		}
	}
	return img, nil
}
