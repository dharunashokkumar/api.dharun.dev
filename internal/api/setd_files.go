package api

// Set D tools that take files: colour palettes, ThumbHash placeholders, icon packs, SVG
// to PNG, watermarks, PDF extras, audio and video (ffmpeg) and spreadsheets. Every
// upload endpoint is Private, as all of them are.

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"github.com/xuri/excelize/v2"
	xdraw "golang.org/x/image/draw"

	"github.com/dharunashokkumar/api.dharun.dev/internal/devtools"
	"github.com/dharunashokkumar/api.dharun.dev/internal/typeset"
)

func (s *Server) registerSetDFiles() {
	cpu := uploadRoute{}
	many := uploadRoute{maxFiles: pdfMaxFiles, limit: mergeLimit}
	for _, tool := range []struct {
		route, path string
		cost        int
		spec        uploadRoute
		handler     func(http.ResponseWriter, *http.Request, uploadInput)
	}{
		{"color_palette", "/v1/color/palette", costUpload, cpu, s.handlePalette},
		{"image_thumbhash", "/v1/image/thumbhash", costUpload, cpu, s.handleThumbHash},
		{"image_icons", "/v1/image/icons", costUpload, cpu, s.handleIconPack},
		{"svg_to_png", "/v1/image/svg-to-png", costUpload, cpu, s.handleSVGToPNG},
		{"image_watermark", "/v1/image/watermark", costUpload, cpu, s.handleImageWatermark},
		{"pdf_watermark", "/v1/pdf/watermark", costUpload, cpu, s.handlePDFWatermark},
		{"pdf_page_numbers", "/v1/pdf/page-numbers", costUpload, cpu, s.handlePDFPageNumbers},
		{"pdf_from_images", "/v1/pdf/from-images", costUpload, many, s.handlePDFFromImages},
		{"pdf_unlock", "/v1/pdf/unlock", costUpload, cpu, s.handlePDFUnlock},
		{"media", "/v1/media/convert", costModel, cpu, s.handleMediaConvert},
		{"media", "/v1/media/trim", costModel, cpu, s.handleMediaTrim},
		{"media", "/v1/media/waveform", costModel, cpu, s.handleMediaWaveform},
		{"media", "/v1/media/thumbnail", costModel, cpu, s.handleMediaThumbnail},
		{"sheet", "/v1/sheet/to-json", costUpload, cpu, s.handleSheetToJSON},
	} {
		s.handleUpload(tool.route, tool.cost, "POST "+tool.path, tool.spec, tool.handler)
	}
	s.handle("sheet", costHeavy, "POST /v1/sheet/from-json", s.handleSheetFromJSON)
}

// ---- pictures ----

func (s *Server) picture(w http.ResponseWriter, r *http.Request, in uploadInput) (image.Image, bool) {
	picture, err := s.loadPicture(r.Context(), in.Files[0].Data)
	if err != nil {
		s.notPicture(w, r)
		return nil, false
	}
	return picture, true
}

func (s *Server) handlePalette(w http.ResponseWriter, r *http.Request, in uploadInput) {
	picture, ok := s.picture(w, r, in)
	if !ok {
		return
	}
	defer freeLarge(picture)
	count := 6
	if raw := in.option(r, "count"); raw != "" {
		var err error
		if count, err = strconv.Atoi(raw); err != nil || count < 1 || count > 16 {
			s.badRequest(w, r, "count is 1 to 16.")
			return
		}
	}
	var swatches []devtools.Swatch
	s.extras.render.work(func() { swatches = devtools.Palette(picture, count) })
	if len(swatches) == 0 {
		s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "The picture has no visible pixels.")
		return
	}
	out := make([]map[string]any, len(swatches))
	for i, sw := range swatches {
		c, _ := devtools.ParseColor(sw.Hex)
		out[i] = map[string]any{"hex": sw.Hex, "share": sw.Share, "textColor": describeColour(c).TextColor}
	}
	s.ok(w, r, out, meta{Note: "The main colours, largest share first (median cut, then k-means). Transparent pixels are left out."}, "private, no-store")
}

