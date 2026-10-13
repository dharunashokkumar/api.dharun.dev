// Package india holds reference data about Indian states and union territories that
// several APIs share: one place to turn the many spellings found in source files into
// a single code.
package india

import "strings"

// State is one state or union territory.
type State struct {
	Code         string   `json:"code"`         // common two-letter code, e.g. TN
	Name         string   `json:"name"`         // current official name
	Type         string   `json:"type"`         // "state" or "union_territory"
	GSTCode      string   `json:"gstCode"`      // first two digits of a GSTIN
	VehicleCodes []string `json:"vehicleCodes"` // registration plate prefixes, current first
	aliases      []string
}

// States lists all 28 states and 8 union territories, ordered by name.
var States = []State{
	{Code: "AN", Name: "Andaman and Nicobar Islands", Type: "union_territory", GSTCode: "35", VehicleCodes: []string{"AN"}, aliases: []string{"andaman & nicobar islands", "andaman and nicobar", "andaman nicobar"}},
	{Code: "AP", Name: "Andhra Pradesh", Type: "state", GSTCode: "37", VehicleCodes: []string{"AP"}},
	{Code: "AR", Name: "Arunachal Pradesh", Type: "state", GSTCode: "12", VehicleCodes: []string{"AR"}},
	{Code: "AS", Name: "Assam", Type: "state", GSTCode: "18", VehicleCodes: []string{"AS"}},
	{Code: "BR", Name: "Bihar", Type: "state", GSTCode: "10", VehicleCodes: []string{"BR"}},
	{Code: "CH", Name: "Chandigarh", Type: "union_territory", GSTCode: "04", VehicleCodes: []string{"CH"}},
	{Code: "CG", Name: "Chhattisgarh", Type: "state", GSTCode: "22", VehicleCodes: []string{"CG"}, aliases: []string{"chattisgarh", "chhatisgarh"}},
	{Code: "DH", Name: "Dadra and Nagar Haveli and Daman and Diu", Type: "union_territory", GSTCode: "26", VehicleCodes: []string{"DD", "DN"}, aliases: []string{"the dadra and nagar haveli and daman and diu", "dadra and nagar haveli", "dadar and nagar haveli", "dadra & nagar haveli", "daman and diu", "daman & diu", "dadra and nagar haveli and daman & diu", "dadra & nagar haveli and daman & diu"}},
	{Code: "DL", Name: "Delhi", Type: "union_territory", GSTCode: "07", VehicleCodes: []string{"DL"}, aliases: []string{"nct of delhi", "new delhi", "national capital territory of delhi", "delhi nct", "nct delhi"}},
	{Code: "GA", Name: "Goa", Type: "state", GSTCode: "30", VehicleCodes: []string{"GA"}},
	{Code: "GJ", Name: "Gujarat", Type: "state", GSTCode: "24", VehicleCodes: []string{"GJ"}},
	{Code: "HR", Name: "Haryana", Type: "state", GSTCode: "06", VehicleCodes: []string{"HR"}},
	{Code: "HP", Name: "Himachal Pradesh", Type: "state", GSTCode: "02", VehicleCodes: []string{"HP"}},
	{Code: "JK", Name: "Jammu and Kashmir", Type: "union_territory", GSTCode: "01", VehicleCodes: []string{"JK"}, aliases: []string{"jammu & kashmir"}},
	{Code: "JH", Name: "Jharkhand", Type: "state", GSTCode: "20", VehicleCodes: []string{"JH"}},
	{Code: "KA", Name: "Karnataka", Type: "state", GSTCode: "29", VehicleCodes: []string{"KA"}},
	{Code: "KL", Name: "Kerala", Type: "state", GSTCode: "32", VehicleCodes: []string{"KL"}},
	{Code: "LA", Name: "Ladakh", Type: "union_territory", GSTCode: "38", VehicleCodes: []string{"LA"}},
	{Code: "LD", Name: "Lakshadweep", Type: "union_territory", GSTCode: "31", VehicleCodes: []string{"LD"}},
	{Code: "MP", Name: "Madhya Pradesh", Type: "state", GSTCode: "23", VehicleCodes: []string{"MP"}, aliases: []string{"madhy pradesh"}},
	{Code: "MH", Name: "Maharashtra", Type: "state", GSTCode: "27", VehicleCodes: []string{"MH"}},
	{Code: "MN", Name: "Manipur", Type: "state", GSTCode: "14", VehicleCodes: []string{"MN"}},
	{Code: "ML", Name: "Meghalaya", Type: "state", GSTCode: "17", VehicleCodes: []string{"ML"}},
	{Code: "MZ", Name: "Mizoram", Type: "state", GSTCode: "15", VehicleCodes: []string{"MZ"}},
	{Code: "NL", Name: "Nagaland", Type: "state", GSTCode: "13", VehicleCodes: []string{"NL"}},
	{Code: "OD", Name: "Odisha", Type: "state", GSTCode: "21", VehicleCodes: []string{"OD", "OR"}, aliases: []string{"orissa", "odhisa"}},
	{Code: "PY", Name: "Puducherry", Type: "union_territory", GSTCode: "34", VehicleCodes: []string{"PY"}, aliases: []string{"pondicherry"}},
	{Code: "PB", Name: "Punjab", Type: "state", GSTCode: "03", VehicleCodes: []string{"PB"}},
	{Code: "RJ", Name: "Rajasthan", Type: "state", GSTCode: "08", VehicleCodes: []string{"RJ"}},
	{Code: "SK", Name: "Sikkim", Type: "state", GSTCode: "11", VehicleCodes: []string{"SK"}},
	{Code: "TN", Name: "Tamil Nadu", Type: "state", GSTCode: "33", VehicleCodes: []string{"TN"}, aliases: []string{"tamilnadu"}},
	{Code: "TS", Name: "Telangana", Type: "state", GSTCode: "36", VehicleCodes: []string{"TG", "TS"}, aliases: []string{"telengana"}},
	{Code: "TR", Name: "Tripura", Type: "state", GSTCode: "16", VehicleCodes: []string{"TR"}},
	{Code: "UP", Name: "Uttar Pradesh", Type: "state", GSTCode: "09", VehicleCodes: []string{"UP"}},
	{Code: "UK", Name: "Uttarakhand", Type: "state", GSTCode: "05", VehicleCodes: []string{"UK", "UA"}, aliases: []string{"uttaranchal", "uttarkhand"}},
	{Code: "WB", Name: "West Bengal", Type: "state", GSTCode: "19", VehicleCodes: []string{"WB"}},
}

