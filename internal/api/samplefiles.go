package api

import (
	"embed"
	"net/http"
	"path"
)

// Sample files for the upload endpoints: the documentation's examples post them, tests
// post them, and anyone can download one to try an endpoint. All were made for this
// project (by Typst, Piper's public-domain LJ Speech voice and this API's own QR code
// endpoint) and are MIT-0 like the guides.

//go:embed samples/*
var sampleFiles embed.FS

var sampleTypes = map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".pdf": "application/pdf", ".mp3": "audio/mpeg"}

func sampleFile(name string) ([]byte, bool) {
	data, err := sampleFiles.ReadFile("samples/" + path.Base(name))
	return data, err == nil
}

func (s *Server) handleSampleFile(w http.ResponseWriter, r *http.Request) {
	info(r).route = "samples"
	data, ok := sampleFile(r.PathValue("file"))
	if !ok {
		s.notFound(w, r, "No such sample file.")
		return
	}
	kind, ok := sampleTypes[path.Ext(r.PathValue("file"))]
	if !ok {
		kind = "application/octet-stream"
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+path.Base(r.PathValue("file"))+`"`)
	writeBody(w, r, http.StatusOK, data, kind, cacheLong, false)
}
