package dataset

import (
	"encoding/json"
	"fmt"
	"os"
)

// Public domain Indian classics beside the Thirukkural: Avvaiyar's Aathichoodi (Tamil),
// the Bhagavad Gita (Sanskrit) and Kabir's sakhis (Hindi), as transcribed on Wikisource
// (tools/classics/fetch_classics.py, which keeps only the original lines and records
// each page's revision). Fixed: the texts are centuries old; a rerun of the script is a
// hand update.

func init() {
	register(&Spec{
		Name:   "classics",
		Schema: 1,
		DDL: `
CREATE TABLE texts (id TEXT PRIMARY KEY, title TEXT NOT NULL, title_en TEXT NOT NULL, author TEXT NOT NULL,
  language TEXT NOT NULL, part_name TEXT NOT NULL, unit_name TEXT NOT NULL, source TEXT NOT NULL, note TEXT NOT NULL) WITHOUT ROWID;
CREATE TABLE parts (text TEXT NOT NULL, part INTEGER NOT NULL, name TEXT NOT NULL, PRIMARY KEY (text, part)) WITHOUT ROWID;
CREATE TABLE units (text TEXT NOT NULL, part INTEGER NOT NULL, number INTEGER NOT NULL, seq INTEGER NOT NULL, body TEXT NOT NULL,
  PRIMARY KEY (text, part, number)) WITHOUT ROWID;
CREATE INDEX units_seq ON units(text, seq);`,
		Build: buildClassics,
		Main:  "units", MaxShrink: 0.05,
		Fixed: true,
	})
}

type classicText struct {
	id, title, titleEn, author, language, partName, unitName, source, note string
	parts                                                                  []classicPart
}

type classicPart struct {
	number int
	name   string
	units  []classicUnit
}

type classicUnit struct {
	number int
	body   string
}

func buildClassics(b *Builder) error {
	var texts []classicText

	var aathichoodi struct {
		Title, Author, Source string
		TitleEn               string   `json:"title_en"`
		Invocation            []string `json:"invocation"`
		Lines                 []struct {
			Number        int
			Section, Text string
		}
	}
	if err := readJSONFile(b.Path("classics", "aathichoodi.json"), &aathichoodi); err != nil {
		return err
	}
	a := classicText{id: "aathichoodi", title: aathichoodi.Title, titleEn: aathichoodi.TitleEn, author: "Avvaiyar", language: "ta",
		partName: "section", unitName: "line", source: aathichoodi.Source,
		note: "109 one-line maxims in the order of the Tamil alphabet, with the two-line invocation as section 0, line 1 and 2."}
	a.parts = append(a.parts, classicPart{number: 0, name: "கடவுள் வாழ்த்து"})
	for i, line := range aathichoodi.Invocation {
		a.parts[0].units = append(a.parts[0].units, classicUnit{i + 1, line})
	}
	for _, line := range aathichoodi.Lines {
		last := &a.parts[len(a.parts)-1]
		if last.name != line.Section {
			a.parts = append(a.parts, classicPart{number: len(a.parts), name: line.Section})
			last = &a.parts[len(a.parts)-1]
		}
		last.units = append(last.units, classicUnit{line.Number, line.Text})
	}
	texts = append(texts, a)

	var gita struct {
		Title, Source string
		TitleEn       string `json:"title_en"`
		Chapters      []struct {
			Chapter int
			Name    string
			Verses  []struct {
				Verse int
				Text  string
			}
		}
	}
	if err := readJSONFile(b.Path("classics", "gita.json"), &gita); err != nil {
		return err
	}
	g := classicText{id: "gita", title: gita.Title, titleEn: gita.TitleEn, author: "Vyasa (attributed)", language: "sa",
		partName: "chapter", unitName: "verse", source: gita.Source,
		note: "The Sanskrit text, 700 verses in 18 chapters (Mahabharata, Bhishma Parva). Speakers are not marked."}
	for _, c := range gita.Chapters {
		part := classicPart{number: c.Chapter, name: c.Name}
		for _, v := range c.Verses {
			part.units = append(part.units, classicUnit{v.Verse, v.Text})
		}
		g.parts = append(g.parts, part)
	}
	texts = append(texts, g)

	var kabir struct {
		Title, Source, Edition string
		TitleEn                string `json:"title_en"`
		Angas                  []struct {
			Anga     int
			Name     string
			Couplets []struct {
				Number int
				Text   string
			}
		}
	}
	if err := readJSONFile(b.Path("classics", "kabir.json"), &kabir); err != nil {
		return err
	}
	k := classicText{id: "kabir", title: kabir.Title, titleEn: kabir.TitleEn, author: "Kabir", language: "hi",
		partName: "anga", unitName: "couplet", source: kabir.Source,
		note: "Sakhis (couplets) of Kabir in the 17 angas (themes) of " + kabir.Edition + " that Wikisource has transcribed; numbers are the edition's, so some are missing."}
	for _, an := range kabir.Angas {
		part := classicPart{number: an.Anga, name: an.Name}
		for _, c := range an.Couplets {
			part.units = append(part.units, classicUnit{c.Number, c.Text})
		}
		k.parts = append(k.parts, part)
	}
	texts = append(texts, k)

	for _, t := range texts {
		if _, err := b.Tx.Exec(`INSERT INTO texts VALUES (?,?,?,?,?,?,?,?,?)`, t.id, t.title, t.titleEn, t.author, t.language, t.partName, t.unitName, t.source, t.note); err != nil {
			return err
		}
		seq := 0
		for _, p := range t.parts {
			if _, err := b.Tx.Exec(`INSERT INTO parts VALUES (?,?,?)`, t.id, p.number, p.name); err != nil {
				return err
			}
			for _, u := range p.units {
				seq++
				if _, err := b.Tx.Exec(`INSERT INTO units VALUES (?,?,?,?,?)`, t.id, p.number, u.number, seq, u.body); err != nil {
					return fmt.Errorf("%s %d.%d: %w", t.id, p.number, u.number, err)
				}
				b.Add("units", 1)
			}
		}
		b.Add("texts", 1)
		b.Add(t.id, seq)
	}
	return nil
}

func readJSONFile(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
