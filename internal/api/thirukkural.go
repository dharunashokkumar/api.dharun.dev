package api

import (
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"
)

type kuralLevelView struct {
	Number          int     `json:"number"`
	Name            string  `json:"name"`
	Translation     *string `json:"translation"`
	Transliteration *string `json:"transliteration"`
}

type kuralView struct {
	Number          int               `json:"number"`
	Lines           [2]string         `json:"lines"`
	Transliteration [2]*string        `json:"transliteration"`
	Translations    map[string]string `json:"translations"`
	Chapter         kuralLevelView    `json:"chapter"`
	Iyal            kuralLevelView    `json:"iyal"`
	Paal            kuralLevelView    `json:"paal"`
}

const kuralColumns = `k.number, k.line1, k.line2, k.transliteration1, k.transliteration2, k.pope, k.drew_lazarus,
	c.number, c.name, c.translation, c.transliteration, c.iyal, c.iyal_name, c.iyal_translation, c.iyal_transliteration,
	c.paal, c.paal_name, c.paal_translation, c.paal_transliteration
	FROM kurals k JOIN chapters c ON c.number = k.chapter`

func scanKural(scan func(...any) error) (kuralView, error) {
	var view kuralView
	var tr1, tr2, pope, drew, chapterTr, chapterTl, iyalTr, iyalTl, paalTr, paalTl sql.NullString
	err := scan(&view.Number, &view.Lines[0], &view.Lines[1], &tr1, &tr2, &pope, &drew,
		&view.Chapter.Number, &view.Chapter.Name, &chapterTr, &chapterTl,
		&view.Iyal.Number, &view.Iyal.Name, &iyalTr, &iyalTl,
		&view.Paal.Number, &view.Paal.Name, &paalTr, &paalTl)
	view.Transliteration = [2]*string{str(tr1), str(tr2)}
	view.Chapter.Translation, view.Chapter.Transliteration = str(chapterTr), str(chapterTl)
	view.Iyal.Translation, view.Iyal.Transliteration = str(iyalTr), str(iyalTl)
	view.Paal.Translation, view.Paal.Transliteration = str(paalTr), str(paalTl)
	view.Translations = map[string]string{}
	if pope.Valid {
		view.Translations["pope"] = pope.String
	}
	if drew.Valid {
		view.Translations["drewLazarus"] = drew.String
	}
	return view, err
}

const kuralNote = "translations.pope is G.U. Pope's verse translation (1886); translations.drewLazarus is the prose translation by W.H. Drew and John Lazarus (1840, 1885)."

func (s *Server) sendKural(w http.ResponseWriter, r *http.Request, set *handle, number int, cache string) {
	view, err := scanKural(set.row(`SELECT `+kuralColumns+` WHERE k.number = ?`, number).Scan)
	if err == sql.ErrNoRows {
		s.notFound(w, r, "There is no kural "+strconv.Itoa(number)+". Kurals are numbered 1 to 1330.")
		return
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	s.ok(w, r, view, meta{Source: s.source("thirukkural"), Note: kuralNote}, cache)
}

func (s *Server) handleKural(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "thirukkural")
	if set == nil {
		return
	}
	info(r).subject = r.PathValue("number")
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil || number < 1 || number > 1330 {
		s.badRequest(w, r, "Kurals are numbered 1 to 1330.")
		return
	}
	s.sendKural(w, r, set, number, cacheLong)
}

// handleKuralOfTheDay picks the same kural for everyone on a given day in India, and a
// different one the next day.
func (s *Server) handleKuralOfTheDay(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "thirukkural")
	if set == nil {
		return
	}
	day := time.Now().In(indiaTime).Format(isoDay)
	if raw := r.URL.Query().Get("date"); raw != "" {
		parsed, err := parseDay(raw)
		if err != nil {
			s.badRequest(w, r, err.Error()+".")
			return
		}
		day = parsed.Format(isoDay)
	}
	sum := sha256.Sum256([]byte("thirukkural " + day))
	number := int(binary.BigEndian.Uint64(sum[:8])%1330) + 1
	s.sendKural(w, r, set, number, "public, max-age=600")
}

func (s *Server) handleKuralRandom(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "thirukkural")
	if set == nil {
		return
	}
	s.sendKural(w, r, set, rand.IntN(1330)+1, cacheNone)
}

