package dataset

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// The Thirukkural: 1,330 couplets in 133 chapters. Only text that is free to share is
// kept: the couplets, a transliteration, G.U. Pope's verse translation (1886) and the
// prose translation of W.H. Drew and John Lazarus (1840, 1885). The source repository
// also carries a 1968 English translation and three modern Tamil commentaries; those are
// dropped at import because none is confirmed to be out of copyright.

func init() {
	register(&Spec{
		Name:   "thirukkural",
		Schema: 1,
		DDL: `
CREATE TABLE chapters (
  number INTEGER PRIMARY KEY, name TEXT NOT NULL, translation TEXT, transliteration TEXT,
  iyal INTEGER NOT NULL, iyal_name TEXT NOT NULL, iyal_translation TEXT, iyal_transliteration TEXT,
  paal INTEGER NOT NULL, paal_name TEXT NOT NULL, paal_translation TEXT, paal_transliteration TEXT,
  first INTEGER NOT NULL, last INTEGER NOT NULL
);
CREATE TABLE kurals (
  number INTEGER PRIMARY KEY, chapter INTEGER NOT NULL REFERENCES chapters(number),
  line1 TEXT NOT NULL, line2 TEXT NOT NULL, transliteration1 TEXT, transliteration2 TEXT,
  pope TEXT, drew_lazarus TEXT,
  search TEXT NOT NULL
);
CREATE INDEX kurals_chapter ON kurals(chapter);`,
		Build:  buildThirukkural,
		Import: importThirukkural,
		Fixed:  true,
	})
}

// Kural is one couplet as kept in data/thirukkural/kurals.json.
type Kural struct {
	Number           int    `json:"number"`
	Line1            string `json:"line1"`
	Line2            string `json:"line2"`
	Transliteration1 string `json:"transliteration1"`
	Transliteration2 string `json:"transliteration2"`
	Pope             string `json:"pope"`
	DrewLazarus      string `json:"drewLazarus"`
}

type kuralLevel struct {
	Name            string `json:"name"`
	Translation     string `json:"translation"`
	Transliteration string `json:"transliteration"`
	Number          int    `json:"number"`
	Start           int    `json:"start"`
	End             int    `json:"end"`
}

// kuralStructure is the shape of the source's detail.json: பால் > இயல் > அதிகாரம்.
type kuralStructure []struct {
	Section struct {
		Detail []struct {
			kuralLevel
			ChapterGroup struct {
				Detail []struct {
					kuralLevel
					Chapters struct {
						Detail []kuralLevel `json:"detail"`
					} `json:"chapters"`
				} `json:"detail"`
			} `json:"chapterGroup"`
		} `json:"detail"`
	} `json:"section"`
}

