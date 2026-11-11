package api

// Pictures made from a few values (growth plan Part 4): avatars with initials in any
// Indian script, placeholder boxes for mock-ups, and charts from JSON for places where
// JavaScript charts cannot run (emails, chat bots, READMEs). Text goes through the same
// typesetter as the share images, so Indian scripts are shaped properly.

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dharunashokkumar/api.dharun.dev/internal/typeset"
)

// pictureFormat reads format= for these pictures: svg by default, as they are small.
func pictureFormat(raw string) (string, error) {
	switch format := strings.ToLower(strings.TrimSpace(raw)); format {
	case "", "svg":
		return "svg", nil
	case "png":
		return "png", nil
	}
	return "", fmt.Errorf("format must be svg or png")
}

// textAt draws one line centred on cx with its middle at cy.
func textAt(d *canvas, text string, style typeset.Style, size, cx, cy float64, fill string, anchor string) {
	line := d.Shape(text, style)
	width := float64(line.Width) * size / typeset.Em
	x := cx - width/2
	switch anchor {
	case "start":
		x = cx
	case "end":
		x = cx - width
	}
	d.text(line, x, cy+size*0.35, size, fill, false)
}

// ---- avatars ----

// initials takes the first letter of the first and last words. A letter is a whole
// grapheme: in Indian scripts a consonant keeps its vowel sign or virama and the
// consonant that follows a virama.
func initials(name string) string {
	words := strings.Fields(name)
	if len(words) == 0 {
		return "?"
	}
	first := func(word string) string {
		runes := []rune(word)
		end := 1
		for end < len(runes) {
			r := runes[end]
			if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) {
				end++
				continue
			}
			if isVirama(runes[end-1]) && unicode.IsLetter(r) {
				end++
				continue
			}
			break
		}
		return strings.ToUpper(string(runes[:end]))
	}
	out := first(words[0])
	if len(words) > 1 {
		out += first(words[len(words)-1])
	}
	return out
}

func isVirama(r rune) bool {
	switch r {
	case 0x094D, 0x09CD, 0x0A4D, 0x0ACD, 0x0B4D, 0x0BCD, 0x0C4D, 0x0CCD, 0x0D4D:
		return true
	}
	return false
}

func nameColor(name string) color.RGBA {
	sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(name))))
	return monogramColors[int(sum[0])%len(monogramColors)]
}

func (s *Server) handleAvatar(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	name := strings.Join(strings.Fields(drawable(r.PathValue("name"))), " ")
	name = strings.TrimSuffix(strings.TrimSuffix(name, ".svg"), ".png")
	if utf8.RuneCountInString(name) > 100 || name == "" {
		s.badRequest(w, r, "The name must be 1 to 100 characters.")
		return
	}
	format, err := pictureFormat(query.Get("format"))
	if strings.HasSuffix(r.PathValue("name"), ".png") {
		format = "png"
	}
	if err != nil {
		s.badRequest(w, r, err.Error())
		return
	}
	size, err := intParam(r, "size", 128, 16, 1024)
	if err != nil {
		s.badRequest(w, r, err.Error())
		return
	}
	background, err := parseColor(query.Get("bg"), nameColor(name))
	if err != nil {
		s.badRequest(w, r, "bg: "+err.Error())
		return
	}
	foreground, err := parseColor(query.Get("fg"), color.RGBA{0xff, 0xff, 0xff, 0xff})
	if err != nil {
		s.badRequest(w, r, "fg: "+err.Error())
		return
	}
	shape := strings.ToLower(query.Get("shape"))
	if shape == "" {
		shape = "circle"
	}
	if shape != "circle" && shape != "square" && shape != "rounded" {
		s.badRequest(w, r, "shape must be circle, square or rounded.")
		return
	}
	letters := initials(name)
	if custom := query.Get("initials"); custom != "" {
		letters = strings.Join(strings.Fields(drawable(custom)), "")
		if utf8.RuneCountInString(letters) > 4 {
			s.badRequest(w, r, "initials can be at most 4 characters.")
			return
		}
	}
	var svg []byte
	s.extras.render.work(func() {
		d := newCanvas()
		const edge = 100.0
		switch shape {
		case "circle":
			d.printf(`<circle cx="50" cy="50" r="50" fill="%s"/>`, cssColor(background))
		case "rounded":
			d.printf(`<rect width="100" height="100" rx="18" fill="%s"/>`, cssColor(background))
		default:
			d.printf(`<rect width="100" height="100" fill="%s"/>`, cssColor(background))
		}
		fontSize := 42.0
		if utf8.RuneCountInString(letters) <= 1 {
			fontSize = 50
		}
		line := d.Shape(letters, typeset.Style{Bold: true})
		if width := float64(line.Width) * fontSize / typeset.Em; width > 76 {
			fontSize *= 76 / width
		}
		textAt(d, letters, typeset.Style{Bold: true}, fontSize, edge/2, edge/2, cssColor(foreground), "")
		svg = d.svg(100, 100, name)
	})
	svg = []byte(strings.Replace(string(svg), `width="100" height="100"`, `width="`+strconv.Itoa(size)+`" height="`+strconv.Itoa(size)+`"`, 1))
	s.sendImage(w, r, svg, format, 1, false, cacheLong)
}

