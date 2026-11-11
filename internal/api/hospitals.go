package api

// Hospitals from the National Hospital Directory of May 2016 (server-only dataset hospitals).

import (
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

func (s *Server) registerHospitals() {
	s.handle("hospitals", costLight, "GET /v1/hospitals", s.handleHospitals)
}

type hospitalView struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Category    string   `json:"category,omitempty"`
	CareType    string   `json:"care_type,omitempty"`
	System      string   `json:"system_of_medicine,omitempty"`
	Address     string   `json:"address,omitempty"`
	District    string   `json:"district"`
	State       string   `json:"state"`
	Pincode     string   `json:"pincode,omitempty"`
	Telephone   string   `json:"telephone,omitempty"`
	Emergency   string   `json:"emergency,omitempty"`
	Ambulance   string   `json:"ambulance,omitempty"`
	Website     string   `json:"website,omitempty"`
	Specialties string   `json:"specialties,omitempty"`
	Facilities  string   `json:"facilities,omitempty"`
	Beds        *int     `json:"beds,omitempty"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
	DistanceKM  *float64 `json:"distance_km,omitempty"`
}

const hospitalNote = "From the National Hospital Directory as of May 2016: hospitals opened since are missing and some have closed or moved; some addresses are shifted by a column, as published. Call before you go. In an emergency dial 112 or 108."

func (s *Server) handleHospitals(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	where, args := []string{}, []any{}
	var nearLat, nearLon float64
	near := strings.TrimSpace(query.Get("near"))
	switch {
	case near != "":
		lat, lon, ok := strings.Cut(near, ",")
		var err1, err2 error
		nearLat, err1 = strconv.ParseFloat(strings.TrimSpace(lat), 64)
		nearLon, err2 = strconv.ParseFloat(strings.TrimSpace(lon), 64)
		if !ok || err1 != nil || err2 != nil || math.Abs(nearLat) > 90 || math.Abs(nearLon) > 180 {
			s.badRequest(w, r, "near is latitude,longitude, such as 11.22,78.17.")
			return
		}
		// A box of about 50 km first; distances are worked out for what is in it.
		where = append(where, "latitude BETWEEN ? AND ? AND longitude BETWEEN ? AND ?")
		args = append(args, nearLat-0.5, nearLat+0.5, nearLon-0.5, nearLon+0.5)
	case query.Get("pincode") != "":
		pin := strings.TrimSpace(query.Get("pincode"))
		if len(pin) != 6 || strings.Trim(pin, "0123456789") != "" {
			s.badRequest(w, r, "pincode is six digits.")
			return
		}
		where, args = append(where, "pincode = ?"), append(args, pin)
	case query.Get("district") != "":
		where, args = append(where, "district = ? COLLATE NOCASE"), append(args, strings.TrimSpace(query.Get("district")))
		if state := strings.TrimSpace(query.Get("state")); state != "" {
			where, args = append(where, "state = ? COLLATE NOCASE"), append(args, state)
		}
	case query.Get("q") != "":
		q := strings.TrimSpace(query.Get("q"))
		if len(q) < 3 || len(q) > 80 {
			s.badRequest(w, r, "q is 3 to 80 characters of a hospital's name.")
			return
		}
		where, args = append(where, "name LIKE ?"), append(args, "%"+strings.NewReplacer("%", "", "_", "").Replace(q)+"%")
		if state := strings.TrimSpace(query.Get("state")); state != "" {
			where, args = append(where, "state = ? COLLATE NOCASE"), append(args, state)
		}
	default:
		s.badRequest(w, r, "Give near (latitude,longitude), pincode, district (with state to narrow it), or q (part of a name).")
		return
	}
	if system := strings.TrimSpace(query.Get("system")); system != "" {
		where, args = append(where, "system LIKE ?"), append(args, "%"+system+"%")
	}
	info(r).subject = query.Get("pincode") + query.Get("district")
	set := s.need(w, r, "hospitals")
	if set == nil {
		return
	}
	rows, err := set.rows(`SELECT id, name, category, care_type, system, address, district, state, pincode, telephone, emergency, ambulance, website, specialties, facilities, beds, latitude, longitude
FROM hospitals WHERE `+strings.Join(where, " AND ")+` ORDER BY name LIMIT 500`, args...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()
	out := []hospitalView{}
	for rows.Next() {
		var h hospitalView
		if rows.Scan(&h.ID, &h.Name, &h.Category, &h.CareType, &h.System, &h.Address, &h.District, &h.State, &h.Pincode, &h.Telephone, &h.Emergency,
			&h.Ambulance, &h.Website, &h.Specialties, &h.Facilities, &h.Beds, &h.Latitude, &h.Longitude) != nil {
			continue
		}
		if near != "" && h.Latitude != nil && h.Longitude != nil {
			d := math.Round(haversineKM(nearLat, nearLon, *h.Latitude, *h.Longitude)*10) / 10
			h.DistanceKM = &d
		}
		out = append(out, h)
	}
	if near != "" {
		sort.SliceStable(out, func(i, j int) bool { return *out[i].DistanceKM < *out[j].DistanceKM })
		out = out[:min(len(out), 10)]
	} else {
		out = out[:min(len(out), 100)]
	}
	total := len(out)
	note := hospitalNote
	if near != "" {
		note = "The ten nearest hospitals within about 50 km that have a position (about a third do). " + hospitalNote
	}
	s.ok(w, r, out, meta{Total: &total, Source: s.source("hospitals"), Note: note}, cacheAPI)
}
