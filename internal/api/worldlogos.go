package api

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"net/http"
	"net/url"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

// worldLogoSet is one of the logo sets beyond the Indian listing (plan Part 1): its
// dataset, its address, and what a caller may filter it by.
type worldLogoSet struct {
	name    string // the X-Logo-Set value
	dataset string
	path    string // /v1/<path>/{<param>}
	param   string
	filter  string // the query parameter that narrows a lookup: exchange, kind or category
	values  []string
}

var (
	stockExchangeList = []string{"NASDAQ", "NYSE", "AMEX", "LSE", "TSE", "HKEX", "XETR", "FWB", "EURONEXT", "TSX", "ASX", "SIX"}
	stockExchanges    = func() map[string]bool {
		set := map[string]bool{}
		for _, name := range stockExchangeList {
			set[name] = true
		}
		return set
	}()
	assetKinds     = []string{"crypto", "index", "etf", "commodity"}
	brandCategorys = []string{"payments", "fintech", "insurance", "mutual-fund", "commerce", "health", "mobility",
		"travel", "food", "education", "media", "gaming", "telecom", "tech", "government"}

	brandLogoSet = worldLogoSet{name: "brand", dataset: "brand-logos", path: "brand-logos", param: "name", filter: "category", values: brandCategorys}
	stockLogoSet = worldLogoSet{name: "stock", dataset: "stock-logos", path: "stock-logos", param: "symbol", filter: "exchange", values: stockExchangeList}
	assetLogoSet = worldLogoSet{name: "asset", dataset: "asset-logos", path: "asset-logos", param: "symbol", filter: "kind", values: assetKinds}
	// The order /v1/logos looks in after the Indian listing (Dharun, 2026-11-11).
	worldLogoSets = []worldLogoSet{brandLogoSet, stockLogoSet, assetLogoSet}
)

// worldLogo is one entry of a set, as stored.
type worldLogo struct {
	set        worldLogoSet
	data       *handle
	ID         string   `json:"id"`
	Symbol     string   `json:"symbol"`
	Exchange   string   `json:"exchange,omitempty"`
	Kind       string   `json:"kind,omitempty"`
	Category   string   `json:"category,omitempty"`
	Name       string   `json:"name,omitempty"`
	Domain     string   `json:"domain,omitempty"`
	Country    string   `json:"country,omitempty"`
	Sector     string   `json:"sector,omitempty"`
	Industry   string   `json:"industry,omitempty"`
	ISIN       string   `json:"isin,omitempty"`
	Currency   string   `json:"currency,omitempty"`
	MarketCap  *float64 `json:"marketCap,omitempty"`
	AUM        *float64 `json:"assetsUnderManagement,omitempty"`
	Aliases    []string `json:"aliases"`
	HasLogo    bool     `json:"hasLogo"`
	hasSVG     bool
	hasPNG     bool
	artwork    sql.NullString
	artworkURL sql.NullString
	licence    sql.NullString
	author     sql.NullString
}

const worldLogoColumns = `e.id, e.symbol, coalesce(e.exchange, ''), e.kind, coalesce(e.name, ''), coalesce(e.domain, ''),
  coalesce(e.country, ''), coalesce(e.sector, ''), coalesce(e.industry, ''), coalesce(e.isin, ''), coalesce(e.currency, ''),
  e.market_cap, e.aliases, e.artwork, e.artwork_url, e.licence, e.author, e.svg IS NOT NULL, e.png IS NOT NULL`

func scanWorldLogo(set worldLogoSet, data *handle, scan func(...any) error) (*worldLogo, error) {
	entry := &worldLogo{set: set, data: data}
	var marketCap sql.NullFloat64
	var aliases string
	if err := scan(&entry.ID, &entry.Symbol, &entry.Exchange, &entry.Kind, &entry.Name, &entry.Domain, &entry.Country,
		&entry.Sector, &entry.Industry, &entry.ISIN, &entry.Currency, &marketCap, &aliases, &entry.artwork,
		&entry.artworkURL, &entry.licence, &entry.author, &entry.hasSVG, &entry.hasPNG); err != nil {
		return nil, err
	}
	if marketCap.Valid {
		value := marketCap.Float64
		if entry.Kind == "etf" {
			entry.AUM = &value
		} else if entry.Kind != "crypto" || value > 0 {
			entry.MarketCap = &value
		}
	}
	if set.name == "brand" {
		entry.Category, entry.Kind = entry.Kind, ""
	}
	json.Unmarshal([]byte(aliases), &entry.Aliases)
	if entry.Aliases == nil {
		entry.Aliases = []string{}
	}
	entry.HasLogo = entry.hasSVG || entry.hasPNG
	return entry, nil
}

