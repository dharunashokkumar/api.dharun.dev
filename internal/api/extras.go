package api

import (
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/textkit"
)

// extras holds state that belongs to single APIs rather than to the server as a whole:
// things loaded on first use and dropped when a dataset changes.
type extras struct {
	ip     atomic.Pointer[ipReaders]
	tokens *textkit.Tokenizer
	stop   chan struct{}
	// dns answers DNS queries; nil means the real network. Tests set it.
	dns dnsExchanger
}

// datasetChanged drops whatever was derived from a dataset that was just replaced.
func (e *extras) datasetChanged(name string) {}

// start sets up what the extra APIs need and the housekeeping they do in the background.
func (e *extras) start() {
	e.tokens = textkit.NewTokenizer()
	e.stop = make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// A tokenizer vocabulary is 4 to 8 MB; keep it only while it is being used.
				e.tokens.DropIdle(10 * time.Minute)
			case <-e.stop:
				return
			}
		}
	}()
}

func (e *extras) close() {
	if e.stop != nil {
		close(e.stop)
	}
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
	s.handle("transliterate", costLight, "GET /v1/transliterate", s.handleTransliterate)
	s.handle("transliterate", costLight, "POST /v1/transliterate", s.handleTransliterate)
	s.handle("mutual_fund", costLight, "GET /v1/mutual-funds/{code}", s.handleScheme)
	s.handle("mutual_fund_search", costLight, "GET /v1/mutual-funds", s.handleSchemeSearch)
	s.handle("mutual_fund", costLight, "GET /v1/mutual-funds/fund-houses", s.handleFundHouses)
	s.handle("hsn", costLight, "GET /v1/hsn/{code}", s.handleHSN)
	s.handle("hsn_search", costLight, "GET /v1/hsn", s.handleHSNSearch)
	s.handle("station", costLight, "GET /v1/stations/{code}", s.handleStation)
	s.handle("station_search", costLight, "GET /v1/stations", s.handleStationSearch)
	s.handle("station", costLight, "GET /v1/stations/zones", s.handleStationZones)
	s.handle("panchang", costHeavy, "GET /v1/panchang/{date}", s.handlePanchang)
	s.handle("constituency", costLight, "GET /v1/constituency", s.handleConstituency)
	for _, tool := range []struct {
		route, path string
		handler     http.HandlerFunc
	}{
		{"text_chunk", "/v1/text/chunk", s.handleChunk},
		{"text_redact", "/v1/text/redact", s.handleRedact},
		{"text_tokens", "/v1/text/tokens", s.handleTokens},
		{"text_language", "/v1/text/language", s.handleLanguage},
	} {
		s.handle(tool.route, costHeavy, "GET "+tool.path, tool.handler)
		s.handle(tool.route, costHeavy, "POST "+tool.path, tool.handler)
	}
}

func itoa(value int) string { return strconv.Itoa(value) }
