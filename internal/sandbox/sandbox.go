// Package sandbox is what the API and the sandbox controller share: the requests and
// answers that cross the private link between them, the languages and images on offer,
// and a client for the controller. The controller (cmd/sandboxd) runs in its own VM and
// starts a Firecracker microVM for each run; see docs/sandbox-design.md.
package sandbox

import (
	"time"
)

// Images a sandbox can start from. code has the languages; the rest are distributions
// to try out in the terminal.
const (
	ImageCode    = "code"
	ImageBrowser = "browser" // Chromium, for the browser tools only
	ImageAlpine  = "alpine"
	ImageArch    = "arch"
	ImageDebian  = "debian"
	ImageFedora  = "fedora"
)

// Distributions lists the images people can open a session in besides code.
var Distributions = []string{ImageAlpine, ImageArch, ImageDebian, ImageFedora}

// SessionImages are the images a session may use.
var SessionImages = append([]string{ImageCode}, Distributions...)

// Language is one language the code image runs: the file the code is written to and the
// command that builds and runs it, in /work.
type Language struct {
	Name    string   `json:"name"`
	File    string   `json:"file"`
	Command []string `json:"command"`
	Version string   `json:"version,omitempty"` // filled in by the controller from the image
}

// Languages are the languages of the code image, by the names callers use.
var Languages = map[string]Language{
	"python":     {Name: "Python", File: "main.py", Command: []string{"python3", "main.py"}},
	"javascript": {Name: "JavaScript (Node.js)", File: "main.js", Command: []string{"node", "main.js"}},
	"go":         {Name: "Go", File: "main.go", Command: []string{"go", "run", "main.go"}},
	"c":          {Name: "C (GCC)", File: "main.c", Command: []string{"sh", "-c", "gcc -O2 -o /tmp/main main.c -lm && exec /tmp/main"}},
	"cpp":        {Name: "C++ (GCC)", File: "main.cpp", Command: []string{"sh", "-c", "g++ -O2 -o /tmp/main main.cpp && exec /tmp/main"}},
	"ruby":       {Name: "Ruby", File: "main.rb", Command: []string{"ruby", "main.rb"}},
	"php":        {Name: "PHP", File: "main.php", Command: []string{"php", "main.php"}},
	"bash":       {Name: "Bash", File: "main.sh", Command: []string{"bash", "main.sh"}},
}

// LanguageAliases are other names callers use for a language.
var LanguageAliases = map[string]string{
	"py": "python", "python3": "python", "js": "javascript", "node": "javascript", "nodejs": "javascript",
	"golang": "go", "c++": "cpp", "cxx": "cpp", "rb": "ruby", "sh": "bash", "shell": "bash",
}

// Limits of one run, and of a session.
const (
	RunTimeout       = 30 * time.Second
	MaxCodeBytes     = 64 << 10
	MaxOutputBytes   = 64 << 10 // of stdout and of stderr, each
	MaxFiles         = 20
	MaxFileBytes     = 1 << 20 // a file sent in or read back
	SessionLifetime  = 5 * time.Minute
	DistroLifetime   = 10 * time.Minute
	SessionsPerUser  = 2
	ExecTimeoutLimit = 60 * time.Second
	// DownloadCap is what a session may download through the package proxy.
	DownloadCap = 500 << 20
)

// File is a file put in the working directory before a run, or read back after.
type File struct {
	Path    string `json:"path"`
	Content []byte `json:"content"` // base64 in JSON
}

// RunRequest asks for code to be run once in a fresh microVM, which is then destroyed.
type RunRequest struct {
	Language string   `json:"language"`
	Code     string   `json:"code"`
	Stdin    string   `json:"stdin,omitempty"`
	Args     []string `json:"args,omitempty"`
	Files    []File   `json:"files,omitempty"`
	// Account names the caller in the controller's own log (a GitHub login).
	Account string `json:"account,omitempty"`
}

// Limits a run can hit, as reported in Result.LimitsHit.
const (
	LimitTime      = "time"
	LimitMemory    = "memory"
	LimitOutput    = "output"
	LimitProcesses = "processes"
	LimitDisk      = "disk"
)

// Result is what a run or a command in a session gave.
type Result struct {
	Stdout          string   `json:"stdout"`
	Stderr          string   `json:"stderr"`
	ExitCode        int      `json:"exitCode"`
	Signal          string   `json:"signal,omitempty"`
	DurationMillis  int      `json:"durationMs"`
	CPUMillis       int      `json:"cpuMs"`
	LimitsHit       []string `json:"limitsHit"`
	StdoutTruncated bool     `json:"stdoutTruncated,omitempty"`
	StderrTruncated bool     `json:"stderrTruncated,omitempty"`
	Files           []File   `json:"files,omitempty"`
}