// ---- placeholders ----

func (s *Server) handlePlaceholder(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	raw := strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(r.PathValue("size"), ".svg"), ".png"))
	format, err := pictureFormat(query.Get("format"))
	if strings.HasSuffix(r.PathValue("size"), ".png") {
		format = "png"
	}
	if err != nil {
		s.badRequest(w, r, err.Error())
		return
	}
	wide, high, found := strings.Cut(raw, "x")
	width, err1 := strconv.Atoi(wide)
	height := width
	var err2 error
	if found {
		height, err2 = strconv.Atoi(high)
	}
	if err1 != nil || err2 != nil || width < 8 || height < 8 || width > 4000 || height > 4000 {
		s.badRequest(w, r, "The size is WIDTHxHEIGHT or one number for a square, each 8 to 4000, for example 600x400.")
		return
	}
	background, err := parseColor(query.Get("bg"), color.RGBA{0xdd, 0xe3, 0xea, 0xff})
	if err != nil {
		s.badRequest(w, r, "bg: "+err.Error())
		return
	}
	foreground, err := parseColor(query.Get("fg"), color.RGBA{0x55, 0x60, 0x70, 0xff})
	if err != nil {
		s.badRequest(w, r, "fg: "+err.Error())
		return
	}
	text := strings.Join(strings.Fields(drawable(query.Get("text"))), " ")
	if text == "" {
		text = strconv.Itoa(width) + " × " + strconv.Itoa(height)
	}
	if utf8.RuneCountInString(text) > 80 {
		s.badRequest(w, r, "text can be at most 80 characters.")
		return
	}
	var svg []byte
	s.extras.render.work(func() {
		d := newCanvas()
		d.printf(`<rect width="%d" height="%d" fill="%s"/>`, width, height, cssColor(background))
		size := math.Min(float64(height)/5, float64(width)/8)
		line := d.Shape(text, typeset.Style{Bold: true})
		if w := float64(line.Width) * size / typeset.Em; w > float64(width)*0.86 {
			size *= float64(width) * 0.86 / w
		}
		textAt(d, text, typeset.Style{Bold: true}, math.Max(size, 4), float64(width)/2, float64(height)/2, cssColor(foreground), "")
		svg = d.svg(width, height, text)
	})
	large := width*height > 1600*1600
	s.sendImage(w, r, svg, format, 1, large, cacheLong)
}

// ---- charts ----

type chartSeries struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"`
	Color  string    `json:"color"`
}

type chartRequest struct {
	Type   string        `json:"type"`
	Title  string        `json:"title"`
	Labels []string      `json:"labels"`
	Series []chartSeries `json:"series"`
	Width  int           `json:"width"`
	Height int           `json:"height"`
	Format string        `json:"format"`
	Theme  string        `json:"theme"`
}

// chartPalette is the default series colours: one cobalt and its company, readable on
// both light and dark backgrounds.
var chartPalette = []string{"#2563eb", "#0f9d8a", "#d97706", "#db2777", "#7c3aed", "#65a30d", "#0891b2", "#b91c1c"}

