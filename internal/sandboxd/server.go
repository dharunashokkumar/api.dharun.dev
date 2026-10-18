//go:build linux

package sandboxd

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/dharunashokkumar/api.dharun.dev/internal/sandbox"
)

// Handler is the controller's API on the private link. Only the API calls it, with the
// shared secret.
func (c *Controller) Handler(secret string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) { answer(w, c.status()) })
	mux.HandleFunc("POST /run", func(w http.ResponseWriter, r *http.Request) {
		var req sandbox.RunRequest
		if !decode(w, r, &req) {
			return
		}
		result, err := c.Run(r.Context(), req)
		reply(w, result, err)
	})
	mux.HandleFunc("POST /sessions", func(w http.ResponseWriter, r *http.Request) {
		var req sandbox.SessionRequest
		if !decode(w, r, &req) {
			return
		}
		session, err := c.CreateSession(r.Context(), req)
		reply(w, session, err)
	})
	mux.HandleFunc("GET /sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		session, err := c.Session(r.PathValue("id"))
		reply(w, session, err)
	})
	mux.HandleFunc("DELETE /sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		session, err := c.StopSession(r.PathValue("id"))
		reply(w, session, err)
	})
	mux.HandleFunc("POST /sessions/{id}/exec", func(w http.ResponseWriter, r *http.Request) {
		var req sandbox.ExecRequest
		if !decode(w, r, &req) {
			return
		}
		result, err := c.Exec(r.Context(), r.PathValue("id"), req)
		reply(w, result, err)
	})
	mux.HandleFunc("POST /sessions/{id}/run", func(w http.ResponseWriter, r *http.Request) {
		var req sandbox.RunRequest
		if !decode(w, r, &req) {
			return
		}
		result, err := c.RunInSession(r.Context(), r.PathValue("id"), req)
		reply(w, result, err)
	})
	mux.HandleFunc("PUT /sessions/{id}/files", func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, sandbox.MaxFileBytes))
		if err != nil {
			reply(w, nil, badRequest("the file is over %d KB", sandbox.MaxFileBytes>>10))
			return
		}
		err = c.PutFile(r.Context(), r.PathValue("id"), r.URL.Query().Get("path"), data)
		reply(w, map[string]any{"written": len(data)}, err)
	})
	mux.HandleFunc("GET /sessions/{id}/files", func(w http.ResponseWriter, r *http.Request) {
		id, path := r.PathValue("id"), r.URL.Query().Get("path")
		if r.URL.Query().Get("list") == "true" {
			entries, err := c.ListFiles(r.Context(), id, path)
			reply(w, entries, err)
			return
		}
		data, err := c.GetFile(r.Context(), id, path)
		if err != nil {
			reply(w, nil, err)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(data)
	})
	mux.HandleFunc("GET /sessions/{id}/tty", func(w http.ResponseWriter, r *http.Request) { c.serveTTY(w, r) })
	mux.HandleFunc("POST /browser", func(w http.ResponseWriter, r *http.Request) {
		var req sandbox.BrowserRequest
		if !decode(w, r, &req) {
			return
		}
		contentType, body, err := c.Browser(r.Context(), req)
		if err != nil {
			reply(w, nil, err)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Write(body)
	})
	mux.HandleFunc("POST /databases", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Account string `json:"account"`
		}
		if !decode(w, r, &req) {
			return
		}
		if c.db == nil {
			reply(w, nil, errUnavailable)
			return
		}
		if c.pausedReason() != "" {
			reply(w, nil, errPaused)
			return
		}
		database, err := c.db.create(req.Account)
		reply(w, database, err)
	})
	mux.HandleFunc("GET /databases/{name}", func(w http.ResponseWriter, r *http.Request) {
		if c.db == nil {
			reply(w, nil, errUnavailable)
			return
		}
		database, err := c.db.get(r.PathValue("name"))
		reply(w, database, err)
	})
	mux.HandleFunc("DELETE /databases/{name}", func(w http.ResponseWriter, r *http.Request) {
		if c.db == nil {
			reply(w, nil, errUnavailable)
			return
		}
		if _, err := c.db.get(r.PathValue("name")); err != nil {
			reply(w, nil, err)
			return
		}
		err := c.db.drop(r.PathValue("name"))
		if err == nil {
			c.metrics.databases.add("dropped", 1)
		}
		reply(w, map[string]any{"dropped": err == nil}, err)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		given, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if secret == "" || subtle.ConstantTimeCompare([]byte(given), []byte(secret)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// MetricsHandler serves /metrics for Prometheus.
func (c *Controller) MetricsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		io.WriteString(w, c.renderMetrics())
	})
}

