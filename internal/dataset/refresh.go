package dataset

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StatusFile is written into the live directory after every refresh run. The API reads
// it to report, per dataset, when a refresh last worked.
const StatusFile = "refresh.json"

// retryAfter is how long a dataset waits after a failed refresh before trying again,
// so a source that is down is not asked every time the timer fires.
const retryAfter = 6 * time.Hour

// Status is the content of the status file.
type Status struct {
	Updated  string                   `json:"updated"`
	Datasets map[string]DatasetStatus `json:"datasets"`
}

// DatasetStatus is the outcome of the most recent refresh attempts for one dataset.
type DatasetStatus struct {
	LastAttempt string `json:"lastAttempt,omitempty"`
	LastSuccess string `json:"lastSuccess,omitempty"`
	// Result is "updated", "failed" or "held". A held refresh fetched and built fine, but
	// the result differed too much from the copy in use to be trusted without a look.
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// ReadStatus loads the status file; a missing or unreadable file is an empty status.
func ReadStatus(liveDir string) Status {
	status := Status{Datasets: map[string]DatasetStatus{}}
	raw, err := os.ReadFile(filepath.Join(liveDir, StatusFile))
	if err != nil {
		return status
	}
	if json.Unmarshal(raw, &status) != nil || status.Datasets == nil {
		return Status{Datasets: map[string]DatasetStatus{}}
	}
	return status
}

func writeStatus(liveDir string, status Status) error {
	encoded, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(liveDir, StatusFile+".tmp")
	if err := os.WriteFile(tmp, append(encoded, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(liveDir, StatusFile))
}

// RefreshOptions says where a server keeps its data and what to refresh.
type RefreshOptions struct {
	// SeedDir holds the copies that came with the release.
	SeedDir string
	// LiveDir holds the copies this server built. It is the only directory written.
	LiveDir string
	// Only limits the run to these datasets; empty means every dataset that is due.
	Only []string
	// Force refreshes even when the copy in use is recent, and accepts a result that
	// would otherwise be held back.
	Force bool
	// Keep is how many copies of a dataset stay on disk. The previous one is kept so the
	// newest can be removed by hand if it turns out wrong.
	Keep int
	Now  time.Time
	Log  func(format string, args ...any)
}

// RefreshResult reports what happened to one dataset in a run.
type RefreshResult struct {
	Dataset string
	// Outcome is "updated", "unchanged" (checked, source not changed), "fresh" (not due),
	// "waiting" (failed recently), "failed" or "held".
	Outcome string
	Err     error
	Meta    Meta
}

// Current finds the copy of a dataset a server should serve: its own newest copy when
// that matches the tables this program expects and is at least as recent as the seed,
// otherwise the seed. The boolean says whether the chosen copy is the server's own.
func Current(spec *Spec, seedDir, liveDir string) (path string, meta Meta, live, ok bool) {
	var seedMeta Meta
	seedPath := ""
	if seedDir != "" && !spec.NoSeed {
		candidate := SeedPath(seedDir, spec.Name)
		if found, err := ReadMetaFile(candidate); err == nil && found.Dataset == spec.Name && found.Schema == spec.Schema {
			seedPath, seedMeta = candidate, found
		}
	}
	if liveDir != "" {
		copies := LiveFiles(liveDir, spec.Name)
		for i := len(copies) - 1; i >= 0; i-- {
			found, err := ReadMetaFile(copies[i])
			if err != nil || found.Dataset != spec.Name || found.Schema != spec.Schema {
				continue
			}
			if !filesPresent(liveDir, found) {
				continue
			}
			if seedPath != "" && Newer(seedMeta.Fetched(), found.Fetched()) {
				break // the release brought something more recent than anything fetched here
			}
			return copies[i], found, true, true
		}
	}
	if seedPath != "" {
		return seedPath, seedMeta, false, true
	}
	return "", Meta{}, false, false
}

// Refresh fetches every dataset that is due, builds it and puts the new copy in place.
// A dataset that fails leaves the copy in use untouched. The returned error is only for
// problems with the run itself; per-dataset failures are in the results.
func Refresh(opts RefreshOptions) ([]RefreshResult, error) {
	if opts.Log == nil {
		opts.Log = func(string, ...any) {}
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	if opts.Keep < 1 {
		opts.Keep = 2
	}
	if err := os.MkdirAll(opts.LiveDir, 0o755); err != nil {
		return nil, err
	}
	only := map[string]bool{}
	for _, name := range opts.Only {
		spec, ok := Lookup(name)
		if !ok {
			return nil, fmt.Errorf("unknown dataset %q", name)
		}
		if spec.Import == nil {
			return nil, fmt.Errorf("dataset %q has no source a program can fetch", name)
		}
		only[name] = true
	}

	status := ReadStatus(opts.LiveDir)
	var results []RefreshResult
	for _, spec := range Specs() {
		if spec.Import == nil {
			continue
		}
		if len(only) > 0 {
			if !only[spec.Name] {
				continue
			}
		} else if spec.Every == 0 {
			continue
		}

		result := refreshOne(spec, opts, status.Datasets[spec.Name])
		results = append(results, result)

		now := opts.Now.UTC().Format(time.RFC3339)
		entry := status.Datasets[spec.Name]
		switch result.Outcome {
		case "updated":
			entry = DatasetStatus{LastAttempt: now, LastSuccess: now, Result: "updated"}
			opts.Log("%s: updated, %d %s", spec.Name, result.Meta.Counts[spec.Main], spec.Main)
		case "failed", "held":
			entry.LastAttempt, entry.Result, entry.Error = now, result.Outcome, result.Err.Error()
			opts.Log("%s: %s: %v", spec.Name, result.Outcome, result.Err)
		case "unchanged":
			entry = DatasetStatus{LastAttempt: now, LastSuccess: now, Result: "unchanged"}
			opts.Log("%s: source unchanged since %s", spec.Name, result.Meta.Fetched())
		case "waiting":
			opts.Log("%s: not retried yet, the last attempt at %s failed", spec.Name, entry.LastAttempt)
			continue
		default:
			opts.Log("%s: not due, source fetched %s", spec.Name, result.Meta.Fetched())
			continue // nothing was attempted, so there is nothing new to record
		}
		status.Datasets[spec.Name] = entry
		prune(spec, opts)
	}
	status.Updated = opts.Now.UTC().Format(time.RFC3339)
	return results, writeStatus(opts.LiveDir, status)
}

func refreshOne(spec *Spec, opts RefreshOptions, last DatasetStatus) RefreshResult {
	result := RefreshResult{Dataset: spec.Name}
	currentPath, currentMeta, live, have := Current(spec, opts.SeedDir, opts.LiveDir)
	result.Meta = currentMeta

	if !opts.Force {
		// After a release changed a dataset's tables, the server's own copies no longer
		// fit and the seed is served. Rebuild at once rather than when the seed falls due.
		outdated := !live && len(LiveFiles(opts.LiveDir, spec.Name)) > 0
		// A source found unchanged at the last check counts as fetched then.
		age := Age(currentMeta, opts.Now)
		if last.Result == "unchanged" && last.LastSuccess != "" {
			age = min(age, opts.Now.Sub(stamp(last.LastSuccess)))
		}
		if have && !outdated && age < spec.Every {
			result.Outcome = "fresh"
			return result
		}
		if (last.Result == "failed" || last.Result == "held") && opts.Now.Sub(stamp(last.LastAttempt)) < retryAfter {
			result.Outcome = "waiting"
			return result
		}
	}

	fail := func(outcome string, err error) RefreshResult {
		result.Outcome, result.Err = outcome, err
		return result
	}

	var current *sql.DB
	if have {
		var err error
		if current, err = Open(currentPath); err != nil {
			return fail("failed", err)
		}
		defer current.Close()
	}

	work, err := os.MkdirTemp(opts.LiveDir, ".import-"+spec.Name+"-")
	if err != nil {
		return fail("failed", err)
	}
	defer os.RemoveAll(work)

	source, err := spec.Import(&Importer{Dir: work, Current: current, Now: opts.Now, Log: opts.Log})
	if errors.Is(err, ErrNotChanged) {
		result.Outcome = "unchanged"
		return result
	}
	if err != nil {
		return fail("failed", fmt.Errorf("fetching: %w", err))
	}
	source.Retrieved = opts.Now.UTC().Format(time.RFC3339)

	out := filepath.Join(opts.LiveDir, LiveName(spec.Name, opts.Now))
	meta, _, err := Build(spec, BuildOptions{Dir: work, Out: out, Source: source, Log: opts.Log})
	if err != nil {
		return fail("failed", fmt.Errorf("building: %w", err))
	}

	if have && !opts.Force {
		if err := plausible(spec, current, currentMeta, out, meta); err != nil {
			os.Remove(out)
			removeAttachment(opts.LiveDir, meta)
			return fail("held", err)
		}
	}
	result.Outcome, result.Meta = "updated", meta
	return result
}

// plausible refuses a fresh copy that looks like the product of a broken source.
func plausible(spec *Spec, current *sql.DB, currentMeta Meta, freshPath string, freshMeta Meta) error {
	if spec.Main != "" {
		before, after := currentMeta.Counts[spec.Main], freshMeta.Counts[spec.Main]
		if before > 0 && float64(after) < float64(before)*(1-spec.MaxShrink) {
			return fmt.Errorf("%s fell from %d to %d, more than the %.0f%% allowed; run the refresh with -force after checking the source",
				spec.Main, before, after, spec.MaxShrink*100)
		}
	}
	if spec.Check == nil {
		return nil
	}
	fresh, err := Open(freshPath)
	if err != nil {
		return err
	}
	defer fresh.Close()
	if err := spec.Check(current, fresh); err != nil {
		return fmt.Errorf("%w; run the refresh with -force after checking the source", err)
	}
	return nil
}

func removeAttachment(liveDir string, meta Meta) {
	for _, name := range meta.Files {
		os.Remove(filepath.Join(liveDir, name))
	}
}

func filesPresent(dir string, meta Meta) bool {
	for _, name := range meta.Files {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return false
		}
	}
	return true
}

// prune deletes copies a server no longer needs: all but the newest few, and any built
// for tables this program no longer uses.
func prune(spec *Spec, opts RefreshOptions) {
	copies := LiveFiles(opts.LiveDir, spec.Name)
	kept := 0
	for i := len(copies) - 1; i >= 0; i-- {
		meta, err := ReadMetaFile(copies[i])
		usable := err == nil && meta.Schema == spec.Schema
		if usable && kept < opts.Keep {
			kept++
			continue
		}
		// On Windows a copy still open in the API cannot be removed; the next run retries.
		if os.Remove(copies[i]) == nil && err == nil {
			removeAttachment(opts.LiveDir, meta)
		}
	}
}