func (s *Server) handleChart(w http.ResponseWriter, r *http.Request) {
	var in chartRequest
	query := r.URL.Query()
	if r.Method == http.MethodPost {
		raw, err := io.ReadAll(io.LimitReader(r.Body, int64(s.cfg.MaxBodyBytes)+1))
		if err != nil || len(raw) > s.cfg.MaxBodyBytes {
			s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "The chart body is too large.")
			return
		}
		if err := json.Unmarshal(raw, &in); err != nil {
			s.badRequest(w, r, `Send JSON such as {"type": "bar", "labels": ["Jan", "Feb"], "series": [{"name": "Sales", "values": [3, 5]}]}.`)
			return
		}
	} else {
		in.Type, in.Title = query.Get("type"), query.Get("title")
		if labels := query.Get("labels"); labels != "" {
			in.Labels = strings.Split(labels, ",")
		}
		// Series names come one per names parameter, or comma-separated in one.
		names := query["names"]
		if len(names) == 1 && len(query["values"]) > 1 {
			names = strings.Split(names[0], ",")
		}
		for i, raw := range query["values"] {
			series := chartSeries{}
			for _, part := range strings.Split(raw, ",") {
				value, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
				if err != nil {
					s.badRequest(w, r, "values must be numbers separated by commas.")
					return
				}
				series.Values = append(series.Values, value)
			}
			if i < len(names) {
				series.Name = names[i]
			}
			in.Series = append(in.Series, series)
		}
		in.Width, _ = strconv.Atoi(query.Get("width"))
		in.Height, _ = strconv.Atoi(query.Get("height"))
		in.Theme = query.Get("theme")
	}
	if f := query.Get("format"); f != "" {
		in.Format = f
	}
	format, err := pictureFormat(in.Format)
	if err != nil {
		s.badRequest(w, r, err.Error())
		return
	}
	svg, problem := drawChart(s, in)
	if problem != "" {
		s.badRequest(w, r, problem)
		return
	}
	s.sendImage(w, r, svg, format, 1, in.Width*in.Height > 1600*1200, cacheLong)
}

