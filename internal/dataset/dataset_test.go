package dataset

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var (
	fixtureData  = filepath.Join("..", "api", "testdata", "data")
	fixtureLogos = filepath.Join("..", "api", "testdata", "logos")
)

func mustSpec(t *testing.T, name string) *Spec {
	t.Helper()
	spec, ok := Lookup(name)
	if !ok {
		t.Fatalf("no dataset %q", name)
	}
	return spec
}

func TestBuildSeedsFromFixtures(t *testing.T) {
	out := t.TempDir()
	built, warnings, err := BuildSeeds(SeedOptions{DataDir: fixtureData, LogosDir: fixtureLogos, OutDir: out, Strict: true, Only: FixtureNames()})
	if err != nil {
		t.Fatal(err)
	}
	fixed, _, err := BuildSeeds(SeedOptions{DataDir: filepath.Join("..", "..", "data"), OutDir: out, Strict: true, Only: FixedNames()})
	built = append(built, fixed...)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]map[string]int{}
	for _, meta := range built {
		counts[meta.Dataset] = meta.Counts
	}
	for dataset, minimum := range map[string]map[string]int{
		"logos":            {"logos": 10},
		"pincode":          {"pincodes": 3, "post_offices": 10},
		"ifsc":             {"banks": 3, "branches": 10},
		"rto":              {"rto_codes": 100},
		"holidays-bank":    {"holidays": 10, "year:2026": 10},
		"holidays-market":  {"holidays": 10},
		"holidays-central": {"holidays": 10},
		"thirukkural":      {"kurals": 1330, "chapters": 133},
	} {
		for kind, want := range minimum {
			if got := counts[dataset][kind]; got < want {
				t.Errorf("%s has %d %s, want at least %d", dataset, got, kind, want)
			}
		}
	}
	for _, warning := range warnings {
		if strings.Contains(warning, "no entry in sources.json") {
			t.Errorf("a fixture dataset has no source record: %s", warning)
		}
	}
	if leftovers, _ := filepath.Glob(filepath.Join(out, "*.tmp")); len(leftovers) > 0 {
		t.Errorf("temporary files were left behind: %v", leftovers)
	}

	meta, err := ReadMetaFile(SeedPath(out, "rto"))
	if err != nil {
		t.Fatal(err)
	}
	if meta.Dataset != "rto" || meta.Schema != mustSpec(t, "rto").Schema || meta.Source.Licence != "CC BY-SA 4.0" || meta.Fetched() == "" {
		t.Errorf("the file does not describe itself: %+v", meta)
	}

	pincodes, err := Open(SeedPath(out, "pincode"))
	if err != nil {
		t.Fatal(err)
	}
	defer pincodes.Close()
	var placeholder int
	if err := pincodes.QueryRow(`SELECT COUNT(*) FROM pincodes WHERE pincode = '999999'`).Scan(&placeholder); err != nil || placeholder != 0 {
		t.Errorf("placeholder post office was imported (%d, %v)", placeholder, err)
	}
	var halfPlaced int
	if err := pincodes.QueryRow(`SELECT COUNT(*) FROM pincodes WHERE (lat IS NULL) != (lng IS NULL)`).Scan(&halfPlaced); err != nil || halfPlaced != 0 {
		t.Errorf("%d post offices have half a position (%v)", halfPlaced, err)
	}

	branches, err := Open(SeedPath(out, "ifsc"))
	if err != nil {
		t.Fatal(err)
	}
	defer branches.Close()
	var unnamed int
	if err := branches.QueryRow(`SELECT COUNT(*) FROM ifsc i LEFT JOIN banks b ON b.code = i.bank_code WHERE b.code IS NULL`).Scan(&unnamed); err != nil || unnamed != 0 {
		t.Errorf("%d branches have no bank row (%v)", unnamed, err)
	}
}