// lookup finds the entry a name means in one set. exact limits it to the symbol and
// other exact names; otherwise only the loose spellings and names are tried. filter is
// an exchange, kind or category, or "".
func (set worldLogoSet) lookup(data *handle, name, filter string, exact bool) (*worldLogo, error) {
	keys := []any{dataset.LogoKey(name), dataset.LooseLogoKey(name)}
	ranks := "k.rank <= 1"
	if !exact {
		ranks = "k.rank >= 2"
	}
	column := map[string]string{"exchange": "e.exchange", "kind": "e.kind", "category": "e.kind"}[set.filter]
	row := data.row(`SELECT `+worldLogoColumns+` FROM entry_keys k JOIN entries e ON e.id = k.id
  WHERE k.key IN (?, ?) AND `+ranks+` AND (? = '' OR `+column+` = ?)
  ORDER BY k.rank, k.key = ? DESC, coalesce(e.market_cap, -1) DESC, e.id LIMIT 1`,
		append(keys, filter, filter, keys[0])...)
	entry, err := scanWorldLogo(set, data, row.Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return entry, err
}

// findWorldLogo looks through sets in order, every set's exact names before any loose
// match, so a ticker in a later set beats a near spelling in an earlier one. searched
// says whether any of the sets is held by this server.
func (s *Server) findWorldLogo(name string, sets []worldLogoSet, filter string) (*worldLogo, bool) {
	searched := false
	for _, exact := range []bool{true, false} {
		for _, set := range sets {
			data := s.data(set.dataset)
			if data == nil {
				continue
			}
			searched = true
			setFilter := ""
			if set.filter == "exchange" {
				setFilter = filter
			}
			found, err := set.lookup(data, name, setFilter, exact)
			if err != nil {
				logError("logo lookup in "+set.dataset, err)
				continue
			}
			if found != nil {
				return found, true
			}
		}
	}
	return nil, searched
}

// serveWorldLogo answers an entry's image: its SVG, its PNG master scaled, an SVG holding
// the PNG for a brand known only by its site icon, or the tile when it has no artwork.
func (s *Server) serveWorldLogo(w http.ResponseWriter, r *http.Request, req logoRequest, entry *worldLogo) {
	w.Header().Set("X-Logo-Set", entry.set.name)
	if !entry.HasLogo {
		s.serveMonogram(w, r, entry.Symbol, req.format, req.size, "fallback", cacheShort)
		return
	}
	s.metrics.observeLogo(req.format, "found")
	if req.format == "png" {
		s.servePNGMaster(w, r, entry.data, entry.ID, `SELECT png FROM entries WHERE id = ?`, req.size)
		return
	}
	var svg, master []byte
	if err := entry.data.row(`SELECT svg, png FROM entries WHERE id = ?`, entry.ID).Scan(&svg, &master); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Could not read the logo.")
		return
	}
	if svg == nil {
		svg = pngInSVG(master)
	}
	w.Header().Set("Content-Security-Policy", svgPolicy)
	writeBody(w, r, http.StatusOK, svg, typeSVG, cacheLong, false)
}

// pngInSVG wraps a picture that exists only as PNG, so format=svg still answers an image.
func pngInSVG(master []byte) []byte {
	width, height := 256, 256
	if config, err := png.DecodeConfig(bytes.NewReader(master)); err == nil {
		width, height = config.Width, config.Height
	}
	return []byte(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d"><image width="%d" height="%d" href="data:image/png;base64,%s"/></svg>`,
		width, height, width, height, base64.StdEncoding.EncodeToString(master)))
}

// handleWorldLogo serves /v1/brand-logos/{name}, /v1/stock-logos/{symbol} and
// /v1/asset-logos/{symbol}: the same options and never-fail rule as /v1/logos.
func (s *Server) handleWorldLogo(set worldLogoSet) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, ok := s.readLogoRequest(w, r, set.param)
		if !ok {
			return
		}
		filter, ok := s.readLogoFilter(w, r, set)
		if !ok {
			return
		}
		data := s.data(set.dataset)
		var found *worldLogo
		if data != nil {
			for _, exact := range []bool{true, false} {
				entry, err := set.lookup(data, req.symbol, filter, exact)
				if err != nil {
					s.fail(w, r, http.StatusInternalServerError, "internal_error", "Could not read the logo.")
					return
				}
				if entry != nil {
					found = entry
					break
				}
			}
		}
		if found == nil {
			s.serveUnknownLogo(w, r, req, data != nil)
			return
		}
		s.serveWorldLogo(w, r, req, found)
	}
}

func (s *Server) readLogoFilter(w http.ResponseWriter, r *http.Request, set worldLogoSet) (string, bool) {
	value := strings.TrimSpace(r.URL.Query().Get(set.filter))
	if set.filter == "exchange" {
		value = strings.ToUpper(value)
	} else {
		value = strings.ToLower(value)
	}
	if value == "" {
		return "", true
	}
	for _, allowed := range set.values {
		if strings.EqualFold(allowed, value) {
			return value, true
		}
	}
	s.badRequest(w, r, set.filter+" is one of "+strings.Join(set.values, ", ")+".")
	return "", false
}

// worldLogoView is an entry with its addresses and, where it is not a trademark file from
// the usual source, where the artwork came from and under what terms.
type worldLogoView struct {
	*worldLogo
	Logo    map[string]string `json:"logo"`
	Artwork *artworkView      `json:"artwork,omitempty"`
}