func buildThirukkural(b *Builder) error {
	raw, err := os.ReadFile(b.Path("thirukkural", "kurals.json"))
	if err != nil {
		return err
	}
	var kurals []Kural
	if err := json.Unmarshal(raw, &kurals); err != nil {
		return fmt.Errorf("kurals.json: %w", err)
	}
	raw, err = os.ReadFile(b.Path("thirukkural", "structure.json"))
	if err != nil {
		return err
	}
	var structure kuralStructure
	if err := json.Unmarshal(raw, &structure); err != nil || len(structure) != 1 {
		return fmt.Errorf("structure.json: not the expected shape (%v)", err)
	}

	chapterOf := map[int]int{}
	insertChapter, err := b.Tx.Prepare(`INSERT INTO chapters VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insertChapter.Close()
	for _, paal := range structure[0].Section.Detail {
		for _, iyal := range paal.ChapterGroup.Detail {
			for _, chapter := range iyal.Chapters.Detail {
				if chapter.Start < 1 || chapter.End < chapter.Start || chapter.End > 1330 {
					return fmt.Errorf("chapter %d covers %d to %d", chapter.Number, chapter.Start, chapter.End)
				}
				if _, err := insertChapter.Exec(chapter.Number, chapter.Name, chapter.Translation, chapter.Transliteration,
					iyal.Number, iyal.Name, iyal.Translation, iyal.Transliteration,
					paal.Number, paal.Name, paal.Translation, paal.Transliteration, chapter.Start, chapter.End); err != nil {
					return fmt.Errorf("chapter %d: %w", chapter.Number, err)
				}
				for n := chapter.Start; n <= chapter.End; n++ {
					chapterOf[n] = chapter.Number
				}
				b.Add("chapters", 1)
			}
		}
	}

	insert, err := b.Tx.Prepare(`INSERT INTO kurals VALUES (?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	for _, kural := range kurals {
		chapter, ok := chapterOf[kural.Number]
		if !ok {
			return fmt.Errorf("kural %d belongs to no chapter", kural.Number)
		}
		// One lower-cased, NFC string per kural for search in either language.
		search := strings.ToLower(norm.NFC.String(strings.Join([]string{
			kural.Line1, kural.Line2, kural.Transliteration1, kural.Transliteration2, kural.Pope, kural.DrewLazarus,
		}, " ")))
		if _, err := insert.Exec(kural.Number, chapter, kural.Line1, kural.Line2, nullable(kural.Transliteration1), nullable(kural.Transliteration2),
			nullable(kural.Pope), nullable(kural.DrewLazarus), search); err != nil {
			return fmt.Errorf("kural %d: %w", kural.Number, err)
		}
		b.Add("kurals", 1)
	}
	if b.counts["kurals"] != 1330 || b.counts["chapters"] != 133 {
		return fmt.Errorf("found %d kurals in %d chapters, want 1330 in 133", b.counts["kurals"], b.counts["chapters"])
	}
	return nil
}

const thirukkuralRepo = "https://raw.githubusercontent.com/tk120404/thirukkural/"

func importThirukkural(im *Importer) (Source, error) {
	version := im.Version
	if version == "" {
		return Source{}, errors.New("give -version <commit> of github.com/tk120404/thirukkural, so the text served can be traced to an exact copy")
	}
	raw, err := im.Get(thirukkuralRepo + version + "/thirukkural.json")
	if err != nil {
		return Source{}, err
	}
	var source struct {
		Kural []struct {
			Number           int    `json:"Number"`
			Line1            string `json:"Line1"`
			Line2            string `json:"Line2"`
			Transliteration1 string `json:"transliteration1"`
			Transliteration2 string `json:"transliteration2"`
			Couplet          string `json:"couplet"`
			Explanation      string `json:"explanation"`
		} `json:"kural"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		return Source{}, fmt.Errorf("thirukkural.json: %w", err)
	}
	if len(source.Kural) != 1330 {
		return Source{}, fmt.Errorf("thirukkural.json has %d kurals, want 1330", len(source.Kural))
	}
	// Only the fields known to be free to share are written into the repository.
	kept := make([]Kural, len(source.Kural))
	for i, k := range source.Kural {
		kept[i] = Kural{Number: k.Number, Line1: norm.NFC.String(k.Line1), Line2: norm.NFC.String(k.Line2),
			Transliteration1: k.Transliteration1, Transliteration2: k.Transliteration2, Pope: k.Couplet, DrewLazarus: k.Explanation}
	}
	encoded, err := json.MarshalIndent(kept, "", " ")
	if err != nil {
		return Source{}, err
	}
	structure, err := im.Get(thirukkuralRepo + version + "/detail.json")
	if err != nil {
		return Source{}, err
	}
	if !json.Valid(structure) {
		return Source{}, errors.New("detail.json is not valid JSON")
	}
	if err := im.Write("thirukkural/kurals.json", append(encoded, '\n')); err != nil {
		return Source{}, err
	}
	if err := im.Write("thirukkural/structure.json", structure); err != nil {
		return Source{}, err
	}
	return Source{
		Name:       "Thirukkural",
		Publisher:  "Thiruvalluvar; compiled as JSON by github.com/tk120404/thirukkural",
		URL:        "https://github.com/tk120404/thirukkural/tree/" + version,
		Licence:    "Public domain text and translations; the compilation is Apache-2.0",
		LicenceURL: "https://www.apache.org/licenses/LICENSE-2.0",
		Version:    "commit " + version[:min(12, len(version))],
		SHA256:     digest(raw),
		Attribution: "Thirukkural text with English translations by G.U. Pope (1886) and by W.H. Drew and John Lazarus (1840, 1885), " +
			"from the tk120404/thirukkural compilation (Apache-2.0).",
	}, nil
}
