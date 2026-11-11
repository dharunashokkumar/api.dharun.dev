package dataset

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The Local Government Directory (LGD) of the Ministry of Panchayati Raj, from data.gov.in
// (Government Open Data License - India): every state, district, sub-district and village
// with its LGD code and Census 2011 code, and the PIN code of each village. LGD updates the
// lists on the first of each month. 720,000 villages are too many to hold in memory in the
// refresh job, so each page is written to a compressed file as it arrives. A server's own
// copy only (NoSeed); it needs DATA_GOV_IN_KEY like mandi and air-quality.

const (
	lgdDistricts    = "37231365-78ba-44d5-ac22-3deec40b9197"
	lgdSubdistricts = "6be51a29-876a-403a-a6da-42fde795e751"
	lgdVillages     = "c967fe8f-69c4-42df-8afc-8a2c98057437"
	lgdVillagePins  = "f17a1608-5f10-4610-bb50-a63c80d83974"
)

func init() {
	register(&Spec{
		Name:   "lgd",
		Schema: 1,
		DDL: `
CREATE TABLE districts (code INTEGER PRIMARY KEY, name TEXT NOT NULL, name_local TEXT NOT NULL, census2011 TEXT NOT NULL,
  state_code INTEGER NOT NULL, state TEXT NOT NULL, state_local TEXT NOT NULL);
CREATE INDEX districts_state ON districts(state_code);
CREATE TABLE subdistricts (code INTEGER PRIMARY KEY, name TEXT NOT NULL, name_local TEXT NOT NULL, census2011 TEXT NOT NULL,
  district_code INTEGER NOT NULL);
CREATE INDEX subdistricts_district ON subdistricts(district_code);
CREATE TABLE villages (code INTEGER PRIMARY KEY, name TEXT NOT NULL, name_local TEXT NOT NULL, census2011 TEXT NOT NULL,
  subdistrict_code INTEGER NOT NULL, pincode TEXT);
CREATE INDEX villages_subdistrict ON villages(subdistrict_code);
CREATE INDEX villages_name ON villages(name COLLATE NOCASE);
CREATE INDEX villages_pincode ON villages(pincode) WHERE pincode IS NOT NULL;`,
		Build:  buildLGD,
		Import: importLGD,
		Every:  30 * 24 * time.Hour,
		Main:   "villages", MaxShrink: 0.05,
		NoSeed: true,
	})
}

