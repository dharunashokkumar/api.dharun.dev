package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Logo symbols that matched no company are counted so that their logos can be added.
// Kept: the normalised name, how often, first and last seen. Never the caller.
const (
	missedLogoLimit   = 5000
	missedLogoNameMax = 24
	MissedLogosFile   = "missed-logos.json"
)

// MissedLogo is one name in STATE_DIR/missed-logos.json.
type MissedLogo struct {
	Name  string    `json:"name"`
	Count uint64    `json:"count"`
	First time.Time `json:"first"`
	Last  time.Time `json:"last"`
}

type missedLogos struct {
	mu    sync.Mutex
	names map[string]*MissedLogo
	dirty bool
	path  string
	now   func() time.Time
}

func newMissedLogos(stateDir string) *missedLogos {
	m := &missedLogos{names: map[string]*MissedLogo{}, now: time.Now}
	if stateDir == "" {
		return m
	}
	m.path = filepath.Join(stateDir, MissedLogosFile)
	if raw, err := os.ReadFile(m.path); err == nil {
		var saved []MissedLogo
		if json.Unmarshal(raw, &saved) == nil {
			for i := range saved {
				if name := normaliseMissedLogo(saved[i].Name); name != "" && len(m.names) < missedLogoLimit {
					entry := saved[i]
					entry.Name = name
					m.names[name] = &entry
				}
			}
		}
	}
	return m
}

// normaliseMissedLogo keeps upper-case letters, digits and & - _ . so that what is stored
// is a plausible name and never markup or a long text. "" means nothing worth keeping.
func normaliseMissedLogo(symbol string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(symbol) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("&-_.", r) {
			b.WriteRune(r)
			if b.Len() == missedLogoNameMax {
				break
			}
		}
	}
	return b.String()
}

// add counts one miss. When the list is full, the least-seen name (the oldest of those)
// makes room.
func (m *missedLogos) add(symbol string) {
	name := normaliseMissedLogo(symbol)
	if name == "" {
		return
	}
	now := m.now().UTC().Truncate(time.Second)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dirty = true
	if entry := m.names[name]; entry != nil {
		entry.Count++
		entry.Last = now
		return
	}
	if len(m.names) >= missedLogoLimit {
		var drop *MissedLogo
		for _, entry := range m.names {
			if drop == nil || entry.Count < drop.Count || (entry.Count == drop.Count && entry.Last.Before(drop.Last)) {
				drop = entry
			}
		}
		delete(m.names, drop.Name)
	}
	m.names[name] = &MissedLogo{Name: name, Count: 1, First: now, Last: now}
}

// list returns the names, most missed first.
func (m *missedLogos) list() []MissedLogo {
	m.mu.Lock()
	out := make([]MissedLogo, 0, len(m.names))
	for _, entry := range m.names {
		out = append(out, *entry)
	}
	m.mu.Unlock()
	SortMissedLogos(out)
	return out
}

// SortMissedLogos orders by count, then by name.
func SortMissedLogos(list []MissedLogo) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].Count != list[j].Count {
			return list[i].Count > list[j].Count
		}
		return list[i].Name < list[j].Name
	})
}

// save writes the list when it changed since the last save.
func (m *missedLogos) save() {
	if m.path == "" {
		return
	}
	m.mu.Lock()
	dirty := m.dirty
	m.dirty = false
	m.mu.Unlock()
	if !dirty {
		return
	}
	encoded, _ := json.MarshalIndent(m.list(), "", "  ")
	if err := writeState(m.path, append(encoded, '\n'), 0o600); err != nil {
		logError("saving missed logos", err)
		m.mu.Lock()
		m.dirty = true
		m.mu.Unlock()
	}
}