// shrink scales a picture down to fit in size by size.
func shrink(picture image.Image, size int) image.Image {
	b := picture.Bounds()
	scale := math.Min(float64(size)/float64(b.Dx()), float64(size)/float64(b.Dy()))
	if scale >= 1 {
		return picture
	}
	w, h := max(1, int(math.Round(float64(b.Dx())*scale))), max(1, int(math.Round(float64(b.Dy())*scale)))
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(out, out.Bounds(), picture, b, xdraw.Src, nil)
	return out
}

// square fits a picture in a size by size square, centred, with a transparent margin.
func square(picture image.Image, size int) *image.NRGBA {
	b := picture.Bounds()
	scale := math.Min(float64(size)/float64(b.Dx()), float64(size)/float64(b.Dy()))
	w, h := max(1, int(math.Round(float64(b.Dx())*scale))), max(1, int(math.Round(float64(b.Dy())*scale)))
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	x, y := (size-w)/2, (size-h)/2
	xdraw.CatmullRom.Scale(out, image.Rect(x, y, x+w, y+h), picture, b, xdraw.Over, nil)
	return out
}

func (s *Server) handleThumbHash(w http.ResponseWriter, r *http.Request, in uploadInput) {
	picture, ok := s.picture(w, r, in)
	if !ok {
		return
	}
	defer freeLarge(picture)
	var hash []byte
	var err error
	b := picture.Bounds()
	s.extras.render.work(func() { hash, err = devtools.ThumbHash(shrink(picture, 100)) })
	if err != nil {
		s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "The picture cannot be hashed.")
		return
	}
	s.ok(w, r, map[string]any{"thumbhash": base64.StdEncoding.EncodeToString(hash), "bytes": len(hash), "width": b.Dx(), "height": b.Dy()},
		meta{Note: "Store the thumbhash with the picture and draw it with the thumbhash library (thumbHashToDataURL) while the picture loads."}, "private, no-store")
}

// encodeICO writes PNG pictures as one .ico file, as browsers accept since Windows Vista.
func encodeICO(pictures []*image.NRGBA) []byte {
	var images [][]byte
	for _, p := range pictures {
		var b bytes.Buffer
		png.Encode(&b, p)
		images = append(images, b.Bytes())
	}
	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, [3]uint16{0, 1, uint16(len(images))})
	offset := 6 + 16*len(images)
	for i, data := range images {
		size := pictures[i].Bounds().Dx()
		dim := byte(size)
		if size >= 256 {
			dim = 0
		}
		out.Write([]byte{dim, dim, 0, 0})
		binary.Write(&out, binary.LittleEndian, [2]uint16{1, 32})
		binary.Write(&out, binary.LittleEndian, [2]uint32{uint32(len(data)), uint32(offset)})
		offset += len(data)
	}
	for _, data := range images {
		out.Write(data)
	}
	return out.Bytes()
}