// govStream downloads every record of a data.gov.in resource and writes each as one JSON
// line to a gzip file under the import directory, a page at a time. It returns the count.
func govStream(im *Importer, resource, name string) (int, error) {
	key := strings.TrimSpace(os.Getenv("DATA_GOV_IN_KEY"))
	if key == "" {
		return 0, errNoKey
	}
	clean := func(err error) error { return errors.New(strings.ReplaceAll(err.Error(), key, "REDACTED")) }
	path := im.Path(filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	file, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	zipped, _ := gzip.NewWriterLevel(file, gzip.BestCompression)
	const page = 10000
	count := 0
	for offset := 0; ; offset += page {
		query := url.Values{"api-key": {key}, "format": {"json"}, "limit": {strconv.Itoa(page)}, "offset": {strconv.Itoa(offset)}}
		var raw []byte
		for attempt := 0; ; attempt++ {
			raw, err = im.Get("https://api.data.gov.in/resource/" + resource + "?" + query.Encode())
			if err == nil || attempt == 2 {
				break
			}
			time.Sleep(10 * time.Second)
		}
		if err != nil {
			return 0, clean(err)
		}
		var answer struct {
			Status  string            `json:"status"`
			Message string            `json:"message"`
			Total   int               `json:"total"`
			Records []json.RawMessage `json:"records"`
		}
		if err := json.Unmarshal(raw, &answer); err != nil {
			return 0, fmt.Errorf("data.gov.in resource %s, offset %d: not JSON", resource, offset)
		}
		if answer.Status != "ok" {
			return 0, clean(fmt.Errorf("data.gov.in resource %s: %s", resource, answer.Message))
		}
		// The platform pretty-prints its answers: each record is compacted to one line.
		var line bytes.Buffer
		for _, r := range answer.Records {
			line.Reset()
			if json.Compact(&line, r) != nil {
				continue
			}
			line.WriteByte('\n')
			zipped.Write(line.Bytes())
		}
		count += len(answer.Records)
		if im.Log != nil && offset%100000 == 0 {
			im.Logf("lgd: %s %d of %d", name, count, answer.Total)
		}
		if len(answer.Records) < page || count >= answer.Total || offset > 2000000 {
			break
		}
	}
	return count, zipped.Close()
}

// eachLine calls fn with each JSON line of a gzip file.
func eachLine(path string, fn func(map[string]any) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	zipped, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(zipped)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		var r map[string]any
		if json.Unmarshal(scanner.Bytes(), &r) != nil {
			continue
		}
		if err := fn(r); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func lgdCode(r map[string]any, name string) int {
	switch v := r[name].(type) {
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(v))
		return n
	}
	return 0
}

// lgdText trims a name and drops the platform's placeholders.
func lgdText(r map[string]any, name string) string {
	s := govText(r, name)
	if s == "NA" || s == "na" {
		return ""
	}
	return s
}

func buildLGD(b *Builder) error {
	dir := b.Path("lgd")
	district, err := b.Tx.Prepare(`INSERT OR REPLACE INTO districts VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer district.Close()
	if err := eachLine(filepath.Join(dir, "districts.jsonl.gz"), func(r map[string]any) error {
		_, err := district.Exec(lgdCode(r, "district_code"), lgdText(r, "district_name_english"), lgdText(r, "district_name_local"), lgdText(r, "district_census2011_code"),
			lgdCode(r, "state_code"), lgdText(r, "state_name_english"), lgdText(r, "state_name_local"))
		b.Add("districts", 1)
		return err
	}); err != nil {
		return err
	}
	sub, err := b.Tx.Prepare(`INSERT OR REPLACE INTO subdistricts VALUES (?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer sub.Close()
	if err := eachLine(filepath.Join(dir, "subdistricts.jsonl.gz"), func(r map[string]any) error {
		_, err := sub.Exec(lgdCode(r, "subdistrict_code"), lgdText(r, "subdistrict_name_english"), lgdText(r, "subdistrict_name_local"),
			lgdText(r, "subdistrict_census2011_code"), lgdCode(r, "district_code"))
		b.Add("subdistricts", 1)
		return err
	}); err != nil {
		return err
	}
	villages := 0
	village, err := b.Tx.Prepare(`INSERT OR REPLACE INTO villages VALUES (?,?,?,?,?,NULL)`)
	if err != nil {
		return err
	}
	defer village.Close()
	if err := eachLine(filepath.Join(dir, "villages.jsonl.gz"), func(r map[string]any) error {
		code := lgdCode(r, "villageCode")
		if code == 0 {
			return nil
		}
		_, err := village.Exec(code, lgdText(r, "villageNameEnglish"), lgdText(r, "villageNameLocal"), lgdText(r, "villageCensus2011Code"), lgdCode(r, "subdistrictCode"))
		b.Add("villages", 1)
		villages++
		return err
	}); err != nil {
		return err
	}
	// The real list has over 700,000; a test fixture a few dozen. None means the files
	// could not be read, and an empty copy must never replace a good one.
	if villages == 0 {
		return errors.New("lgd: no village could be read from the source files")
	}
	pin, err := b.Tx.Prepare(`UPDATE villages SET pincode = ? WHERE code = ?`)
	if err != nil {
		return err
	}
	defer pin.Close()
	return eachLine(filepath.Join(dir, "village-pincodes.jsonl.gz"), func(r map[string]any) error {
		if p := lgdCode(r, "pincode"); p >= 110000 && p <= 999999 {
			_, err := pin.Exec(strconv.Itoa(p), lgdCode(r, "villageCode"))
			return err
		}
		return nil
	})
}

func importLGD(im *Importer) (Source, error) {
	hash := sha256.New()
	for _, part := range []struct{ resource, name string }{
		{lgdDistricts, "lgd/districts.jsonl.gz"}, {lgdSubdistricts, "lgd/subdistricts.jsonl.gz"},
		{lgdVillages, "lgd/villages.jsonl.gz"}, {lgdVillagePins, "lgd/village-pincodes.jsonl.gz"},
	} {
		count, err := govStream(im, part.resource, part.name)
		if errors.Is(err, errNoKey) {
			im.Logf("lgd: %v; skipped", err)
			return Source{}, ErrNotChanged
		}
		if err != nil {
			return Source{}, err
		}
		if count < 30 {
			return Source{}, fmt.Errorf("LGD %s gave %d rows: fewer than usual", part.name, count)
		}
		file, err := os.Open(im.Path(filepath.FromSlash(part.name)))
		if err != nil {
			return Source{}, err
		}
		zipped, err := gzip.NewReader(file)
		if err == nil {
			io.Copy(hash, zipped)
		}
		file.Close()
	}
	version := hex.EncodeToString(hash.Sum(nil))[:12]
	if im.CurrentVersion() == version {
		return Source{}, ErrNotChanged
	}
	return govSource("Local Government Directory (LGD): districts, sub-districts, villages and their PIN codes",
		"Ministry of Panchayati Raj (Local Government Directory), via data.gov.in", lgdVillages, version,
		"Local Government Directory codes from the Ministry of Panchayati Raj, published on data.gov.in under the Government Open Data License - India."), nil
}
