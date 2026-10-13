package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

type constituencyView struct {
	Number          int     `json:"number"`
	Name            string  `json:"name"`
	NameHindi       *string `json:"nameHindi,omitempty"`
	Reserved        *string `json:"reserved"` // GEN, SC or ST
	StateCode       *string `json:"stateCode"`
	State           *string `json:"state"`
	District        *string `json:"district,omitempty"`
	Parliament      *int    `json:"parliamentNumber,omitempty"`
	ParliamentName  *string `json:"parliamentName,omitempty"`
	Predelimitation bool    `json:"predelimitation,omitempty"`
	Wikidata        *string `json:"wikidata,omitempty"`
}

type constituencyAnswer struct {
	Latitude   float64           `json:"latitude"`
	Longitude  float64           `json:"longitude"`
	Pincode    string            `json:"pincode,omitempty"`
	Assembly   *constituencyView `json:"assembly"`
	Parliament *constituencyView `json:"parliament"`
}

// pincodePosition is the average position of a pincode's post offices that have one.
// It writes the error response itself and returns false when there is none.
func (s *Server) pincodePosition(w http.ResponseWriter, r *http.Request, pincode string) (lat, lng float64, district string, ok bool) {
	set := s.need(w, r, "pincode")
	if set == nil {
		return 0, 0, "", false
	}
	if !sixDigits.MatchString(pincode) {
		s.badRequest(w, r, "A pincode is six digits and does not start with 0.")
		return 0, 0, "", false
	}
	var latitude, longitude *float64
	var name *string
	if err := set.row(`SELECT AVG(lat), AVG(lng), MIN(district) FROM pincodes WHERE pincode = ? AND lat IS NOT NULL`, pincode).Scan(&latitude, &longitude, &name); err != nil || latitude == nil {
		s.notFound(w, r, "No position is known for pincode "+pincode+". Give lat and lng instead.")
		return 0, 0, "", false
	}
	if name != nil {
		district = *name
	}
	return *latitude, *longitude, district, true
}

func (s *Server) handleConstituency(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "constituencies")
	if set == nil {
		return
	}
	answer := constituencyAnswer{}
	if pincode := strings.TrimSpace(r.URL.Query().Get("pincode")); pincode != "" {
		lat, lng, _, ok := s.pincodePosition(w, r, pincode)
		if !ok {
			return
		}
		answer.Latitude, answer.Longitude, answer.Pincode = round(lat, 5), round(lng, 5), pincode
		info(r).subject = pincode
	} else {
		lat, lng, ok := s.coordinateParams(w, r)
		if !ok {
			return
		}
		answer.Latitude, answer.Longitude = lat, lng
		info(r).subject = strconv.FormatFloat(lat, 'f', 2, 64) + "," + strconv.FormatFloat(lng, 'f', 2, 64)
	}

	rows, err := set.rows(`SELECT c.kind, c.number, c.name, c.name_hi, c.reserved, c.state_code, c.state, c.district,
		c.parliament_number, c.parliament_name, c.predelimitation, c.wikidata, c.rings
		FROM constituency_boxes b JOIN constituencies c ON c.id = b.id
		WHERE b.min_lng <= ? AND b.max_lng >= ? AND b.min_lat <= ? AND b.max_lat >= ?`,
		answer.Longitude, answer.Longitude, answer.Latitude, answer.Latitude)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var view constituencyView
		var nameHi, reserved, stateCode, state, district, parliamentName, wikidata sql.NullString
		var parliament sql.NullInt64
		var rings []byte
		if err := rows.Scan(&kind, &view.Number, &view.Name, &nameHi, &reserved, &stateCode, &state, &district,
			&parliament, &parliamentName, &view.Predelimitation, &wikidata, &rings); err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		decoded, err := dataset.DecodeRings(rings)
		if err != nil || !dataset.Contains(decoded, answer.Longitude, answer.Latitude) {
			continue
		}
		view.NameHindi, view.Reserved, view.StateCode, view.State = str(nameHi), str(reserved), str(stateCode), str(state)
		view.District, view.ParliamentName, view.Wikidata = str(district), str(parliamentName), str(wikidata)
		if parliament.Valid {
			number := int(parliament.Int64)
			view.Parliament = &number
		}
		found := view
		switch {
		case kind == "assembly" && answer.Assembly == nil:
			answer.Assembly = &found
		case kind == "parliament" && answer.Parliament == nil:
			answer.Parliament = &found
		}
	}
	if err := rows.Err(); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	// The assembly map still files Telangana's seats under Andhra Pradesh; the 2019
	// parliamentary map has the state right.
	if answer.Assembly != nil && answer.Parliament != nil && answer.Parliament.StateCode != nil && answer.Assembly.StateCode != nil &&
		*answer.Assembly.StateCode == "AP" && *answer.Parliament.StateCode == "TS" {
		answer.Assembly.StateCode, answer.Assembly.State = answer.Parliament.StateCode, answer.Parliament.State
	}
	if answer.Assembly == nil && answer.Parliament == nil {
		s.notFound(w, r, "That point is in no constituency on the maps (it may be at sea or just over a border).")
		return
	}
	s.ok(w, r, answer, meta{Source: s.source("constituencies"), Note: constituencyNote}, cacheLong)
}

const constituencyNote = "Boundaries from community maps, approximate near edges. Assembly seats in Arunachal Pradesh, Assam, Jammu and Kashmir, " +
	"Jharkhand, Manipur and Nagaland predate the latest delimitation (marked predelimitation). Sitting members are not given."

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "find_constituency",
		Description: "Find the assembly (MLA) and parliamentary (MP, Lok Sabha) constituency of a place in India, by pincode or by latitude and longitude.",
		InputSchema: schema(nil, map[string]any{
			"pincode": text("Indian pincode."), "lat": map[string]any{"type": "number"}, "lng": map[string]any{"type": "number"},
		}),
		path: func(args map[string]any) (string, error) {
			return "/v1/constituency" + query(args, map[string]string{"pincode": "pincode", "lat": "lat", "lng": "lng"}), nil
		},
	})
}