func (s *Server) handleIconPack(w http.ResponseWriter, r *http.Request, in uploadInput) {
	picture, ok := s.picture(w, r, in)
	if !ok {
		return
	}
	defer freeLarge(picture)
	name := strings.TrimSpace(in.option(r, "name"))
	if name == "" {
		name = "My app"
	}
	if utf8.RuneCountInString(name) > 60 {
		s.badRequest(w, r, "name is at most 60 characters.")
		return
	}
	theme := "#ffffff"
	if raw := in.option(r, "theme"); raw != "" {
		c, err := devtools.ParseColor(raw)
		if err != nil {
			s.badRequest(w, r, "theme is a colour, such as 2563eb.")
			return
		}
		theme = c.Hex()
	}
	var archive bytes.Buffer
	s.extras.render.work(func() {
		z := zip.NewWriter(&archive)
		add := func(file string, data []byte) {
			f, _ := z.CreateHeader(&zip.FileHeader{Name: file, Method: zip.Deflate, Modified: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
			f.Write(data)
		}
		pngOf := func(size int) []byte {
			var b bytes.Buffer
			png.Encode(&b, square(picture, size))
			return b.Bytes()
		}
		add("favicon.ico", encodeICO([]*image.NRGBA{square(picture, 16), square(picture, 32), square(picture, 48)}))
		add("favicon-32x32.png", pngOf(32))
		add("apple-touch-icon.png", pngOf(180))
		add("icon-192.png", pngOf(192))
		add("icon-512.png", pngOf(512))
		manifest, _ := json.MarshalIndent(map[string]any{"name": name, "short_name": name, "theme_color": theme, "background_color": theme, "display": "standalone",
			"icons": []map[string]string{{"src": "/icon-192.png", "sizes": "192x192", "type": "image/png"}, {"src": "/icon-512.png", "sizes": "512x512", "type": "image/png"}}}, "", "  ")
		add("site.webmanifest", manifest)
		add("head.html", []byte(`<link rel="icon" href="/favicon.ico" sizes="any">
<link rel="icon" type="image/png" sizes="32x32" href="/favicon-32x32.png">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
<link rel="manifest" href="/site.webmanifest">
<meta name="theme-color" content="`+theme+`">
`))
		z.Close()
	})
	sendFile(w, r, archive.Bytes(), "application/zip", "icons.zip")
}

var (
	svgDanger    = regexp.MustCompile(`(?is)<script|<foreignObject|\bon[a-z]+\s*=`)
	svgExternal  = regexp.MustCompile(`(?is)(?:xlink:)?href\s*=\s*["']\s*([^"'#][^"']*)["']`)
	svgDimension = regexp.MustCompile(`(?is)<svg[^>]*?\b(width|height)\s*=\s*["']\s*([\d.]+)`)
)

// cleanSVG refuses scripts and every reference outside the file itself: resvg would
// otherwise read files named by an image's href.
func cleanSVG(data []byte) ([]byte, error) {
	if !utf8.Valid(data) || !bytes.Contains(data, []byte("<svg")) {
		return nil, fmt.Errorf("the file is not an SVG")
	}
	clean := svgDanger.ReplaceAll(data, []byte("<!-- removed -->"))
	for _, m := range svgExternal.FindAllSubmatch(clean, -1) {
		if !bytes.HasPrefix(bytes.TrimSpace(m[1]), []byte("data:image/")) {
			return nil, fmt.Errorf("the SVG refers to another file (%s); embed it as a data: address", clip(string(m[1]), 60))
		}
	}
	return clean, nil
}

func (s *Server) handleSVGToPNG(w http.ResponseWriter, r *http.Request, in uploadInput) {
	svg, err := cleanSVG(in.Files[0].Data)
	if err != nil {
		s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", strings.ToUpper(err.Error()[:1])+err.Error()[1:]+".")
		return
	}
	zoom := 1
	if raw := in.option(r, "zoom"); raw != "" {
		if zoom, err = strconv.Atoi(raw); err != nil || zoom < 1 || zoom > 8 {
			s.badRequest(w, r, "zoom is 1 to 8.")
			return
		}
	}
	width, height := 0.0, 0.0
	for _, m := range svgDimension.FindAllSubmatch(svg, 2) {
		v, _ := strconv.ParseFloat(string(m[2]), 64)
		if strings.EqualFold(string(m[1]), "width") {
			width = v
		} else {
			height = v
		}
	}
	if width*float64(zoom) > 8000 || height*float64(zoom) > 8000 {
		s.badRequest(w, r, "The picture would be more than 8,000 pixels wide or high.")
		return
	}
	if !s.extras.render.available() {
		s.fail(w, r, http.StatusServiceUnavailable, "not_available", "This server cannot draw PNG pictures yet.")
		return
	}
	out, err := s.extras.render.png(r.Context(), svg, zoom, true)
	if err != nil {
		s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "The SVG cannot be drawn.")
		return
	}
	sendFile(w, r, out, "image/png", "picture.png")
}

func (s *Server) handleImageWatermark(w http.ResponseWriter, r *http.Request, in uploadInput) {
	text := strings.Join(strings.Fields(drawable(in.option(r, "text"))), " ")
	if text == "" || utf8.RuneCountInString(text) > 80 {
		s.badRequest(w, r, "text is required: up to 80 characters, in any script.")
		return
	}
	picture, ok := s.picture(w, r, in)
	if !ok {
		return
	}
	defer freeLarge(picture)
	position := strings.ToLower(in.option(r, "position"))
	if position == "" {
		position = "bottom-right"
	}
	if position != "bottom-right" && position != "bottom-left" && position != "center" && position != "tile" {
		s.badRequest(w, r, "position is bottom-right, bottom-left, center or tile.")
		return
	}
	opacity := 0.5
	if raw := in.option(r, "opacity"); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v <= 0 || v > 1 {
			s.badRequest(w, r, "opacity is above 0 and at most 1.")
			return
		}
		opacity = v
	}
	picture = shrink(picture, 4000)
	b := picture.Bounds()
	var encoded bytes.Buffer
	png.Encode(&encoded, picture)
	var svg []byte
	s.extras.render.work(func() {
		d := newCanvas()
		d.printf(`<image width="%d" height="%d" href="data:image/png;base64,%s"/>`, b.Dx(), b.Dy(), base64.StdEncoding.EncodeToString(encoded.Bytes()))
		size := math.Max(12, float64(min(b.Dx(), b.Dy()))/18)
		style := typeset.Style{Bold: true}
		line := d.Shape(text, style)
		width := float64(line.Width) * size / typeset.Em
		d.printf(`<g opacity="%s">`, coordinate(opacity))
		margin := size
		switch position {
		case "bottom-right":
			textAt(d, text, style, size, float64(b.Dx())-margin-width/2, float64(b.Dy())-margin, "#ffffff", "")
		case "bottom-left":
			textAt(d, text, style, size, margin+width/2, float64(b.Dy())-margin, "#ffffff", "")
		case "center":
			big := size * 2
			textAt(d, text, style, big, float64(b.Dx())/2, float64(b.Dy())/2, "#ffffff", "")
		case "tile":
			for y := size * 2; y < float64(b.Dy()); y += size * 5 {
				for x := width / 2; x < float64(b.Dx())+width; x += width + size*3 {
					textAt(d, text, style, size, x, y, "#ffffff", "")
				}
			}
		}
		d.printf(`</g>`)
		svg = d.svg(b.Dx(), b.Dy(), text)
	})
	if !s.extras.render.available() {
		s.fail(w, r, http.StatusServiceUnavailable, "not_available", "This server cannot draw PNG pictures yet.")
		return
	}
	out, err := s.extras.render.png(r.Context(), svg, 1, true)
	if err != nil {
		s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "The picture cannot be drawn.")
		return
	}
	sendFile(w, r, out, "image/png", "watermarked.png")
}

