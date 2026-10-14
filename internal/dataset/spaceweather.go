package dataset

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

// Space weather from NOAA's Space Weather Prediction Center: solar flux, sunspot
// number, the planetary K-index, the X-ray flux and NOAA's R, S and G scales. They feed
// the HF propagation estimate. The data is a work of the US government and in the
// public domain. It goes stale within hours, so it is never kept in the repository: a
// server fetches it every hour.

func init() {
	register(&Spec{
		Name:   "space-weather",
		Schema: 1,
		DDL: `
CREATE TABLE readings (name TEXT PRIMARY KEY, value REAL, text TEXT, observed TEXT NOT NULL) WITHOUT ROWID;
CREATE TABLE kp (time TEXT PRIMARY KEY, kp REAL NOT NULL, a INTEGER NOT NULL) WITHOUT ROWID;
CREATE TABLE daily (date TEXT PRIMARY KEY, flux INTEGER, sunspots INTEGER, flares_c INTEGER, flares_m INTEGER, flares_x INTEGER) WITHOUT ROWID;`,
		Build:  buildSpaceWeather,
		Import: importSpaceWeather,
		// The refresh timer fires hourly; just under an hour keeps it from skipping one.
		Every: 50 * time.Minute,
		Main:  "kp_readings", MaxShrink: 0.5,
		NoSeed: true,
	})
}

const swpc = "https://services.swpc.noaa.gov/"

// spaceWeatherFiles are fetched on every refresh. The optional ones may fail without
// failing the refresh; the response then leaves their readings out.
var spaceWeatherFiles = []struct {
	path     string
	optional bool
}{
	{"products/summary/10cm-flux.json", false},
	{"products/noaa-planetary-k-index.json", false},
	{"text/daily-solar-indices.txt", false},
	{"json/goes/primary/xray-flares-latest.json", false},
	{"products/noaa-scales.json", true},
	{"products/summary/solar-wind-speed.json", true},
}

func spaceWeatherFile(path string) string {
	return "space-weather/" + path[strings.LastIndex(path, "/")+1:]
}

func importSpaceWeather(im *Importer) (Source, error) {
	fetched := 0
	for _, file := range spaceWeatherFiles {
		raw, err := im.Get(swpc + file.path)
		if err != nil {
			if file.optional {
				im.Logf("space-weather: %v; going on without it", err)
				continue
			}
			return Source{}, err
		}
		if err := im.Write(spaceWeatherFile(file.path), raw); err != nil {
			return Source{}, err
		}
		fetched++
	}
	return Source{
		Name:        "Space weather data",
		Publisher:   "NOAA Space Weather Prediction Center",
		URL:         "https://www.swpc.noaa.gov/",
		Licence:     "Public domain (work of the US government)",
		LicenceURL:  "https://www.usa.gov/government-copyright",
		Version:     fmt.Sprintf("%d of %d files", fetched, len(spaceWeatherFiles)),
		Attribution: "Space weather data from the NOAA Space Weather Prediction Center.",
	}, nil
}

