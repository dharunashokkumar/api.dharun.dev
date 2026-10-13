package api

import (
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/fake"
)

var fakeKinds = []string{"people", "companies", "addresses"}

// fakePlace picks a real post office for a made-up address: any in the country by row
// number, or the nth in a state through the state index.
func (s *Server) fakePlace(set *handle, counts map[string]int) func(string, func(int) int) (fake.Place, bool) {
	return func(stateCode string, pick func(int) int) (fake.Place, bool) {
		var place fake.Place
		var err error
		if stateCode == "" {
			total := set.count("post_offices")
			if total == 0 {
				return place, false
			}
			err = set.row(`SELECT pincode, office, COALESCE(district, ''), COALESCE(state_code, ''), COALESCE(state, '') FROM pincodes
				WHERE rowid >= ? AND state_code IS NOT NULL LIMIT 1`, 1+pick(total)).Scan(&place.Pincode, &place.PostOffice, &place.District, &place.StateCode, &place.State)
		} else {
			total, known := counts[stateCode]
			if !known {
				set.row(`SELECT COUNT(*) FROM pincodes WHERE state_code = ?`, stateCode).Scan(&total)
				counts[stateCode] = total
			}
			if total == 0 {
				return place, false
			}
			err = set.row(`SELECT pincode, office, COALESCE(district, ''), state_code, COALESCE(state, '') FROM pincodes
				WHERE rowid = (SELECT rowid FROM pincodes WHERE state_code = ? LIMIT 1 OFFSET ?)`, stateCode, pick(total)).
				Scan(&place.Pincode, &place.PostOffice, &place.District, &place.StateCode, &place.State)
		}
		return place, err == nil
	}
}

func (s *Server) handleFake(w http.ResponseWriter, r *http.Request) {
	kind := strings.ToLower(r.PathValue("kind"))
	if !contains(fakeKinds, kind) {
		s.notFound(w, r, "Unknown kind of fake data. Use one of: "+strings.Join(fakeKinds, ", ")+".")
		return
	}
	info(r).subject = kind
	count, err := intParam(r, "count", 10, 1, 100)
	if err != nil {
		s.badRequest(w, r, err.Error())
		return
	}
	stateCode, stateOK := stateParam(r)
	if !stateOK {
		s.badRequest(w, r, "state is not a known state name or code. See /v1/states.")
		return
	}
	query := r.URL.Query()
	seed, cache := rand.Uint64()>>12, cacheNone
	if raw := query.Get("seed"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 52)
		if err != nil {
			s.badRequest(w, r, "seed is a whole number up to 4503599627370495.")
			return
		}
		seed, cache = parsed, cacheLong
	}

	generator := fake.New(seed)
	generator.State = stateCode
	if set := s.data("pincode"); set != nil {
		generator.Place = s.fakePlace(set, map[string]int{})
	}
	if set := s.data("phones"); set != nil {
		if rows, err := set.rows(`SELECT series FROM series WHERE operator IS NULL AND circle IS NULL ORDER BY series`); err == nil {
			for rows.Next() {
				var series string
				if rows.Scan(&series) == nil {
					generator.PhoneSeries = append(generator.PhoneSeries, series)
				}
			}
			rows.Close()
		}
	}

	records := make([]any, 0, count)
	for i := 0; i < count; i++ {
		switch kind {
		case "people":
			records = append(records, generator.Person())
		case "companies":
			records = append(records, generator.Company())
		case "addresses":
			address := generator.Address()
			if address == nil {
				s.need(w, r, "pincode")
				return
			}
			records = append(records, address)
		}
	}
	total := len(records)
	s.ok(w, r, records, meta{Total: &total, Source: s.source("pincode"),
		Note: "Made-up test data: no record describes a real person or business. Pincodes, districts and states are real and belong together; " +
			"everything else is invented. Phone numbers are in series not known to be in use, but never call or message them. seed=" +
			strconv.FormatUint(seed, 10) + " gives these same records again."}, cache)
}

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "make_fake_indian_data",
		Description: "Generate made-up Indian test data for building apps: people (name, gender, birth date, email, phone, PAN, address) or companies (name, PAN, GSTIN with a valid check character, address) or addresses, at real pincodes. Never real people.",
		InputSchema: schema([]string{"kind"}, map[string]any{
			"kind":  oneOf("What to generate.", fakeKinds...),
			"count": map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "default": 10},
			"state": stateArg,
			"seed":  map[string]any{"type": "integer", "description": "Give the same seed to get the same records."},
		}),
		path: func(args map[string]any) (string, error) {
			kind, err := required(args, "kind")
			return "/v1/fake/" + kind + query(args, map[string]string{"count": "count", "state": "state", "seed": "seed"}), err
		},
	})
}