// ---- PDF extras ----

func pdfTextDesc(position string, size int, opacity float64, rotation int) string {
	return fmt.Sprintf("font:Helvetica, points:%d, pos:%s, scale:1 abs, rot:%d, op:%s, fillc:#808080", size, position, rotation, strconv.FormatFloat(opacity, 'f', 2, 64))
}

func (s *Server) handlePDFWatermark(w http.ResponseWriter, r *http.Request, in uploadInput) {
	if !s.needPDF(w, r, in) {
		return
	}
	text := strings.TrimSpace(in.option(r, "text"))
	if text == "" || utf8.RuneCountInString(text) > 80 || strings.ContainsAny(text, ",%") {
		s.badRequest(w, r, "text is required: up to 80 characters, without commas or %.")
		return
	}
	for _, c := range text {
		if c > 0xFF {
			s.badRequest(w, r, "PDF watermarks take Latin letters only for now.")
			return
		}
	}
	pages, ok := pagesOption(in.option(r, "pages"))
	if !ok {
		s.badRequest(w, r, pagesHelp)
		return
	}
	ctx, cancel := pdfJob(r)
	defer cancel()
	wm, err := pdfapi.TextWatermark(ctx, text, pdfTextDesc("c", 48, 0.3, 45), true, false, types.POINTS, pdfConfig())
	if err != nil {
		s.badRequest(w, r, "The watermark cannot be made from that text.")
		return
	}
	var out bytes.Buffer
	if err := pdfapi.AddWatermarks(ctx, bytes.NewReader(in.Files[0].Data), &out, pages, wm, pdfConfig()); err != nil {
		s.pdfError(w, r, err)
		return
	}
	sendFile(w, r, out.Bytes(), "application/pdf", "watermarked.pdf")
}

