package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"math"
	"math/bits"
	"os"
	"path/filepath"
	"time"
)

// Unique visitors, counted without keeping addresses. Each client address is hashed
// with a random key that lives for one period (a day or a month, India time) into a
// HyperLogLog sketch: a few thousand small counters from which the number of distinct
// addresses can be estimated, while no address can be read back out. The sketches and
// the keys of the periods still running are saved, so a deploy does not start the count
// again; when a period ends its key and sketches are thrown away.

const (
	visitorPrecision = 14 // 16,384 counters: about 0.8% error
	countryPrecision = 10 // 1,024 counters per country and day: about 3%
)

// sketch is a HyperLogLog: one counter per bucket of hashes, holding the longest run of
// leading zeros seen in that bucket.
type sketch []uint8

func newSketch(precision int) sketch { return make(sketch, 1<<precision) }

// add records a 64-bit hash and reports whether the sketch changed.
func (s sketch) add(hash uint64) bool {
	precision := bits.TrailingZeros(uint(len(s)))
	bucket := hash >> (64 - precision)
	// The set bit below the bucket's bits caps the run, so the rank fits a byte.
	rank := uint8(bits.LeadingZeros64(hash<<precision|1<<(precision-1)) + 1)
	if rank > s[bucket] {
		s[bucket] = rank
		return true
	}
	return false
}

// estimate is the number of distinct hashes added, with the small-range correction.
func (s sketch) estimate() uint64 {
	buckets := float64(len(s))
	var sum float64
	zeros := 0
	for _, rank := range s {
		sum += math.Ldexp(1, -int(rank))
		if rank == 0 {
			zeros++
		}
	}
	estimate := 0.7213 / (1 + 1.079/buckets) * buckets * buckets / sum
	if estimate <= 2.5*buckets && zeros > 0 {
		estimate = buckets * math.Log(buckets/float64(zeros))
	}
	return uint64(math.Round(estimate))
}

// visitorPeriod is one day or month of counting.
type visitorPeriod struct {
	Key       string            `json:"key"` // 2026-10-15 or 2026-10
	Salt      []byte            `json:"salt"`
	All       sketch            `json:"all"`
	Countries map[string]sketch `json:"countries,omitempty"`
	// Previous is the final count of the period just before this one, when the server
	// was running at its end.
	Previous    uint64 `json:"previous"`
	PreviousKey string `json:"previousKey,omitempty"`
}

func newVisitorPeriod(key string, before *visitorPeriod, previousKey string) *visitorPeriod {
	period := &visitorPeriod{Key: key, Salt: make([]byte, 32), All: newSketch(visitorPrecision)}
	rand.Read(period.Salt)
	if before != nil && before.Key == previousKey {
		period.Previous, period.PreviousKey = before.All.estimate(), before.Key
	}
	return period
}

func (p *visitorPeriod) hash(client string) uint64 {
	sum := sha256.Sum256(append(append([]byte{}, p.Salt...), client...))
	return binary.BigEndian.Uint64(sum[:8])
}

func (p *visitorPeriod) valid(key string) bool {
	if p == nil || p.Key != key || len(p.Salt) != 32 || len(p.All) != 1<<visitorPrecision {
		return false
	}
	for _, s := range p.Countries {
		if len(s) != 1<<countryPrecision {
			return false
		}
	}
	return true
}

type visitors struct {
	Day   *visitorPeriod `json:"day"`
	Month *visitorPeriod `json:"month"`
	dirty bool
	now   func() time.Time
	path  string
}

// visitorClock tells the time for counting periods; tests move it.
var visitorClock = time.Now

func newVisitors(stateDir string) *visitors {
	v := &visitors{now: visitorClock}
	if stateDir != "" {
		v.path = filepath.Join(stateDir, "visitors.json")
		if raw, err := os.ReadFile(v.path); err == nil {
			json.Unmarshal(raw, v)
		}
	}
	v.roll()
	return v
}

// roll starts a new day or month when the clock has passed into one. A saved period
// that does not match the current one, or does not look right, is dropped.
func (v *visitors) roll() {
	now := v.now().In(indiaTime)
	day, month := now.Format("2006-01-02"), now.Format("2006-01")
	if !v.Day.valid(day) {
		v.Day = newVisitorPeriod(day, v.Day, now.AddDate(0, 0, -1).Format("2006-01-02"))
		v.dirty = true
	}
	if !v.Month.valid(month) {
		firstOfMonth := time.Date(now.Year(), now.Month(), 1, 12, 0, 0, 0, indiaTime)
		v.Month = newVisitorPeriod(month, v.Month, firstOfMonth.AddDate(0, 0, -1).Format("2006-01"))
		v.dirty = true
	}
}

// add counts a client address seen from a country ("" when unknown).
func (v *visitors) add(client, country string) {
	v.roll()
	day := v.Day.hash(client)
	if v.Day.All.add(day) {
		v.dirty = true
	}
	if country != "" {
		if v.Day.Countries == nil {
			v.Day.Countries = map[string]sketch{}
		}
		perCountry := v.Day.Countries[country]
		if perCountry == nil {
			perCountry = newSketch(countryPrecision)
			v.Day.Countries[country] = perCountry
			v.dirty = true
		}
		if perCountry.add(day) {
			v.dirty = true
		}
	}
	if v.Month.All.add(v.Month.hash(client)) {
		v.dirty = true
	}
}

// encode returns what to save when anything changed since the last save. The file holds
// the keys of the running periods, so only the service may read it.
func (v *visitors) encode() []byte {
	if v.path == "" || !v.dirty {
		return nil
	}
	v.dirty = false
	encoded, _ := json.Marshal(v)
	return encoded
}