func TestBuildSeedsStrictness(t *testing.T) {
	empty, out := t.TempDir(), t.TempDir()
	if _, _, err := BuildSeeds(SeedOptions{DataDir: empty, LogosDir: empty, OutDir: out, Strict: true}); err == nil {
		t.Error("a strict build with no data should fail")
	}
	built, warnings, err := BuildSeeds(SeedOptions{DataDir: empty, LogosDir: empty, OutDir: out})
	if err != nil {
		t.Fatalf("a lenient build should skip what is missing: %v", err)
	}
	if len(built) != 0 || len(warnings) < 5 {
		t.Errorf("each missing dataset should be reported and none built: built %d, warnings %v", len(built), warnings)
	}
	if files, _ := filepath.Glob(filepath.Join(out, "*")); len(files) != 0 {
		t.Errorf("a failed build left files behind: %v", files)
	}
	if _, _, err := BuildSeeds(SeedOptions{DataDir: empty, OutDir: out, Only: []string{"nonsense"}}); err == nil {
		t.Error("an unknown dataset name should be an error")
	}
}

func TestBuildRejectsBadHolidays(t *testing.T) {
	cases := map[string]string{
		"wrong year":      `{"year":2026,"holidays":[{"date":"2027-01-26","name":"Republic Day","type":"market","region":"NSE","states":["*"]}]}`,
		"type of another": `{"year":2026,"holidays":[{"date":"2026-01-26","name":"Republic Day","type":"bank","region":"NSE","states":["*"]}]}`,
		"unknown type":    `{"year":2026,"holidays":[{"date":"2026-01-26","name":"Republic Day","type":"school","region":"NSE","states":["*"]}]}`,
		"unknown state":   `{"year":2026,"holidays":[{"date":"2026-01-26","name":"Republic Day","type":"market","region":"X","states":["ZZ"]}]}`,
		"no states":       `{"year":2026,"holidays":[{"date":"2026-01-26","name":"Republic Day","type":"market","region":"NSE","states":[]}]}`,
		"bad date":        `{"year":2026,"holidays":[{"date":"26-01-2026","name":"Republic Day","type":"market","region":"NSE","states":["*"]}]}`,
		"impossible date": `{"year":2026,"holidays":[{"date":"2026-02-30","name":"Republic Day","type":"market","region":"NSE","states":["*"]}]}`,
	}
	for name, content := range cases {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "holidays", "market"), 0o755)
		os.WriteFile(filepath.Join(dir, "holidays", "market", "2026.json"), []byte(content), 0o644)
		if _, _, err := Build(mustSpec(t, "holidays-market"), BuildOptions{Dir: dir, Out: filepath.Join(t.TempDir(), "x.db")}); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestParseNSEHolidays(t *testing.T) {
	raw := []byte(`{"CM":[
		{"tradingDate":"26-Jan-2026","weekDay":"Monday","description":"Republic Day"},
		{"tradingDate":"08-Nov-2026","weekDay":"Sunday","description":"Diwali Laxmi Pujan*"}],
		"FO":[{"tradingDate":"01-Jan-2026","description":"Other segment"}]}`)
	holidays, err := parseNSEHolidays(raw, "market")
	if err != nil || len(holidays) != 2 {
		t.Fatalf("got %v (%v)", holidays, err)
	}
	if holidays[0].Date != "2026-01-26" || holidays[0].Type != "market" || holidays[0].States[0] != "*" {
		t.Errorf("first holiday: %+v", holidays[0])
	}
	if holidays[1].Name != "Diwali Laxmi Pujan" || !strings.Contains(holidays[1].Notes, "Muhurat") {
		t.Errorf("the Muhurat marker was not understood: %+v", holidays[1])
	}
	if clearing, _ := parseNSEHolidays(raw, "clearing"); clearing[1].Notes != "" || clearing[1].Type != "clearing" {
		t.Errorf("clearing entries carry no trading note: %+v", clearing[1])
	}
	if _, err := parseNSEHolidays([]byte(`{"FO":[]}`), "market"); err == nil {
		t.Error("a response without the equities segment should be an error")
	}
	if _, err := parseNSEHolidays([]byte(`<html>blocked</html>`), "market"); err == nil {
		t.Error("a response that is not JSON should be an error")
	}
}

// ---- seed against server copy ----