func (s *Server) handlePDFPageNumbers(w http.ResponseWriter, r *http.Request, in uploadInput) {
	if !s.needPDF(w, r, in) {
		return
	}
	style := strings.ToLower(in.option(r, "style"))
	text := map[string]string{"": "%p", "number": "%p", "of": "Page %p of %P", "page": "Page %p"}[style]
	if text == "" {
		s.badRequest(w, r, "style is number, page or of.")
		return
	}
	position := map[string]string{"": "bc", "bottom-center": "bc", "bottom-right": "br", "bottom-left": "bl", "top-center": "tc", "top-right": "tr"}[strings.ToLower(in.option(r, "position"))]
	if position == "" {
		s.badRequest(w, r, "position is bottom-center, bottom-right, bottom-left, top-center or top-right.")
		return
	}
	ctx, cancel := pdfJob(r)
	defer cancel()
	desc := fmt.Sprintf("font:Helvetica, points:10, pos:%s, off:0 18, scale:1 abs, rot:0, op:1, fillc:#333333", position)
	if position[0] == 't' {
		desc = strings.Replace(desc, "off:0 18", "off:0 -18", 1)
	}
	wm, err := pdfapi.TextWatermark(ctx, text, desc, true, false, types.POINTS, pdfConfig())
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	var out bytes.Buffer
	if err := pdfapi.AddWatermarks(ctx, bytes.NewReader(in.Files[0].Data), &out, nil, wm, pdfConfig()); err != nil {
		s.pdfError(w, r, err)
		return
	}
	sendFile(w, r, out.Bytes(), "application/pdf", "numbered.pdf")
}

func (s *Server) handlePDFFromImages(w http.ResponseWriter, r *http.Request, in uploadInput) {
	pageSize := strings.ToUpper(in.option(r, "page"))
	if pageSize == "" {
		pageSize = "A4"
	}
	if pageSize != "A4" && pageSize != "LETTER" && pageSize != "FIT" {
		s.badRequest(w, r, "page is A4, Letter or fit (each page the size of its picture).")
		return
	}
	var readers []*bytes.Reader
	for _, file := range in.Files {
		picture, err := s.loadPicture(r.Context(), file.Data)
		if err != nil {
			s.notPicture(w, r)
			return
		}
		// pdfcpu takes JPEG, PNG and TIFF; everything else is sent as PNG.
		data := file.Data
		if !(bytes.HasPrefix(data, []byte{0xFF, 0xD8}) || bytes.HasPrefix(data, []byte("\x89PNG"))) {
			var b bytes.Buffer
			png.Encode(&b, picture)
			data = b.Bytes()
		}
		freeLarge(picture)
		readers = append(readers, bytes.NewReader(data))
	}
	description := "pos:c, sc:0.95 rel"
	if pageSize != "FIT" {
		description = "f:" + map[string]string{"A4": "A4", "LETTER": "Letter"}[pageSize] + ", " + description
	}
	imp, err := pdfapi.Import(description, types.POINTS)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	if pageSize == "FIT" {
		imp = pdfapi.DefaultImportConfig()
	}
	ctx, cancel := pdfJob(r)
	defer cancel()
	var out bytes.Buffer
	images := make([]io.Reader, len(readers))
	for i, reader := range readers {
		images[i] = reader
	}
	if err := pdfapi.ImportImages(ctx, nil, &out, images, imp, pdfConfig()); err != nil {
		s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "The pictures cannot be put in a PDF.")
		return
	}
	sendFile(w, r, out.Bytes(), "application/pdf", "pictures.pdf")
}

func (s *Server) handlePDFUnlock(w http.ResponseWriter, r *http.Request, in uploadInput) {
	if !s.needPDF(w, r, in) {
		return
	}
	password := in.option(r, "password")
	conf := pdfConfig()
	conf.UserPW, conf.OwnerPW = password, password
	ctx, cancel := pdfJob(r)
	defer cancel()
	var out bytes.Buffer
	if err := pdfapi.Decrypt(ctx, bytes.NewReader(in.Files[0].Data), &out, conf); err != nil {
		lower := strings.ToLower(err.Error())
		switch {
		case strings.Contains(lower, "not encrypted"):
			s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "The PDF has no password.")
		case strings.Contains(lower, "password") || strings.Contains(lower, "authenticat"):
			s.fail(w, r, http.StatusUnprocessableEntity, "wrong_password", "The password is wrong. Send the PDF's password as the password field.")
		default:
			s.pdfError(w, r, err)
		}
		return
	}
	sendFile(w, r, out.Bytes(), "application/pdf", "unlocked.pdf")
}

