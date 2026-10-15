package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// company is one listed company. The set is small (a few thousand), so it lives in
// memory and only the image bytes are read from the database.
type company struct {
	ID                 string   `json:"id"`
	Exchange           string   `json:"exchange"`
	Ticker             string   `json:"ticker"`
	Company            string   `json:"company"`
	Sector             string   `json:"sector"`
	Industry           string   `json:"industry"`
	OneLine            string   `json:"oneLine"`
	ISIN               string   `json:"isin"`
	Currency           string   `json:"currency"`
	MarketCap          *float64 `json:"marketCap"`
	IsIndexConstituent bool     `json:"isIndexConstituent"`
	Indexes            []string `json:"indexes"`
	Listed             bool     `json:"listed"`
	DelistedOn         *string  `json:"delistedOn"`
	Aliases            []string `json:"aliases"`
	HasLogo            bool     `json:"hasLogo"`

	marketCap float64
	search    string
}

type companyIndex struct {
	all        []*company
	byExact    map[string][]*company
	byLoose    map[string][]*company
	byISIN     map[string][]*company
	byAlias    map[string][]*company // earlier tickers, exact and punctuation-insensitive
	sectors    []string
	indexNames []string
}

var (
	errBadExchange = errors.New(`exchange must be "nse" or "bse".`)
	isinPattern    = regexp.MustCompile(`^IN[A-Z0-9]{10}$`)
	exchangePrefix = regexp.MustCompile(`^(NSE|BSE)[:_](.+)$`)
	yahooSuffix    = regexp.MustCompile(`^(.+)\.(NS|BO)$`)
	tokenSplit     = regexp.MustCompile(`[^a-z0-9&.]+`)
)

func newCompanyIndex() *companyIndex {
	return &companyIndex{byExact: map[string][]*company{}, byLoose: map[string][]*company{}, byISIN: map[string][]*company{},
		byAlias: map[string][]*company{}}
}

