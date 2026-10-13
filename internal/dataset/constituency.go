package dataset

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/india"
)

// Assembly and parliamentary constituency boundaries from the DataMeet community's maps,
// used only to answer which constituency a point lies in; no map is published (Dharun,
// 2026-10-13). Assembly boundaries are CC BY 2.5 India; the parliamentary boundaries are
// the simplified 2019 set, CC0. The publisher warns that the assembly set is
// pre-delimitation for six north-eastern and northern states, marks Telangana's seats as
// Andhra Pradesh's, and is shifted in places.

func init() {
	register(&Spec{
		Name:   "constituencies",
		Schema: 1,
		DDL: `
CREATE TABLE constituencies (
  id INTEGER PRIMARY KEY, kind TEXT NOT NULL, state_code TEXT, state TEXT, number INTEGER NOT NULL,
  name TEXT NOT NULL, name_hi TEXT, reserved TEXT, district TEXT, parliament_number INTEGER,
  parliament_name TEXT, predelimitation INTEGER NOT NULL DEFAULT 0, wikidata TEXT, rings BLOB NOT NULL
);
CREATE INDEX constituencies_state ON constituencies(kind, state_code, number);
CREATE VIRTUAL TABLE constituency_boxes USING rtree(id, min_lng, max_lng, min_lat, max_lat);`,
		Build:  buildConstituencies,
		Import: importConstituencies,
		// The boundaries change only with a delimitation; refreshed by hand. Tests use the
		// real maps: cutting a shapefile down to a fixture would test the cutting.
		Main: "assembly", MaxShrink: 0.05,
		Fixed: true,
	})
}

var reservedSuffix = regexp.MustCompile(`\s*\((SC|ST)\)\s*$`)

func gunzipped(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(reader, 200<<20))
}