type artworkView struct {
	Origin  string `json:"origin"`
	URL     string `json:"url,omitempty"`
	Licence string `json:"licence,omitempty"`
	Author  string `json:"author,omitempty"`
}

func (s *Server) worldLogoView(r *http.Request, entry *worldLogo) worldLogoView {
	address := s.origin(r) + "/v1/" + entry.set.path + "/" + url.PathEscape(entry.Symbol)
	if entry.set.name == "stock" && entry.Exchange != "" {
		address += "?exchange=" + strings.ToLower(entry.Exchange)
	} else if entry.set.name == "asset" {
		address += "?kind=" + entry.Kind
	}
	join := "?"
	if strings.Contains(address, "?") {
		join = "&"
	}
	view := worldLogoView{worldLogo: entry, Logo: map[string]string{"svg": address, "png": address + join + "format=png&size=128"}}
	if entry.artwork.Valid && entry.artwork.String != "tradingview" {
		view.Artwork = &artworkView{Origin: entry.artwork.String, URL: entry.artworkURL.String,
			Licence: entry.licence.String, Author: entry.author.String}
	}
	return view
}

// handleWorldLogoSearch lists a set, or searches it by the start of a name or symbol.
func (s *Server) handleWorldLogoSearch(set worldLogoSet) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, err := intParam(r, "limit", 20, 1, 100)
		if err != nil {
			s.badRequest(w, r, err.Error())
			return
		}
		offset, err := intParam(r, "offset", 0, 0, 100000)
		if err != nil {
			s.badRequest(w, r, err.Error())
			return
		}
		filter, ok := s.readLogoFilter(w, r, set)
		if !ok {
			return
		}
		text := r.URL.Query().Get("q")
		info(r).subject = text
		data := s.need(w, r, set.dataset)
		if data == nil {
			return
		}
		column := map[string]string{"exchange": "e.exchange", "kind": "e.kind", "category": "e.kind"}[set.filter]
		where, args := `(? = '' OR `+column+` = ?)`, []any{filter, filter}
		from := `entries e`
		order := `coalesce(e.market_cap, -1) DESC, e.symbol`
		if prefix := dataset.LooseLogoKey(text); prefix != "" {
			from = `(SELECT id, min(rank) AS rank FROM entry_keys WHERE key >= ? AND key < ? GROUP BY id) k JOIN entries e ON e.id = k.id`
			args = append([]any{prefix, prefix + "\U0010FFFF"}, args...)
			order = `k.rank, ` + order
		} else if strings.TrimSpace(text) != "" {
			s.badRequest(w, r, "q needs at least one letter or digit.")
			return
		}
		var total int
		if err := data.row(`SELECT count(*) FROM `+from+` WHERE `+where, args...).Scan(&total); err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Could not search the logos.")
			return
		}
		rows, err := data.rows(`SELECT `+worldLogoColumns+` FROM `+from+` WHERE `+where+` ORDER BY `+order+` LIMIT ? OFFSET ?`,
			append(args, limit, offset)...)
		if err != nil {
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "Could not search the logos.")
			return
		}
		defer rows.Close()
		views := []worldLogoView{}
		for rows.Next() {
			entry, err := scanWorldLogo(set, data, rows.Scan)
			if err != nil {
				s.fail(w, r, http.StatusInternalServerError, "internal_error", "Could not search the logos.")
				return
			}
			views = append(views, s.worldLogoView(r, entry))
		}
		m := paging(total, limit, offset)
		m.Source = s.source(set.dataset)
		s.ok(w, r, views, m, cacheAPI)
	}
}

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name: "find_logo",
		Description: "Find logos beyond India's listed companies: unlisted Indian brands (PhonePe, Zepto, UPI, DigiLocker, mutual fund houses, insurers), " +
			"global stocks (US, London, Tokyo, Hong Kong, Frankfurt, Euronext, Toronto, ASX, SIX) or assets (crypto, indices such as NIFTY50 and SENSEX, ETFs, commodities). " +
			"Returns matches with SVG and PNG logo URLs. For NSE and BSE companies use get_company_logo.",
		InputSchema: schema([]string{"query", "set"}, map[string]any{
			"query": text("Name, ticker, symbol or start of one."),
			"set":   oneOf("brand: unlisted Indian brands; stock: global stocks; asset: crypto, indices, ETFs, commodities.", "brand", "stock", "asset"),
			"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 25, "default": 5},
		}),
		path: func(args map[string]any) (string, error) {
			_, err := required(args, "query")
			if err != nil {
				return "", err
			}
			set, err := required(args, "set")
			if err != nil {
				return "", err
			}
			if set != "brand" && set != "stock" && set != "asset" {
				return "", fmt.Errorf("set is brand, stock or asset")
			}
			if arg(args, "limit") == "" {
				args["limit"] = "5"
			}
			return "/v1/" + set + "-logos" + query(args, map[string]string{"query": "q", "limit": "limit"}), nil
		},
	})
}

func lower(values []string) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = strings.ToLower(value)
	}
	return out
}
