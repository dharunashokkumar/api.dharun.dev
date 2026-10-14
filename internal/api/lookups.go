package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/india"
)

// ---- shared helpers ----

// stateParam turns ?state=TN or ?state=Tamil%20Nadu into a state code. The second
// return value is false when the parameter is present but not a state.
func stateParam(r *http.Request) (string, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("state"))
	if raw == "" {
		return "", true
	}
	state, ok := india.Lookup(raw)
	return state.Code, ok
}

func (s *Server) listParams(w http.ResponseWriter, r *http.Request, defaultLimit int) (limit, offset int, ok bool) {
	limit, err := intParam(r, "limit", defaultLimit, 1, 100)
	if err == nil {
		offset, err = intParam(r, "offset", 0, 0, 100000)
	}
	if err != nil {
		s.badRequest(w, r, err.Error())
		return 0, 0, false
	}
	return limit, offset, true
}

func str(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func number(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return &value.Float64
}

// likePrefix escapes a user string for use in LIKE ? ESCAPE '\' and adds the wildcard.
func likePattern(value string, contains bool) string {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
	if contains {
		return "%" + escaped + "%"
	}
	return escaped + "%"
}

// ---- pincode ----

type postOffice struct {
	Pincode   string   `json:"pincode,omitempty"`
	Name      string   `json:"name"`
	Type      *string  `json:"type"`
	Delivery  bool     `json:"delivery"`
	Division  *string  `json:"division"`
	Region    *string  `json:"region"`
	Circle    *string  `json:"circle"`
	District  *string  `json:"district"`
	StateCode *string  `json:"stateCode"`
	State     *string  `json:"state"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type pincodeView struct {
	Pincode   string       `json:"pincode"`
	District  *string      `json:"district"`
	StateCode *string      `json:"stateCode"`
	State     *string      `json:"state"`
	Offices   []postOffice `json:"offices"`
}

var sixDigits = regexp.MustCompile(`^[1-9][0-9]{5}$`)

const pincodeQuery = `SELECT office, office_type, delivery, division, region, circle, district, state_code, state, lat, lng FROM pincodes WHERE pincode = ? ORDER BY office_type = 'Head Office' DESC, office_type = 'Post Office' DESC, office`

func (s *Server) handlePincode(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "pincode")
	if set == nil {
		return
	}
	pincode := strings.Join(strings.Fields(r.PathValue("pincode")), "")
	info(r).subject = pincode
	if !sixDigits.MatchString(pincode) {
		s.badRequest(w, r, "A pincode is six digits and does not start with 0.")
		return
	}

	rows, err := set.rows(pincodeQuery, pincode)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()

	view := pincodeView{Pincode: pincode}
	for rows.Next() {
		var office postOffice
		var officeType, division, region, circle, district, stateCode, state sql.NullString
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&office.Name, &officeType, &office.Delivery, &division, &region, &circle, &district, &stateCode, &state, &lat, &lng); err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		office.Type, office.Division, office.Region, office.Circle = str(officeType), str(division), str(region), str(circle)
		office.District, office.StateCode, office.State = str(district), str(stateCode), str(state)
		office.Latitude, office.Longitude = number(lat), number(lng)
		view.Offices = append(view.Offices, office)
	}
	if rows.Err() != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	if len(view.Offices) == 0 {
		s.notFound(w, r, "No post office found for pincode "+pincode+".")
		return
	}
	// The head or sub office sorts first and speaks for the whole pincode.
	view.District, view.StateCode, view.State = view.Offices[0].District, view.Offices[0].StateCode, view.Offices[0].State
	s.ok(w, r, view, meta{Source: s.source("pincode")}, cacheLong)
}

func (s *Server) handlePincodeSearch(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "pincode")
	if set == nil {
		return
	}
	query := r.URL.Query()
	text := strings.TrimSpace(query.Get("q"))
	district := strings.TrimSpace(query.Get("district"))
	stateCode, stateOK := stateParam(r)
	limit, offset, ok := s.listParams(w, r, 20)
	if !ok {
		return
	}
	if !stateOK {
		s.badRequest(w, r, "state is not a known state name or code. See /v1/states.")
		return
	}
	if len(text) < 2 && district == "" {
		s.badRequest(w, r, "Give q (at least 2 characters of a post office name, or the start of a pincode) or a district.")
		return
	}
	info(r).subject = text

	var where []string
	var args []any
	if text != "" {
		if strings.Trim(text, "0123456789") == "" {
			// A range uses the pincode index; LIKE would scan every row.
			where = append(where, `pincode >= ? AND pincode < ?`)
			args = append(args, text, text+":")
		} else {
			where = append(where, `office LIKE ? ESCAPE '\'`)
			args = append(args, likePattern(text, false))
		}
	}
	if stateCode != "" {
		where = append(where, `state_code = ?`)
		args = append(args, stateCode)
	}
	if district != "" {
		where = append(where, `district = ? COLLATE NOCASE`)
		args = append(args, district)
	}
	condition := strings.Join(where, " AND ")

	var total int
	if err := set.db.QueryRow(`SELECT COUNT(*) FROM pincodes WHERE `+condition, args...).Scan(&total); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	rows, err := set.db.Query(`SELECT pincode, office, office_type, delivery, division, region, circle, district, state_code, state, lat, lng
		FROM pincodes WHERE `+condition+` ORDER BY office COLLATE NOCASE, pincode LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()

	offices := []postOffice{}
	for rows.Next() {
		var office postOffice
		var officeType, division, region, circle, districtName, code, state sql.NullString
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&office.Pincode, &office.Name, &officeType, &office.Delivery, &division, &region, &circle, &districtName, &code, &state, &lat, &lng); err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		office.Type, office.Division, office.Region, office.Circle = str(officeType), str(division), str(region), str(circle)
		office.District, office.StateCode, office.State = str(districtName), str(code), str(state)
		office.Latitude, office.Longitude = number(lat), number(lng)
		offices = append(offices, office)
	}
	if rows.Err() != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	m := paging(total, limit, offset)
	m.Source = s.source("pincode")
	s.ok(w, r, offices, m, cacheAPI)
}

// ---- IFSC and banks ----

type bankView struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Type     *string `json:"type,omitempty"`
	MICR     *string `json:"micr,omitempty"`
	UPI      *bool   `json:"upi,omitempty"`
	Branches *int    `json:"branches,omitempty"`
}

type branchView struct {
	IFSC      string   `json:"ifsc"`
	Bank      bankView `json:"bank"`
	Branch    *string  `json:"branch"`
	Address   *string  `json:"address"`
	City      *string  `json:"city"`
	Centre    *string  `json:"centre"`
	District  *string  `json:"district"`
	StateCode *string  `json:"stateCode"`
	State     *string  `json:"state"`
	Contact   *string  `json:"contact"`
	MICR      *string  `json:"micr"`
	SWIFT     *string  `json:"swift"`
	IMPS      bool     `json:"imps"`
	NEFT      bool     `json:"neft"`
	RTGS      bool     `json:"rtgs"`
	UPI       bool     `json:"upi"`
}

var (
	ifscShape = regexp.MustCompile(`^[A-Z]{4}0[A-Z0-9]{6}$`)
	bankShape = regexp.MustCompile(`^[A-Z]{4}$`)
)

const branchColumns = `i.code, i.bank_code, b.name, i.branch, i.centre, i.district, i.state_code, i.state, i.city, i.address, i.contact, i.micr, i.swift, i.imps, i.neft, i.rtgs, i.upi`

func scanBranch(scan func(...any) error) (branchView, error) {
	var view branchView
	var branch, centre, district, stateCode, state, city, address, contact, micr, swift sql.NullString
	err := scan(&view.IFSC, &view.Bank.Code, &view.Bank.Name, &branch, &centre, &district, &stateCode, &state, &city, &address, &contact, &micr, &swift, &view.IMPS, &view.NEFT, &view.RTGS, &view.UPI)
	view.Branch, view.Centre, view.District, view.StateCode, view.State = str(branch), str(centre), str(district), str(stateCode), str(state)
	view.City, view.Address, view.Contact, view.MICR, view.SWIFT = str(city), str(address), str(contact), str(micr), str(swift)
	return view, err
}

func (s *Server) handleIFSC(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "ifsc")
	if set == nil {
		return
	}
	code := strings.ToUpper(strings.Join(strings.Fields(r.PathValue("code")), ""))
	info(r).subject = code
	if !ifscShape.MatchString(code) {
		s.badRequest(w, r, "An IFSC is 11 characters: four letters, the digit 0, then six letters or digits.")
		return
	}
	view, err := scanBranch(set.row(`SELECT `+branchColumns+` FROM ifsc i JOIN banks b ON b.code = i.bank_code WHERE i.code = ?`, code).Scan)
	if err == sql.ErrNoRows {
		s.notFound(w, r, "No branch found for IFSC "+code+".")
		return
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	s.ok(w, r, view, meta{Source: s.source("ifsc")}, cacheLong)
}

func (s *Server) handleIFSCSearch(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "ifsc")
	if set == nil {
		return
	}
	query := r.URL.Query()
	bank := strings.ToUpper(strings.TrimSpace(query.Get("bank")))
	text := strings.TrimSpace(query.Get("q"))
	city := strings.TrimSpace(query.Get("city"))
	stateCode, stateOK := stateParam(r)
	limit, offset, ok := s.listParams(w, r, 20)
	if !ok {
		return
	}
	if !bankShape.MatchString(bank) {
		s.badRequest(w, r, "bank is required: the four-letter bank code, for example SBIN. Find codes at /v1/banks?q=.")
		return
	}
	if !stateOK {
		s.badRequest(w, r, "state is not a known state name or code. See /v1/states.")
		return
	}
	info(r).subject = bank + " " + text

	where := []string{`i.bank_code = ?`}
	args := []any{bank}
	if stateCode != "" {
		where = append(where, `i.state_code = ?`)
		args = append(args, stateCode)
	}
	if city != "" {
		where = append(where, `i.city = ? COLLATE NOCASE`)
		args = append(args, city)
	}
	if text != "" {
		where = append(where, `(i.branch LIKE ? ESCAPE '\' OR i.address LIKE ? ESCAPE '\' OR i.city LIKE ? ESCAPE '\')`)
		pattern := likePattern(text, true)
		args = append(args, pattern, pattern, pattern)
	}
	condition := strings.Join(where, " AND ")

	var total int
	if err := set.db.QueryRow(`SELECT COUNT(*) FROM ifsc i WHERE `+condition, args...).Scan(&total); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	rows, err := set.db.Query(`SELECT `+branchColumns+` FROM ifsc i JOIN banks b ON b.code = i.bank_code WHERE `+condition+
		` ORDER BY i.state, i.city, i.branch LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()

	branches := []branchView{}
	for rows.Next() {
		view, err := scanBranch(rows.Scan)
		if err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		branches = append(branches, view)
	}
	if rows.Err() != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	m := paging(total, limit, offset)
	m.Source = s.source("ifsc")
	s.ok(w, r, branches, m, cacheAPI)
}

func scanBank(scan func(...any) error) (bankView, error) {
	var view bankView
	var bankType, micr sql.NullString
	var upi bool
	var branches int
	err := scan(&view.Code, &view.Name, &bankType, &micr, &upi, &branches)
	view.Type, view.MICR, view.UPI, view.Branches = str(bankType), str(micr), &upi, &branches
	return view, err
}

func (s *Server) handleBank(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "ifsc")
	if set == nil {
		return
	}
	code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	info(r).subject = code
	if !bankShape.MatchString(code) {
		s.badRequest(w, r, "A bank code is four letters, for example SBIN.")
		return
	}
	view, err := scanBank(set.row(`SELECT code, name, type, micr, upi, branches FROM banks WHERE code = ?`, code).Scan)
	if err == sql.ErrNoRows {
		s.notFound(w, r, "No bank found for code "+code+".")
		return
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	s.ok(w, r, view, meta{Source: s.source("ifsc")}, cacheLong)
}

func (s *Server) handleBankSearch(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "ifsc")
	if set == nil {
		return
	}
	text := strings.TrimSpace(r.URL.Query().Get("q"))
	limit, offset, ok := s.listParams(w, r, 50)
	if !ok {
		return
	}
	info(r).subject = text

	pattern := likePattern(text, true)
	var total int
	if err := set.db.QueryRow(`SELECT COUNT(*) FROM banks WHERE name LIKE ? ESCAPE '\' OR code LIKE ? ESCAPE '\'`, pattern, pattern).Scan(&total); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	rows, err := set.db.Query(`SELECT code, name, type, micr, upi, branches FROM banks
		WHERE name LIKE ? ESCAPE '\' OR code LIKE ? ESCAPE '\' ORDER BY branches DESC, name LIMIT ? OFFSET ?`, pattern, pattern, limit, offset)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()

	banks := []bankView{}
	for rows.Next() {
		view, err := scanBank(rows.Scan)
		if err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		banks = append(banks, view)
	}
	if rows.Err() != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	m := paging(total, limit, offset)
	m.Source = s.source("ifsc")
	s.ok(w, r, banks, m, cacheAPI)
}

// ---- RTO ----

type rtoView struct {
	Code         string  `json:"code"`
	Office       string  `json:"office"`
	Jurisdiction *string `json:"jurisdiction"`
	StateCode    *string `json:"stateCode"`
	State        *string `json:"state"`
	Notes        *string `json:"notes"`
}

// A code such as TN-28, TN28 or "tn 28z", or a whole plate such as TN28AB1234.
var rtoInput = regexp.MustCompile(`^([A-Z]{2})[ -]?([0-9]{1,2})[ -]?([A-Z]{0,3})[ -]?[0-9]{0,4}$`)

func scanRTO(scan func(...any) error) (rtoView, error) {
	var view rtoView
	var prefix string
	var stateCode, state, jurisdiction, notes sql.NullString
	err := scan(&view.Code, &prefix, &stateCode, &state, &view.Office, &jurisdiction, &notes)
	view.StateCode, view.State, view.Jurisdiction, view.Notes = str(stateCode), str(state), str(jurisdiction), str(notes)
	return view, err
}

func (s *Server) handleRTO(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "rto")
	if set == nil {
		return
	}
	input := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	info(r).subject = input
	match := rtoInput.FindStringSubmatch(input)
	if match == nil {
		s.badRequest(w, r, "Give an RTO code such as TN-28, or a registration number such as TN28AB1234.")
		return
	}
	digits, _ := strconv.Atoi(match[2])
	base := fmt.Sprintf("%s-%02d", match[1], digits)

	// A few offices have a letter suffix (TN-28Z). Try that reading first, then the plain code.
	candidates := []string{base}
	if match[3] != "" {
		candidates = []string{base + match[3], base + match[3][:1], base}
	}
	for _, code := range candidates {
		view, err := scanRTO(set.row(`SELECT code, prefix, state_code, state, office, jurisdiction, notes FROM rto WHERE code = ?`, code).Scan)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		s.ok(w, r, view, meta{Source: s.source("rto")}, cacheLong)
		return
	}
	s.notFound(w, r, "No RTO found for "+base+".")
}

