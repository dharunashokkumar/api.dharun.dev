package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// Client calls the controller over the private link. Every request carries the shared
// secret; the controller answers nothing without it.
type Client struct {
	base   string // http://<controller>:7070
	secret string
	http   *http.Client
}

// NewClient returns a client for the controller at base.
func NewClient(base, secret string) *Client {
	return &Client{
		base: strings.TrimRight(base, "/"), secret: secret,
		// Runs take up to RunTimeout plus a boot; a browser page about as long.
		http: &http.Client{Timeout: RunTimeout + 30*time.Second},
	}
}

// RemoteError is a refusal from the controller.
type RemoteError struct {
	Status int
	Detail Error
}

func (e *RemoteError) Error() string {
	return fmt.Sprintf("controller answered %d %s: %s", e.Status, e.Detail.Code, e.Detail.Message)
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.secret)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= 300 {
		defer response.Body.Close()
		remote := &RemoteError{Status: response.StatusCode}
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		if json.Unmarshal(raw, &remote.Detail) != nil || remote.Detail.Code == "" {
			remote.Detail = Error{Code: CodeFailed, Message: strings.TrimSpace(string(raw))}
		}
		return nil, remote
	}
	return response, nil
}

// call sends in as JSON and decodes the answer into out.
func (c *Client) call(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		encoded, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	response, err := c.do(ctx, method, path, body, "application/json")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if out == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(response.Body, 64<<20)).Decode(out)
}

// Status reports the controller's state.
func (c *Client) Status(ctx context.Context) (Status, error) {
	var status Status
	return status, c.call(ctx, http.MethodGet, "/status", nil, &status)
}

// Run runs code once in a fresh microVM.
func (c *Client) Run(ctx context.Context, request RunRequest) (Result, error) {
	var result Result
	return result, c.call(ctx, http.MethodPost, "/run", request, &result)
}

// CreateSession starts a sandbox that stays up until it expires or is stopped.
func (c *Client) CreateSession(ctx context.Context, request SessionRequest) (Session, error) {
	var session Session
	return session, c.call(ctx, http.MethodPost, "/sessions", request, &session)
}

// Session describes a running session, with the processor time used since last asked.
func (c *Client) Session(ctx context.Context, id string) (Session, error) {
	var session Session
	return session, c.call(ctx, http.MethodGet, "/sessions/"+url.PathEscape(id), nil, &session)
}

// Exec runs a command in a session.
func (c *Client) Exec(ctx context.Context, id string, request ExecRequest) (Result, error) {
	var result Result
	return result, c.call(ctx, http.MethodPost, "/sessions/"+url.PathEscape(id)+"/exec", request, &result)
}

// RunIn runs code in a session, which keeps its files and packages between runs.
func (c *Client) RunIn(ctx context.Context, id string, request RunRequest) (Result, error) {
	var result Result
	return result, c.call(ctx, http.MethodPost, "/sessions/"+url.PathEscape(id)+"/run", request, &result)
}

// PutFile writes a file into a session; a relative path is under /work.
func (c *Client) PutFile(ctx context.Context, id, path string, data []byte) error {
	response, err := c.do(ctx, http.MethodPut, "/sessions/"+url.PathEscape(id)+"/files?path="+url.QueryEscape(path),
		bytes.NewReader(data), "application/octet-stream")
	if err != nil {
		return err
	}
	response.Body.Close()
	return nil
}

// GetFile reads a file from a session.
func (c *Client) GetFile(ctx context.Context, id, path string) ([]byte, error) {
	response, err := c.do(ctx, http.MethodGet, "/sessions/"+url.PathEscape(id)+"/files?path="+url.QueryEscape(path), nil, "")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	return io.ReadAll(io.LimitReader(response.Body, MaxFileBytes+1))
}

// ListFiles lists a directory in a session; an empty path is /work.
func (c *Client) ListFiles(ctx context.Context, id, path string) ([]Entry, error) {
	entries := []Entry{}
	err := c.call(ctx, http.MethodGet, "/sessions/"+url.PathEscape(id)+"/files?list=true&path="+url.QueryEscape(path), nil, &entries)
	return entries, err
}

// Stop ends a session and reports the processor time it used since last asked.
func (c *Client) Stop(ctx context.Context, id string) (Session, error) {
	var session Session
	return session, c.call(ctx, http.MethodDelete, "/sessions/"+url.PathEscape(id), nil, &session)
}

// TTY opens a terminal in a session: binary messages are the terminal's bytes both ways,
// and a text message {"cols":80,"rows":24} resizes it.
func (c *Client) TTY(ctx context.Context, id string, cols, rows int) (*websocket.Conn, error) {
	address := strings.Replace(c.base, "http", "ws", 1) + "/sessions/" + url.PathEscape(id) + "/tty?cols=" +
		strconv.Itoa(cols) + "&rows=" + strconv.Itoa(rows)
	conn, response, err := websocket.Dial(ctx, address, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer " + c.secret}},
	})
	if err != nil {
		if response != nil && response.StatusCode >= 300 {
			return nil, &RemoteError{Status: response.StatusCode, Detail: Error{Code: CodeFailed, Message: "the terminal could not be opened"}}
		}
		return nil, err
	}
	conn.SetReadLimit(1 << 20)
	return conn, nil
}

// Browser asks Chromium for a screenshot, a PDF or a page's text. It returns the answer's
// media type and body.
func (c *Client) Browser(ctx context.Context, request BrowserRequest) (string, []byte, error) {
	encoded, err := json.Marshal(request)
	if err != nil {
		return "", nil, err
	}
	response, err := c.do(ctx, http.MethodPost, "/browser", bytes.NewReader(encoded), "application/json")
	if err != nil {
		return "", nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	return response.Header.Get("Content-Type"), body, err
}

// CreateDatabase makes a throwaway Postgres database and its user.
func (c *Client) CreateDatabase(ctx context.Context, account string) (Database, error) {
	var database Database
	return database, c.call(ctx, http.MethodPost, "/databases", map[string]string{"account": account}, &database)
}

// Database describes a database: its size now and when it goes.
func (c *Client) Database(ctx context.Context, name string) (Database, error) {
	var database Database
	return database, c.call(ctx, http.MethodGet, "/databases/"+url.PathEscape(name), nil, &database)
}

// DropDatabase deletes a database at once.
func (c *Client) DropDatabase(ctx context.Context, name string) error {
	return c.call(ctx, http.MethodDelete, "/databases/"+url.PathEscape(name), nil, nil)
}
