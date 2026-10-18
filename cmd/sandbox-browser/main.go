//go:build linux

// Command sandbox-browser runs inside a browser microVM. It reads a sandbox.BrowserRequest
// on standard input, opens the page in headless Chromium through the VM's proxy, and
// writes the answer's media type on one line followed by the answer: a PNG, JPEG or WebP
// screenshot, a PDF, or the page's text as JSON. It talks to Chromium over the DevTools
// protocol on 127.0.0.1; pages cannot reach that port, as every connection they make,
// loopback included, goes through the proxy, which refuses private addresses.
package main

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coder/websocket"

	"github.com/dharunashokkumar/api.dharun.dev/internal/sandbox"
)

const (
	debugPort   = "9222"
	loadTimeout = 25 * time.Second
	maxHeight   = 16384
)

//go:embed text.js
var textScript string

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var req sandbox.BrowserRequest
	if err := json.NewDecoder(io.LimitReader(os.Stdin, 16<<20)).Decode(&req); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	width, height := clamp(req.Width, 320, 3840, 1280), clamp(req.Height, 200, 2160, 800)
	if req.Mobile && req.Width == 0 {
		width, height = 390, 844
	}
	target := req.URL
	if req.HTML != "" {
		if err := os.WriteFile("/tmp/page.html", []byte(req.HTML), 0o644); err != nil {
			return err
		}
		target = "file:///tmp/page.html"
	}

	chrome := exec.Command("chromium",
		"--headless=new", "--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage",
		"--remote-debugging-address=127.0.0.1", "--remote-debugging-port="+debugPort,
		"--user-data-dir=/tmp/chrome", "--proxy-server=http://"+sandbox.GuestProxy, "--proxy-bypass-list=<-loopback>",
		"--force-webrtc-ip-handling-policy=disable_non_proxied_udp",
		"--no-first-run", "--no-default-browser-check", "--disable-extensions", "--disable-sync", "--disable-default-apps",
		"--disable-background-networking", "--disable-component-update", "--mute-audio", "--hide-scrollbars",
		"--font-render-hinting=none", "--lang=en-IN", fmt.Sprintf("--window-size=%d,%d", width, height), "about:blank")
	chrome.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := chrome.Start(); err != nil {
		return fmt.Errorf("chromium: %w", err)
	}
	defer syscall.Kill(-chrome.Process.Pid, syscall.SIGKILL)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	browser, err := connect(ctx)
	if err != nil {
		return err
	}
	defer browser.close()
	var created struct{ TargetID string }
	if err := browser.call(ctx, "", "Target.createTarget", map[string]any{"url": "about:blank"}, &created); err != nil {
		return err
	}
	var attached struct{ SessionID string }
	if err := browser.call(ctx, "", "Target.attachToTarget", map[string]any{"targetId": created.TargetID, "flatten": true}, &attached); err != nil {
		return err
	}
	page := attached.SessionID
	for _, method := range []string{"Page.enable", "Network.enable", "Runtime.enable"} {
		if err := browser.call(ctx, page, method, nil, nil); err != nil {
			return err
		}
	}
	if err := browser.call(ctx, page, "Emulation.setDeviceMetricsOverride", map[string]any{
		"width": width, "height": height, "deviceScaleFactor": 1, "mobile": req.Mobile,
	}, nil); err != nil {
		return err
	}
	// The browser names itself, as robots.txt is read for that name: Chromium's own user
	// agent with "india-api-browser" and where to read about it.
	var version struct{ UserAgent string }
	browser.call(ctx, "", "Browser.getVersion", nil, &version)
	agent := strings.Replace(version.UserAgent, "HeadlessChrome", "Chrome", 1)
	if req.Mobile {
		agent = "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + chromeVersion(agent) + " Mobile Safari/537.36"
	}
	browser.call(ctx, page, "Emulation.setUserAgentOverride", map[string]any{
		"userAgent": agent + " india-api-browser (+https://api.dharun.dev/docs/browser)",
	}, nil)

	loaded := browser.subscribe("Page.loadEventFired")
	var navigated struct {
		FrameID   string
		ErrorText string
	}
	if err := browser.call(ctx, page, "Page.navigate", map[string]any{"url": target}, &navigated); err != nil {
		return err
	}
	if navigated.ErrorText != "" {
		return fmt.Errorf("the page could not be opened: %s", navigated.ErrorText)
	}
	select {
	case <-loaded:
	case <-time.After(loadTimeout):
		// A page that never finishes loading is captured as it stands.
	case <-ctx.Done():
		return errors.New("the page took too long")
	}
	time.Sleep(time.Duration(clamp(req.WaitMillis, 0, 10000, 500)) * time.Millisecond)

	switch req.Action {
	case sandbox.BrowserScreenshot:
		return screenshot(ctx, browser, page, req, width, height)
	case sandbox.BrowserPDF:
		return printPDF(ctx, browser, page, req)
	case sandbox.BrowserText:
		return pageText(ctx, browser, page, req, browser.status(navigated.FrameID))
	}
	return errors.New("unknown action")
}

