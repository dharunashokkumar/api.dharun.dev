package sandbox

import (
	"encoding/binary"
	"errors"
	"io"
)

// The controller talks to the agent inside each microVM over vsock: the controller opens
// a connection to AgentPort, writes one AgentRequest as a JSON line, and the two then
// exchange frames (one kind byte, a four-byte big-endian length, the payload) or, for
// files, plain bytes. One request per connection; several connections may be open.

// AgentPort is the vsock port the agent listens on. ProxyPort is the one it connects to
// on the host for the package proxy and the browser's traffic.
const (
	AgentPort = 1024
	ProxyPort = 3128
	// GuestProxy is where programs in the guest find the proxy.
	GuestProxy = "127.0.0.1:3128"
)

// Frame kinds. The controller sends stdin, closes it, resizes a terminal or kills the
// command; the agent sends output, the exit status, or an error.
const (
	FrameStdin      byte = 'i'
	FrameCloseStdin byte = 'c'
	FrameResize     byte = 'r'
	FrameKill       byte = 'k'
	FrameStdout     byte = 'o'
	FrameStderr     byte = 'e'
	FrameExit       byte = 'x'
	FrameError      byte = 'E'
)

// maxFrame bounds one frame's payload.
const maxFrame = 1 << 20

// ErrFrameTooLarge means a frame said it was larger than any frame may be.
var ErrFrameTooLarge = errors.New("frame too large")

// WriteFrame writes one frame.
func WriteFrame(w io.Writer, kind byte, payload []byte) error {
	header := make([]byte, 5, 5+len(payload))
	header[0] = kind
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	_, err := w.Write(append(header, payload...))
	return err
}

// ReadFrame reads one frame.
func ReadFrame(r io.Reader) (byte, []byte, error) {
	var header [5]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, err
	}
	size := binary.BigEndian.Uint32(header[1:])
	if size > maxFrame {
		return 0, nil, ErrFrameTooLarge
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return header[0], payload, nil
}

// Agent operations.
const (
	OpPing  = "ping"  // reply: the agent's facts
	OpInit  = "init"  // set the clock and seed the random pool; once, before anything else
	OpExec  = "exec"  // run a command; frames follow
	OpWrite = "write" // Size bytes follow; reply when written
	OpRead  = "read"  // reply with Size, then that many bytes
	OpList  = "list"  // reply with Entries
)

// TermSize is a terminal's size.
type TermSize struct {
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// AgentRequest is the first line on a connection to the agent.
type AgentRequest struct {
	Op string `json:"op"`

	// init
	TimeUnixNano int64             `json:"time,omitempty"`
	Entropy      []byte            `json:"entropy,omitempty"`
	Hostname     string            `json:"hostname,omitempty"`
	Proxy        bool              `json:"proxy,omitempty"` // start the local end of the package proxy
	Env          map[string]string `json:"env,omitempty"`   // init: set for every command; exec: added for this one

	// exec
	Argv          []string  `json:"argv,omitempty"`
	Cwd           string    `json:"cwd,omitempty"`
	TimeoutMillis int       `json:"timeoutMs,omitempty"`
	TTY           *TermSize `json:"tty,omitempty"`
	Root          bool      `json:"root,omitempty"` // run as root rather than the sandbox user
	MaxOutput     int       `json:"maxOutput,omitempty"`
	Stdin         bool      `json:"stdin,omitempty"` // stdin frames follow; otherwise stdin is empty

	// write, read, list
	Path string `json:"path,omitempty"`
	Mode uint32 `json:"mode,omitempty"`
	Size int64  `json:"size,omitempty"`
}

// ExitStatus ends a command's frames.
type ExitStatus struct {
	Code            int    `json:"code"`
	Signal          string `json:"signal,omitempty"`
	TimedOut        bool   `json:"timedOut,omitempty"`
	StdoutTruncated bool   `json:"stdoutTruncated,omitempty"`
	StderrTruncated bool   `json:"stderrTruncated,omitempty"`
	DurationMillis  int    `json:"durationMs"`
	// OutOfMemory is set when the kernel killed a process of the command for memory.
	OutOfMemory bool `json:"outOfMemory,omitempty"`
}

// Entry is one file in a directory listing.
type Entry struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Dir  bool   `json:"dir,omitempty"`
	Mode uint32 `json:"mode"`
}

// AgentReply answers ping, init, write, read and list.
type AgentReply struct {
	Error   string            `json:"error,omitempty"`
	Size    int64             `json:"size,omitempty"`
	Entries []Entry           `json:"entries,omitempty"`
	Facts   map[string]string `json:"facts,omitempty"` // ping: versions of what the image holds
}
