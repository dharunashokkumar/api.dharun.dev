package api

import (
	"encoding/json"
	"os"
	"sort"
	"testing"
	"time"
)

func TestGeneratedFilesAreCurrentPageDates(t *testing.T) {
	current := pageFingerprints()
	next := map[string]pageDate{}
	today := time.Now().In(indiaTime).Format(time.DateOnly)
	var stale []string
	for path, hash := range current {
		old, ok := pageDates[path]
		if ok && old.Hash == hash {
			next[path] = old
			continue
		}
		stale = append(stale, path)
		next[path] = pageDate{Hash: hash, Date: today}
	}
	if len(stale) == 0 && len(next) == len(pageDates) {
		return
	}
	if !*updateGenerated {
		sort.Strings(stale)
		t.Fatalf("pages changed or added without a new date (%d, such as %v): run go test ./internal/api -run TestGeneratedFilesAreCurrent -update", len(stale), stale[:min(len(stale), 3)])
	}
	raw, _ := json.MarshalIndent(next, "", "  ")
	if err := os.WriteFile("pagedates.json", append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
