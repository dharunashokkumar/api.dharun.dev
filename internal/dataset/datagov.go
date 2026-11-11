package dataset

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Two datasets from the Open Government Data Platform India (data.gov.in), under the
// Government Open Data License - India, which allows redistribution with attribution:
//
//   - mandi: the day's wholesale prices of farm produce in the markets (mandis) that
//     report to Agmarknet; a server's own copy keeps the last 30 days.
//   - air-quality: the latest readings of CPCB's continuous monitoring stations, every
//     pollutant's sub-index; refreshed hourly, the latest hour only.
//
// The platform needs an API key (DATA_GOV_IN_KEY), sent in the address, as it accepts no
// other way. The key is never logged: every error is cleaned of it. Without a key a
// refresh is skipped quietly, so a server without one raises no alarm.

const (
	mandiResource      = "9ef84268-d588-465a-a308-a864a43d0070"
	airQualityResource = "3b01bcb8-0b14-4abf-b6f2-c1bfd384ba69"
	mandiKeepDays      = 30
)

func init() {
	register(&Spec{
		Name:   "mandi",
		Schema: 1,
		DDL: `
CREATE TABLE prices (date TEXT NOT NULL, state TEXT NOT NULL, district TEXT NOT NULL, market TEXT NOT NULL, commodity TEXT NOT NULL,
  variety TEXT NOT NULL, grade TEXT NOT NULL, min_price REAL, max_price REAL, modal_price REAL,
  PRIMARY KEY (date, state, district, market, commodity, variety, grade)) WITHOUT ROWID;
CREATE INDEX prices_commodity ON prices(commodity COLLATE NOCASE, date);`,
		Build:  buildMandi,
		Import: importMandi,
		Every:  3 * time.Hour,
		Main:   "prices", MaxShrink: 0.5,
		NoSeed: true,
	})
	register(&Spec{
		Name:   "air-quality",
		Schema: 1,
		DDL: `
CREATE TABLE readings (station TEXT NOT NULL, city TEXT NOT NULL, state TEXT NOT NULL, latitude REAL, longitude REAL, updated TEXT NOT NULL,
  pollutant TEXT NOT NULL, min REAL, max REAL, avg REAL, PRIMARY KEY (station, pollutant)) WITHOUT ROWID;`,
		Build:  buildAirQuality,
		Import: importAirQuality,
		Every:  50 * time.Minute,
		Main:   "readings", MaxShrink: 0.5,
		NoSeed: true,
	})
}

// errNoKey is why a data.gov.in refresh was skipped.
var errNoKey = errors.New("DATA_GOV_IN_KEY is not set")

// govRecords downloads every record of a data.gov.in resource, a page at a time.
func govRecords(im *Importer, resource string, filters url.Values) ([]map[string]any, error) {
	key := strings.TrimSpace(os.Getenv("DATA_GOV_IN_KEY"))
	if key == "" {
		return nil, errNoKey
	}
	clean := func(err error) error { return errors.New(strings.ReplaceAll(err.Error(), key, "REDACTED")) }
	const page = 2000
	var out []map[string]any
	for offset := 0; ; offset += page {
		query := url.Values{"api-key": {key}, "format": {"json"}, "limit": {strconv.Itoa(page)}, "offset": {strconv.Itoa(offset)}}
		for k, v := range filters {
			query[k] = v
		}
		raw, err := im.Get("https://api.data.gov.in/resource/" + resource + "?" + query.Encode())
		if err != nil {
			return nil, clean(err)
		}
		var answer struct {
			Status  string           `json:"status"`
			Message string           `json:"message"`
			Total   int              `json:"total"`
			Records []map[string]any `json:"records"`
		}
		if err := json.Unmarshal(raw, &answer); err != nil {
			return nil, fmt.Errorf("data.gov.in resource %s: not JSON", resource)
		}
		if answer.Status != "ok" {
			return nil, clean(fmt.Errorf("data.gov.in resource %s: %s", resource, answer.Message))
		}
		out = append(out, answer.Records...)
		if len(answer.Records) < page || len(out) >= answer.Total || offset > 200000 {
			return out, nil
		}
	}
}

