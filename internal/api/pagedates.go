package api

// The date each documentation, tool and demo page last changed, for the sitemap
// (growth plan Part 3 section 5: dates from documentation edits, not only data dates).
// pagedates.json holds a fingerprint and a date per page; TestGeneratedFilesAreCurrentPageDates fails
// when a page changed, and -update writes the new fingerprint with that day's date.

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
)

//go:embed pagedates.json
var pageDatesFile []byte

type pageDate struct {
	Hash string `json:"hash"`
	Date string `json:"date"`
}

var pageDates = func() map[string]pageDate {
	out := map[string]pageDate{}
	json.Unmarshal(pageDatesFile, &out)
	return out
}()

func pageFingerprint(v any) string {
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}

// pageFingerprints is what each page is made from, by its path.
func pageFingerprints() map[string]string {
	out := map[string]string{}
	for _, group := range catalog {
		out["/docs/"+group.ID] = pageFingerprint(group)
	}
	for _, t := range tools {
		out["/tools/"+t.Slug] = pageFingerprint(t)
	}
	for _, d := range demos {
		out["/demos/"+d.Slug] = pageFingerprint(d)
	}
	return out
}

// pageDate is the day a page's content last changed, or "".
func editedOn(path string) string { return pageDates[path].Date }
