package api

import (
	"strconv"
	"sync/atomic"
)

// extras holds state that belongs to single APIs rather than to the server as a whole:
// things loaded on first use and dropped when a dataset changes.
type extras struct {
	ip atomic.Pointer[ipReaders]
	// dns answers DNS queries; nil means the real network. Tests set it.
	dns dnsExchanger
}

// datasetChanged drops whatever was derived from a dataset that was just replaced.
func (e *extras) datasetChanged(name string) {}

func (e *extras) close() {
	if readers := e.ip.Swap(nil); readers != nil {
		readers.close()
	}
}

// registerMore registers the routes of the APIs added after the first release. Each
// lives in its own file.
func (s *Server) registerMore() {
	s.handle("pincode_near", costLight, "GET /v1/pincode/near", s.handlePincodeNear)
	s.handle("working_day", costLight, "GET /v1/working-days/{date}", s.handleWorkingDay)
	s.handle("working_days", costLight, "GET /v1/working-days", s.handleWorkingDays)
	s.handle("market_status", costLight, "GET /v1/market/status", s.handleMarketStatus)
	s.handle("qr", costLight, "GET /v1/qr", s.handleQR)
	s.handle("qr", costLight, "POST /v1/qr", s.handleQR)
	s.handle("upi_qr", costLight, "GET /v1/upi-qr", s.handleUPIQR)
	s.handle("barcode", costLight, "GET /v1/barcode/{type}", s.handleBarcode)
	s.handle("barcode", costLight, "POST /v1/barcode/{type}", s.handleBarcode)
	s.handle("thirukkural", costLight, "GET /v1/thirukkural/{number}", s.handleKural)
	s.handle("thirukkural", costLight, "GET /v1/thirukkural/today", s.handleKuralOfTheDay)
	s.handle("thirukkural", costLight, "GET /v1/thirukkural/random", s.handleKuralRandom)
	s.handle("thirukkural_search", costLight, "GET /v1/thirukkural", s.handleKuralSearch)
	s.handle("thirukkural", costLight, "GET /v1/thirukkural/chapters", s.handleKuralChapters)
	s.handle("thirukkural", costLight, "GET /v1/thirukkural/chapters/{number}", s.handleKuralChapter)
	s.handle("airport", costLight, "GET /v1/airports/{code}", s.handleAirport)
	s.handle("airport_search", costLight, "GET /v1/airports", s.handleAirportSearch)
	s.handle("ip", costLight, "GET /v1/ip/{address}", s.handleIP)
	s.handle("ip", costLight, "GET /v1/ip", s.handleMyIP)
	s.handle("email", costHeavy, "GET /v1/email/{address}", s.handleEmail)
	s.handle("dns", costHeavy, "GET /v1/dns/{name}", s.handleDNS)
}

func itoa(value int) string { return strconv.Itoa(value) }