func buildConstituencies(b *Builder) error {
	insert, err := b.Tx.Prepare(`INSERT INTO constituencies VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	insertBox, err := b.Tx.Prepare(`INSERT INTO constituency_boxes VALUES (?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insertBox.Close()
	id := 0
	add := func(kind string, state india.State, known bool, number int, name, nameHi, reserved, district string, pcNumber int, pcName string, pre bool, wikidata string, rings []ring) error {
		if len(rings) == 0 {
			return nil
		}
		id++
		var stateCode, stateName any
		if known {
			stateCode, stateName = state.Code, state.Name
		}
		pcNo := any(nil)
		if pcNumber > 0 {
			pcNo = pcNumber
		}
		if _, err := insert.Exec(id, kind, stateCode, stateName, number, name, nullable(nameHi), nullable(reserved), nullable(district),
			pcNo, nullable(pcName), pre, nullable(wikidata), encodeRings(rings)); err != nil {
			return err
		}
		minLng, maxLng, minLat, maxLat := bounds(rings)
		_, err := insertBox.Exec(id, minLng, maxLng, minLat, maxLat)
		b.Add(kind, 1)
		return err
	}

	// Assembly constituencies: a shapefile and its attribute table.
	shp, err := gunzipped(b.Path("constituencies", "India_AC.shp.gz"))
	if err != nil {
		return err
	}
	dbf, err := gunzipped(b.Path("constituencies", "India_AC.dbf.gz"))
	if err != nil {
		return err
	}
	shapes, err := readPolygons(shp)
	if err != nil {
		return fmt.Errorf("India_AC.shp: %w", err)
	}
	rows, err := readDBF(dbf)
	if err != nil {
		return fmt.Errorf("India_AC.dbf: %w", err)
	}
	if len(shapes) != len(rows) {
		return fmt.Errorf("%d shapes but %d attribute rows", len(shapes), len(rows))
	}
	unknown := map[string]int{}
	for i, row := range rows {
		state, known := india.Lookup(row["ST_NAME"])
		if !known {
			unknown[row["ST_NAME"]]++
		}
		number, _ := strconv.Atoi(row["AC_NO"])
		pcNumber, _ := strconv.Atoi(row["PC_NO"])
		name, reserved := row["AC_NAME"], "GEN"
		if match := reservedSuffix.FindStringSubmatch(name); match != nil {
			name, reserved = reservedSuffix.ReplaceAllString(name, ""), match[1]
		}
		district := titleCase(strings.Trim(row["DIST_NAME"], " *"))
		pre := strings.Contains(strings.ToLower(row["STATUS"]), "pre")
		if err := add("assembly", state, known, number, strings.TrimSpace(name), "", reserved, district, pcNumber, titleCase(row["PC_NAME"]), pre, "", shapes[i]); err != nil {
			return err
		}
	}
	if len(unknown) > 0 {
		b.Warnf("assembly constituencies in places that are not a state: %s", topKeys(unknown, 8))
	}

	// Parliamentary constituencies: GeoJSON multipolygons.
	raw, err := gunzipped(b.Path("constituencies", "india_pc_2019_simplified.geojson.gz"))
	if err != nil {
		return err
	}
	var collection struct {
		Features []struct {
			Properties struct {
				Number   int    `json:"pc_no"`
				Name     string `json:"pc_name"`
				NameHi   string `json:"pc_name_hi"`
				Category string `json:"pc_category"`
				State    string `json:"st_name"`
				Wikidata string `json:"wikidata_qid"`
			} `json:"properties"`
			Geometry struct {
				Type        string          `json:"type"`
				Coordinates json.RawMessage `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}
	if err := json.Unmarshal(raw, &collection); err != nil {
		return fmt.Errorf("parliamentary constituencies: %w", err)
	}
	for _, feature := range collection.Features {
		var polygons [][][][2]float64
		switch feature.Geometry.Type {
		case "MultiPolygon":
			err = json.Unmarshal(feature.Geometry.Coordinates, &polygons)
		case "Polygon":
			var polygon [][][2]float64
			err = json.Unmarshal(feature.Geometry.Coordinates, &polygon)
			polygons = [][][][2]float64{polygon}
		default:
			err = fmt.Errorf("geometry %s", feature.Geometry.Type)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", feature.Properties.Name, err)
		}
		var rings []ring
		for _, polygon := range polygons {
			for _, points := range polygon {
				r := make(ring, 0, 2*len(points))
				for _, point := range points {
					r = append(r, point[0], point[1])
				}
				rings = append(rings, r)
			}
		}
		state, known := india.Lookup(feature.Properties.State)
		p := feature.Properties
		if err := add("parliament", state, known, p.Number, p.Name, p.NameHi, p.Category, "", 0, "", false, p.Wikidata, rings); err != nil {
			return err
		}
	}
	return nil
}

const dataMeet = "https://raw.githubusercontent.com/datameet/maps/"

func importConstituencies(im *Importer) (Source, error) {
	commit := im.Version
	if commit == "" {
		return Source{}, fmt.Errorf("give -version <commit> of github.com/datameet/maps, so the boundaries can be traced to an exact copy")
	}
	files := map[string]string{
		"assembly-constituencies/India_AC.shp":                          "constituencies/India_AC.shp.gz",
		"assembly-constituencies/India_AC.dbf":                          "constituencies/India_AC.dbf.gz",
		"parliamentary-constituencies/india_pc_2019_simplified.geojson": "constituencies/india_pc_2019_simplified.geojson.gz",
	}
	var all bytes.Buffer
	for from, to := range files {
		raw, err := im.Get(dataMeet + commit + "/" + from)
		if err != nil {
			return Source{}, err
		}
		all.Write(raw)
		if err := im.WriteGzip(to, raw); err != nil {
			return Source{}, err
		}
	}
	return Source{
		Name:        "Maps of assembly and parliamentary constituencies",
		Publisher:   "DataMeet India community (github.com/datameet/maps); parliamentary boundaries simplified by Arun Ganesh",
		URL:         "https://github.com/datameet/maps/tree/" + commit,
		Licence:     "Assembly boundaries CC BY 2.5 India; parliamentary boundaries (simplified, 2019) CC0",
		LicenceURL:  "https://creativecommons.org/licenses/by/2.5/in/",
		Version:     "commit " + commit[:min(12, len(commit))],
		SHA256:      digest(all.Bytes()),
		Attribution: "Constituency boundaries from the DataMeet India community's maps (assembly: CC BY 2.5 India; parliamentary: CC0).",
	}, nil
}