func (s *Server) handleRTOSearch(w http.ResponseWriter, r *http.Request) {
	set := s.need(w, r, "rto")
	if set == nil {
		return
	}
	text := strings.TrimSpace(r.URL.Query().Get("q"))
	stateCode, stateOK := stateParam(r)
	limit, offset, ok := s.listParams(w, r, 50)
	if !ok {
		return
	}
	if !stateOK {
		s.badRequest(w, r, "state is not a known state name or code. See /v1/states.")
		return
	}
	if stateCode == "" && len(text) < 2 {
		s.badRequest(w, r, "Give a state, or q with at least 2 characters of an office or district name.")
		return
	}
	info(r).subject = text

	var where []string
	var args []any
	if stateCode != "" {
		where = append(where, `state_code = ?`)
		args = append(args, stateCode)
	}
	if text != "" {
		where = append(where, `(office LIKE ? ESCAPE '\' OR jurisdiction LIKE ? ESCAPE '\')`)
		pattern := likePattern(text, true)
		args = append(args, pattern, pattern)
	}
	condition := strings.Join(where, " AND ")

	var total int
	if err := set.db.QueryRow(`SELECT COUNT(*) FROM rto WHERE `+condition, args...).Scan(&total); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	rows, err := set.db.Query(`SELECT code, prefix, state_code, state, office, jurisdiction, notes FROM rto WHERE `+condition+
		` ORDER BY code LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer rows.Close()

	offices := []rtoView{}
	for rows.Next() {
		view, err := scanRTO(rows.Scan)
		if err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
			return
		}
		offices = append(offices, view)
	}
	if rows.Err() != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	m := paging(total, limit, offset)
	m.Source = s.source("rto")
	s.ok(w, r, offices, m, cacheAPI)
}
