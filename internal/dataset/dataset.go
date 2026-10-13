// Package dataset turns the source files under data/ into the single read-only SQLite
// file the API serves from, and opens that file again at run time.
package dataset

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Source records where a dataset came from, so every response can credit it.
type Source struct {
	Name        string `json:"name"`
	Publisher   string `json:"publisher"`
	URL         string `json:"url"`
	Licence     string `json:"licence"`
	LicenceURL  string `json:"licenceUrl,omitempty"`
	Version     string `json:"version,omitempty"`
	Retrieved   string `json:"retrieved"`
	SHA256      string `json:"sha256,omitempty"`
	Attribution string `json:"attribution,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

// RTO is one Regional Transport Office registration code.
type RTO struct {
	Code         string `json:"code"`
	Prefix       string `json:"prefix"`
	Office       string `json:"office"`
	Jurisdiction string `json:"jurisdiction,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

// Holiday is one entry of a data/holidays/<year>.json file.
type Holiday struct {
	Date string `json:"date"`
	Name string `json:"name"`
	// Type is one of: bank, central_gazetted, central_restricted, market, state.
	Type string `json:"type"`
	// Region names who observes it: "India", an RBI office, an exchange or a state.
	Region string `json:"region"`
	// States lists the state codes covered; ["*"] means the whole country.
	States []string `json:"states"`
	Notes  string   `json:"notes,omitempty"`
}

// HolidayFile is the shape of data/holidays/<year>.json.
type HolidayFile struct {
	Year     int       `json:"year"`
	Holidays []Holiday `json:"holidays"`
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
	raw, err := json.MarshalIndent(sources, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, sourcesFile), append(raw, '\n'), 0o644)
}

// Open opens a built database read-only. The file is never written at run time, so
// SQLite is told it is immutable and skips locking altogether.
func Open(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	// SQLite wants file:///C:/dir/api.db on Windows and file:///dir/api.db elsewhere.
	slashed := filepath.ToSlash(absolute)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	dsn := "file://" + (&url.URL{Path: slashed}).EscapedPath() + "?mode=ro&immutable=1&_pragma=cache_size(-1024)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// A handful of connections is plenty for point lookups and keeps memory flat.
	db.SetMaxOpenConns(4)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// Sources reads the provenance records stored in a built database.
func Sources(db *sql.DB) (map[string]Source, error) {
	rows, err := db.Query(`SELECT key, value FROM meta WHERE key LIKE 'source:%'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sources := map[string]Source{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		var source Source
		if err := json.Unmarshal([]byte(value), &source); err != nil {
			return nil, err
		}
		sources[key[len("source:"):]] = source
	}
	return sources, rows.Err()
}