func buildSpaceWeather(b *Builder) error {
	reading, err := b.Tx.Prepare(`INSERT OR REPLACE INTO readings VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer reading.Close()
	read := func(path string, into any) (bool, error) {
		raw, err := os.ReadFile(b.Path(spaceWeatherFile(path)))
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if err := json.Unmarshal(raw, into); err != nil {
			return false, fmt.Errorf("%s: %w", path, err)
		}
		return true, nil
	}

	// Solar flux at 10.7 cm, measured three times a day at Penticton.
	var flux []struct {
		Flux float64 `json:"flux"`
		Time string  `json:"time_tag"`
	}
	if ok, err := read("products/summary/10cm-flux.json", &flux); err != nil || !ok || len(flux) == 0 {
		return fmt.Errorf("10cm-flux.json: no reading (%v)", err)
	}
	if _, err := reading.Exec("solar_flux", flux[0].Flux, nil, utcStamp(flux[0].Time)); err != nil {
		return err
	}

	// The planetary K-index, every three hours for the last week.
	var kp []struct {
		Time string  `json:"time_tag"`
		Kp   float64 `json:"Kp"`
		A    int     `json:"a_running"`
	}
	if ok, err := read("products/noaa-planetary-k-index.json", &kp); err != nil || !ok || len(kp) == 0 {
		return fmt.Errorf("noaa-planetary-k-index.json: no readings (%v)", err)
	}
	insertKp, err := b.Tx.Prepare(`INSERT OR REPLACE INTO kp VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insertKp.Close()
	for _, row := range kp {
		if row.Kp < 0 || row.Kp > 9 || row.Time == "" {
			continue
		}
		if _, err := insertKp.Exec(utcStamp(row.Time), row.Kp, row.A); err != nil {
			return err
		}
		b.Add("kp_readings", 1)
	}

	// The current X-ray class and the latest flare, from the primary GOES satellite.
	var xray []struct {
		Time     string  `json:"time_tag"`
		Class    string  `json:"current_class"`
		MaxClass string  `json:"max_class"`
		MaxTime  string  `json:"max_time"`
		MaxFlux  float64 `json:"max_xrlong"`
	}
	if ok, err := read("json/goes/primary/xray-flares-latest.json", &xray); err != nil || !ok || len(xray) == 0 || xray[0].Class == "" {
		return fmt.Errorf("xray-flares-latest.json: no reading (%v)", err)
	}
	current, err := xrayFlux(xray[0].Class)
	if err != nil {
		return err
	}
	if _, err := reading.Exec("xray", current, xray[0].Class, utcStamp(xray[0].Time)); err != nil {
		return err
	}
	if xray[0].MaxClass != "" && xray[0].MaxTime != "" {
		if _, err := reading.Exec("xray_latest_flare", xray[0].MaxFlux, xray[0].MaxClass, utcStamp(xray[0].MaxTime)); err != nil {
			return err
		}
	}

	// NOAA's scales for radio blackouts (R), solar radiation storms (S) and geomagnetic
	// storms (G); key "0" is now.
	var scales map[string]struct {
		Date string                  `json:"DateStamp"`
		Time string                  `json:"TimeStamp"`
		R    struct{ Scale *string } `json:"R"`
		S    struct{ Scale *string } `json:"S"`
		G    struct{ Scale *string } `json:"G"`
	}
	if ok, err := read("products/noaa-scales.json", &scales); err != nil {
		return err
	} else if now, found := scales["0"]; ok && found {
		observed := now.Date + "T" + now.Time + "Z"
		for name, scale := range map[string]*string{"scale_r": now.R.Scale, "scale_s": now.S.Scale, "scale_g": now.G.Scale} {
			if scale == nil {
				continue
			}
			level, err := strconv.Atoi(*scale)
			if err != nil {
				continue
			}
			if _, err := reading.Exec(name, level, nil, observed); err != nil {
				return err
			}
		}
	}

	var wind []struct {
		Speed float64 `json:"proton_speed"`
		Time  string  `json:"time_tag"`
	}
	if ok, err := read("products/summary/solar-wind-speed.json", &wind); err != nil {
		return err
	} else if ok && len(wind) > 0 && wind[0].Speed > 0 {
		if _, err := reading.Exec("solar_wind_speed", wind[0].Speed, nil, utcStamp(wind[0].Time)); err != nil {
			return err
		}
	}

	return buildDailySolar(b)
}

// buildDailySolar reads the last 30 days of flux, sunspot number and flare counts.
func buildDailySolar(b *Builder) error {
	raw, err := os.ReadFile(b.Path("space-weather", "daily-solar-indices.txt"))
	if err != nil {
		return err
	}
	insert, err := b.Tx.Prepare(`INSERT OR REPLACE INTO daily VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 12 || strings.HasPrefix(fields[0], "#") || strings.HasPrefix(fields[0], ":") {
			continue
		}
		date, err := time.Parse("2006 01 02", strings.Join(fields[:3], " "))
		if err != nil {
			continue
		}
		number := func(field string) any {
			value, err := strconv.Atoi(field)
			if err != nil || value < 0 {
				return nil
			}
			return value
		}
		// Date, flux, sunspot number, sunspot area, new regions, mean field, X-ray
		// background, then C, M and X flare counts.
		if _, err := insert.Exec(date.Format("2006-01-02"), number(fields[3]), number(fields[4]), number(fields[9]), number(fields[10]), number(fields[11])); err != nil {
			return err
		}
		b.Add("days", 1)
	}
	if b.counts["days"] == 0 {
		return errors.New("daily-solar-indices.txt: no days found; the file may have changed shape")
	}
	return scanner.Err()
}

// xrayFlux turns a class such as C1.1 or M2.5 into watts per square metre.
func xrayFlux(class string) (float64, error) {
	if len(class) < 2 {
		return 0, fmt.Errorf("X-ray class %q", class)
	}
	scale, ok := map[byte]float64{'A': 1e-8, 'B': 1e-7, 'C': 1e-6, 'M': 1e-5, 'X': 1e-4}[class[0]]
	value, err := strconv.ParseFloat(class[1:], 64)
	if !ok || err != nil || value < 0 || math.IsInf(value, 0) {
		return 0, fmt.Errorf("X-ray class %q", class)
	}
	return value * scale, nil
}

// utcStamp makes NOAA's times, some without a zone and all in UTC, into RFC 3339.
func utcStamp(value string) string {
	value = strings.TrimSpace(strings.Replace(value, " ", "T", 1))
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC().Format(time.RFC3339)
		}
	}
	return value
}