// ---- audio and video ----

var mediaFormats = map[string]struct {
	ext, mime string
	args      []string
	video     bool
}{
	"mp3":  {"mp3", "audio/mpeg", []string{"-vn", "-c:a", "libmp3lame", "-q:a", "4"}, false},
	"wav":  {"wav", "audio/wav", []string{"-vn", "-c:a", "pcm_s16le"}, false},
	"ogg":  {"ogg", "audio/ogg", []string{"-vn", "-c:a", "libopus", "-b:a", "96k"}, false},
	"m4a":  {"m4a", "audio/mp4", []string{"-vn", "-c:a", "aac", "-b:a", "128k"}, false},
	"flac": {"flac", "audio/flac", []string{"-vn", "-c:a", "flac"}, false},
	"mp4":  {"mp4", "video/mp4", []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", "26", "-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart"}, true},
	"webm": {"webm", "video/webm", []string{"-c:v", "libvpx-vp9", "-deadline", "realtime", "-cpu-used", "8", "-b:v", "0", "-crf", "36", "-c:a", "libopus"}, true},
	"gif":  {"gif", "image/gif", []string{"-vf", "fps=10,scale='min(480,iw)':-2:flags=lanczos", "-an"}, true},
}

const mediaMaxSeconds = 600

func mediaNames() []string {
	return []string{"mp3", "wav", "ogg", "m4a", "flac", "mp4", "webm", "gif"}
}

func (s *Server) ffmpeg(r *http.Request, in uploadInput, outName string, args func(input, output string) []string, timeout time.Duration) ([]byte, error) {
	dir, path, done, err := writeUpload(in.Files[0].Data, "in")
	if err != nil {
		return nil, err
	}
	defer done()
	output := filepath.Join(dir, outName)
	all := append([]string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y", "-threads", "2"}, args(path, output)...)
	if _, err := s.run(r.Context(), job{program: "ffmpeg", args: all, dir: dir, timeout: timeout}); err != nil {
		return nil, err
	}
	return os.ReadFile(output)
}

func (s *Server) handleMediaConvert(w http.ResponseWriter, r *http.Request, in uploadInput) {
	name := strings.ToLower(in.option(r, "format"))
	format, ok := mediaFormats[name]
	if !ok {
		s.badRequest(w, r, "format is one of "+strings.Join(mediaNames(), ", ")+".")
		return
	}
	out, err := s.ffmpeg(r, in, "out."+format.ext, func(input, output string) []string {
		args := []string{"-i", input, "-t", itoa(mediaMaxSeconds), "-map_metadata", "-1"}
		return append(append(args, format.args...), output)
	}, 120*time.Second)
	if err != nil {
		s.toolFailed(w, r, err, "convert the file")
		return
	}
	sendFile(w, r, out, format.mime, "converted."+format.ext)
}

// mediaTime reads 90, 1:30 or 00:01:30.5.
func mediaTime(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	var total float64
	for _, part := range strings.Split(raw, ":") {
		v, err := strconv.ParseFloat(part, 64)
		if err != nil || v < 0 {
			return 0, false
		}
		total = total*60 + v
	}
	return total, true
}

func (s *Server) handleMediaTrim(w http.ResponseWriter, r *http.Request, in uploadInput) {
	start, okStart := mediaTime(in.option(r, "start"))
	if in.option(r, "start") == "" {
		start, okStart = 0, true
	}
	end, okEnd := mediaTime(in.option(r, "end"))
	if !okStart || !okEnd || end <= start {
		s.badRequest(w, r, "start and end are times such as 90, 1:30 or 00:01:30.5, end after start.")
		return
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(in.Files[0].Name), "."))
	format, known := mediaFormats[ext]
	if !known || ext == "gif" {
		ext, format = "mp3", mediaFormats["mp3"]
		if in.option(r, "format") != "" {
			if f, ok := mediaFormats[strings.ToLower(in.option(r, "format"))]; ok {
				ext, format = strings.ToLower(in.option(r, "format")), f
			}
		}
	}
	out, err := s.ffmpeg(r, in, "out."+format.ext, func(input, output string) []string {
		args := []string{"-ss", strconv.FormatFloat(start, 'f', 3, 64), "-to", strconv.FormatFloat(end, 'f', 3, 64), "-i", input, "-map_metadata", "-1"}
		return append(append(args, format.args...), output)
	}, 120*time.Second)
	if err != nil {
		s.toolFailed(w, r, err, "trim the file")
		return
	}
	sendFile(w, r, out, format.mime, "trimmed."+ext)
}

func (s *Server) handleMediaWaveform(w http.ResponseWriter, r *http.Request, in uploadInput) {
	width, height := 1200, 200
	if raw := in.option(r, "width"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 100 && v <= 4000 {
			width = v
		} else {
			s.badRequest(w, r, "width is 100 to 4000.")
			return
		}
	}
	if raw := in.option(r, "height"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 40 && v <= 1000 {
			height = v
		} else {
			s.badRequest(w, r, "height is 40 to 1000.")
			return
		}
	}
	colour := "#2563eb"
	if raw := in.option(r, "color"); raw != "" {
		c, err := devtools.ParseColor(raw)
		if err != nil {
			s.badRequest(w, r, "color is a colour such as 2563eb.")
			return
		}
		colour = c.Hex()
	}
	out, err := s.ffmpeg(r, in, "wave.png", func(input, output string) []string {
		return []string{"-t", itoa(mediaMaxSeconds), "-i", input, "-filter_complex",
			fmt.Sprintf("aformat=channel_layouts=mono,showwavespic=s=%dx%d:colors=%s", width, height, strings.TrimPrefix(colour, "#")), "-frames:v", "1", output}
	}, 60*time.Second)
	if err != nil {
		s.toolFailed(w, r, err, "draw the waveform")
		return
	}
	sendFile(w, r, out, "image/png", "waveform.png")
}

func (s *Server) handleMediaThumbnail(w http.ResponseWriter, r *http.Request, in uploadInput) {
	at := 1.0
	if raw := in.option(r, "at"); raw != "" {
		var ok bool
		if at, ok = mediaTime(raw); !ok {
			s.badRequest(w, r, "at is a time such as 5, 0:05 or 00:00:05.")
			return
		}
	}
	width := 640
	if raw := in.option(r, "width"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 32 && v <= 3840 {
			width = v
		} else {
			s.badRequest(w, r, "width is 32 to 3840.")
			return
		}
	}
	out, err := s.ffmpeg(r, in, "frame.jpg", func(input, output string) []string {
		return []string{"-ss", strconv.FormatFloat(at, 'f', 3, 64), "-i", input, "-frames:v", "1", "-vf", "scale=" + itoa(width) + ":-2", "-q:v", "3", output}
	}, 60*time.Second)
	if err != nil || len(out) == 0 {
		if err == nil {
			s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "There is no picture at that time.")
			return
		}
		s.toolFailed(w, r, err, "take a picture from the video")
		return
	}
	sendFile(w, r, out, "image/jpeg", "thumbnail.jpg")
}

