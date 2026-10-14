package api

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
	"golang.org/x/image/draw"
)

// Background removal. The picture is read and scaled here; the neural network (IS-Net,
// dataset "matting") runs in onnx-run, a small helper in the release that links ONNX
// Runtime (MIT) and runs one model on one input. It gets the picture as a 1024 by 1024
// tensor and answers with how likely each pixel belongs to the foreground; that mask,
// scaled back to the picture's size, becomes its transparency.

const matteMaxSide = 4096 // the largest picture returned

// matteInput turns a picture into IS-Net's input: RGB scaled to a square, each value
// divided by the brightest and centred on zero, channels first.
func matteInput(picture image.Image) []byte {
	const side = dataset.MattingSize
	square := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.CatmullRom.Scale(square, square.Bounds(), onWhite(picture), picture.Bounds(), draw.Src, nil)
	brightest := 1.0
	for i, v := range square.Pix {
		if i%4 != 3 && float64(v) > brightest {
			brightest = float64(v)
		}
	}
	values := make([]float32, 3*side*side)
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			offset := square.PixOffset(x, y)
			for channel := 0; channel < 3; channel++ {
				values[channel*side*side+y*side+x] = float32(float64(square.Pix[offset+channel])/brightest - 0.5)
			}
		}
	}
	out := make([]byte, 4*len(values))
	for i, v := range values {
		binary.LittleEndian.PutUint32(out[4*i:], math.Float32bits(v))
	}
	return out
}

// matteMask reads the model's answer and stretches it to the full range of grey.
func matteMask(raw []byte) (*image.Gray, bool) {
	const side = dataset.MattingSize
	if len(raw) != 4*side*side {
		return nil, false
	}
	values := make([]float32, side*side)
	low, high := float32(math.Inf(1)), float32(math.Inf(-1))
	for i := range values {
		v := math.Float32frombits(binary.LittleEndian.Uint32(raw[4*i:]))
		values[i] = v
		low, high = min(low, v), max(high, v)
	}
	mask := image.NewGray(image.Rect(0, 0, side, side))
	spread := high - low
	if spread <= 0 {
		spread = 1
	}
	for i, v := range values {
		mask.Pix[i] = uint8(math.Round(float64((v - low) / spread * 255)))
	}
	return mask, true
}

func (s *Server) handleRemoveBackground(w http.ResponseWriter, r *http.Request, in uploadInput) {
	// A large photo and its cut-out take a few hundred megabytes in this process; give
	// them back at once rather than when the collector next gets round to it, as the
	// next model job needs the room.
	defer debug.FreeOSMemory()
	format := strings.ToLower(in.option(r, "format"))
	if format == "" {
		format = "png"
	}
	if format != "png" && format != "webp" && format != "mask" {
		s.badRequest(w, r, "format must be png, webp, or mask (the mask alone, in grey).")
		return
	}
	set := s.need(w, r, "matting")
	if set == nil {
		return
	}
	model := set.attachment("isnet")
	picture, err := s.loadPicture(r.Context(), in.Files[0].Data)
	if err != nil {
		s.notPicture(w, r)
		return
	}
	bounds := picture.Bounds()
	if longest := max(bounds.Dx(), bounds.Dy()); longest > matteMaxSide {
		scale := float64(matteMaxSide) / float64(longest)
		smaller := image.NewNRGBA(image.Rect(0, 0, max(1, int(float64(bounds.Dx())*scale)), max(1, int(float64(bounds.Dy())*scale))))
		s.extras.render.work(func() { draw.CatmullRom.Scale(smaller, smaller.Bounds(), picture, bounds, draw.Src, nil) })
		picture, bounds = smaller, smaller.Bounds()
	}
	var input []byte
	s.extras.render.work(func() { input = matteInput(picture) })

	dir, done, err := workspace()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer done()
	side := strconv.Itoa(dataset.MattingSize)
	raw, err := s.run(r.Context(), job{
		program: "onnx-run",
		args:    []string{"--model", model, "--shape", "1,3," + side + "," + side, "--threads", itoa(min(runtime.NumCPU(), 8))},
		stdin:   input, dir: dir, timeout: 80 * time.Second,
	})
	if err != nil {
		s.toolFailed(w, r, err, "remove the background")
		return
	}
	small, ok := matteMask(raw)
	if !ok {
		s.toolFailed(w, r, os.ErrInvalid, "remove the background")
		return
	}

	var body []byte
	s.extras.render.work(func() {
		mask := image.NewGray(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
		draw.CatmullRom.Scale(mask, mask.Bounds(), small, small.Bounds(), draw.Src, nil)
		if format == "mask" {
			body, err = fastPNG(mask)
			return
		}
		cut := image.NewNRGBA(mask.Bounds())
		draw.Draw(cut, cut.Bounds(), picture, bounds.Min, draw.Src)
		for y := 0; y < bounds.Dy(); y++ {
			row := cut.Pix[y*cut.Stride:]
			for x := 0; x < bounds.Dx(); x++ {
				row[4*x+3] = uint8(uint16(row[4*x+3]) * uint16(mask.Pix[y*mask.Stride+x]) / 255)
			}
		}
		body, err = fastPNG(cut)
	})
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	if format == "webp" {
		png := filepath.Join(dir, "cut.png")
		if err := os.WriteFile(png, body, 0o600); err == nil {
			out := filepath.Join(dir, "cut.webp")
			if _, err := s.run(r.Context(), job{program: "vips", args: []string{"copy", png, out + "[Q=90,keep=none]"}, dir: dir, timeout: 30 * time.Second}); err == nil {
				if webp, err := os.ReadFile(out); err == nil {
					sendFile(w, r, webp, "image/webp", "cutout.webp")
					return
				}
			}
		}
		s.fail(w, r, http.StatusServiceUnavailable, "not_available", "WebP output is not available on this server; ask for png.")
		return
	}
	name := "cutout.png"
	if format == "mask" {
		name = "mask.png"
	}
	sendFile(w, r, body, "image/png", name)
}

// fastPNG favours speed over size: a cut-out photo is large, and compressing it hard
// takes seconds.
func fastPNG(picture image.Image) ([]byte, error) {
	var out bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	err := encoder.Encode(&out, picture)
	return out.Bytes(), err
}
