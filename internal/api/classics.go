package api

// Public domain classics (dataset classics): Aathichoodi, the Bhagavad Gita in
// Sanskrit and Kabir's sakhis, with a Roman transliteration made by internal/translit.

import (
	"database/sql"
	"math/rand/v2"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/translit"
)

func (s *Server) registerClassics() {
	s.handle("classics", costLight, "GET /v1/classics", s.handleClassicsList)
	s.handle("classics", costLight, "GET /v1/classics/{text}", s.handleClassicText)
	s.handle("classics", costLight, "GET /v1/classics/{text}/{ref}", s.handleClassicRef)
}

type classicInfo struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	TitleEn  string        `json:"title_en"`
	Author   string        `json:"author"`
	Language string        `json:"language"`
	PartName string        `json:"part_name"`
	UnitName string        `json:"unit_name"`
	Units    int           `json:"units"`
	Note     string        `json:"note"`
	Source   string        `json:"source"`
	Parts    []classicPart `json:"parts,omitempty"`
}

type classicPart struct {
	Number int    `json:"number"`
	Name   string `json:"name"`
	Units  int    `json:"units"`
}

type classicUnit struct {
	Text            string   `json:"text"`
	Ref             string   `json:"ref"`
	Part            int      `json:"part"`
	PartName        string   `json:"part_name"`
	Number          int      `json:"number"`
	Lines           []string `json:"lines"`
	Transliteration []string `json:"transliteration"`
}

const classicsNote = "Public domain texts as transcribed on Wikisource; the transliteration (ISO 15919) is made by this API from the script and is a guide to reading, not a scholarly edition."

// Aathichoodi's lines are numbered 1 to 109 straight through; the other texts by part
// and number (Gita 2.47, Kabir 1.3).
var classicFlat = map[string]bool{"aathichoodi": true}

var classicRefShape = regexp.MustCompile(`^(\d{1,3})(?:\.(\d{1,3}))?$`)

func (s *Server) classicInfo(set *handle, id string) (classicInfo, error) {
	var c classicInfo
	err := set.row(`SELECT id, title, title_en, author, language, part_name, unit_name, note, source,
(SELECT count(*) FROM units u WHERE u.text = t.id) FROM texts t WHERE id = ?`, id).Scan(
		&c.ID, &c.Title, &c.TitleEn, &c.Author, &c.Language, &c.PartName, &c.UnitName, &c.Note, &c.Source, &c.Units)
	return c, err
}

func (s *Server) handleClassicsList(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "classics")
	if set == nil {
		return
	}
	rows, err := set.rows(`SELECT id FROM texts ORDER BY id`)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	out := []classicInfo{}
	for _, id := range ids {
		if c, err := s.classicInfo(set, id); err == nil {
			out = append(out, c)
		}
	}
	total := len(out)
	s.ok(w, r, out, meta{Total: &total, Source: s.source("classics"), Note: classicsNote}, cacheAPI)
}

func (s *Server) handleClassicText(w http.ResponseWriter, r *http.Request) {
	id := strings.ToLower(r.PathValue("text"))
	set := s.need(w, r, "classics")
	if set == nil {
		return
	}
	c, err := s.classicInfo(set, id)
	if err == sql.ErrNoRows {
		s.notFound(w, r, "text is aathichoodi, gita or kabir (see /v1/classics).")
		return
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	rows, err := set.rows(`SELECT p.part, p.name, count(u.number) FROM parts p LEFT JOIN units u ON u.text = p.text AND u.part = p.part
WHERE p.text = ? GROUP BY p.part ORDER BY p.part`, id)
	if err == nil {
		for rows.Next() {
			var p classicPart
			if rows.Scan(&p.Number, &p.Name, &p.Units) == nil {
				c.Parts = append(c.Parts, p)
			}
		}
		rows.Close()
	}
	s.ok(w, r, c, meta{Source: s.source("classics"), Note: classicsNote}, cacheAPI)
}

func (s *Server) handleClassicRef(w http.ResponseWriter, r *http.Request) {
	id := strings.ToLower(r.PathValue("text"))
	ref := r.PathValue("ref")
	set := s.need(w, r, "classics")
	if set == nil {
		return
	}
	c, err := s.classicInfo(set, id)
	if err == sql.ErrNoRows {
		s.notFound(w, r, "text is aathichoodi, gita or kabir (see /v1/classics).")
		return
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	info(r).subject = id + " " + ref
	const columns = `SELECT u.part, p.name, u.number, u.body FROM units u JOIN parts p ON p.text = u.text AND p.part = u.part WHERE u.text = ? AND `
	var where string
	var args []any
	cache := cacheAPI
	wholePart := false
	switch m := classicRefShape.FindStringSubmatch(ref); {
	case ref == "random":
		var count int
		set.row(`SELECT count(*) FROM units WHERE text = ?`, id).Scan(&count)
		where, args = `u.seq = ?`, []any{rand.IntN(max(count, 1)) + 1}
		cache = cacheNone
	case m == nil:
		s.badRequest(w, r, "ref is a "+c.UnitName+" number, a "+c.PartName+" ("+c.PartName+" 2), a "+c.PartName+" and "+c.UnitName+" (2.47), or random.")
		return
	case classicFlat[id] && m[2] == "":
		n, _ := strconv.Atoi(m[1])
		where, args = `u.part > 0 AND u.number = ?`, []any{n}
	case m[2] == "":
		n, _ := strconv.Atoi(m[1])
		where, args = `u.part = ? ORDER BY u.number`, []any{n}
		wholePart = true
	default:
		part, _ := strconv.Atoi(m[1])
		n, _ := strconv.Atoi(m[2])
		where, args = `u.part = ? AND u.number = ?`, []any{part, n}
	}
	rows, err := set.rows(columns+where, append([]any{id}, args...)...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	out := []classicUnit{}
	for rows.Next() {
		var u classicUnit
		var body string
		if rows.Scan(&u.Part, &u.PartName, &u.Number, &body) != nil {
			continue
		}
		u.Text = id
		u.Ref = strconv.Itoa(u.Part) + "." + strconv.Itoa(u.Number)
		if classicFlat[id] && u.Part > 0 {
			u.Ref = strconv.Itoa(u.Number)
		}
		u.Lines = strings.Split(body, "\n")
		for _, line := range u.Lines {
			roman, err := translit.Romanise(line, "iso15919")
			if err != nil {
				roman = ""
			}
			u.Transliteration = append(u.Transliteration, roman)
		}
		out = append(out, u)
	}
	if len(out) == 0 {
		s.notFound(w, r, "No "+c.UnitName+" "+ref+" in "+c.TitleEn+".")
		return
	}
	if !wholePart {
		s.ok(w, r, out[0], meta{Source: s.source("classics"), Note: classicsNote}, cache)
		return
	}
	total := len(out)
	s.ok(w, r, out, meta{Total: &total, Source: s.source("classics"), Note: classicsNote}, cache)
}