// GSTOnlyCodes are GSTIN prefixes that are valid but do not map to one current state.
var GSTOnlyCodes = map[string]string{
	"25": "Daman and Diu (merged into code 26)",
	"28": "Andhra Pradesh (before bifurcation)",
	"97": "Other Territory",
	"99": "Centre Jurisdiction",
}

var (
	byCode    = map[string]*State{}
	byName    = map[string]*State{}
	byGST     = map[string]*State{}
	byVehicle = map[string]*State{}
)

func init() {
	for i := range States {
		s := &States[i]
		byCode[s.Code] = s
		byGST[s.GSTCode] = s
		byName[fold(s.Name)] = s
		for _, alias := range s.aliases {
			byName[fold(alias)] = s
		}
		for _, v := range s.VehicleCodes {
			byVehicle[v] = s
		}
	}
}

// Lookup finds a state by code, vehicle prefix or any known spelling of its name.
func Lookup(value string) (State, bool) {
	trimmed := strings.TrimSpace(value)
	if s, ok := byCode[strings.ToUpper(trimmed)]; ok {
		return *s, true
	}
	if s, ok := byName[fold(trimmed)]; ok {
		return *s, true
	}
	if s, ok := byVehicle[strings.ToUpper(trimmed)]; ok {
		return *s, true
	}
	return State{}, false
}

// ByGSTCode finds the state for the first two digits of a GSTIN.
func ByGSTCode(code string) (State, bool) {
	s, ok := byGST[code]
	if !ok {
		return State{}, false
	}
	return *s, true
}

// ByVehicleCode finds the state for a registration plate prefix such as TN or TG.
func ByVehicleCode(code string) (State, bool) {
	s, ok := byVehicle[strings.ToUpper(code)]
	if !ok {
		return State{}, false
	}
	return *s, true
}

func fold(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "&", "and")
	return strings.Join(strings.Fields(value), " ")
}
