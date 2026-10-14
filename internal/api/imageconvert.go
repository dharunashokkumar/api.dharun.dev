package api

import (
	"bufio"
	"bytes"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Image information, resizing and conversion, done by libvips (the vips program, LGPL,
// installed on the server). libvips reads JPEG, PNG, WebP, AVIF, HEIC, TIFF, GIF and more
// and streams through a picture, so a large photo takes little memory. Metadata (camera,
// GPS position) is always dropped from what is returned.

// imageFormats are the formats a picture can be converted to, with the save options
// vips takes for each. %d is the quality.
var imageFormats = map[string]struct {
	extension, mediaType, options string
}{
	"jpeg": {".jpg", "image/jpeg", "Q=%d,keep=none,optimize_coding,interlace,background=255"},
	"png":  {".png", "image/png", "keep=none,compression=9"},
	"webp": {".webp", "image/webp", "Q=%d,keep=none"},
	"avif": {".avif", "image/avif", "Q=%d,keep=none,compression=av1,effort=2"},
	"gif":  {".gif", "image/gif", "keep=none"},
	"tiff": {".tif", "image/tiff", "keep=none,compression=deflate"},
}

var imageFormatNames = []string{"jpeg", "png", "webp", "avif", "gif", "tiff"}

// vipsLoaders names the format of a file from the loader vips chose for it.
var vipsLoaders = map[string]string{
	"jpegload": "jpeg", "pngload": "png", "webpload": "webp", "gifload": "gif", "tiffload": "tiff",
	"heifload": "heif", "svgload": "svg", "jxlload": "jxl", "pdfload": "pdf", "magickload": "other",
	"ppmload": "ppm", "radload": "hdr", "fitsload": "fits", "jp2kload": "jpeg2000",
}

type pictureInfo struct {
	Format      string `json:"format"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Bands       int    `json:"bands"`
	Alpha       bool   `json:"alpha"`
	ColourSpace string `json:"colour_space"`
	Pages       int    `json:"pages"`
	Orientation int    `json:"orientation"`
	HasExif     bool   `json:"has_exif"`
	HasLocation bool   `json:"has_location"`
	Camera      string `json:"camera,omitempty"`
	Bytes       int    `json:"bytes"`
}

// writeUpload saves an upload in a fresh workspace for a program to read.
func writeUpload(data []byte, name string) (dir, path string, done func(), err error) {
	dir, done, err = workspace()
	if err != nil {
		return "", "", nil, err
	}
	path = filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		done()
		return "", "", nil, err
	}
	return dir, path, done, nil
}

func (s *Server) handleImageInfo(w http.ResponseWriter, r *http.Request, in uploadInput) {
	dir, path, done, err := writeUpload(in.Files[0].Data, "in")
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer done()
	out, err := s.run(r.Context(), job{program: "vipsheader", args: []string{"-a", path}, dir: dir, timeout: 20 * time.Second})
	if err != nil {
		if errorsIsNoProgram(err) {
			s.toolFailed(w, r, err, "read pictures")
			return
		}
		s.notPicture(w, r)
		return
	}
	info := pictureInfo{Pages: 1, Orientation: 1, Bytes: len(in.Files[0].Data)}
	fields := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		name, value, ok := strings.Cut(scanner.Text(), ": ")
		if ok {
			fields[name] = value
		}
	}
	info.Width, _ = strconv.Atoi(fields["width"])
	info.Height, _ = strconv.Atoi(fields["height"])
	info.Bands, _ = strconv.Atoi(fields["bands"])
	if pages, err := strconv.Atoi(fields["n-pages"]); err == nil && pages > 0 {
		info.Pages = pages
	}
	if orientation, err := strconv.Atoi(fields["orientation"]); err == nil {
		info.Orientation = orientation
	}
	info.ColourSpace = fields["interpretation"]
	loader := strings.TrimSuffix(strings.TrimSuffix(fields["vips-loader"], "_buffer"), "_source")
	info.Format = vipsLoaders[loader]
	if info.Format == "" {
		info.Format = loader
	}
	if info.Format == "heif" && bytes.Contains(in.Files[0].Data[:min(64, len(in.Files[0].Data))], []byte("avif")) {
		info.Format = "avif"
	}
	info.Alpha = info.Bands == 2 || info.Bands == 4
	_, info.HasExif = fields["exif-data"]
	for name := range fields {
		if strings.Contains(name, "GPSLatitude") {
			info.HasLocation = true
		}
	}
	info.Camera = strings.TrimSpace(exifText(fields["exif-ifd0-Make"]) + " " + exifText(fields["exif-ifd0-Model"]))
	note := ""
	if info.HasLocation {
		note = "This picture carries the GPS position where it was taken. /v1/image/convert returns it without any metadata."
	}
	s.ok(w, r, info, meta{Note: note}, cacheNone)
}

// exifText takes the readable part of a vips EXIF field: "Canon (Canon, ASCII, 6 ...)".
func exifText(value string) string {
	text, _, _ := strings.Cut(value, " (")
	return strings.TrimSpace(text)
}

func errorsIsNoProgram(err error) bool { return errors.Is(err, errNoProgram) }

func (s *Server) handleImageConvert(w http.ResponseWriter, r *http.Request, in uploadInput) {
	format := strings.ToLower(in.option(r, "format"))
	if format == "jpg" {
		format = "jpeg"
	}
	if format == "" {
		format = "webp"
	}
	target, ok := imageFormats[format]
	if !ok {
		s.badRequest(w, r, "format must be one of "+strings.Join(imageFormatNames, ", ")+".")
		return
	}
	number := func(name string, fallback, low, high int) (int, bool) {
		raw := in.option(r, name)
		if raw == "" {
			return fallback, true
		}
		value, err := strconv.Atoi(raw)
		if err != nil || value < low || value > high {
			s.badRequest(w, r, name+" must be an integer from "+itoa(low)+" to "+itoa(high)+".")
			return 0, false
		}
		return value, true
	}
	width, ok := number("width", 0, 1, 8000)
	if !ok {
		return
	}
	height, ok := number("height", 0, 1, 8000)
	if !ok {
		return
	}
	quality, ok := number("quality", 80, 1, 100)
	if !ok {
		return
	}
	fit := strings.ToLower(in.option(r, "fit"))
	if fit == "" {
		fit = "inside"
	}
	if fit != "inside" && fit != "cover" && fit != "fill" {
		s.badRequest(w, r, "fit must be inside, cover or fill.")
		return
	}
	if fit != "inside" && (width == 0 || height == 0) {
		s.badRequest(w, r, "fit="+fit+" needs both width and height.")
		return
	}

	dir, path, done, err := writeUpload(in.Files[0].Data, "in")
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer done()
	// thumbnail turns the picture upright from its EXIF orientation and never makes it
	// larger than it is unless asked to fill a box.
	boxWidth, boxHeight := width, height
	if boxWidth == 0 {
		boxWidth = 100000
	}
	if boxHeight == 0 {
		boxHeight = 100000
	}
	options := target.options
	if strings.Contains(options, "%d") {
		options = strings.Replace(options, "%d", itoa(quality), 1)
	}
	out := filepath.Join(dir, "out"+target.extension)
	args := []string{"thumbnail", path, out + "[" + options + "]", itoa(boxWidth), "--height", itoa(boxHeight)}
	switch fit {
	case "inside":
		args = append(args, "--size", "down")
	case "cover":
		args = append(args, "--crop", "centre")
	case "fill":
		args = append(args, "--size", "force")
	}
	if _, err := s.run(r.Context(), job{program: "vips", args: args, dir: dir, timeout: 60 * time.Second}); err != nil {
		if errorsIsNoProgram(err) {
			s.toolFailed(w, r, err, "convert pictures")
			return
		}
		s.notPicture(w, r)
		return
	}
	body, err := os.ReadFile(out)
	if err != nil {
		s.notPicture(w, r)
		return
	}
	sendFile(w, r, body, target.mediaType, "image"+target.extension)
}

// sendFile answers with a file the request made. It is the caller's own, so it is never
// cached by anyone else; a download name lets a browser save it sensibly.
func sendFile(w http.ResponseWriter, r *http.Request, body []byte, mediaType, name string) {
	w.Header().Set("Content-Disposition", `inline; filename="`+name+`"`)
	writeBody(w, r, http.StatusOK, body, mediaType, "private, no-store", false)
}
