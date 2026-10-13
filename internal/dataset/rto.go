package dataset

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/india"
)

// RTO is one Regional Transport Office registration code.
type RTO struct {
	Code         string `json:"code"`
	Prefix       string `json:"prefix"`
	Office       string `json:"office"`
	Jurisdiction string `json:"jurisdiction,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

func init() {
	register(&Spec{
		Name:   "rto",
		Schema: 1,
		DDL: `
CREATE TABLE rto (
  code TEXT PRIMARY KEY, prefix TEXT NOT NULL, state_code TEXT, state TEXT,
  office TEXT NOT NULL, jurisdiction TEXT, notes TEXT
) WITHOUT ROWID;`,
		Build:  buildRTO,
		Import: importRTO,
		Every:  30 * 24 * time.Hour,
		Main:   "rto_codes", MaxShrink: 0.03,
		Check: checkRTO,
	})
}

func buildRTO(b *Builder) error {
	raw, err := os.ReadFile(b.Path("rto", "rto.json"))
	if err != nil {
		return err
	}
	var offices []RTO
	if err := json.Unmarshal(raw, &offices); err != nil {
		return fmt.Errorf("rto.json: %w", err)
	}
	insert, err := b.Tx.Prepare(`INSERT INTO rto VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	for _, office := range offices {
		var stateCode, stateName any
		if state, ok := india.ByVehicleCode(office.Prefix); ok {
			stateCode, stateName = state.Code, state.Name
		} else {
			b.Warnf("no state for prefix %s (%s)", office.Prefix, office.Code)
		}
		if _, err := insert.Exec(office.Code, office.Prefix, stateCode, stateName, office.Office, nullable(office.Jurisdiction), nullable(office.Notes)); err != nil {
			return fmt.Errorf("%s: %w", office.Code, err)
		}
		b.Add("rto_codes", 1)
	}
	return nil
}

// checkRTO guards against a vandalised or half-edited article: the source is a wiki,
// and a server refreshes from it with nobody reading the diff. A month of honest edits
// touches a handful of offices, not a tenth of them.
func checkRTO(current, fresh *sql.DB) error {
	load := func(db *sql.DB) (map[string]string, error) {
		rows, err := db.Query(`SELECT code, office FROM rto`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		offices := map[string]string{}
		for rows.Next() {
			var code, office string
			if err := rows.Scan(&code, &office); err != nil {
				return nil, err
			}
			offices[code] = office
		}
		return offices, rows.Err()
	}
	before, err := load(current)
	if err != nil {
		return err
	}
	after, err := load(fresh)
	if err != nil {
		return err
	}
	changed := 0
	for code, office := range before {
		if renamed, ok := after[code]; !ok || renamed != office {
			changed++
		}
	}
	if len(before) > 0 && changed*10 > len(before) {
		return fmt.Errorf("%d of %d offices were renamed or removed since the copy in use", changed, len(before))
	}
	return nil
}

const rtoPage = "List_of_Regional_Transport_Office_districts_in_India"

func importRTO(im *Importer) (Source, error) {
	wikitext, revision, err := im.wikipedia(rtoPage)
	if err != nil {
		return Source{}, err
	}
	if im.CurrentVersion() == "revision "+revision {
		return Source{}, ErrNotChanged
	}

	offices, err := parseRTO(string(wikitext))
	if err != nil {
		return Source{}, err
	}
	if len(offices) < 1000 {
		return Source{}, fmt.Errorf("parsed only %d codes, expected well over 1,000: refusing to replace rto.json", len(offices))
	}
	encoded, err := json.MarshalIndent(offices, "", " ")
	if err != nil {
		return Source{}, err
	}
	if err := im.Write("rto/rto.json", append(encoded, '\n')); err != nil {
		return Source{}, err
	}
	return Source{
		Name:        "List of Regional Transport Office districts in India",
		Publisher:   "Wikipedia contributors",
		URL:         wikipediaURL(rtoPage, revision),
		Licence:     "CC BY-SA 4.0",
		LicenceURL:  "https://creativecommons.org/licenses/by-sa/4.0/",
		Version:     "revision " + revision,
		SHA256:      digest(wikitext),
		Attribution: "RTO codes adapted from the Wikipedia article \"List of Regional Transport Office districts in India\", licensed CC BY-SA 4.0. This dataset is shared under the same licence.",
	}, nil
}
