package dataset

import (
	"compress/gzip"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Builder is handed to a dataset's Build function.
type Builder struct {
	Tx *sql.Tx
	// Dir holds the dataset's source files, laid out as in the repository's data/.
	Dir string
	// LogosDir holds the nse/ and bse/ SVG folders. Only the logos dataset reads it.
	LogosDir string
	// Resvg is the resvg executable for PNG logos; empty means no PNG output.
	Resvg string
	// Out is the database file being written, for datasets that keep a file beside it.
	Out string

	counts map[string]int
	files  []string
	log    func(format string, args ...any)
	warn   func(format string, args ...any)
}

// Count records how many records of a kind went in. The API reports these.
func (b *Builder) Count(name string, n int) { b.counts[name] = n }

// Add increases a count by n.
func (b *Builder) Add(name string, n int) { b.counts[name] += n }

// Logf reports progress.
func (b *Builder) Logf(format string, args ...any) { b.log(format, args...) }

// Warnf reports something a maintainer should look at; it does not stop the build.
func (b *Builder) Warnf(format string, args ...any) { b.warn(format, args...) }

// Path joins a name onto the source directory.
func (b *Builder) Path(parts ...string) string {
	return filepath.Join(append([]string{b.Dir}, parts...)...)
}

// Attach moves a file into place next to the database, as part of the dataset. kind
// names it ("city" gives <dataset file>.city.<ext>). A file is attached for datasets
// whose data is better read in its own format than copied into tables.
func (b *Builder) Attach(path, kind string) error {
	name := strings.TrimSuffix(filepath.Base(b.Out), ".db") + "." + kind + filepath.Ext(path)
	target := filepath.Join(filepath.Dir(b.Out), name)
	if err := os.Rename(path, target); err != nil {
		// Across file systems a rename fails; copy instead.
		if err := copyFile(path, target); err != nil {
			return err
		}
	}
	b.files = append(b.files, name)
	return nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// BuildOptions says what to build and from where.
type BuildOptions struct {
	Dir      string
	LogosDir string
	Resvg    string
	// Out is the database file to write.
	Out string
	// Source is recorded in the file and repeated in API responses.
	Source Source
	Log    func(format string, args ...any)
}

// Build writes one dataset file. It builds into a temporary file and renames it into
// place, so a failed build never leaves a half-written file behind.
func Build(spec *Spec, opts BuildOptions) (Meta, []string, error) {
	if opts.Log == nil {
		opts.Log = func(string, ...any) {}
	}
	var warnings []string
	meta := Meta{Dataset: spec.Name, Schema: spec.Schema, Built: time.Now().UTC().Format(time.RFC3339), Source: opts.Source, Counts: map[string]int{}}

	if err := os.MkdirAll(filepath.Dir(opts.Out), 0o755); err != nil {
		return meta, nil, err
	}
	tmp := opts.Out + ".tmp"
	os.Remove(tmp)
	db, err := sql.Open("sqlite", tmp)
	if err != nil {
		return meta, nil, err
	}
	defer func() {
		db.Close()
		os.Remove(tmp)
	}()
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{"PRAGMA journal_mode=OFF", "PRAGMA synchronous=OFF", "PRAGMA page_size=4096"} {
		if _, err := db.Exec(pragma); err != nil {
			return meta, nil, err
		}
	}
	if _, err := db.Exec(`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL) WITHOUT ROWID;` + spec.DDL); err != nil {
		return meta, nil, fmt.Errorf("%s: creating tables: %w", spec.Name, err)
	}

	tx, err := db.Begin()
	if err != nil {
		return meta, nil, err
	}
	builder := &Builder{
		Tx: tx, Dir: opts.Dir, LogosDir: opts.LogosDir, Resvg: opts.Resvg, Out: opts.Out, counts: meta.Counts,
		log: opts.Log,
		warn: func(format string, args ...any) {
			message := fmt.Sprintf(format, args...)
			warnings = append(warnings, message)
			opts.Log("warning: %s: %s", spec.Name, message)
		},
	}
	removeFiles := func() {
		for _, name := range builder.files {
			os.Remove(filepath.Join(filepath.Dir(opts.Out), name))
		}
	}
	if err := spec.Build(builder); err != nil {
		tx.Rollback()
		removeFiles()
		return meta, warnings, fmt.Errorf("%s: %w", spec.Name, err)
	}
	meta.Files = builder.files
	encoded, _ := json.Marshal(meta)
	if _, err := tx.Exec(`INSERT INTO meta (key, value) VALUES ('dataset', ?)`, string(encoded)); err != nil {
		tx.Rollback()
		return meta, warnings, err
	}
	if err := tx.Commit(); err != nil {
		return meta, warnings, err
	}
	for _, statement := range []string{"ANALYZE", "VACUUM"} {
		if _, err := db.Exec(statement); err != nil {
			return meta, warnings, err
		}
	}
	if err := db.Close(); err != nil {
		return meta, warnings, err
	}
	os.Remove(opts.Out)
	return meta, warnings, os.Rename(tmp, opts.Out)
}

// SeedOptions says how to build the repository's copies.
type SeedOptions struct {
	DataDir  string // holds sources.json and every dataset's source files
	LogosDir string
	OutDir   string
	Resvg    string
	// Strict makes a missing dataset an error instead of a warning.
	Strict bool
	// Only limits the build to these datasets; empty builds all.
	Only []string
	Log  func(format string, args ...any)
}

// BuildSeeds builds every dataset the repository holds source files for.
func BuildSeeds(opts SeedOptions) ([]Meta, []string, error) {
	if opts.Log == nil {
		opts.Log = func(string, ...any) {}
	}
	sources, err := ReadSources(opts.DataDir)
	if err != nil {
		return nil, nil, err
	}
	wanted := map[string]bool{}
	for _, name := range opts.Only {
		if _, ok := Lookup(name); !ok {
			return nil, nil, fmt.Errorf("unknown dataset %q", name)
		}
		wanted[name] = true
	}

	var built []Meta
	var warnings []string
	for _, spec := range Specs() {
		if spec.NoSeed || (len(wanted) > 0 && !wanted[spec.Name]) {
			continue
		}
		source, credited := sources[spec.Name]
		if !credited {
			warnings = append(warnings, spec.Name+": no entry in "+sourcesFile+", responses will carry no source credit")
			source.Retrieved = "1970-01-01"
		}
		meta, notes, err := Build(spec, BuildOptions{
			Dir: opts.DataDir, LogosDir: opts.LogosDir, Resvg: opts.Resvg,
			Out: SeedPath(opts.OutDir, spec.Name), Source: source, Log: opts.Log,
		})
		for _, note := range notes {
			warnings = append(warnings, spec.Name+": "+note)
		}
		if errors.Is(err, os.ErrNotExist) && !opts.Strict {
			warnings = append(warnings, fmt.Sprintf("%s: source file missing, dataset not built (%v)", spec.Name, err))
			continue
		}
		if err != nil {
			return built, warnings, err
		}
		built = append(built, meta)
	}
	return built, warnings, nil
}

// ---- helpers shared by dataset builders ----

// readGzipCSV returns the header plus every row as wide as the header, and how many
// rows it dropped for having a different width.
func readGzipCSV(path string) ([][]string, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()
	unzipped, err := gzip.NewReader(file)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	defer unzipped.Close()
	return readCSV(unzipped, filepath.Base(path))
}

func readCSV(source io.Reader, name string) ([][]string, int, error) {
	reader := csv.NewReader(source)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	var records [][]string
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", name, err)
		}
		records = append(records, record)
	}
	if len(records) < 2 {
		return nil, 0, fmt.Errorf("%s: no data rows", name)
	}
	width := len(records[0])
	kept := records[:1]
	for _, record := range records[1:] {
		if len(record) == width {
			kept = append(kept, record)
		}
	}
	return kept, len(records) - len(kept), nil
}

