package api

import (
	"database/sql"
	"net/http"
	"net/url"
	"strings"
)

type hsnCode struct {
	Code        string `json:"code"`
	Kind        string `json:"kind"`
	Level       string `json:"level"`
	Description string `json:"description"`
}

type hsnView struct {
	hsnCode
	// Parents runs from the chapter (or service section) down to the direct parent.
	Parents  []hsnCode `json:"parents"`
	Children []hsnCode `json:"children"`
}

const hsnNote = "Codes and descriptions only; GST rates are not included. Descriptions are as published, including their capitals."

// hsnDigits takes the digits of a code as people write it: "0902 10 10", "0902.10.10".
func hsnDigits(raw string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		if r == ' ' || r == '.' || r == '-' {
			return -1
		}
		return 'x'
	}, strings.TrimSpace(raw))
}

func (s *Server) handleHSN(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "hsn")
	if set == nil {
		return
	}
	code := hsnDigits(r.PathValue("code"))
	info(r).subject = code
	if len(code) < 2 || len(code) > 8 || strings.Contains(code, "x") {
		s.badRequest(w, r, "An HSN code has 2 to 8 digits (a SAC code starts with 99).")
		return
	}
	lookup := func(code string) (hsnCode, sql.NullString, error) {
		var c hsnCode
		var parent sql.NullString
		err := set.row(`SELECT code, kind, level, description, parent FROM codes WHERE code = ?`, code).Scan(&c.Code, &c.Kind, &c.Level, &c.Description, &parent)
		return c, parent, err
	}
	found, parent, err := lookup(code)
	if err == sql.ErrNoRows {
		// Say which shorter code does exist, so the caller can look one level up.
		for length := len(code) - 1; length >= 2; length-- {
			if nearest, _, err := lookup(code[:length]); err == nil {
				s.notFound(w, r, "No code "+code+". The nearest listed code is "+nearest.Code+": "+nearest.Description+".")
				return
			}
		}
		s.notFound(w, r, "No code "+code+".")
		return
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	view := hsnView{hsnCode: found, Parents: []hsnCode{}, Children: []hsnCode{}}
	for parent.Valid {
		var above hsnCode
		above, parent, err = lookup(parent.String)
		if err != nil {
			break
		}
		view.Parents = append([]hsnCode{above}, view.Parents...)
	}
	rows, err := set.rows(`SELECT code, kind, level, description FROM codes WHERE parent = ? ORDER BY code`, found.Code)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var child hsnCode
		if err := rows.Scan(&child.Code, &child.Kind, &child.Level, &child.Description); err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		view.Children = append(view.Children, child)
	}
	s.ok(w, r, view, meta{Source: s.source("hsn"), Note: hsnNote}, cacheLong)
}

func (s *Server) handleHSNSearch(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "hsn")
	if set == nil {
		return
	}
	query := r.URL.Query()
	text := strings.TrimSpace(query.Get("q"))
	kind := strings.ToLower(strings.TrimSpace(query.Get("kind")))
	limit, offset, ok := s.listParams(w, r, 20)
	if !ok {
		return
	}
	if kind != "" && kind != "hsn" && kind != "sac" {
		s.badRequest(w, r, "kind must be hsn (goods) or sac (services).")
		return
	}
	if len(text) < 2 {
		s.badRequest(w, r, "Give q: at least 2 characters of a description, or the start of a code.")
		return
	}
	info(r).subject = text
	where := []string{"1 = 1"}
	var args []any
	if digits := hsnDigits(text); !strings.Contains(digits, "x") {
		where = append(where, `code >= ? AND code < ?`)
		args = append(args, digits, digits+":")
	} else {
		for _, word := range strings.Fields(strings.ToLower(text)) {
			where = append(where, `search LIKE ? ESCAPE '\'`)
			args = append(args, likePattern(word, true))
		}
	}
	if kind != "" {
		where = append(where, `kind = ?`)
		args = append(args, kind)
	}
	condition := strings.Join(where, " AND ")
	var total int
	if err := set.db.QueryRow(`SELECT COUNT(*) FROM codes WHERE `+condition, args...).Scan(&total); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	// Broader codes first: a heading tells more than one of its many tariff items.
	rows, err := set.db.Query(`SELECT code, kind, level, description FROM codes WHERE `+condition+
		` ORDER BY length(code), code LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	codes := []hsnCode{}
	for rows.Next() {
		var c hsnCode
		if err := rows.Scan(&c.Code, &c.Kind, &c.Level, &c.Description); err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		codes = append(codes, c)
	}
	m := paging(total, limit, offset)
	m.Source, m.Note = s.source("hsn"), hsnNote
	s.ok(w, r, codes, m, cacheAPI)
}

func init() {
	mcpTools = append(mcpTools,
		mcpTool{
			Name:        "lookup_hsn_code",
			Description: "Look up an Indian HSN code (goods) or SAC code (services) used in GST invoices: its description, the chapter and heading above it, and the codes below it. No GST rates.",
			InputSchema: schema([]string{"code"}, map[string]any{"code": text("HSN or SAC code, 2 to 8 digits.")}),
			path: func(args map[string]any) (string, error) {
				code, err := required(args, "code")
				return "/v1/hsn/" + url.PathEscape(code), err
			},
		},
		mcpTool{
			Name:        "search_hsn_codes",
			Description: "Find Indian HSN (goods) or SAC (services) codes by words of their description, such as 'green tea' or 'construction'.",
			InputSchema: schema([]string{"query"}, map[string]any{
				"query": text("Words of the description, or the start of a code."), "kind": oneOf("Goods or services.", "hsn", "sac"),
			}),
			path: func(args map[string]any) (string, error) {
				_, err := required(args, "query")
				return "/v1/hsn" + query(args, map[string]string{"query": "q", "kind": "kind"}), err
			},
		},
	)
}