func loadCompanies(logos *handle) (*companyIndex, error) {
	if logos == nil {
		return newCompanyIndex(), nil
	}
	aliases := map[string][]string{}
	aliasRows, err := logos.db.Query(`SELECT id, alias FROM logo_aliases ORDER BY alias`)
	if err != nil {
		return nil, err
	}
	for aliasRows.Next() {
		var id, alias string
		if err := aliasRows.Scan(&id, &alias); err != nil {
			aliasRows.Close()
			return nil, err
		}
		aliases[id] = append(aliases[id], alias)
	}
	aliasRows.Close()
	if err := aliasRows.Err(); err != nil {
		return nil, err
	}

	rows, err := logos.db.Query(`SELECT id, exchange, ticker, company, sector, industry, one_line, isin, currency, market_cap,
  is_index_constituent, indexes, listed, delisted_on, svg IS NOT NULL FROM logos`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	index := newCompanyIndex()
	sectors, indexNames := map[string]bool{}, map[string]bool{}
	for rows.Next() {
		var c company
		var name, sector, industry, oneLine, isin, currency, delistedOn sql.NullString
		var marketCap sql.NullFloat64
		var indexes string
		if err := rows.Scan(&c.ID, &c.Exchange, &c.Ticker, &name, &sector, &industry, &oneLine, &isin, &currency, &marketCap,
			&c.IsIndexConstituent, &indexes, &c.Listed, &delistedOn, &c.HasLogo); err != nil {
			return nil, err
		}
		if delistedOn.Valid {
			c.DelistedOn = &delistedOn.String
		}
		c.Aliases = aliases[c.ID]
		if c.Aliases == nil {
			c.Aliases = []string{}
		}
		c.Company, c.Sector, c.Industry, c.OneLine, c.ISIN, c.Currency = name.String, sector.String, industry.String, oneLine.String, isin.String, currency.String
		if marketCap.Valid {
			c.MarketCap, c.marketCap = &marketCap.Float64, marketCap.Float64
		}
		if err := json.Unmarshal([]byte(indexes), &c.Indexes); err != nil {
			return nil, err
		}
		c.search = fold(strings.Join(append(append([]string{c.ID, c.Ticker, c.Company, c.Sector, c.Industry, c.ISIN}, c.Indexes...), c.Aliases...), " "))

		entry := &c
		index.all = append(index.all, entry)
		index.byExact[strings.ToUpper(c.Ticker)] = append(index.byExact[strings.ToUpper(c.Ticker)], entry)
		index.byLoose[looseKey(c.Ticker)] = append(index.byLoose[looseKey(c.Ticker)], entry)
		if c.ISIN != "" {
			index.byISIN[strings.ToUpper(c.ISIN)] = append(index.byISIN[strings.ToUpper(c.ISIN)], entry)
		}
		for _, alias := range c.Aliases {
			index.byAlias[alias] = append(index.byAlias[alias], entry)
			if loose := looseKey(alias); loose != alias {
				index.byAlias[loose] = append(index.byAlias[loose], entry)
			}
		}
		if c.Sector != "" {
			sectors[c.Sector] = true
		}
		for _, name := range c.Indexes {
			indexNames[name] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Listed first, then NSE, then the larger company, so a bare ticker resolves the way most callers expect.
	for _, group := range []map[string][]*company{index.byExact, index.byLoose, index.byISIN, index.byAlias} {
		for _, matches := range group {
			sort.SliceStable(matches, func(a, b int) bool { return preferred(matches[a], matches[b]) })
		}
	}
	index.sectors, index.indexNames = setToSorted(sectors), setToSorted(indexNames)
	return index, nil
}

func preferred(a, b *company) bool {
	if a.Listed != b.Listed {
		return a.Listed
	}
	if a.Exchange != b.Exchange {
		return a.Exchange == "NSE"
	}
	return a.marketCap > b.marketCap
}

// resolve accepts TCS, NSE:TCS, NSE_TCS, TCS.NS / TCS.BO, an ISIN, an earlier ticker of a
// renamed company, and punctuation variants such as M&M / MM or BAJAJ-AUTO / BAJAJ_AUTO.
func (index *companyIndex) resolve(symbol, exchangeHint string) (*company, error) {
	ticker, exchange := parseSymbol(symbol)
	if hint, err := parseExchange(exchangeHint); err != nil {
		return nil, err
	} else if hint != "" {
		exchange = hint
	}
	if ticker == "" {
		return nil, nil
	}

	// Exact spelling wins over an ISIN, then an earlier ticker, then the punctuation-insensitive
	// matches, so a ticker in use today always beats a company's old name for it.
	candidates := [][]*company{index.byExact[ticker]}
	if isinPattern.MatchString(ticker) {
		candidates = append(candidates, index.byISIN[ticker])
	}
	candidates = append(candidates, index.byAlias[ticker], index.byLoose[looseKey(ticker)], index.byAlias[looseKey(ticker)])
	for _, list := range candidates {
		for _, c := range list {
			if exchange == "" || c.Exchange == exchange {
				return c, nil
			}
		}
	}
	return nil, nil
}

func parseSymbol(input string) (ticker, exchange string) {
	ticker = strings.ToUpper(strings.Join(strings.Fields(input), ""))
	if match := exchangePrefix.FindStringSubmatch(ticker); match != nil {
		exchange, ticker = match[1], match[2]
	}
	if match := yahooSuffix.FindStringSubmatch(ticker); match != nil {
		ticker = match[1]
		exchange = map[string]string{"NS": "NSE", "BO": "BSE"}[match[2]]
	}
	return ticker, exchange
}

func parseExchange(value string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "", "AUTO", "ALL":
		return "", nil
	case "NSE":
		return "NSE", nil
	case "BSE":
		return "BSE", nil
	}
	return "", errBadExchange
}

type companyQuery struct {
	text, exchange, sector, index string
	delisted                      bool
	limit, offset                 int
}

func (index *companyIndex) searchCompanies(q companyQuery) (total int, results []*company) {
	tokens := tokenize(q.text)
	sector, indexName := fold(q.sector), fold(q.index)

	type scored struct {
		c     *company
		score int
	}
	var matches []scored
next:
	for _, c := range index.all {
		if !c.Listed && !q.delisted {
			continue
		}
		if q.exchange != "" && c.Exchange != q.exchange {
			continue
		}
		if sector != "" && fold(c.Sector) != sector {
			continue
		}
		if indexName != "" {
			member := false
			for _, name := range c.Indexes {
				if fold(name) == indexName {
					member = true
					break
				}
			}
			if !member {
				continue
			}
		}
		for _, token := range tokens {
			if !strings.Contains(c.search, token) {
				continue next
			}
		}
		matches = append(matches, scored{c, score(c, tokens)})
	}

	sort.SliceStable(matches, func(a, b int) bool {
		if matches[a].score != matches[b].score {
			return matches[a].score > matches[b].score
		}
		return matches[a].c.marketCap > matches[b].c.marketCap
	})
	for i := q.offset; i < len(matches) && i < q.offset+q.limit; i++ {
		results = append(results, matches[i].c)
	}
	return len(matches), results
}

func score(c *company, tokens []string) int {
	ticker, name, sector := fold(c.Ticker), fold(c.Company), fold(c.Sector)
	total := 0
	for _, token := range tokens {
		switch {
		case ticker == token:
			total += 120
		case strings.HasPrefix(ticker, token):
			total += 75
		case strings.Contains(ticker, token):
			total += 40
		}
		switch {
		case strings.HasPrefix(name, token):
			total += 45
		case strings.Contains(name, token):
			total += 22
		}
		if strings.Contains(sector, token) {
			total += 12
		}
	}
	return total
}

func looseKey(ticker string) string {
	return strings.Map(func(r rune) rune {
		r = unicode.ToUpper(r)
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, ticker)
}

// fold lowercases and strips accents so searches ignore both.
func fold(value string) string {
	decomposed := norm.NFKD.String(strings.ToLower(value))
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, decomposed)
}

func tokenize(value string) []string {
	var tokens []string
	for _, token := range tokenSplit.Split(fold(value), -1) {
		if token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func setToSorted(set map[string]bool) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

// ---- JSON views and handlers ----

type companyView struct {
	*company
	Logo  map[string]string `json:"logo"`
	Links map[string]string `json:"links"`
}

// symbolRef is the "NSE:TCS" form used in links: unambiguous and readable in a URL.
func symbolRef(c *company) string {
	// An ampersand is legal in a path, but escaping it keeps the link safe to paste anywhere.
	return strings.NewReplacer("%3A", ":", "&", "%26").Replace(url.PathEscape(c.ID))
}

func (s *Server) companyView(r *http.Request, c *company) companyView {
	base := s.origin(r) + "/v1"
	ref := symbolRef(c)
	return companyView{
		company: c,
		Logo: map[string]string{
			"svg": base + "/logos/" + ref,
			"png": base + "/logos/" + ref + "?format=png&size=128",
		},
		Links: map[string]string{"self": base + "/companies/" + ref},
	}
}

func (s *Server) handleCompany(w http.ResponseWriter, r *http.Request) {
	symbol := r.PathValue("symbol")
	info(r).subject = symbol
	c, err := s.companies.Load().resolve(symbol, r.URL.Query().Get("exchange"))
	if err != nil {
		s.badRequest(w, r, err.Error())
		return
	}
	if c == nil {
		s.notFound(w, r, `No company found for "`+symbol+`". Try /v1/companies?q=`+url.QueryEscape(symbol)+`.`)
		return
	}
	s.ok(w, r, s.companyView(r, c), meta{Source: s.source("logos")}, cacheAPI)
}

func (s *Server) handleCompanySearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
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
	exchange, err := parseExchange(query.Get("exchange"))
	if err != nil {
		s.badRequest(w, r, err.Error())
		return
	}
	var delisted bool
	switch strings.ToLower(strings.TrimSpace(query.Get("delisted"))) {
	case "", "false":
	case "true":
		delisted = true
	default:
		s.badRequest(w, r, "delisted must be true or false.")
		return
	}
	info(r).subject = query.Get("q")

	total, found := s.companies.Load().searchCompanies(companyQuery{
		text: query.Get("q"), exchange: exchange, sector: query.Get("sector"), index: query.Get("index"),
		delisted: delisted, limit: limit, offset: offset,
	})
	views := make([]companyView, len(found))
	for i, c := range found {
		views[i] = s.companyView(r, c)
	}
	m := paging(total, limit, offset)
	m.Source = s.source("logos")
	s.ok(w, r, views, m, cacheAPI)
}
