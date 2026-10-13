package api

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

// A handle is one opened dataset file. Handlers fetch the current handle per request,
// so a dataset can be replaced while the server runs.
type handle struct {
	db   *sql.DB
	meta dataset.Meta
	path string
	dir  string
	live bool // built by this server, as opposed to the copy that came with the release

	fingerprint string
	mu          sync.Mutex
	closed      bool
	statements  map[string]*sql.Stmt
}

// prepared returns a statement for a fixed query, preparing it on first use.
func (h *handle) prepared(query string) (*sql.Stmt, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if statement, ok := h.statements[query]; ok {
		return statement, nil
	}
	statement, err := h.db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("preparing %q: %w", query, err)
	}
	h.statements[query] = statement
	return statement, nil
}

// row runs a fixed single-row query through a prepared statement.
func (h *handle) row(query string, args ...any) *sql.Row {
	statement, err := h.prepared(query)
	if err != nil {
		// QueryRow on the same broken query carries the error to the caller's Scan.
		return h.db.QueryRow(query, args...)
	}
	return statement.QueryRow(args...)
}

// rows runs a fixed multi-row query through a prepared statement.
func (h *handle) rows(query string, args ...any) (*sql.Rows, error) {
	statement, err := h.prepared(query)
	if err != nil {
		return nil, err
	}
	return statement.Query(args...)
}

// count is one of the record counts stored in the file when it was built.
func (h *handle) count(name string) int {
	if h == nil {
		return 0
	}
	return h.meta.Counts[name]
}

// attachment is the path of the dataset's extra file of a kind ("city"), or "".
func (h *handle) attachment(kind string) string {
	for _, name := range h.meta.Files {
		base := strings.TrimSuffix(name, filepath.Ext(name))
		if strings.HasSuffix(base, "."+kind) {
			return filepath.Join(h.dir, name)
		}
	}
	return ""
}

// close may be called more than once: by the timer after a swap and again at shutdown.
func (h *handle) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for _, statement := range h.statements {
		statement.Close()
	}
	h.statements = map[string]*sql.Stmt{}
	h.db.Close()
}

// store holds the copy in use of every dataset and swaps a copy when a newer one
// appears on disk.
type store struct {
	seedDir, liveDir string
	handles          map[string]*atomic.Pointer[handle]
	// changed is called with a dataset's name after its handle was swapped.
	changed func(name string)

	mu          sync.Mutex
	listing     string
	retired     []*handle // replaced copies, closed a minute after the swap or at shutdown
	status      dataset.Status
	statusStamp string
	stop        chan struct{}
	done        chan struct{}
}

func newStore(seedDir, liveDir string) *store {
	st := &store{seedDir: seedDir, liveDir: liveDir, handles: map[string]*atomic.Pointer[handle]{},
		status: dataset.Status{Datasets: map[string]dataset.DatasetStatus{}}}
	for _, spec := range dataset.Specs() {
		st.handles[spec.Name] = &atomic.Pointer[handle]{}
	}
	return st
}

// get returns the copy in use of a dataset, or nil when there is none.
func (st *store) get(name string) *handle {
	pointer, ok := st.handles[name]
	if !ok {
		return nil
	}
	return pointer.Load()
}

// listing describes the directory's dataset files cheaply, so an unchanged directory costs
// two directory reads per check and no file is opened.
func listing(dir string) string {
	if dir == "" {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var out strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		if info, err := entry.Info(); err == nil {
			fmt.Fprintf(&out, "%s|%d|%d\n", entry.Name(), info.Size(), info.ModTime().UnixNano())
		}
	}
	return out.String()
}

func fingerprint(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s|%d|%d", path, info.Size(), info.ModTime().UnixNano())
}

// reload looks at both directories and opens any dataset whose best copy changed. It
// returns the names of the datasets that were swapped.
func (st *store) reload() []string {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.loadStatus()
	seen := listing(st.seedDir) + "--\n" + listing(st.liveDir)
	if seen == st.listing {
		return nil
	}
	st.listing = seen

	var swapped []string
	for _, spec := range dataset.Specs() {
		pointer := st.handles[spec.Name]
		current := pointer.Load()
		path, meta, live, ok := dataset.Current(spec, st.seedDir, st.liveDir)
		if !ok {
			// A copy that vanished from disk stays in use: an open file outlives its name.
			continue
		}
		mark := fingerprint(path)
		if current != nil && current.fingerprint == mark {
			continue
		}
		db, err := dataset.Open(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, `{"msg":"error","error":"opening dataset %s: %s"}`+"\n", spec.Name, err)
			continue
		}
		pointer.Store(&handle{db: db, meta: meta, path: path, dir: filepath.Dir(path), live: live, fingerprint: mark, statements: map[string]*sql.Stmt{}})
		if current != nil {
			// Requests already running finish against the old copy; the write timeout bounds them.
			time.AfterFunc(time.Minute, current.close)
			st.retired = append(st.retired, current)
		}
		swapped = append(swapped, spec.Name)
	}
	return swapped
}

func (st *store) loadStatus() {
	if st.liveDir == "" {
		return
	}
	mark := fingerprint(filepath.Join(st.liveDir, dataset.StatusFile))
	if mark == st.statusStamp {
		return
	}
	st.statusStamp = mark
	st.status = dataset.ReadStatus(st.liveDir)
}

// watch checks for new copies until the store is closed.
func (st *store) watch(every time.Duration) {
	st.stop, st.done = make(chan struct{}), make(chan struct{})
	go func() {
		defer close(st.done)
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				for _, name := range st.reload() {
					fmt.Fprintf(os.Stdout, `{"msg":"dataset","dataset":%q,"fetched":%q}`+"\n", name, st.get(name).meta.Fetched())
					if st.changed != nil {
						st.changed(name)
					}
				}
			case <-st.stop:
				return
			}
		}
	}()
}

func (st *store) close() {
	if st.stop != nil {
		close(st.stop)
		<-st.done
	}
	for _, pointer := range st.handles {
		if current := pointer.Swap(nil); current != nil {
			current.close()
		}
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, old := range st.retired {
		old.close()
	}
	st.retired = nil
}

// datasetState is what the health check and the metrics report for one dataset.
type datasetState struct {
	Name string `json:"-"`
	// Copy is "server" for a copy this server fetched, "seed" for the one that came with
	// the release, "none" when the dataset is not available.
	Copy    string         `json:"copy"`
	Fetched string         `json:"fetched,omitempty"`
	Records map[string]int `json:"records,omitempty"`
	// Refresh is the outcome of the last refresh attempt, for datasets a server refreshes.
	Refresh     string `json:"refresh,omitempty"`
	LastSuccess string `json:"lastRefresh,omitempty"`
	scheduled   bool
	every       time.Duration // how often a server refreshes it; zero when it does not
}

func (st *store) states() []datasetState {
	st.mu.Lock()
	status := st.status
	st.mu.Unlock()

	var states []datasetState
	for _, spec := range dataset.Specs() {
		state := datasetState{Name: spec.Name, Copy: "none", scheduled: spec.Every > 0, every: spec.Every}
		if current := st.get(spec.Name); current != nil {
			state.Copy, state.Fetched, state.Records = "seed", current.meta.Fetched(), current.meta.Counts
			if current.live {
				state.Copy = "server"
			}
		}
		if entry, ok := status.Datasets[spec.Name]; ok {
			state.Refresh, state.LastSuccess = entry.Result, entry.LastSuccess
		}
		states = append(states, state)
	}
	sort.Slice(states, func(a, b int) bool { return states[a].Name < states[b].Name })
	return states
}
