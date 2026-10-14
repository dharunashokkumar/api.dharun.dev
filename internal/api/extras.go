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
	render *renderer
	// programs finds the outside programs the upload tools run.
	programs *programs
	stop     chan struct{}
	// dns answers DNS queries; nil means the real network. Tests set it.
	dns dnsExchanger
	// trace runs a traceroute; nil means the real one. Tests set it.
	trace tracer
	// traces holds a slot for each traceroute running.
	traces chan struct{}
}

// datasetChanged drops whatever was derived from a dataset that was just replaced.
func (e *extras) datasetChanged(name string) {}

// start sets up what the extra APIs need and the housekeeping they do in the background.
func (e *extras) start(cfg Config) {
	e.tokens = textkit.NewTokenizer()
	e.render = newRenderer(cfg)
	e.programs = newPrograms(cfg.Programs)
	e.traces = make(chan struct{}, mtrConcurrency)
	e.stop = make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// A tokenizer vocabulary is 4 to 8 MB and takes a moment to load, so one
				// that is in use stays; one nobody has asked for in half a day goes.
				e.tokens.DropIdle(12 * time.Hour)
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
	s.handle("sun_moon", costLight, "GET /v1/sun-moon/{date}", s.handleSunMoon)
	s.handle("propagation", costLight, "GET /v1/propagation", s.handlePropagation)
	s.handle("satellites", costLight, "GET /v1/satellites", s.handleSatellites)
	s.handle("satellite", costLight, "GET /v1/satellites/{id}", s.handleSatellite)
	s.handle("sat_pass", costHeavy, "GET /v1/sat-pass", s.handleSatellitePasses)
	s.handle("mtr", costProbe, "GET /v1/mtr/{host}", s.handleMTR)
	s.handle("constituency", costLight, "GET /v1/constituency", s.handleConstituency)
	s.handle("phone", costLight, "GET /v1/phone/{number}", s.handlePhone)
	s.handle("fake", costLight, "GET /v1/fake/{kind}", s.handleFake)
	s.handle("tamil_font", costFree, "GET /v1/tamil-font/encodings", s.handleTamilEncodings)
	s.handle("tamil_font", costLight, "GET /v1/tamil-font/convert", s.handleTamilConvert)
	s.handle("tamil_font", costLight, "POST /v1/tamil-font/convert", s.handleTamilConvert)
	s.handle("og_image", costHeavy, "GET /v1/og-image", s.handleOGImage)
	s.handle("code_image", costHeavy, "GET /v1/code-image", s.handleCodeImage)
	s.handle("code_image", costHeavy, "POST /v1/code-image", s.handleCodeImage)
	s.registerFileTools()
	for _, tool := range []struct {
		route, path string
		handler     http.HandlerFunc
	}{
		{"text_chunk", "/v1/text/chunk", s.handleChunk},
		{"text_redact", "/v1/text/redact", s.handleRedact},
		{"text_tokens", "/v1/text/tokens", s.handleTokens},
		{"text_language", "/v1/text/language", s.handleLanguage},
		{"regex", "/v1/regex", s.handleRegex},
	} {
		s.handle(tool.route, costHeavy, "GET "+tool.path, tool.handler)
		s.handle(tool.route, costHeavy, "POST "+tool.path, tool.handler)
	}
}

func itoa(value int) string { return strconv.Itoa(value) }

// registerFileTools registers the endpoints that take files (round 2). Their work waits
// for a CPU slot, or for a model slot when it runs a neural network.
func (s *Server) registerFileTools() {
	cpu := uploadRoute{}
	model := uploadRoute{pool: s.models, wait: modelWait}
	for _, tool := range []struct {
		route, path string
		cost        int
		spec        uploadRoute
		handler     func(http.ResponseWriter, *http.Request, uploadInput)
	}{
		{"ocr", "/v1/ocr", costUpload, cpu, s.handleOCR},
		{"barcode_read", "/v1/barcode/read", costUpload, cpu, s.handleBarcodeRead},
		{"image_info", "/v1/image/info", costUpload, cpu, s.handleImageInfo},
		{"image_convert", "/v1/image/convert", costUpload, cpu, s.handleImageConvert},
		{"image_hash", "/v1/image/hash", costUpload, cpu, s.handlePictureHash},
		{"image_compare", "/v1/image/compare", costUpload, uploadRoute{maxFiles: 2, limit: 2 * uploadLimit}, s.handlePictureCompare},
		{"remove_background", "/v1/image/remove-background", costModel, model, s.handleRemoveBackground},
		{"pdf_info", "/v1/pdf/info", costUpload, cpu, s.handlePDFInfo},
		{"pdf_text", "/v1/pdf/text", costUpload, cpu, s.handlePDFText},
		{"pdf_merge", "/v1/pdf/merge", costUpload, uploadRoute{maxFiles: pdfMaxFiles, limit: mergeLimit}, s.handlePDFMerge},
		{"pdf_pages", "/v1/pdf/pages", costUpload, cpu, s.handlePDFPages},
		{"pdf_rotate", "/v1/pdf/rotate", costUpload, cpu, s.handlePDFRotate},
		{"pdf_compress", "/v1/pdf/compress", costUpload, cpu, s.handlePDFCompress},
		{"pdf_render", "/v1/pdf/render", costUpload, cpu, s.handlePDFRender},
		{"denoise", "/v1/audio/denoise", costModel, model, s.handleDenoise},
	} {
		s.handleUpload(tool.route, tool.cost, "POST "+tool.path, tool.spec, tool.handler)
	}
	s.handle("invoice", costHeavy, "POST /v1/invoice", s.handleInvoice)
	s.handle("typst", costHeavy, "GET /v1/typst", s.handleTypst)
	s.handle("typst", costHeavy, "POST /v1/typst", s.handleTypst)
	s.handle("tts", costModel, "GET /v1/tts", s.handleTTS)
	s.handle("tts", costModel, "POST /v1/tts", s.handleTTS)
	s.handle("tts_voices", costLight, "GET /v1/tts/voices", s.handleVoices)
	s.handle("samples", costFree, "GET /samples/{file}", s.handleSampleFile)
}