// Some exported CSV files start with a UTF-8 byte order mark glued to the first header.
var byteOrderMark = string([]byte{0xEF, 0xBB, 0xBF})

// columns maps the wanted header names (lower case) to their positions.
func columns(header []string, wanted ...string) (map[string]int, error) {
	positions := map[string]int{}
	for i, name := range header {
		positions[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, byteOrderMark)))] = i
	}
	result := map[string]int{}
	for _, name := range wanted {
		position, ok := positions[name]
		if !ok {
			return nil, fmt.Errorf("column %q not found in header %v", name, header)
		}
		result[name] = position
	}
	return result, nil
}

func nullable(value string) any {
	if value == "" || strings.EqualFold(value, "NA") {
		return nil
	}
	return value
}

// titleCase turns "TAMIL NADU" into "Tamil Nadu". Source files shout most names.
func titleCase(value string) string {
	words := strings.Fields(strings.ToLower(strings.TrimSpace(value)))
	for i, word := range words {
		runes := []rune(word)
		upperNext := true
		for j, r := range runes {
			if upperNext && r >= 'a' && r <= 'z' {
				runes[j] = r - 32
			}
			upperNext = r == '-' || r == '(' || r == '.' || r == '/'
		}
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

func sumValues(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

func topKeys(counts map[string]int, limit int) string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(a, b int) bool { return counts[keys[a]] > counts[keys[b]] })
	if len(keys) > limit {
		keys = keys[:limit]
	}
	parts := make([]string, len(keys))
	for i, key := range keys {
		parts[i] = fmt.Sprintf("%q x%d", key, counts[key])
	}
	return strings.Join(parts, ", ")
}