// SessionRequest asks for a sandbox that stays up for a while.
type SessionRequest struct {
	Image   string `json:"image"`
	Account string `json:"account"`
	// Network gives the session the package proxy: each language's registry and nothing
	// else. Distributions have no network.
	Network bool `json:"network"`
}

// Session describes a running sandbox.
type Session struct {
	ID        string `json:"id"`
	Image     string `json:"image"`
	Network   bool   `json:"network"`
	CreatedAt string `json:"createdAt"`
	ExpiresAt string `json:"expiresAt"`
	// Account is whose session it is, as the API named it when it was made.
	Account string `json:"account,omitempty"`
	// CPUMillis is processor time used since the last time it was reported.
	CPUMillis int `json:"cpuMs,omitempty"`
}

// ExecRequest runs a command in a session: a shell line, or an argument list.
type ExecRequest struct {
	Command       string            `json:"command,omitempty"`
	Argv          []string          `json:"argv,omitempty"`
	Stdin         string            `json:"stdin,omitempty"`
	Cwd           string            `json:"cwd,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	TimeoutMillis int               `json:"timeoutMs,omitempty"`
}

// Browser actions.
const (
	BrowserScreenshot = "screenshot"
	BrowserPDF        = "pdf"
	BrowserText       = "text"
)

// BrowserRequest asks Chromium for a picture, a PDF or the text of a page. Either URL or
// HTML is set.
type BrowserRequest struct {
	Action     string `json:"action"`
	URL        string `json:"url,omitempty"`
	HTML       string `json:"html,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	FullPage   bool   `json:"fullPage,omitempty"`
	Format     string `json:"format,omitempty"` // png, jpeg, webp for a screenshot
	Paper      string `json:"paper,omitempty"`  // a4, letter, legal for a PDF
	Landscape  bool   `json:"landscape,omitempty"`
	Background bool   `json:"background,omitempty"` // print background colours and images
	WaitMillis int    `json:"waitMs,omitempty"`     // extra time after the page loads
	Mobile     bool   `json:"mobile,omitempty"`
	Markdown   bool   `json:"markdown,omitempty"` // text as Markdown rather than plain text
}

// PageText is the text of a page, for BrowserText.
type PageText struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	Text     string `json:"text"`
	Format   string `json:"format"` // text or markdown
	Language string `json:"language,omitempty"`
	Status   int    `json:"status,omitempty"`
}

// Database is a throwaway Postgres database.
type Database struct {
	Name       string `json:"name"`
	User       string `json:"user"`
	Password   string `json:"password,omitempty"`
	CreatedAt  string `json:"createdAt"`
	ExpiresAt  string `json:"expiresAt"`
	SizeBytes  int64  `json:"sizeBytes"`
	LimitBytes int64  `json:"limitBytes"`
	Account    string `json:"account,omitempty"`
}

// Database limits.
const (
	DatabaseLifetime    = time.Hour
	DatabaseLimit       = 100 << 20
	DatabaseConnections = 5
	DatabasesPerUser    = 3
)

// Status is the controller's state, for the API's health and metrics.
type Status struct {
	Version    string              `json:"version"`
	Paused     string              `json:"paused,omitempty"` // why, when the kill switch is on
	Running    map[string]int      `json:"running"`          // microVMs by image
	Warm       int                 `json:"warm"`
	Queued     int                 `json:"queued"`
	MemoryUsed int64               `json:"memoryUsed"`
	MemoryCap  int64               `json:"memoryCap"`
	Sessions   int                 `json:"sessions"`
	Databases  int                 `json:"databases"`
	Languages  map[string]Language `json:"languages"`
	Images     []string            `json:"images"`
}

// Error is how the controller answers a request it refuses.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error codes the controller uses; the API passes them on.
const (
	CodeBusy        = "busy"      // every slot is in use; try again
	CodePaused      = "paused"    // the kill switch is on
	CodeNotFound    = "not_found" // no such session or database
	CodeBadRequest  = "bad_request"
	CodeUnavailable = "not_available" // an image or tool is missing
	CodeLimit       = "limit"         // the caller has as many sessions as allowed
	CodeRefused     = "refused"       // a browser target that may not be reached
	CodeFailed      = "failed"        // the page or command failed
)
