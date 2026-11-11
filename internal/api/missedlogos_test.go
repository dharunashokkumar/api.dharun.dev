package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestMissedLogosNormalised(t *testing.T) {
	for in, want := range map[string]string{
		"phonepe":                       "PHONEPE",
		"NSE:Zomato":                    "NSEZOMATO",
		"m&m.ns":                        "M&M.NS",
		"BAJAJ-AUTO_x":                  "BAJAJ-AUTO_X",
		`"><script>alert(1)</script>`:   "SCRIPTALERT1SCRIPT",
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ123": "ABCDEFGHIJKLMNOPQRSTUVWX",
		"<>/ ":                          "",
		"ಕನ್ನಡ":                         "",
	} {
		if got := normaliseMissedLogo(in); got != want {
			t.Errorf("normaliseMissedLogo(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMissedLogosBoundedAndSaved(t *testing.T) {
	dir := t.TempDir()
	m := newMissedLogos(dir)
	clock := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return clock }

	m.add("zepto")
	m.add("ZEPTO")
	m.add("<>") // nothing left to keep
	clock = clock.Add(time.Hour)
	m.add("Zepto")
	for i := 0; i < missedLogoLimit; i++ {
		clock = clock.Add(time.Second)
		m.add("N" + strconv.Itoa(i))
	}
	if len(m.names) != missedLogoLimit {
		t.Fatalf("kept %d names, limit %d", len(m.names), missedLogoLimit)
	}
	list := m.list()
	if list[0].Name != "ZEPTO" || list[0].Count != 3 || !list[0].First.Before(list[0].Last) {
		t.Errorf("most missed = %+v", list[0])
	}
	// The first single-count name made room for the last one.
	if m.names["N0"] != nil || m.names["N"+strconv.Itoa(missedLogoLimit-1)] == nil {
		t.Error("the least-seen, oldest name was not the one dropped")
	}

	m.save()
	info, err := os.Stat(filepath.Join(dir, MissedLogosFile))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("saved file: %v %v", info, err)
	}
	again := newMissedLogos(dir)
	if len(again.names) != missedLogoLimit || again.names["ZEPTO"].Count != 3 {
		t.Errorf("after a restart: %d names, ZEPTO %+v", len(again.names), again.names["ZEPTO"])
	}
}

func TestUnknownLogoIsWrittenDown(t *testing.T) {
	count := func(name string) uint64 {
		for _, entry := range testServer.metrics.missedLogos.list() {
			if entry.Name == name {
				return entry.Count
			}
		}
		return 0
	}
	before := count("TCS")
	expectStatus(t, "/v1/logos/zz-missed-test.png?size=32", http.StatusOK)
	expectStatus(t, "/v1/logos/ZZ-MISSED-TEST?fallback=none", http.StatusNotFound)
	expectStatus(t, "/v1/logos/TCS", http.StatusOK)
	if got := count("ZZ-MISSED-TEST"); got != 2 {
		t.Errorf("ZZ-MISSED-TEST counted %d times, want 2", got)
	}
	if count("TCS") != before {
		t.Error("a found logo was written down as missed")
	}
}
