package devtools

import (
	"errors"
	"image"
	"math"
)

// ThumbHash encodes a picture of at most 100 by 100 pixels as a ThumbHash: a tiny
// placeholder (about 25 bytes) that a page draws, blurred, while the real picture
// loads. Ported from Evan Wallace's reference encoder (github.com/evanw/thumbhash,
// MIT licence, Copyright (c) 2023 Evan Wallace).
func ThumbHash(img image.Image) ([]byte, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > 100 || h > 100 || w < 1 || h < 1 {
		return nil, errors.New("thumbhash needs a picture of at most 100 by 100 pixels")
	}
	n := w * h
	rgba := make([]float64, 4*n)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			i := 4 * (x + y*w)
			// RGBA() is premultiplied; undo it to straight 0..255 values.
			if a > 0 {
				rgba[i], rgba[i+1], rgba[i+2] = float64(r)*255/float64(a), float64(g)*255/float64(a), float64(bl)*255/float64(a)
			}
			rgba[i+3] = float64(a >> 8)
		}
	}
	var avgR, avgG, avgB, avgA float64
	for i := 0; i < n; i++ {
		alpha := rgba[4*i+3] / 255
		avgR += alpha / 255 * rgba[4*i]
		avgG += alpha / 255 * rgba[4*i+1]
		avgB += alpha / 255 * rgba[4*i+2]
		avgA += alpha
	}
	if avgA > 0 {
		avgR, avgG, avgB = avgR/avgA, avgG/avgA, avgB/avgA
	}
	hasAlpha := avgA < float64(n)
	lLimit := 7.0
	if hasAlpha {
		lLimit = 5
	}
	longest := float64(max(w, h))
	lx := max(1, int(math.Round(lLimit*float64(w)/longest)))
	ly := max(1, int(math.Round(lLimit*float64(h)/longest)))
	l, p, q, a := make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n)
	for i := 0; i < n; i++ {
		alpha := rgba[4*i+3] / 255
		r := avgR*(1-alpha) + alpha/255*rgba[4*i]
		g := avgG*(1-alpha) + alpha/255*rgba[4*i+1]
		bl := avgB*(1-alpha) + alpha/255*rgba[4*i+2]
		l[i] = (r + g + bl) / 3
		p[i] = (r+g)/2 - bl
		q[i] = r - g
		a[i] = alpha
	}
	encode := func(channel []float64, nx, ny int) (float64, []float64, float64) {
		var dc, scale float64
		var ac []float64
		fx := make([]float64, w)
		for cy := 0; cy < ny; cy++ {
			for cx := 0; cx*ny < nx*(ny-cy); cx++ {
				f := 0.0
				for x := 0; x < w; x++ {
					fx[x] = math.Cos(math.Pi / float64(w) * float64(cx) * (float64(x) + 0.5))
				}
				for y := 0; y < h; y++ {
					fy := math.Cos(math.Pi / float64(h) * float64(cy) * (float64(y) + 0.5))
					for x := 0; x < w; x++ {
						f += channel[x+y*w] * fx[x] * fy
					}
				}
				f /= float64(n)
				if cx > 0 || cy > 0 {
					ac = append(ac, f)
					scale = math.Max(scale, math.Abs(f))
				} else {
					dc = f
				}
			}
		}
		if scale > 0 {
			for i := range ac {
				ac[i] = 0.5 + 0.5/scale*ac[i]
			}
		}
		return dc, ac, scale
	}
	lDC, lAC, lScale := encode(l, max(3, lx), max(3, ly))
	pDC, pAC, pScale := encode(p, 3, 3)
	qDC, qAC, qScale := encode(q, 3, 3)
	var aDC, aScale float64
	var aAC []float64
	if hasAlpha {
		aDC, aAC, aScale = encode(a, 5, 5)
	}
	round := func(v float64) int { return int(math.Round(v)) }
	landscape := 0
	if w > h {
		landscape = 1
	}
	alphaBit := 0
	if hasAlpha {
		alphaBit = 1
	}
	header24 := round(63*lDC) | round(31.5+31.5*pDC)<<6 | round(31.5+31.5*qDC)<<12 | round(31*lScale)<<18 | alphaBit<<23
	side := lx
	if landscape == 1 {
		side = ly
	}
	header16 := side | round(63*pScale)<<3 | round(63*qScale)<<9 | landscape<<15
	hash := []byte{byte(header24), byte(header24 >> 8), byte(header24 >> 16), byte(header16), byte(header16 >> 8)}
	start := 5
	if hasAlpha {
		hash = append(hash, byte(round(15*aDC)|round(15*aScale)<<4))
		start = 6
	}
	channels := [][]float64{lAC, pAC, qAC}
	if hasAlpha {
		channels = append(channels, aAC)
	}
	index := 0
	for _, ac := range channels {
		for _, f := range ac {
			at := start + index>>1
			for len(hash) <= at {
				hash = append(hash, 0)
			}
			hash[at] |= byte(round(15*f) << ((index & 1) << 2))
			index++
		}
	}
	return hash, nil
}
