package api

// The correspondence between India's new criminal laws and the ones they replaced
// (dataset law): BNS and IPC, BNSS and CrPC, BSA and the Indian Evidence Act.

import (
	"net/http"
	"regexp"
	"strings"
)

func (s *Server) registerLaw() {
	s.handle("law", costLight, "GET /v1/law/{law}", s.handleLawSearch)
	s.handle("law", costLight, "GET /v1/law/{law}/{section}", s.handleLawSection)
}

type lawSection struct {
	Law        string `json:"law"`
	Section    string `json:"section"`
	Part       string `json:"part,omitempty"`
	Title      string `json:"title"`
	OldLaw     string `json:"old_law"`
	Old        string `json:"old_section"` // as the table gives it; empty for a new provision
	NewInLaw   bool   `json:"new_provision"`
	Reference  string `json:"reference"`
	OldRefText string `json:"old_reference,omitempty"`
}

var lawNames = map[string]string{
	"bns": "Bharatiya Nyaya Sanhita, 2023", "bnss": "Bharatiya Nagarik Suraksha Sanhita, 2023", "bsa": "Bharatiya Sakshya Adhiniyam, 2023",
	"ipc": "Indian Penal Code, 1860", "crpc": "Code of Criminal Procedure, 1973", "iea": "Indian Evidence Act, 1872",
}

var oldToNew = map[string]string{"ipc": "bns", "crpc": "bnss", "iea": "bsa"}

var lawSectionShape = regexp.MustCompile(`^\d{1,3}[A-Za-z]{0,3}(\([0-9A-Za-z]+\))*$`)

const lawNote = "From the Bureau of Police Research and Development's correspondence tables. A section of the old law can be spread over several new ones, and the reverse. The new laws apply to offences from 1 July 2024; earlier ones stay under the old laws. Check the Act as published before relying on a section."

func (s *Server) lawRows(set *handle, where, limit string, args ...any) ([]lawSection, error) {
	rows, err := set.rows(`SELECT law, section, part, title, old_law, old FROM sections WHERE `+where+` ORDER BY law, seq`+limit, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []lawSection{}
	for rows.Next() {
		var v lawSection
		if rows.Scan(&v.Law, &v.Section, &v.Part, &v.Title, &v.OldLaw, &v.Old) == nil {
			v.NewInLaw = v.Old == ""
			v.Reference = strings.ToUpper(v.Law) + " " + v.Section
			if v.Old != "" {
				v.OldRefText = strings.ToUpper(v.OldLaw) + " " + v.Old
			}
			out = append(out, v)
		}
	}
	return out, nil
}

func (s *Server) handleLawSection(w http.ResponseWriter, r *http.Request) {
	law := strings.ToLower(r.PathValue("law"))
	section := strings.ReplaceAll(r.PathValue("section"), " ", "")
	if _, ok := lawNames[law]; !ok {
		s.notFound(w, r, "law is bns, bnss or bsa (the new laws) or ipc, crpc or iea (the old ones).")
		return
	}
	if !lawSectionShape.MatchString(section) {
		s.badRequest(w, r, "section is a section number such as 103, 103(1) or 65B.")
		return
	}
	info(r).subject = law + " " + section
	set := s.need(w, r, "law")
	if set == nil {
		return
	}
	var out []lawSection
	var err error
	if newLaw, old := oldToNew[law]; old {
		// An old section: every new section the table says it went to.
		out, err = s.lawRows(set, `law = ? AND seq IN (SELECT seq FROM old_sections WHERE law = ? AND old_section = ?)`, "", newLaw, newLaw, strings.ToUpper(strings.SplitN(section, "(", 2)[0]))
	} else if strings.Contains(section, "(") {
		out, err = s.lawRows(set, `law = ? AND section = ?`, "", law, section)
	} else {
		out, err = s.lawRows(set, `law = ? AND base = ?`, "", law, section)
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	if len(out) == 0 {
		message := "No section " + section + " in the " + lawNames[law] + " table."
		if _, old := oldToNew[law]; old {
			message = "No section of the " + lawNames[oldToNew[law]] + " is shown as replacing section " + section + " of the " + lawNames[law] + ": it may have been repealed or merged without a mention."
		}
		s.notFound(w, r, message)
		return
	}
	total := len(out)
	s.ok(w, r, out, meta{Total: &total, Source: s.source("law"), Note: lawNote}, cacheAPI)
}

func (s *Server) handleLawSearch(w http.ResponseWriter, r *http.Request) {
	law := strings.ToLower(r.PathValue("law"))
	if _, ok := lawNames[law]; !ok {
		s.notFound(w, r, "law is bns, bnss or bsa (the new laws) or ipc, crpc or iea (the old ones).")
		return
	}
	if newLaw, old := oldToNew[law]; old {
		law = newLaw // titles are the new law's
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 3 || len(q) > 100 {
		s.badRequest(w, r, "q is 3 to 100 characters of a section's title, such as murder or bail.")
		return
	}
	info(r).subject = q
	set := s.need(w, r, "law")
	if set == nil {
		return
	}
	pattern := "%" + strings.NewReplacer("%", "", "_", "").Replace(q) + "%"
	out, err := s.lawRows(set, `law = ? AND title LIKE ?`, " LIMIT 50", law, pattern)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	total := len(out)
	s.ok(w, r, out, meta{Total: &total, Source: s.source("law"), Note: "Titles of the " + lawNames[law] + " that contain the words, at most 50. " + lawNote}, cacheAPI)
}