var errUnavailable = errors.New("throwaway databases are not set up on this server")

func decode(w http.ResponseWriter, r *http.Request, into any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(into); err != nil && !errors.Is(err, io.EOF) {
		reply(w, nil, badRequest("%v", err))
		return false
	}
	return true
}

func answer(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(value)
}

// reply answers a value, or the error as a sandbox.Error with the status it calls for.
func reply(w http.ResponseWriter, value any, err error) {
	if err == nil {
		answer(w, value)
		return
	}
	status, code := http.StatusInternalServerError, sandbox.CodeFailed
	var page *errPage
	switch {
	case errors.Is(err, errBusy):
		status, code = http.StatusServiceUnavailable, sandbox.CodeBusy
	case errors.Is(err, errPaused):
		status, code = http.StatusServiceUnavailable, sandbox.CodePaused
	case errors.Is(err, errNotFound):
		status, code = http.StatusNotFound, sandbox.CodeNotFound
	case errors.Is(err, errLimit), errors.Is(err, errTooMany):
		status, code = http.StatusTooManyRequests, sandbox.CodeLimit
	case errors.Is(err, errBadRequest):
		status, code = http.StatusBadRequest, sandbox.CodeBadRequest
	case errors.Is(err, errUnavailable):
		status, code = http.StatusServiceUnavailable, sandbox.CodeUnavailable
	case errors.As(err, &page):
		status, code = http.StatusUnprocessableEntity, sandbox.CodeFailed
	default:
		logf("error: %v", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(sandbox.Error{Code: code, Message: err.Error()})
}

// serveTTY carries a terminal between a WebSocket and the agent: binary messages are the
// terminal's bytes, a text message {"cols":..,"rows":..} resizes it.
func (c *Controller) serveTTY(w http.ResponseWriter, r *http.Request) {
	size := sandbox.TermSize{Cols: 80, Rows: 24}
	if cols, err := strconv.Atoi(r.URL.Query().Get("cols")); err == nil && cols > 0 && cols < 1000 {
		size.Cols = uint16(cols)
	}
	if rows, err := strconv.Atoi(r.URL.Query().Get("rows")); err == nil && rows > 0 && rows < 1000 {
		size.Rows = uint16(rows)
	}
	agent, err := c.OpenTTY(r.Context(), r.PathValue("id"), size)
	if err != nil {
		reply(w, nil, err)
		return
	}
	defer agent.Close()
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() {
		defer cancel()
		for {
			kind, data, err := conn.Read(ctx)
			if err != nil {
				sandbox.WriteFrame(agent, sandbox.FrameKill, nil)
				return
			}
			if kind == websocket.MessageText {
				sandbox.WriteFrame(agent, sandbox.FrameResize, data)
			} else {
				sandbox.WriteFrame(agent, sandbox.FrameStdin, data)
			}
		}
	}()
	for {
		kind, payload, err := sandbox.ReadFrame(agent)
		if err != nil {
			conn.Close(websocket.StatusNormalClosure, "")
			return
		}
		switch kind {
		case sandbox.FrameStdout, sandbox.FrameStderr:
			writeCtx, done := context.WithTimeout(ctx, 30*time.Second)
			err := conn.Write(writeCtx, websocket.MessageBinary, payload)
			done()
			if err != nil {
				return
			}
		case sandbox.FrameExit, sandbox.FrameError:
			conn.Close(websocket.StatusNormalClosure, "the shell ended")
			return
		}
	}
}