func drawChart(s *Server, in chartRequest) ([]byte, string) {
	in.Type = strings.ToLower(strings.TrimSpace(in.Type))
	if in.Type == "" {
		in.Type = "bar"
	}
	if in.Type != "bar" && in.Type != "line" && in.Type != "pie" && in.Type != "sparkline" {
		return nil, "type must be bar, line, pie or sparkline."
	}
	if len(in.Series) == 0 || len(in.Series) > 8 {
		return nil, "Give 1 to 8 series of values."
	}
	points := 0
	for i := range in.Series {
		if n := len(in.Series[i].Values); n == 0 || n > 500 {
			return nil, "Each series needs 1 to 500 values."
		}
		points = max(points, len(in.Series[i].Values))
		for _, v := range in.Series[i].Values {
			if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e15 {
				return nil, "Values must be ordinary numbers."
			}
		}
		if in.Series[i].Color == "" {
			in.Series[i].Color = chartPalette[i%len(chartPalette)]
		} else if c, err := parseColor(in.Series[i].Color, color.RGBA{}); err == nil {
			in.Series[i].Color = cssColor(c)
		} else {
			return nil, "series color: " + err.Error()
		}
		in.Series[i].Name = clip(strings.Join(strings.Fields(drawable(in.Series[i].Name)), " "), 60)
	}
	if len(in.Labels) > 500 {
		return nil, "At most 500 labels."
	}
	for i := range in.Labels {
		in.Labels[i] = clip(strings.Join(strings.Fields(drawable(in.Labels[i])), " "), 40)
	}
	in.Title = clip(strings.Join(strings.Fields(drawable(in.Title)), " "), 120)
	if in.Type == "sparkline" {
		in.Width, in.Height = defaultInt(in.Width, 160), defaultInt(in.Height, 40)
	} else {
		in.Width, in.Height = defaultInt(in.Width, 800), defaultInt(in.Height, 450)
	}
	if in.Width < 40 || in.Height < 20 || in.Width > 3000 || in.Height > 3000 {
		return nil, "width and height must be 40 to 3000 and 20 to 3000."
	}
	paper, ink, muted, rule := "#ffffff", "#111825", "#5b6474", "#e3e8ef"
	if strings.EqualFold(in.Theme, "dark") {
		paper, ink, muted, rule = "#111825", "#e8edf4", "#9aa5b5", "#2a3343"
	}
	var svg []byte
	s.extras.render.work(func() {
		d := newCanvas()
		W, H := float64(in.Width), float64(in.Height)
		if in.Type != "sparkline" {
			d.printf(`<rect width="%d" height="%d" fill="%s"/>`, in.Width, in.Height, paper)
		}
		top, left, right, bottom := 16.0, 16.0, 16.0, 16.0
		if in.Title != "" && in.Type != "sparkline" {
			textAt(d, in.Title, typeset.Style{Bold: true}, 20, 16, 26, ink, "start")
			top = 52
		}
		legend := len(in.Series) > 1 || (in.Type == "pie")
		if legend && in.Type != "sparkline" {
			bottom = 40
		}
		switch in.Type {
		case "sparkline":
			lo, hi := bounds(in.Series)
			for _, series := range in.Series {
				d.printf(`<polyline fill="none" stroke="%s" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" points="%s"/>`, series.Color, polyline(series.Values, 2, 2, W-4, H-4, lo, hi, points))
			}
		case "pie":
			values := in.Series[0].Values
			total := 0.0
			for _, v := range values {
				total += math.Max(v, 0)
			}
			cx, cy := W/2, top+(H-top-bottom)/2
			radius := math.Min(W-left-right, H-top-bottom) / 2 * 0.92
			angle := -math.Pi / 2
			for i, v := range values {
				if v <= 0 || total == 0 {
					continue
				}
				sweep := v / total * 2 * math.Pi
				fill := chartPalette[i%len(chartPalette)]
				if sweep >= 2*math.Pi-1e-9 {
					d.printf(`<circle cx="%s" cy="%s" r="%s" fill="%s"/>`, coordinate(cx), coordinate(cy), coordinate(radius), fill)
				} else {
					x1, y1 := cx+radius*math.Cos(angle), cy+radius*math.Sin(angle)
					x2, y2 := cx+radius*math.Cos(angle+sweep), cy+radius*math.Sin(angle+sweep)
					large := 0
					if sweep > math.Pi {
						large = 1
					}
					d.printf(`<path d="M%s %sL%s %sA%s %s 0 %d 1 %s %sZ" fill="%s" stroke="%s" stroke-width="2"/>`,
						coordinate(cx), coordinate(cy), coordinate(x1), coordinate(y1), coordinate(radius), coordinate(radius), large, coordinate(x2), coordinate(y2), fill, paper)
				}
				angle += sweep
			}
			var names []chartSeries
			for i, v := range values {
				label := strconv.Itoa(i + 1)
				if i < len(in.Labels) {
					label = in.Labels[i]
				}
				names = append(names, chartSeries{Name: label + " " + strconv.FormatFloat(math.Round(v/math.Max(total, 1e-12)*1000)/10, 'f', -1, 64) + "%", Color: chartPalette[i%len(chartPalette)]})
			}
			drawLegend(d, names, H-22, ink)
		default:
			lo, hi := bounds(in.Series)
			if in.Type == "bar" {
				lo = math.Min(lo, 0)
				hi = math.Max(hi, 0)
			}
			lo, hi, step := niceScale(lo, hi)
			// The axis labels set the left margin.
			labelSize := 12.0
			widest := 0
			for v := lo; v <= hi+step/2; v += step {
				widest = max(widest, d.Shape(formatTick(v), typeset.Style{}).Width)
			}
			left = 16 + float64(widest)*labelSize/typeset.Em + 8
			plotBottom := H - bottom - 22
			plotW, plotH := W-left-right, plotBottom-top
			y := func(v float64) float64 { return top + plotH - (v-lo)/(hi-lo)*plotH }
			for v := lo; v <= hi+step/2; v += step {
				d.printf(`<line x1="%s" x2="%s" y1="%s" y2="%s" stroke="%s"/>`, coordinate(left), coordinate(W-right), coordinate(y(v)), coordinate(y(v)), rule)
				textAt(d, formatTick(v), typeset.Style{}, labelSize, left-8, y(v), muted, "end")
			}
			slot := plotW / float64(points)
			every := max(1, int(math.Ceil(float64(len(in.Labels))*60/plotW)))
			for i, label := range in.Labels {
				if i >= points || i%every != 0 {
					continue
				}
				textAt(d, d.Fit(label, typeset.Style{}, int(slot*float64(every)*typeset.Em/labelSize)), typeset.Style{}, labelSize, left+slot*(float64(i)+0.5), plotBottom+14, muted, "")
			}
			if in.Type == "bar" {
				group := slot * 0.78
				barW := group / float64(len(in.Series))
				for si, series := range in.Series {
					for i, v := range series.Values {
						x := left + slot*float64(i) + (slot-group)/2 + barW*float64(si)
						y0, y1 := y(0), y(v)
						d.printf(`<rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="%s"/>`, coordinate(x+1), coordinate(math.Min(y0, y1)), coordinate(math.Max(barW-2, 1)), coordinate(math.Abs(y1-y0)), series.Color)
					}
				}
			} else {
				for _, series := range in.Series {
					var pts []string
					for i, v := range series.Values {
						pts = append(pts, coordinate(left+slot*(float64(i)+0.5))+","+coordinate(y(v)))
					}
					d.printf(`<polyline fill="none" stroke="%s" stroke-width="2.5" stroke-linejoin="round" stroke-linecap="round" points="%s"/>`, series.Color, strings.Join(pts, " "))
				}
			}
			if legend {
				drawLegend(d, in.Series, H-22, ink)
			}
		}
		label := in.Title
		if label == "" {
			label = in.Type + " chart"
		}
		svg = d.svg(in.Width, in.Height, label)
	})
	return svg, ""
}