func govText(r map[string]any, name string) string {
	switch v := r[name].(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

// govNumber reads a number the platform may send as a number or as text; "NA" is none.
func govNumber(r map[string]any, name string) *float64 {
	switch v := r[name].(type) {
	case float64:
		return &v
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return &f
		}
	}
	return nil
}

func govSource(name, publisher, resource, version, attribution string) Source {
	return Source{
		Name: name, Publisher: publisher, URL: "https://www.data.gov.in/resource/" + resource,
		Licence: "Government Open Data License - India", Version: version, Attribution: attribution,
	}
}

// ---- mandi prices ----

// MandiPrice is one commodity's price in one market on one day, in rupees a quintal.
type MandiPrice struct {
	Date      string   `json:"date"`
	State     string   `json:"state"`
	District  string   `json:"district"`
	Market    string   `json:"market"`
	Commodity string   `json:"commodity"`
	Variety   string   `json:"variety"`
	Grade     string   `json:"grade"`
	Min       *float64 `json:"min_price"`
	Max       *float64 `json:"max_price"`
	Modal     *float64 `json:"modal_price"`
}

func (p MandiPrice) key() string {
	return strings.Join([]string{p.Date, p.State, p.District, p.Market, p.Commodity, p.Variety, p.Grade}, "|")
}

func buildMandi(b *Builder) error {
	raw, err := os.ReadFile(b.Path("mandi", "prices.json"))
	if err != nil {
		return err
	}
	var prices []MandiPrice
	if err := json.Unmarshal(raw, &prices); err != nil {
		return err
	}
	insert, err := b.Tx.Prepare(`INSERT OR REPLACE INTO prices VALUES (?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	for _, p := range prices {
		if _, err := insert.Exec(p.Date, p.State, p.District, p.Market, p.Commodity, p.Variety, p.Grade, p.Min, p.Max, p.Modal); err != nil {
			return err
		}
		b.Add("prices", 1)
	}
	return nil
}

func importMandi(im *Importer) (Source, error) {
	records, err := govRecords(im, mandiResource, nil)
	if errors.Is(err, errNoKey) {
		im.Logf("mandi: %v; skipped", err)
		return Source{}, ErrNotChanged
	}
	if err != nil {
		return Source{}, err
	}
	all := map[string]MandiPrice{}
	newest := ""
	for _, r := range records {
		// arrival_date is DD/MM/YYYY.
		d, err := time.Parse("02/01/2006", govText(r, "arrival_date"))
		if err != nil {
			continue
		}
		p := MandiPrice{Date: d.Format(time.DateOnly), State: govText(r, "state"), District: govText(r, "district"), Market: govText(r, "market"),
			Commodity: govText(r, "commodity"), Variety: govText(r, "variety"), Grade: govText(r, "grade"),
			Min: govNumber(r, "min_price"), Max: govNumber(r, "max_price"), Modal: govNumber(r, "modal_price")}
		if p.Market == "" || p.Commodity == "" {
			continue
		}
		all[p.key()] = p
		if p.Date > newest {
			newest = p.Date
		}
	}
	if len(all) < 100 {
		return Source{}, fmt.Errorf("Agmarknet gave %d prices: fewer than usual", len(all))
	}
	// Carry earlier days forward from the copy in use, keeping 30 days.
	cutoff := ""
	if t, err := time.Parse(time.DateOnly, newest); err == nil {
		cutoff = t.AddDate(0, 0, -mandiKeepDays).Format(time.DateOnly)
	}
	if im.Current != nil {
		if rows, err := im.Current.Query(`SELECT date, state, district, market, commodity, variety, grade, min_price, max_price, modal_price FROM prices WHERE date > ?`, cutoff); err == nil {
			for rows.Next() {
				var p MandiPrice
				if rows.Scan(&p.Date, &p.State, &p.District, &p.Market, &p.Commodity, &p.Variety, &p.Grade, &p.Min, &p.Max, &p.Modal) == nil {
					if _, ok := all[p.key()]; !ok {
						all[p.key()] = p
					}
				}
			}
			rows.Close()
		}
	}
	list := make([]MandiPrice, 0, len(all))
	for _, p := range all {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].key() < list[j].key() })
	raw, _ := json.Marshal(list)
	version := digest(raw)[:12]
	if im.CurrentVersion() == version {
		return Source{}, ErrNotChanged
	}
	if err := im.Write("mandi/prices.json", raw); err != nil {
		return Source{}, err
	}
	return govSource("Current daily price of various commodities from various markets (Mandi)", "Directorate of Marketing and Inspection (Agmarknet), Ministry of Agriculture and Farmers Welfare, via data.gov.in",
		mandiResource, version, "Mandi prices from Agmarknet, published on data.gov.in under the Government Open Data License - India."), nil
}

// ---- air quality ----

// AirReading is one pollutant at one station: CPCB's sub-index for it over the last
// hours, its lowest, highest and average.
type AirReading struct {
	Station   string   `json:"station"`
	City      string   `json:"city"`
	State     string   `json:"state"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Updated   string   `json:"updated"` // YYYY-MM-DD HH:MM, India time
	Pollutant string   `json:"pollutant"`
	Min       *float64 `json:"min"`
	Max       *float64 `json:"max"`
	Avg       *float64 `json:"avg"`
}

func buildAirQuality(b *Builder) error {
	raw, err := os.ReadFile(b.Path("air-quality", "readings.json"))
	if err != nil {
		return err
	}
	var readings []AirReading
	if err := json.Unmarshal(raw, &readings); err != nil {
		return err
	}
	insert, err := b.Tx.Prepare(`INSERT OR REPLACE INTO readings VALUES (?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	for _, r := range readings {
		if _, err := insert.Exec(r.Station, r.City, r.State, r.Latitude, r.Longitude, r.Updated, r.Pollutant, r.Min, r.Max, r.Avg); err != nil {
			return err
		}
		b.Add("readings", 1)
	}
	return nil
}

func importAirQuality(im *Importer) (Source, error) {
	records, err := govRecords(im, airQualityResource, nil)
	if errors.Is(err, errNoKey) {
		im.Logf("air-quality: %v; skipped", err)
		return Source{}, ErrNotChanged
	}
	if err != nil {
		return Source{}, err
	}
	var list []AirReading
	for _, r := range records {
		// last_update is DD-MM-YYYY HH:MM:SS.
		t, err := time.Parse("02-01-2006 15:04:05", govText(r, "last_update"))
		if err != nil {
			continue
		}
		reading := AirReading{Station: govText(r, "station"), City: govText(r, "city"), State: govText(r, "state"),
			Latitude: govNumber(r, "latitude"), Longitude: govNumber(r, "longitude"), Updated: t.Format("2006-01-02 15:04"),
			Pollutant: govText(r, "pollutant_id"), Min: govNumber(r, "min_value"), Max: govNumber(r, "max_value"), Avg: govNumber(r, "avg_value")}
		if reading.Station == "" || reading.Pollutant == "" {
			continue
		}
		list = append(list, reading)
	}
	if len(list) < 100 {
		return Source{}, fmt.Errorf("CPCB gave %d readings: fewer than usual", len(list))
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Station != list[j].Station {
			return list[i].Station < list[j].Station
		}
		return list[i].Pollutant < list[j].Pollutant
	})
	raw, _ := json.Marshal(list)
	version := digest(raw)[:12]
	if im.CurrentVersion() == version {
		return Source{}, ErrNotChanged
	}
	if err := im.Write("air-quality/readings.json", raw); err != nil {
		return Source{}, err
	}
	return govSource("Real time Air Quality Index from various locations", "Central Pollution Control Board, Ministry of Environment, Forest and Climate Change, via data.gov.in",
		airQualityResource, version, "Air quality from CPCB's monitoring stations, published on data.gov.in under the Government Open Data License - India."), nil
}