func (s *Server) handleKuralSearch(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "thirukkural")
	if set == nil {
		return
	}
	query := r.URL.Query()
	text := strings.ToLower(norm.NFC.String(strings.TrimSpace(query.Get("q"))))
	limit, offset, ok := s.listParams(w, r, 10)
	if !ok {
		return
	}
	chapter, err := intParam(r, "chapter", 0, 1, 133)
	if err != nil {
		s.badRequest(w, r, err.Error())
		return
	}
	paal, err := intParam(r, "paal", 0, 1, 3)
	if err != nil {
		s.badRequest(w, r, err.Error())
		return
	}
	if len([]rune(text)) < 2 && chapter == 0 && paal == 0 {
		s.badRequest(w, r, "Give q (at least 2 characters, Tamil or English), a chapter or a paal.")
		return
	}
	info(r).subject = text

	where := []string{"1 = 1"}
	var args []any
	if text != "" {
		where = append(where, `k.search LIKE ? ESCAPE '\'`)
		args = append(args, likePattern(text, true))
	}
	if chapter != 0 {
		where = append(where, `k.chapter = ?`)
		args = append(args, chapter)
	}
	if paal != 0 {
		where = append(where, `c.paal = ?`)
		args = append(args, paal)
	}
	condition := strings.Join(where, " AND ")

	var total int
	if err := set.db.QueryRow(`SELECT COUNT(*) FROM kurals k JOIN chapters c ON c.number = k.chapter WHERE `+condition, args...).Scan(&total); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	rows, err := set.db.Query(`SELECT `+kuralColumns+` WHERE `+condition+` ORDER BY k.number LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	kurals := []kuralView{}
	for rows.Next() {
		view, err := scanKural(rows.Scan)
		if err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		kurals = append(kurals, view)
	}
	if rows.Err() != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	m := paging(total, limit, offset)
	m.Source, m.Note = s.source("thirukkural"), kuralNote
	s.ok(w, r, kurals, m, cacheLong)
}

type kuralChapterView struct {
	kuralLevelView
	Iyal  kuralLevelView `json:"iyal"`
	Paal  kuralLevelView `json:"paal"`
	First int            `json:"first"`
	Last  int            `json:"last"`
	Links struct {
		Kurals string `json:"kurals"`
	} `json:"links"`
}

func (s *Server) chapters(r *http.Request, set *handle, number int) ([]kuralChapterView, error) {
	statement := `SELECT number, name, translation, transliteration, iyal, iyal_name, iyal_translation, iyal_transliteration,
		paal, paal_name, paal_translation, paal_transliteration, first, last FROM chapters`
	var args []any
	if number != 0 {
		statement += ` WHERE number = ?`
		args = append(args, number)
	}
	rows, err := set.db.Query(statement+` ORDER BY number`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chapters []kuralChapterView
	for rows.Next() {
		var view kuralChapterView
		var tr, tl, iyalTr, iyalTl, paalTr, paalTl sql.NullString
		if err := rows.Scan(&view.Number, &view.Name, &tr, &tl, &view.Iyal.Number, &view.Iyal.Name, &iyalTr, &iyalTl,
			&view.Paal.Number, &view.Paal.Name, &paalTr, &paalTl, &view.First, &view.Last); err != nil {
			return nil, err
		}
		view.Translation, view.Transliteration = str(tr), str(tl)
		view.Iyal.Translation, view.Iyal.Transliteration = str(iyalTr), str(iyalTl)
		view.Paal.Translation, view.Paal.Transliteration = str(paalTr), str(paalTl)
		view.Links.Kurals = s.origin(r) + "/v1/thirukkural?" + url.Values{"chapter": {strconv.Itoa(view.Number)}}.Encode()
		chapters = append(chapters, view)
	}
	return chapters, rows.Err()
}

func (s *Server) handleKuralChapters(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "thirukkural")
	if set == nil {
		return
	}
	chapters, err := s.chapters(r, set, 0)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	total := len(chapters)
	s.ok(w, r, chapters, meta{Total: &total, Source: s.source("thirukkural")}, cacheLong)
}

func (s *Server) handleKuralChapter(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "thirukkural")
	if set == nil {
		return
	}
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil || number < 1 || number > 133 {
		s.badRequest(w, r, "Chapters (அதிகாரம்) are numbered 1 to 133.")
		return
	}
	info(r).subject = r.PathValue("number")
	chapters, err := s.chapters(r, set, number)
	if err != nil || len(chapters) != 1 {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	s.ok(w, r, chapters[0], meta{Source: s.source("thirukkural")}, cacheLong)
}

func init() {
	mcpTools = append(mcpTools,
		mcpTool{
			Name:        "get_thirukkural",
			Description: "Get a Thirukkural couplet by number (1-1330) in Tamil, with transliteration, English translations by G.U. Pope and by Drew and Lazarus, and its chapter.",
			InputSchema: schema([]string{"number"}, map[string]any{"number": map[string]any{"type": "integer", "minimum": 1, "maximum": 1330}}),
			path: func(args map[string]any) (string, error) {
				number, err := required(args, "number")
				return "/v1/thirukkural/" + url.PathEscape(number), err
			},
		},
		mcpTool{
			Name:        "search_thirukkural",
			Description: "Search the Thirukkural in Tamil or English, or list a chapter (1-133) or a paal (1 virtue, 2 wealth, 3 love).",
			InputSchema: schema(nil, map[string]any{
				"query":   text("Words to find, in Tamil or English."),
				"chapter": map[string]any{"type": "integer", "minimum": 1, "maximum": 133},
				"paal":    map[string]any{"type": "integer", "minimum": 1, "maximum": 3},
			}),
			path: func(args map[string]any) (string, error) {
				return "/v1/thirukkural" + query(args, map[string]string{"query": "q", "chapter": "chapter", "paal": "paal"}), nil
			},
		},
	)
}