// chromeVersion is the version in a Chrome user agent, such as 130.0.6723.58.
func chromeVersion(agent string) string {
	_, after, found := strings.Cut(agent, "Chrome/")
	if !found {
		return "130.0.0.0"
	}
	version, _, _ := strings.Cut(after, " ")
	return version
}

func clamp(value, low, high, fallback int) int {
	if value == 0 {
		return fallback
	}
	return max(low, min(high, value))
}

func emit(contentType string, body []byte) error {
	if _, err := os.Stdout.WriteString(contentType + "\n"); err != nil {
		return err
	}
	_, err := os.Stdout.Write(body)
	return err
}

func screenshot(ctx context.Context, b *cdp, page string, req sandbox.BrowserRequest, width, height int) error {
	format := req.Format
	switch format {
	case "", "png":
		format = "png"
	case "jpeg", "jpg":
		format = "jpeg"
	case "webp":
	default:
		return errors.New("format must be png, jpeg or webp")
	}
	params := map[string]any{"format": format}
	if format != "png" {
		params["quality"] = 85
	}
	if req.FullPage {
		var metrics struct {
			CSSContentSize struct{ Width, Height float64 }
		}
		if err := b.call(ctx, page, "Page.getLayoutMetrics", nil, &metrics); err == nil {
			full := int(math.Min(math.Ceil(metrics.CSSContentSize.Height), maxHeight))
			if full > height {
				b.call(ctx, page, "Emulation.setDeviceMetricsOverride", map[string]any{
					"width": width, "height": full, "deviceScaleFactor": 1, "mobile": req.Mobile,
				}, nil)
				params["captureBeyondViewport"] = true
			}
		}
	}
	var shot struct{ Data string }
	if err := b.call(ctx, page, "Page.captureScreenshot", params, &shot); err != nil {
		return err
	}
	image, err := base64.StdEncoding.DecodeString(shot.Data)
	if err != nil {
		return err
	}
	return emit("image/"+format, image)
}

// Paper sizes in inches.
var papers = map[string][2]float64{"a4": {8.27, 11.69}, "letter": {8.5, 11}, "legal": {8.5, 14}, "a3": {11.69, 16.54}, "a5": {5.83, 8.27}}

func printPDF(ctx context.Context, b *cdp, page string, req sandbox.BrowserRequest) error {
	paper, ok := papers[req.Paper]
	if req.Paper == "" {
		paper, ok = papers["a4"], true
	}
	if !ok {
		return errors.New("paper must be a4, a3, a5, letter or legal")
	}
	var printed struct{ Data string }
	if err := b.call(ctx, page, "Page.printToPDF", map[string]any{
		"paperWidth": paper[0], "paperHeight": paper[1], "landscape": req.Landscape, "printBackground": req.Background,
		"marginTop": 0.4, "marginBottom": 0.4, "marginLeft": 0.4, "marginRight": 0.4, "preferCSSPageSize": true,
	}, &printed); err != nil {
		return err
	}
	pdf, err := base64.StdEncoding.DecodeString(printed.Data)
	if err != nil {
		return err
	}
	return emit("application/pdf", pdf)
}

