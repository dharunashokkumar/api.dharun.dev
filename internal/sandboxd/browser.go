//go:build linux

package sandboxd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/sandbox"
)

// browserTimeout bounds one page: boot, Chromium's start, the load and the capture.
const browserTimeout = 45 * time.Second

// errPage is a page that could not be captured, for the caller to see.
type errPage struct{ message string }

func (e *errPage) Error() string { return e.message }

// Browser opens a page in Chromium in a fresh microVM and returns a screenshot, a PDF or
// the page's text. The VM's traffic leaves only through the proxy: public addresses on
// ports 80 and 443, a few pages a minute per site.
func (c *Controller) Browser(ctx context.Context, req sandbox.BrowserRequest) (string, []byte, error) {
	switch req.Action {
	case sandbox.BrowserScreenshot, sandbox.BrowserPDF, sandbox.BrowserText:
	default:
		return "", nil, badRequest("action must be screenshot, pdf or text")
	}
	if (req.URL == "") == (req.HTML == "") {
		return "", nil, badRequest("give url or html")
	}
	spec, _ := c.cfg.spec(sandbox.ImageBrowser)
	ctx, cancel := context.WithTimeout(ctx, browserTimeout)
	defer cancel()
	cl, err := c.launch(ctx, spec, "browser")
	if err != nil {
		return "", nil, err
	}
	defer c.end(cl)
	if err := cl.vm.init(ctx, map[string]string{}, true); err != nil {
		return "", nil, fmt.Errorf("init: %w", err)
	}
	input, _ := json.Marshal(req)
	deadline, _ := ctx.Deadline()
	out, err := cl.vm.exec(ctx, sandbox.AgentRequest{Argv: []string{"/usr/local/bin/sandbox-browser"},
		TimeoutMillis: int(time.Until(deadline).Milliseconds()), MaxOutput: 40 << 20}, input)
	cpu := cl.vm.cpuSince()
	c.metrics.cpuSeconds.addf("browser", float64(cpu)/1000)
	outcome := "ok"
	defer func() { c.metrics.browser.add(req.Action+"/"+outcome, 1) }()
	if err != nil {
		outcome = "error"
		return "", nil, err
	}
	if out.status.TimedOut {
		outcome = "time"
		return "", nil, &errPage{"the page took too long"}
	}
	if out.status.Code != 0 {
		outcome = "failed"
		message := bytes.TrimSpace(out.stderr.Bytes())
		if len(message) > 300 {
			message = message[:300]
		}
		if len(message) == 0 {
			message = []byte("the page could not be loaded")
		}
		return "", nil, &errPage{string(message)}
	}
	// The helper writes the answer's media type on the first line, then the body.
	body := out.stdout.Bytes()
	newline := bytes.IndexByte(body, '\n')
	if newline < 0 {
		outcome = "error"
		return "", nil, errors.New("the browser helper gave no media type")
	}
	return string(body[:newline]), body[newline+1:], nil
}
