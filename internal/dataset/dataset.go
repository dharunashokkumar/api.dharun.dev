// Package dataset owns the data behind the API. Each dataset is one read-only SQLite
// file built from one source. The repository's copy of a source is the seed; a server
// may hold a fresher copy that it fetched itself. Both are built by the same code.
package dataset

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Source records where a dataset came from, so every response can credit it.
type Source struct {
	Name       string `json:"name"`
	Publisher  string `json:"publisher"`
	URL        string `json:"url"`
	Licence    string `json:"licence"`
	LicenceURL string `json:"licenceUrl,omitempty"`
	Version    string `json:"version,omitempty"`
	// Retrieved is when the source was fetched: a date, or a full timestamp for copies a
	// server fetched itself. Between a seed and a server copy, the later one is served.
	Retrieved   string `json:"retrieved"`
	SHA256      string `json:"sha256,omitempty"`
	Attribution string `json:"attribution,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

// Meta is what a built dataset file says about itself.
type Meta struct {
	Dataset string         `json:"dataset"`
	Schema  int            `json:"schema"`
	Built   string         `json:"built"`
	Source  Source         `json:"source"`
	Counts  map[string]int `json:"counts"`
	// File names an extra file that belongs to the dataset, next to the database.
	File string `json:"file,omitempty"`
}

// Fetched is the moment the dataset's source was retrieved, in a form that sorts.
func (m Meta) Fetched() string { return m.Source.Retrieved }

// Spec describes one dataset: its tables, how to build it from files in a directory,
// and how to fetch those files from the source.
type Spec struct {
	Name string
	// Schema changes whenever the tables or their meaning change. A file built with a
	// different number is ignored by the server.
	Schema int
	DDL    string
	// Build reads the dataset's files from b.Dir and fills the tables.
	Build func(b *Builder) error
	// Import fetches the source into im.Dir, in the layout Build reads. Nil means the
	// dataset has no source that can be fetched by a program.
	Import func(im *Importer) (Source, error)
	// Every is how often a server refreshes the dataset on its own. Zero means never:
	// the dataset only changes when the repository's copy does.
	Every time.Duration
	// Main names the count that must not collapse between refreshes, and MaxShrink is
	// how far it may fall (0.1 = ten percent) before a refresh is refused.
	Main      string
	MaxShrink float64
	// Check compares a freshly built file against the one in use and returns an error
	// when the difference looks like a broken source rather than real change.
	Check func(current, fresh *sql.DB) error
	// NoSeed marks a dataset that is too large for the repository: it exists only as a
	// copy a server fetched.
	NoSeed bool
	// Fixed marks a text that never changes and is small, such as the Thirukkural. Tests
	// build it from the repository's own data/ instead of a cut-down fixture.
	Fixed bool
}

// FixtureNames and FixedNames split the datasets with a seed by how tests build them.
func FixtureNames() []string { return seedNames(false) }

// FixedNames lists the datasets tests build from the repository's data/.
func FixedNames() []string { return seedNames(true) }

func seedNames(fixed bool) []string {
	var names []string
	for _, spec := range Specs() {
		if !spec.NoSeed && spec.Fixed == fixed {
			names = append(names, spec.Name)
		}
	}
	return names
}

var specs []*Spec

func register(spec *Spec) { specs = append(specs, spec) }

// Specs lists every dataset in a stable order.
func Specs() []*Spec {
	sorted := append([]*Spec(nil), specs...)
	sort.Slice(sorted, func(a, b int) bool { return sorted[a].Name < sorted[b].Name })
	return sorted
}

// Lookup finds a dataset by name.
func Lookup(name string) (*Spec, bool) {
	for _, spec := range specs {
		if spec.Name == name {
			return spec, true
		}
	}
	return nil, false
}

const sourcesFile = "sources.json"

// ReadSources loads data/sources.json; a missing file is an empty set.
func ReadSources(dataDir string) (map[string]Source, error) {
	sources := map[string]Source{}
	raw, err := os.ReadFile(filepath.Join(dataDir, sourcesFile))
	if os.IsNotExist(err) {
		return sources, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &sources); err != nil {
		return nil, fmt.Errorf("%s: %w", sourcesFile, err)
	}
	return sources, nil
}

// WriteSources saves data/sources.json.
func WriteSources(dataDir string, sources map[string]Source) error {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false) // keep & in URLs readable
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(sources); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, sourcesFile), out.Bytes(), 0o644)
}

// Open opens a built dataset read-only. The file is never written once built, so
// SQLite is told it is immutable and skips locking altogether.
func Open(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	// SQLite wants file:///C:/dir/x.db on Windows and file:///dir/x.db elsewhere.
	slashed := filepath.ToSlash(absolute)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	dsn := "file://" + (&url.URL{Path: slashed}).EscapedPath() + "?mode=ro&immutable=1&_pragma=cache_size(-512)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Few connections, and idle ones are dropped: with a file per dataset, the page
	// caches of connections nobody is using would otherwise add up.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(1)
	db.SetConnMaxIdleTime(2 * time.Minute)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// ErrNotADataset means the file opened but carries no dataset record.
var ErrNotADataset = errors.New("not a dataset file")

// ReadMeta reads what a built dataset says about itself.
func ReadMeta(db *sql.DB) (Meta, error) {
	var raw string
	if err := db.QueryRow(`SELECT value FROM meta WHERE key = 'dataset'`).Scan(&raw); err != nil {
		return Meta{}, fmt.Errorf("%w: %v", ErrNotADataset, err)
	}
	var meta Meta
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return Meta{}, fmt.Errorf("%w: %v", ErrNotADataset, err)
	}
	if meta.Counts == nil {
		meta.Counts = map[string]int{}
	}
	return meta, nil
}

// ReadMetaFile opens a dataset file just long enough to read its record.
func ReadMetaFile(path string) (Meta, error) {
	db, err := Open(path)
	if err != nil {
		return Meta{}, err
	}
	defer db.Close()
	return ReadMeta(db)
}

// LiveName is the file name of a copy a server built at the given moment. The stamp
// sorts by time, so the newest copy of a dataset is the last name in order.
func LiveName(dataset string, at time.Time) string {
	return dataset + "." + at.UTC().Format("20060102T150405Z") + ".db"
}

// LiveFiles returns a dataset's server-built copies in dir, oldest first.
func LiveFiles(dir, dataset string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, dataset+".*.db"))
	kept := matches[:0]
	for _, match := range matches {
		// "holidays.x.db" must not match the dataset "holidays-bank", nor the other way round.
		stamp := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(match), dataset+"."), ".db")
		if _, err := time.Parse("20060102T150405Z", stamp); err == nil {
			kept = append(kept, match)
		}
	}
	sort.Strings(kept)
	return kept
}

// SeedPath is where the repository's copy of a dataset sits in a seed directory.
func SeedPath(dir, dataset string) string { return filepath.Join(dir, dataset+".db") }

// Newer reports whether moment a is later than b. Both are dates or timestamps as
// stored in Source.Retrieved; a date counts as the start of that day.
func Newer(a, b string) bool { return stamp(a).After(stamp(b)) }

func stamp(value string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

// Age is how long ago a dataset's source was retrieved.
func Age(meta Meta, now time.Time) time.Duration {
	fetched := stamp(meta.Fetched())
	if fetched.IsZero() {
		return time.Duration(1<<62 - 1)
	}
	return now.Sub(fetched)
}

func itoa(value int) string { return strconv.Itoa(value) }
