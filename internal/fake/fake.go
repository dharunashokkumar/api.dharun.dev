// Package fake makes up Indian test data: people, companies and addresses that look
// right and pass format checks, for filling forms and databases while building an app.
// Nothing here describes a real person or business. Given the same seed it makes the
// same data.
package fake

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/india"
	"github.com/dharunashokkumar/api.dharun.dev/internal/validate"
)

var officeSuffix = regexp.MustCompile(`(?i)\s+(B\.?O|S\.?O|H\.?O|G\.?P\.?O|P\.?O)\.?$`)

// Place is a real post office to hang a made-up address on. The generator asks for one
// per address, so that the pincode, district and state belong together.
type Place struct {
	Pincode, PostOffice, District, StateCode, State string
}

// Address is a made-up street address at a real pincode.
type Address struct {
	Line1     string `json:"line1"`
	Line2     string `json:"line2"`
	City      string `json:"city"`
	District  string `json:"district"`
	StateCode string `json:"stateCode"`
	State     string `json:"state"`
	Pincode   string `json:"pincode"`
}

// Person is a made-up person.
type Person struct {
	Fake      bool     `json:"fake"`
	Name      string   `json:"name"`
	FirstName string   `json:"firstName"`
	LastName  string   `json:"lastName"`
	Gender    string   `json:"gender"`
	BirthDate string   `json:"birthDate"`
	Email     string   `json:"email"`
	Phone     string   `json:"phone"`
	PAN       string   `json:"pan"`
	Address   *Address `json:"address,omitempty"`
}

// Company is a made-up business.
type Company struct {
	Fake    bool     `json:"fake"`
	Name    string   `json:"name"`
	PAN     string   `json:"pan"`
	GSTIN   string   `json:"gstin"`
	Email   string   `json:"email"`
	Phone   string   `json:"phone"`
	Address *Address `json:"address,omitempty"`
}

// Generator makes data from one seed.
type Generator struct {
	random *rand.Rand
	// Place returns a real post office, in the given state when one is named. It may
	// return false when no place data is available; addresses are then left out.
	Place func(stateCode string, pick func(n int) int) (Place, bool)
	// PhoneSeries are four-digit mobile series known to be unallocated; numbers are
	// drawn from them so they are unlikely to be anyone's. Empty falls back to 90000.
	PhoneSeries []string
	// State limits everything to one state; empty spreads across the country.
	State string
	// Today anchors birth dates, so a fixed seed keeps giving the same ages.
	Today time.Time
}

// New returns a generator for a seed.
func New(seed uint64) *Generator {
	return &Generator{random: rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15)), Today: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (g *Generator) pick(values []string) string { return values[g.random.IntN(len(values))] }

func (g *Generator) letters(n int) string {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte('A' + g.random.IntN(26))
	}
	return string(out)
}

func (g *Generator) digits(n int) string {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte('0' + g.random.IntN(10))
	}
	return string(out)
}

// pan makes a PAN of the right shape: holder is P for a person or C for a company, and
// the fifth letter is the first letter of the surname or business name, as on real PANs.
// A PAN has no public check digit, so every well-formed one "passes".
func (g *Generator) pan(holder byte, name string) string {
	initial := byte('A' + g.random.IntN(26))
	for _, r := range strings.ToUpper(name) {
		if r >= 'A' && r <= 'Z' {
			initial = byte(r)
			break
		}
	}
	return g.letters(3) + string(holder) + string(initial) + g.digits(4) + g.letters(1)
}

// phone makes a mobile number in a series that is not known to be in use.
func (g *Generator) phone() string {
	series := "9000"
	if len(g.PhoneSeries) > 0 {
		series = g.pick(g.PhoneSeries)
	}
	number := series + g.digits(6)
	return "+91 " + number[:5] + " " + number[5:]
}

func (g *Generator) address() (*Address, string) {
	if g.Place == nil {
		return nil, g.State
	}
	place, ok := g.Place(g.State, g.random.IntN)
	if !ok {
		return nil, g.State
	}
	// A post office is named for its place plus its kind: "Alanganallur SO", "Periyapatti B.O".
	city := strings.TrimSpace(officeSuffix.ReplaceAllString(place.PostOffice, ""))
	return &Address{
		Line1:     fmt.Sprintf("No. %d, %s", 1+g.random.IntN(240), g.pick(streets)),
		Line2:     g.pick(localities),
		City:      city,
		District:  place.District,
		StateCode: place.StateCode,
		State:     place.State,
		Pincode:   place.Pincode,
	}, place.StateCode
}

func (g *Generator) pool(stateCode string) pool {
	if name, ok := statePool[stateCode]; ok {
		return pools[name]
	}
	if stateCode == "" {
		// No state asked for and no place data: any region.
		names := []string{"tamil", "telugu", "kannada", "malayalam", "marathi", "gujarati", "punjabi", "bengali", "hindi"}
		return pools[names[g.random.IntN(len(names))]]
	}
	return pools["hindi"]
}

func emailName(parts ...string) string {
	joined := strings.ToLower(strings.Join(parts, "."))
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' {
			return r
		}
		return -1
	}, joined)
}

// Person makes one person.
func (g *Generator) Person() Person {
	address, stateCode := g.address()
	names := g.pool(stateCode)
	person := Person{Fake: true, Gender: "male", Address: address}
	person.FirstName = g.pick(names.male)
	if g.random.IntN(2) == 0 {
		person.Gender, person.FirstName = "female", g.pick(names.female)
	}
	person.LastName = g.pick(names.family)
	person.Name = person.FirstName + " " + person.LastName
	// Adults between 18 and 78.
	born := g.Today.AddDate(-18-g.random.IntN(60), 0, -g.random.IntN(365))
	person.BirthDate = born.Format("2006-01-02")
	person.Email = emailName(person.FirstName, person.LastName) + fmt.Sprint(10+g.random.IntN(90)) + "@" + g.pick(emailHosts)
	person.Phone = g.phone()
	person.PAN = g.pan('P', person.LastName)
	return person
}

// Company makes one business, with a GSTIN whose state code and check character are right.
func (g *Generator) Company() Company {
	address, stateCode := g.address()
	company := Company{Fake: true, Address: address}
	first, trade := g.pick(companyFirst), g.pick(companyTrade)
	company.Name = first + " " + trade + g.pick(companyForm)
	company.PAN = g.pan('C', company.Name)
	gstState := "33" // Tamil Nadu, when no state is known
	if state, ok := india.Lookup(stateCode); ok && state.GSTCode != "" {
		gstState = state.GSTCode
	}
	first14 := gstState + company.PAN + "1Z"
	company.GSTIN = first14 + string(validate.GSTINCheckChar(first14))
	company.Email = "accounts@" + emailName(strings.ReplaceAll(first+trade, " ", "")) + "." + g.pick(emailHosts)
	company.Phone = g.phone()
	return company
}

// Address makes one address, or nil when no place data is available.
func (g *Generator) Address() *Address {
	address, _ := g.address()
	return address
}