// ---- spreadsheets ----

const sheetMaxCells = 500_000

func (s *Server) handleSheetToJSON(w http.ResponseWriter, r *http.Request, in uploadInput) {
	data := in.Files[0].Data
	header := in.option(r, "header") != "false"
	sheets := map[string][][]string{}
	var order []string
	if bytes.HasPrefix(data, []byte("PK")) {
		book, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{UnzipSizeLimit: 200 << 20, UnzipXMLSizeLimit: 50 << 20})
		if err != nil {
			s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "The file is not an XLSX workbook this server can read.")
			return
		}
		defer book.Close()
		cells := 0
		for _, name := range book.GetSheetList() {
			if only := in.option(r, "sheet"); only != "" && only != name {
				continue
			}
			rows, err := book.GetRows(name)
			if err != nil {
				continue
			}
			for _, row := range rows {
				cells += len(row)
			}
			if cells > sheetMaxCells {
				s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "The workbook has more than 5,00,000 cells.")
				return
			}
			sheets[name], order = rows, append(order, name)
		}
	} else {
		reader := csv.NewReader(bytes.NewReader(data))
		reader.FieldsPerRecord = -1
		rows, err := reader.ReadAll()
		if err != nil {
			s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "The file is neither an XLSX workbook nor CSV.")
			return
		}
		sheets["Sheet1"], order = rows, []string{"Sheet1"}
	}
	out := []map[string]any{}
	for _, name := range order {
		rows := sheets[name]
		entry := map[string]any{"name": name, "rows": len(rows)}
		if header && len(rows) > 0 {
			columns := rows[0]
			records := make([]map[string]string, 0, len(rows)-1)
			for _, row := range rows[1:] {
				record := map[string]string{}
				for i, c := range columns {
					if c == "" {
						c = "column" + itoa(i+1)
					}
					if i < len(row) {
						record[c] = row[i]
					} else {
						record[c] = ""
					}
				}
				records = append(records, record)
			}
			entry["columns"], entry["records"] = columns, records
		} else {
			entry["values"] = rows
		}
		out = append(out, entry)
	}
	s.ok(w, r, out, meta{Note: "Cell values as the sheet shows them (formatted). With header=false, rows of values instead of records named by the first row."}, "private, no-store")
}

