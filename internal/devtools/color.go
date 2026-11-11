package devtools

import (
	"errors"
	"fmt"
	"image"
	"math"
	"sort"
	"strconv"
	"strings"
)

// RGB is a colour with channels 0 to 255.
type RGB struct{ R, G, B uint8 }

var cssNames = map[string]RGB{
	"black": {0, 0, 0}, "white": {255, 255, 255}, "red": {255, 0, 0}, "green": {0, 128, 0}, "lime": {0, 255, 0}, "blue": {0, 0, 255},
	"yellow": {255, 255, 0}, "cyan": {0, 255, 255}, "aqua": {0, 255, 255}, "magenta": {255, 0, 255}, "fuchsia": {255, 0, 255}, "grey": {128, 128, 128},
	"gray": {128, 128, 128}, "silver": {192, 192, 192}, "maroon": {128, 0, 0}, "olive": {128, 128, 0}, "navy": {0, 0, 128}, "purple": {128, 0, 128},
	"teal": {0, 128, 128}, "orange": {255, 165, 0}, "pink": {255, 192, 203}, "brown": {165, 42, 42}, "gold": {255, 215, 0}, "indigo": {75, 0, 130},
	"violet": {238, 130, 238}, "saffron": {255, 153, 51}, "indiagreen": {19, 136, 8}, "navyblue": {0, 0, 128},
}

// ParseColor reads #rgb, #rrggbb, rrggbb, rgb(r, g, b), hsl(h, s%, l%) or a colour name.
func ParseColor(s string) (RGB, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if c, ok := cssNames[strings.ReplaceAll(s, " ", "")]; ok {
		return c, nil
	}
	if strings.HasPrefix(s, "rgb") || strings.HasPrefix(s, "hsl") {
		open, close := strings.Index(s, "("), strings.LastIndex(s, ")")
		if open < 0 || close < open {
			return RGB{}, errors.New("bad colour function")
		}
		parts := strings.FieldsFunc(s[open+1:close], func(r rune) bool { return r == ',' || r == ' ' || r == '/' })
		if len(parts) < 3 {
			return RGB{}, errors.New("a colour function needs three values")
		}
		var v [3]float64
		for i := 0; i < 3; i++ {
			f, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSuffix(parts[i], "%"), "deg"), 64)
			if err != nil {
				return RGB{}, errors.New("bad number " + parts[i])
			}
			v[i] = f
		}
		if strings.HasPrefix(s, "rgb") {
			return RGB{clamp8(v[0]), clamp8(v[1]), clamp8(v[2])}, nil
		}
		return fromHSL(v[0], v[1]/100, v[2]/100), nil
	}
	hex := strings.TrimPrefix(s, "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) == 8 {
		hex = hex[:6]
	}
	if len(hex) != 6 {
		return RGB{}, errors.New("write a colour as #rrggbb, rgb(...), hsl(...) or a name")
	}
	n, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return RGB{}, errors.New("bad hex colour")
	}
	return RGB{uint8(n >> 16), uint8(n >> 8), uint8(n)}, nil
}

func clamp8(v float64) uint8 { return uint8(math.Max(0, math.Min(255, math.Round(v)))) }

func fromHSL(h, s, l float64) RGB {
	h = math.Mod(math.Mod(h, 360)+360, 360) / 360
	if s == 0 {
		return RGB{clamp8(l * 255), clamp8(l * 255), clamp8(l * 255)}
	}
	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	hue := func(t float64) float64 {
		t = math.Mod(t+1, 1)
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 0.5:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}
	return RGB{clamp8(hue(h+1.0/3) * 255), clamp8(hue(h) * 255), clamp8(hue(h-1.0/3) * 255)}
}

