package dataset

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/orbit"
)

// Orbit elements for the space stations and amateur radio satellites, from CelesTrak's
// current GP data, which is derived from the US Space Force's catalogue. The API only
// ever answers with positions and passes computed from them, never the elements
// themselves (Dharun, 2026-10-14; see ROADMAP.md). Elements age within days, so they are
// never kept in the repository: a server fetches them a few times a day.

func init() {
	register(&Spec{
		Name:   "satellites",
		Schema: 1,
		DDL: `
CREATE TABLE satellites (
  number INTEGER PRIMARY KEY, name TEXT NOT NULL, groups TEXT NOT NULL,
  line1 TEXT NOT NULL, line2 TEXT NOT NULL, epoch TEXT NOT NULL,
  period_minutes REAL NOT NULL, inclination REAL NOT NULL
);
CREATE INDEX satellites_name ON satellites(name COLLATE NOCASE);`,
		Build:  buildSatellites,
		Import: importSatellites,
		// CelesTrak updates GP data every two hours and asks to be fetched at most once
		// per update; four hours keeps passes accurate to seconds.
		Every: 4 * time.Hour,
		Main:  "satellites", MaxShrink: 0.3,
		NoSeed: true,
	})
}

// satelliteGroups are CelesTrak's groups that are fetched, with how they are named here.
var satelliteGroups = []struct{ celestrak, name string }{
	{"stations", "space stations"},
	{"amateur", "amateur radio"},
}

const celestrakGP = "https://celestrak.org/NORAD/elements/gp.php?FORMAT=tle&GROUP="

func importSatellites(im *Importer) (Source, error) {
	var all bytes.Buffer
	for _, group := range satelliteGroups {
		raw, err := im.Get(celestrakGP + group.celestrak)
		if err != nil {
			// CelesTrak asks software to stop at the first refusal rather than keep asking.
			return Source{}, err
		}
		if !bytes.Contains(raw, []byte("\n1 ")) {
			return Source{}, fmt.Errorf("CelesTrak's %s group is not in TLE form: %.80q", group.celestrak, raw)
		}
		if err := im.Write("satellites/"+group.celestrak+".tle", raw); err != nil {
			return Source{}, err
		}
		all.Write(raw)
	}
	version := digest(all.Bytes())[:12]
	if im.CurrentVersion() == version {
		return Source{}, ErrNotChanged
	}
	return Source{
		Name:        "CelesTrak current GP element sets",
		Publisher:   "CelesTrak (T.S. Kelso), from US Space Force data via Space-Track.org",
		URL:         "https://celestrak.org/NORAD/elements/",
		Licence:     "None stated by CelesTrak. USSPACECOM allows redistribution of basic space situational awareness data with credit; only results computed from it are published here",
		LicenceURL:  "https://www.space-track.org/documentation#/user_agree",
		Version:     version,
		SHA256:      digest(all.Bytes()),
		Attribution: "Orbit elements from CelesTrak (celestrak.org), based on US Space Force data from Space-Track.org.",
	}, nil
}

func buildSatellites(b *Builder) error {
	type found struct {
		elements     *orbit.Elements
		line1, line2 string
		groups       []string
	}
	satellites := map[int]*found{}
	for _, group := range satelliteGroups {
		raw, err := os.ReadFile(b.Path("satellites", group.celestrak+".tle"))
		if err != nil {
			return err
		}
		var lines []string
		scanner := bufio.NewScanner(bytes.NewReader(raw))
		for scanner.Scan() {
			if line := strings.TrimRight(scanner.Text(), " \r"); line != "" {
				lines = append(lines, line)
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
		if len(lines)%3 != 0 {
			return fmt.Errorf("%s.tle: %d lines, not sets of three", group.celestrak, len(lines))
		}
		for i := 0; i < len(lines); i += 3 {
			elements, err := orbit.Parse(lines[i], lines[i+1], lines[i+2])
			if err != nil {
				b.Warnf("%s: %v; left out", strings.TrimSpace(lines[i]), err)
				continue
			}
			if existing, ok := satellites[elements.Number]; ok {
				existing.groups = append(existing.groups, group.name)
				continue
			}
			satellites[elements.Number] = &found{elements, lines[i+1], lines[i+2], []string{group.name}}
		}
	}
	if len(satellites) == 0 {
		return errors.New("no satellites in the element sets")
	}
	insert, err := b.Tx.Prepare(`INSERT INTO satellites VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	numbers := make([]int, 0, len(satellites))
	for number := range satellites {
		numbers = append(numbers, number)
	}
	sort.Ints(numbers)
	for _, number := range numbers {
		s := satellites[number]
		if _, err := insert.Exec(number, s.elements.Name, strings.Join(s.groups, ","), s.line1, s.line2,
			s.elements.Epoch.UTC().Format(time.RFC3339), s.elements.Period().Minutes(), s.elements.Inclination()); err != nil {
			return err
		}
	}
	b.Count("satellites", len(satellites))
	return nil
}