func (s *Server) handleSheetFromJSON(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	raw, ok := s.readJSONBody(w, r)
	if !ok {
		return
	}
	var body struct {
		Sheets []struct {
			Name    string           `json:"name"`
			Columns []string         `json:"columns"`
			Records []map[string]any `json:"records"`
			Values  [][]any          `json:"values"`
		} `json:"sheets"`
		Records []map[string]any `json:"records"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		s.badRequest(w, r, `Send JSON: {"records": [{...}, ...]} for one sheet, or {"sheets": [{"name": "...", "records": [...]}]}.`)
		return
	}
	if len(body.Sheets) == 0 && body.Records != nil {
		body.Sheets = append(body.Sheets, struct {
			Name    string           `json:"name"`
			Columns []string         `json:"columns"`
			Records []map[string]any `json:"records"`
			Values  [][]any          `json:"values"`
		}{Name: "Sheet1", Records: body.Records})
	}
	if len(body.Sheets) == 0 || len(body.Sheets) > 50 {
		s.badRequest(w, r, "Give records for one sheet, or 1 to 50 sheets.")
		return
	}
	book := excelize.NewFile()
	defer book.Close()
	for i, sheet := range body.Sheets {
		name := strings.TrimSpace(sheet.Name)
		if name == "" {
			name = "Sheet" + itoa(i+1)
		}
		if len([]rune(name)) > 31 || strings.ContainsAny(name, `:\/?*[]`) {
			s.badRequest(w, r, "A sheet name is at most 31 characters, without : \\ / ? * [ ].")
			return
		}
		if i == 0 {
			book.SetSheetName("Sheet1", name)
		} else if _, err := book.NewSheet(name); err != nil {
			s.badRequest(w, r, "Two sheets have the same name.")
			return
		}
		rows := sheet.Values
		if rows == nil {
			columns := sheet.Columns
			if columns == nil {
				seen := map[string]bool{}
				for _, rec := range sheet.Records {
					for k := range rec {
						if !seen[k] {
							seen[k] = true
							columns = append(columns, k)
						}
					}
				}
				sort.Strings(columns)
			}
			head := make([]any, len(columns))
			for j, c := range columns {
				head[j] = c
			}
			rows = append(rows, head)
			for _, rec := range sheet.Records {
				row := make([]any, len(columns))
				for j, c := range columns {
					row[j] = rec[c]
				}
				rows = append(rows, row)
			}
		}
		if len(rows)*max(1, len(rows[0])) > sheetMaxCells {
			s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "At most 5,00,000 cells.")
			return
		}
		for y, row := range rows {
			cell, _ := excelize.CoordinatesToCellName(1, y+1)
			values := make([]any, len(row))
			for x, v := range row {
				switch t := v.(type) {
				case map[string]any, []any:
					encoded, _ := json.Marshal(t)
					values[x] = string(encoded)
				default:
					values[x] = t
				}
			}
			book.SetSheetRow(name, cell, &values)
		}
		if sheet.Values == nil {
			book.SetPanes(name, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
		}
	}
	var out bytes.Buffer
	if err := book.Write(&out); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	sendFile(w, r, out.Bytes(), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "sheet.xlsx")
}
