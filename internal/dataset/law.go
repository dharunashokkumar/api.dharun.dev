package dataset

import (
	"encoding/json"
	"os"
	"strings"
)

// Which section of India's new criminal laws (Bharatiya Nyaya Sanhita, Bharatiya Nagarik
// Suraksha Sanhita and Bharatiya Sakshya Adhiniyam, in force from 1 July 2024) stands for
// which section of the law it replaced (Indian Penal Code, Code of Criminal Procedure,
// Indian Evidence Act), with each section's title. Read from the Bureau of Police
// Research and Development's correspondence tables by tools/law/correspondence.py: only
// the correspondence and the Acts' own titles, not the tables' written summaries. Fixed:
// the Acts change only by amendment, which is a hand update.

func init() {
	register(&Spec{
		Name:   "law",
		Schema: 1,
		DDL: `
CREATE TABLE sections (law TEXT NOT NULL, section TEXT NOT NULL, base TEXT NOT NULL, part TEXT NOT NULL, title TEXT NOT NULL,
  old_law TEXT NOT NULL, old TEXT NOT NULL, seq INTEGER NOT NULL, PRIMARY KEY (law, seq)) WITHOUT ROWID;
CREATE INDEX sections_base ON sections(law, base);
CREATE TABLE old_sections (law TEXT NOT NULL, old_section TEXT NOT NULL, seq INTEGER NOT NULL);
CREATE INDEX old_sections_lookup ON old_sections(law, old_section);`,
		Build: buildLaw,
		Main:  "sections", MaxShrink: 0.05,
		Fixed: true,
	})
}

// LawPairs names each new law with the one it replaced.
var LawPairs = map[string]string{"bns": "ipc", "bnss": "crpc", "bsa": "iea"}

func buildLaw(b *Builder) error {
	insert, err := b.Tx.Prepare(`INSERT INTO sections VALUES (?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	old, err := b.Tx.Prepare(`INSERT INTO old_sections VALUES (?,?,?)`)
	if err != nil {
		return err
	}
	defer old.Close()
	for _, law := range []string{"bns", "bnss", "bsa"} {
		raw, err := os.ReadFile(b.Path("law", law+".json"))
		if err != nil {
			return err
		}
		var rows []struct {
			Section, Part, Title, Old string
			OldSections               []string `json:"old_sections"`
		}
		if err := json.Unmarshal(raw, &rows); err != nil {
			return err
		}
		for i, r := range rows {
			base, _, _ := strings.Cut(r.Section, "(")
			if _, err := insert.Exec(law, r.Section, base, r.Part, r.Title, LawPairs[law], r.Old, i); err != nil {
				return err
			}
			for _, o := range r.OldSections {
				if _, err := old.Exec(law, strings.ToUpper(o), i); err != nil {
					return err
				}
			}
			b.Add("sections", 1)
		}
	}
	return nil
}
