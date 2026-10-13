package api

import (
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

const (
	cacheLong  = "public, max-age=86400, stale-while-revalidate=604800"
	cacheAPI   = "public, max-age=3600"
	cacheShort = "public, max-age=300"
	cacheNone  = "no-store"

	typeJSON = "application/json; charset=utf-8"
)

// envelope is the shape of every successful JSON response.
type envelope struct {
	Data any  `json:"data"`
	Meta meta `json:"meta"`
}

// meta carries what a caller needs besides the data itself: paging and the credit
// the data's licence asks for.
type meta struct {
	Total  *int       `json:"total,omitempty"`
	Limit  *int       `json:"limit,omitempty"`
	Offset *int       `json:"offset,omitempty"`
	Source *sourceRef `json:"source,omitempty"`
	Note   string     `json:"note,omitempty"`
}

type sourceRef struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Licence     string `json:"licence"`
	Version     string `json:"version,omitempty"`
	Retrieved   string `json:"retrieved,omitempty"`
	Attribution string `json:"attribution,omitempty"`
}

func (s *Server) source(key string) *sourceRef {
	source, ok := s.sourceOf(key)
	if !ok {
		return nil
	}
	return sourceFromDataset(source)
}

func sourceFromDataset(source dataset.Source) *sourceRef {
	return &sourceRef{
		Name:        source.Name,
		URL:         source.URL,
		Licence:     source.Licence,
		Version:     source.Version,
		Retrieved:   source.Retrieved,
		Attribution: source.Attribution,
	}
}

func paging(total, limit, offset int) meta {
	return meta{Total: &total, Limit: &limit, Offset: &offset}
}

// ok writes a successful envelope.
func (s *Server) ok(w http.ResponseWriter, r *http.Request, data any, m meta, cacheControl string) {
	// Plain & < > in URLs and names: this is an API, not markup embedded in a page.
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(envelope{Data: data, Meta: m}); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	writeBody(w, r, http.StatusOK, body.Bytes(), typeJSON, cacheControl, true)
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// fail writes an error in the one shape every endpoint uses.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	var body errorBody
	body.Error.Code = code
	body.Error.Message = message
	encoded, _ := json.Marshal(body)
	cacheControl := cacheNone
	if status == http.StatusNotFound {
		cacheControl = "public, max-age=60"
	}
	writeBody(w, r, status, append(encoded, '\n'), typeJSON, cacheControl, false)
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request, message string) {
	s.fail(w, r, http.StatusNotFound, "not_found", message)
}

func (s *Server) badRequest(w http.ResponseWriter, r *http.Request, message string) {
	s.fail(w, r, http.StatusBadRequest, "bad_request", message)
}

// writeBody sends a complete body with validators. Successful responses get an ETag
// and answer If-None-Match; text bodies over a kilobyte are gzipped when accepted.
func writeBody(w http.ResponseWriter, r *http.Request, status int, body []byte, contentType, cacheControl string, compressible bool) {
	header := w.Header()
	header.Set("Content-Type", contentType)
	header.Set("Cache-Control", cacheControl)

	if status == http.StatusOK {
		sum := sha1.Sum(body)
		etag := `"` + base64.RawURLEncoding.EncodeToString(sum[:15]) + `"`
		header.Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	if compressible && len(body) > 1024 {
		header.Set("Vary", "Accept-Encoding")
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			var buffer bytes.Buffer
			writer, _ := gzip.NewWriterLevel(&buffer, gzip.BestSpeed)
			writer.Write(body)
			writer.Close()
			body = buffer.Bytes()
			header.Set("Content-Encoding", "gzip")
		}
	}

	header.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		w.Write(body)
	}
}