// buildRTOCopy builds the RTO fixture as if its source had been fetched at the given moment.
func buildRTOCopy(t *testing.T, out, retrieved string, schema int) {
	t.Helper()
	spec := *mustSpec(t, "rto")
	spec.Schema = schema
	if _, _, err := Build(&spec, BuildOptions{Dir: fixtureData, Out: out, Source: Source{Name: "test", Retrieved: retrieved}}); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentChoosesBetweenSeedAndServerCopy(t *testing.T) {
	spec := mustSpec(t, "rto")
	seed, live := t.TempDir(), t.TempDir()
	at := func(day int) time.Time { return time.Date(2026, 10, day, 12, 0, 0, 0, time.UTC) }
	liveCopy := func(day int, schema int) string {
		path := filepath.Join(live, LiveName("rto", at(day)))
		buildRTOCopy(t, path, at(day).Format(time.RFC3339), schema)
		return path
	}
	check := func(step, wantPath string, wantLive, wantOK bool) {
		t.Helper()
		path, _, isLive, ok := Current(spec, seed, live)
		if path != wantPath || isLive != wantLive || ok != wantOK {
			t.Errorf("%s: got %q (live %v, ok %v), want %q (live %v, ok %v)", step, filepath.Base(path), isLive, ok, filepath.Base(wantPath), wantLive, wantOK)
		}
	}

	check("nothing anywhere", "", false, false)

	seedPath := SeedPath(seed, "rto")
	buildRTOCopy(t, seedPath, "2026-10-10", spec.Schema)
	check("only a seed", seedPath, false, true)

	older := liveCopy(9, spec.Schema)
	check("server copy older than the seed", seedPath, false, true)

	newer := liveCopy(12, spec.Schema)
	check("server copy newer than the seed", newer, true, true)

	sameDay := filepath.Join(live, LiveName("rto", at(10)))
	buildRTOCopy(t, sameDay, at(10).Format(time.RFC3339), spec.Schema)
	os.Remove(newer)
	check("server copy fetched on the seed's own day wins", sameDay, true, true)
	os.Remove(sameDay)
	os.Remove(older)

	liveCopy(20, spec.Schema+1)
	check("server copy built for other tables is ignored", seedPath, false, true)

	// A push that carries a newer seed takes over from what the server fetched before it.
	usable := liveCopy(14, spec.Schema)
	check("server copy ahead of the seed", usable, true, true)
	buildRTOCopy(t, seedPath, "2026-10-15", spec.Schema)
	check("a newer seed arrives with a release", seedPath, false, true)

	os.WriteFile(filepath.Join(live, "rto.notastamp.db"), []byte("junk"), 0o644)
	os.WriteFile(filepath.Join(live, LiveName("rto", at(25))), []byte("not a database"), 0o644)
	check("junk files are skipped", seedPath, false, true)

	if got := LiveFiles(live, "rt"); len(got) != 0 {
		t.Errorf("a dataset must not pick up another's files: %v", got)
	}
}

// ---- refresh ----

// fakeRTOSource replaces the RTO importer with one that serves the fixture, optionally
// altered, and counts how often it was asked.
type fakeRTOSource struct {
	calls  int
	fail   error
	rename int // how many offices to rename
	drop   int // how many offices to leave out
}

func (f *fakeRTOSource) install(t *testing.T) {
	spec := mustSpec(t, "rto")
	original := spec.Import
	t.Cleanup(func() { spec.Import = original })
	spec.Import = func(im *Importer) (Source, error) {
		f.calls++
		if f.fail != nil {
			return Source{}, f.fail
		}
		raw, err := os.ReadFile(filepath.Join(fixtureData, "rto.json"))
		if err != nil {
			return Source{}, err
		}
		var offices []RTO
		if err := json.Unmarshal(raw, &offices); err != nil {
			return Source{}, err
		}
		offices = offices[f.drop:]
		for i := 0; i < f.rename && i < len(offices); i++ {
			offices[i].Office = "Renamed " + offices[i].Office
		}
		encoded, _ := json.Marshal(offices)
		return Source{Name: "fake RTO list", Licence: "CC BY-SA 4.0", Version: fmt.Sprintf("call %d", f.calls)}, im.Write("rto.json", encoded)
	}
}

func TestRefresh(t *testing.T) {
	spec := mustSpec(t, "rto")
	seed, live := t.TempDir(), t.TempDir()
	buildRTOCopy(t, SeedPath(seed, "rto"), "2026-01-01", spec.Schema)
	source := &fakeRTOSource{}
	source.install(t)

	now := time.Date(2026, 10, 13, 6, 0, 0, 0, time.UTC)
	run := func(force bool) RefreshResult {
		t.Helper()
		results, err := Refresh(RefreshOptions{SeedDir: seed, LiveDir: live, Only: []string{"rto"}, Force: force, Now: now})
		if err != nil || len(results) != 1 {
			t.Fatalf("refresh: %v, %d results", err, len(results))
		}
		return results[0]
	}
	current := func() (Meta, bool) {
		_, meta, isLive, _ := Current(spec, seed, live)
		return meta, isLive
	}

	// The seed is months old, so the first run fetches.
	if result := run(false); result.Outcome != "updated" {
		t.Fatalf("first run: %s (%v)", result.Outcome, result.Err)
	}
	meta, isLive := current()
	if !isLive || meta.Source.Name != "fake RTO list" || meta.Fetched() != now.Format(time.RFC3339) {
		t.Errorf("the refreshed copy is not in use: live %v, %+v", isLive, meta.Source)
	}
	if status := ReadStatus(live).Datasets["rto"]; status.Result != "updated" || status.LastSuccess == "" {
		t.Errorf("status not recorded: %+v", status)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(live, ".import-*")); len(leftovers) > 0 {
		t.Errorf("working directories were left behind: %v", leftovers)
	}

	// Nothing is due the next hour.
	now = now.Add(time.Hour)
	if result := run(false); result.Outcome != "fresh" || source.calls != 1 {
		t.Errorf("a recent copy was refreshed again: %s, %d calls", result.Outcome, source.calls)
	}

	// Two months on, the source is down: the copy in use stays, and the failure is recorded.
	now = now.Add(60 * 24 * time.Hour)
	source.fail = errors.New("503 from the source")
	if result := run(false); result.Outcome != "failed" || !strings.Contains(result.Err.Error(), "503") {
		t.Errorf("a failing source: %s (%v)", result.Outcome, result.Err)
	}
	if after, _ := current(); after.Fetched() != meta.Fetched() {
		t.Error("a failed refresh replaced the copy in use")
	}
	if status := ReadStatus(live).Datasets["rto"]; status.Result != "failed" || status.LastSuccess == "" || status.Error == "" {
		t.Errorf("failure not recorded beside the last success: %+v", status)
	}

	// It is not asked again an hour later, but is after the waiting time.
	calls := source.calls
	now = now.Add(time.Hour)
	if result := run(false); result.Outcome != "waiting" || source.calls != calls {
		t.Errorf("a source that just failed was asked again: %s", result.Outcome)
	}
	now = now.Add(retryAfter)
	source.fail = nil

	// The source comes back with a tenth of the offices renamed: that is held back.
	source.rename = 40
	if result := run(false); result.Outcome != "held" || !strings.Contains(result.Err.Error(), "renamed or removed") {
		t.Fatalf("a mangled source: %s (%v)", result.Outcome, result.Err)
	}
	if after, _ := current(); after.Fetched() != meta.Fetched() {
		t.Error("a held refresh replaced the copy in use")
	}
	if copies := LiveFiles(live, "rto"); len(copies) != 1 {
		t.Errorf("a held copy was left on disk: %v", copies)
	}

	// A source that lost many records is held back too.
	now = now.Add(retryAfter)
	source.rename, source.drop = 0, 30
	if result := run(false); result.Outcome != "held" || !strings.Contains(result.Err.Error(), "fell from") {
		t.Fatalf("a shrunken source: %s (%v)", result.Outcome, result.Err)
	}

	// A maintainer who has looked can push it through.
	if result := run(true); result.Outcome != "updated" {
		t.Fatalf("forced refresh: %s (%v)", result.Outcome, result.Err)
	}
	if after, _ := current(); after.Counts["rto_codes"] != meta.Counts["rto_codes"]-30 {
		t.Errorf("the forced copy is not in use: %v", after.Counts)
	}

	// Only the newest copies stay on disk.
	source.drop = 0
	for i := 0; i < 3; i++ {
		now = now.Add(time.Hour)
		if result := run(true); result.Outcome != "updated" {
			t.Fatalf("refresh %d: %s (%v)", i, result.Outcome, result.Err)
		}
	}
	if copies := LiveFiles(live, "rto"); len(copies) != 2 {
		t.Errorf("%d copies kept, want 2: %v", len(copies), copies)
	}
}

func TestRefreshOnlyTouchesWhatIsScheduled(t *testing.T) {
	seed, live := t.TempDir(), t.TempDir()
	if _, _, err := BuildSeeds(SeedOptions{DataDir: fixtureData, LogosDir: fixtureLogos, OutDir: seed, Only: []string{"rto", "pincode"}}); err != nil {
		t.Fatal(err)
	}
	source := &fakeRTOSource{}
	source.install(t)
	// Every other scheduled dataset would reach for the network; switch them off for this run.
	for _, spec := range Specs() {
		if spec.Name != "rto" && spec.Every > 0 {
			every := spec.Every
			spec.Every = 0
			defer func(s *Spec) { s.Every = every }(spec)
		}
	}

	results, err := Refresh(RefreshOptions{SeedDir: seed, LiveDir: live, Now: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Dataset != "rto" || results[0].Outcome != "updated" {
		t.Errorf("a run without names refreshes exactly the scheduled datasets: %+v", results)
	}
	if copies := LiveFiles(live, "pincode"); len(copies) != 0 {
		t.Error("a dataset that is refreshed by hand was fetched")
	}
	if _, err := Refresh(RefreshOptions{SeedDir: seed, LiveDir: live, Only: []string{"logos"}}); err == nil {
		t.Error("asking to refresh a dataset with no fetchable source should be an error")
	}
}

func TestHolidayRefreshKeepsEarlierYears(t *testing.T) {
	spec := mustSpec(t, "holidays-market")
	seed, live := t.TempDir(), t.TempDir()
	if _, _, err := BuildSeeds(SeedOptions{DataDir: fixtureData, LogosDir: fixtureLogos, OutDir: seed, Only: []string{spec.Name}}); err != nil {
		t.Fatal(err)
	}
	original := spec.Import
	defer func() { spec.Import = original }()
	// Stands in for the exchange once the calendar has turned: it lists 2027 only.
	spec.Import = func(im *Importer) (Source, error) {
		var holidays []Holiday
		for day := 1; day <= 12; day++ {
			holidays = append(holidays, Holiday{Date: fmt.Sprintf("2027-%02d-15", day), Name: "Test holiday", Type: "market", Region: "NSE equities", States: []string{"*"}})
		}
		if err := writeHolidayYear(im, "market", 2027, holidays); err != nil {
			return Source{}, err
		}
		return Source{Name: "fake exchange"}, carryForward(im, "market", map[int]bool{2027: true})
	}

	results, err := Refresh(RefreshOptions{SeedDir: seed, LiveDir: live, Only: []string{spec.Name}, Now: time.Date(2027, 1, 5, 0, 0, 0, 0, time.UTC)})
	if err != nil || results[0].Outcome != "updated" {
		t.Fatalf("refresh: %v %+v", err, results)
	}
	counts := results[0].Meta.Counts
	if counts["year:2027"] != 12 || counts["year:2026"] < 10 {
		t.Errorf("the refreshed copy should hold the new year and keep the old one: %v", counts)
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"2026-10-13T08:00:00Z", "2026-10-13", true},
		{"2026-10-13", "2026-10-13T08:00:00Z", false},
		{"2026-10-14", "2026-10-13T23:59:59Z", true},
		{"2026-10-13", "2026-10-13", false},
		{"2026-10-13", "", true},
		{"", "2026-10-13", false},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestRefreshRebuildsCopiesMadeObsoleteByARelease(t *testing.T) {
	spec := mustSpec(t, "rto")
	seed, live := t.TempDir(), t.TempDir()
	now := time.Date(2026, 10, 13, 6, 0, 0, 0, time.UTC)
	// The release brought a seed fetched yesterday, so by age nothing is due; but the
	// server's own copy was built for the previous tables.
	buildRTOCopy(t, SeedPath(seed, "rto"), "2026-10-12", spec.Schema)
	buildRTOCopy(t, filepath.Join(live, LiveName("rto", now.Add(-time.Hour))), now.Add(-time.Hour).Format(time.RFC3339), spec.Schema-1)
	source := &fakeRTOSource{}
	source.install(t)

	results, err := Refresh(RefreshOptions{SeedDir: seed, LiveDir: live, Only: []string{"rto"}, Now: now})
	if err != nil || results[0].Outcome != "updated" {
		t.Fatalf("an obsolete server copy should trigger a rebuild: %v %+v", err, results)
	}
	if copies := LiveFiles(live, "rto"); len(copies) != 1 {
		t.Errorf("the obsolete copy should be pruned: %v", copies)
	}
}