func pageText(ctx context.Context, b *cdp, page string, req sandbox.BrowserRequest, status int) error {
	var evaluated struct {
		Result struct {
			Value sandbox.PageText
		}
		ExceptionDetails *struct{ Text string }
	}
	expression := "(" + textScript + ")(" + strconv.FormatBool(req.Markdown) + ")"
	if err := b.call(ctx, page, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true}, &evaluated); err != nil {
		return err
	}
	if evaluated.ExceptionDetails != nil {
		return errors.New("the page's text could not be read")
	}
	text := evaluated.Result.Value
	text.Status = status
	text.Format = "text"
	if req.Markdown {
		text.Format = "markdown"
	}
	if req.HTML != "" {
		text.URL = ""
	}
	encoded, _ := json.Marshal(text)
	return emit("application/json", encoded)
}

// cdp is a DevTools protocol connection to the browser.
type cdp struct {
	conn    *websocket.Conn
	mu      sync.Mutex
	next    int
	waiting map[int]chan message
	events  map[string][]chan struct{}
	// statuses holds the HTTP status of each frame's document.
	statuses map[string]int
}

type message struct {
	ID        int             `json:"id"`
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params"`
	Result    json.RawMessage `json:"result"`
	SessionID string          `json:"sessionId"`
	Error     *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func connect(ctx context.Context) (*cdp, error) {
	var version struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}}
	for {
		response, err := client.Get("http://127.0.0.1:" + debugPort + "/json/version")
		if err == nil {
			err = json.NewDecoder(response.Body).Decode(&version)
			response.Body.Close()
			if err == nil && version.WebSocketDebuggerURL != "" {
				break
			}
		}
		select {
		case <-ctx.Done():
			return nil, errors.New("chromium did not start")
		case <-time.After(50 * time.Millisecond):
		}
	}
	conn, _, err := websocket.Dial(ctx, version.WebSocketDebuggerURL, nil)
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(64 << 20)
	b := &cdp{conn: conn, waiting: map[int]chan message{}, events: map[string][]chan struct{}{}, statuses: map[string]int{}}
	go b.read()
	return b, nil
}

func (b *cdp) read() {
	for {
		_, data, err := b.conn.Read(context.Background())
		if err != nil {
			b.mu.Lock()
			for id, waiter := range b.waiting {
				close(waiter)
				delete(b.waiting, id)
			}
			b.mu.Unlock()
			return
		}
		var msg message
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		b.mu.Lock()
		if msg.ID != 0 {
			if waiter, ok := b.waiting[msg.ID]; ok {
				waiter <- msg
				delete(b.waiting, msg.ID)
			}
		} else {
			if msg.Method == "Network.responseReceived" {
				var params struct {
					Type     string
					FrameID  string
					Response struct{ Status int }
				}
				if json.Unmarshal(msg.Params, &params) == nil && params.Type == "Document" {
					b.statuses[params.FrameID] = params.Response.Status
				}
			}
			for _, subscriber := range b.events[msg.Method] {
				select {
				case subscriber <- struct{}{}:
				default:
				}
			}
		}
		b.mu.Unlock()
	}
}

func (b *cdp) subscribe(method string) chan struct{} {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	b.events[method] = append(b.events[method], ch)
	b.mu.Unlock()
	return ch
}

func (b *cdp) status(frame string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.statuses[frame]
}

func (b *cdp) call(ctx context.Context, session, method string, params any, result any) error {
	b.mu.Lock()
	b.next++
	id := b.next
	waiter := make(chan message, 1)
	b.waiting[id] = waiter
	b.mu.Unlock()
	request := map[string]any{"id": id, "method": method}
	if params != nil {
		request["params"] = params
	}
	if session != "" {
		request["sessionId"] = session
	}
	encoded, _ := json.Marshal(request)
	if err := b.conn.Write(ctx, websocket.MessageText, encoded); err != nil {
		return err
	}
	select {
	case msg, ok := <-waiter:
		if !ok {
			return errors.New("chromium went away")
		}
		if msg.Error != nil {
			return fmt.Errorf("%s: %s", method, msg.Error.Message)
		}
		if result != nil && len(msg.Result) > 0 {
			return json.Unmarshal(msg.Result, result)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%s: %w", method, ctx.Err())
	}
}

func (b *cdp) close() { b.conn.CloseNow() }