// Hex writes #rrggbb.
func (c RGB) Hex() string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// HSL returns hue in degrees and saturation and lightness in percent.
func (c RGB) HSL() (float64, float64, float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	hi, lo := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l := (hi + lo) / 2
	if hi == lo {
		return 0, 0, round1(l * 100)
	}
	d := hi - lo
	s := d / (1 - math.Abs(2*l-1))
	var h float64
	switch hi {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return round1(h), round1(s * 100), round1(l * 100)
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func linear(v uint8) float64 {
	c := float64(v) / 255
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// Luminance is the WCAG relative luminance.
func (c RGB) Luminance() float64 {
	return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
}

// OKLCH returns lightness (0 to 1), chroma and hue in degrees in the OKLCH space.
func (c RGB) OKLCH() (float64, float64, float64) {
	r, g, b := linear(c.R), linear(c.G), linear(c.B)
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	L := 0.2104542553*l + 0.7936177850*m - 0.0040720468*s
	A := 1.9779984951*l - 2.4285922050*m + 0.4505937099*s
	B := 0.0259040371*l + 0.7827717662*m - 0.8086757660*s
	h := math.Atan2(B, A) * 180 / math.Pi
	if h < 0 {
		h += 360
	}
	return math.Round(L*1000) / 1000, math.Round(math.Hypot(A, B)*1000) / 1000, round1(h)
}

// Contrast is the WCAG 2 contrast ratio between two colours, with what it passes.
type Contrast struct {
	Ratio      float64         `json:"ratio"`
	Passes     map[string]bool `json:"passes"`
	Suggestion string          `json:"suggestion,omitempty"`
}

// ContrastOf compares a text colour with its background.
func ContrastOf(text, background RGB) Contrast {
	a, b := text.Luminance(), background.Luminance()
	if a < b {
		a, b = b, a
	}
	ratio := math.Round((a+0.05)/(b+0.05)*100) / 100
	c := Contrast{Ratio: ratio, Passes: map[string]bool{
		"AA": ratio >= 4.5, "AALarge": ratio >= 3, "AAA": ratio >= 7, "AAALarge": ratio >= 4.5, "nonText": ratio >= 3,
	}}
	if ratio < 4.5 {
		// The nearest lightness of the text colour that reaches AA on this background.
		h, s, l := text.HSL()
		dark := background.Luminance() > 0.18
		for step := 0.0; step <= 100; step++ {
			try := l + step
			if dark {
				try = l - step
			}
			if try < 0 || try > 100 {
				break
			}
			candidate := fromHSL(h, s/100, try/100)
			x, y := candidate.Luminance(), background.Luminance()
			if x < y {
				x, y = y, x
			}
			if (x+0.05)/(y+0.05) >= 4.5 {
				c.Suggestion = candidate.Hex()
				break
			}
		}
	}
	return c
}

// Swatch is one colour of a palette with its share of the picture.
type Swatch struct {
	Hex   string  `json:"hex"`
	Share float64 `json:"share"`
}

// Palette finds the main colours of a picture by median cut on a sample of its pixels.
func Palette(img image.Image, count int) []Swatch {
	bounds := img.Bounds()
	step := max(1, int(math.Sqrt(float64(bounds.Dx()*bounds.Dy())/20000)))
	var pixels [][3]uint8
	for y := bounds.Min.Y; y < bounds.Max.Y; y += step {
		for x := bounds.Min.X; x < bounds.Max.X; x += step {
			r, g, b, a := img.At(x, y).RGBA()
			if a < 0x8000 {
				continue
			}
			pixels = append(pixels, [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)})
		}
	}
	if len(pixels) == 0 {
		return nil
	}
	boxes := [][][3]uint8{pixels}
	for len(boxes) < count {
		// Split the box with the widest channel range.
		best, channel, spread := -1, 0, -1
		for i, box := range boxes {
			if len(box) < 2 {
				continue
			}
			for ch := 0; ch < 3; ch++ {
				lo, hi := uint8(255), uint8(0)
				for _, p := range box {
					lo, hi = min(lo, p[ch]), max(hi, p[ch])
				}
				if int(hi)-int(lo) > spread {
					best, channel, spread = i, ch, int(hi)-int(lo)
				}
			}
		}
		if best < 0 || spread == 0 {
			break
		}
		box := boxes[best]
		sort.Slice(box, func(i, j int) bool { return box[i][channel] < box[j][channel] })
		half := len(box) / 2
		boxes[best] = box[:half]
		boxes = append(boxes, box[half:])
	}
	centres := make([][3]float64, len(boxes))
	for i, box := range boxes {
		for _, p := range box {
			for ch := 0; ch < 3; ch++ {
				centres[i][ch] += float64(p[ch])
			}
		}
		for ch := 0; ch < 3; ch++ {
			centres[i][ch] /= float64(len(box))
		}
	}
	// Two rounds of k-means from the median-cut centres, so shares are real shares.
	counts := make([]int, len(centres))
	for round := 0; round < 2; round++ {
		sums := make([][3]float64, len(centres))
		counts = make([]int, len(centres))
		for _, p := range pixels {
			best, distance := 0, math.MaxFloat64
			for i, c := range centres {
				d := sq(float64(p[0])-c[0]) + sq(float64(p[1])-c[1]) + sq(float64(p[2])-c[2])
				if d < distance {
					best, distance = i, d
				}
			}
			counts[best]++
			for ch := 0; ch < 3; ch++ {
				sums[best][ch] += float64(p[ch])
			}
		}
		for i := range centres {
			if counts[i] > 0 {
				for ch := 0; ch < 3; ch++ {
					centres[i][ch] = sums[i][ch] / float64(counts[i])
				}
			}
		}
	}
	swatches := make([]Swatch, 0, len(boxes))
	for i, c := range centres {
		if counts[i] == 0 {
			continue
		}
		swatches = append(swatches, Swatch{Hex: RGB{clamp8(c[0]), clamp8(c[1]), clamp8(c[2])}.Hex(), Share: math.Round(float64(counts[i])/float64(len(pixels))*1000) / 1000})
	}
	sort.Slice(swatches, func(i, j int) bool { return swatches[i].Share > swatches[j].Share })
	return swatches
}

func sq(v float64) float64 { return v * v }