func defaultInt(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func bounds(series []chartSeries) (float64, float64) {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, s := range series {
		for _, v := range s.Values {
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
	}
	if lo == hi {
		lo, hi = lo-1, hi+1
	}
	return lo, hi
}

// niceScale rounds an axis to steps of 1, 2 or 5 times a power of ten.
func niceScale(lo, hi float64) (float64, float64, float64) {
	raw := (hi - lo) / 5
	power := math.Pow(10, math.Floor(math.Log10(raw)))
	step := power
	for _, m := range []float64{1, 2, 5, 10} {
		if raw <= m*power {
			step = m * power
			break
		}
	}
	return math.Floor(lo/step) * step, math.Ceil(hi/step) * step, step
}

// formatTick writes an axis value the Indian way: 1.5K, 2.5L, 3Cr.
func formatTick(v float64) string {
	a := math.Abs(v)
	trim := func(x float64) string { return strconv.FormatFloat(math.Round(x*100)/100, 'f', -1, 64) }
	switch {
	case a >= 1e7:
		return trim(v/1e7) + "Cr"
	case a >= 1e5:
		return trim(v/1e5) + "L"
	case a >= 1e3:
		return trim(v/1e3) + "K"
	}
	return trim(v)
}

func polyline(values []float64, x0, y0, w, h, lo, hi float64, points int) string {
	var out []string
	for i, v := range values {
		x := x0 + w*float64(i)/math.Max(float64(points-1), 1)
		y := y0 + h - (v-lo)/(hi-lo)*h
		out = append(out, coordinate(x)+","+coordinate(y))
	}
	return strings.Join(out, " ")
}

func drawLegend(d *canvas, series []chartSeries, y float64, ink string) {
	x := 16.0
	for i, s := range series {
		name := s.Name
		if name == "" {
			name = "Series " + strconv.Itoa(i+1)
		}
		d.printf(`<rect x="%s" y="%s" width="12" height="12" rx="2" fill="%s"/>`, coordinate(x), coordinate(y-6), s.Color)
		textAt(d, name, typeset.Style{}, 12, x+18, y, ink, "start")
		x += 18 + float64(d.Shape(name, typeset.Style{}).Width)*12/typeset.Em + 20
	}
}
