package dataset

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The logo sets beyond the Indian listing (plan Part 1, Dharun 2026-11-11): unlisted
// Indian brands, global stocks, and crypto, indices, ETFs and commodities. Each is its
// own seed dataset with the same two tables, built from data/<name>/metadata.json, so
// that every answer credits its own source.
const worldLogoDDL = `
CREATE TABLE entries (
  id TEXT PRIMARY KEY, symbol TEXT NOT NULL, exchange TEXT, kind TEXT NOT NULL, name TEXT,
  domain TEXT, country TEXT, sector TEXT, industry TEXT, isin TEXT, currency TEXT, market_cap REAL,
  aliases TEXT NOT NULL DEFAULT '[]', artwork TEXT, artwork_url TEXT, licence TEXT, author TEXT,
  svg BLOB, png BLOB
) WITHOUT ROWID;
-- Every name an entry is asked for by, normalised with LogoKey or LooseLogoKey. rank orders
-- the matches: 0 the symbol itself, 1 another exact name, 2 a loose spelling, 3 the name.
CREATE TABLE entry_keys (
  key TEXT NOT NULL, rank INTEGER NOT NULL, id TEXT NOT NULL REFERENCES entries(id),
  PRIMARY KEY (key, id)
) WITHOUT ROWID;
CREATE INDEX entries_kind ON entries (kind, market_cap);`

func init() {
	for _, name := range []string{"brand-logos", "stock-logos", "asset-logos"} {
		register(&Spec{Name: name, Schema: 1, DDL: worldLogoDDL, Build: buildWorldLogos(name), Main: "entries", MaxShrink: 0.2})
	}
}

// Yahoo Finance suffixes of the stock exchanges in the set, so VOD.L and 0700.HK resolve.
var yahooSuffixes = map[string][]string{
	"LSE": {"L"}, "TSE": {"T"}, "HKEX": {"HK"}, "XETR": {"DE"}, "FWB": {"F"},
	"EURONEXT": {"PA", "AS", "BR", "LS"}, "TSX": {"TO"}, "ASX": {"AX"}, "SIX": {"SW"},
}

var (
	looseKeyDrop   = regexp.MustCompile(`[^A-Z0-9]+`)
	companySuffix  = regexp.MustCompile(`(?i)[\s,.]+(inc|incorporated|corp|corporation|co|company|ltd|limited|plc|holdings?|group|sa|ag|nv|se|asa|ab|spa|class [a-z]|common stock|ordinary shares|adr|ads|units?)\.?$`)
	logoKeyExtra   = regexp.MustCompile(`\s+`)
	numericHKTiker = regexp.MustCompile(`^\d{1,4}$`)
)

// LogoKey is the exact form of a name an entry is looked up by: upper case, no spaces.
func LogoKey(name string) string {
	return logoKeyExtra.ReplaceAllString(strings.ToUpper(strings.TrimSpace(name)), "")
}

// LooseLogoKey keeps letters and digits only, so PHONE-PE, Phone Pe and phonepe meet.
func LooseLogoKey(name string) string {
	return looseKeyDrop.ReplaceAllString(strings.ToUpper(name), "")
}

// shortCompanyName drops the legal suffixes, so "Apple Inc." is found as APPLE.
func shortCompanyName(name string) string {
	for {
		short := companySuffix.ReplaceAllString(name, "")
		if short == name {
			return strings.TrimSpace(short)
		}
		name = short
	}
}

type worldLogoMetadata struct {
	Logos []struct {
		ID         string   `json:"id"`
		Symbol     string   `json:"symbol"`
		Slug       string   `json:"slug"`
		Exchange   string   `json:"exchange"`
		Kind       string   `json:"kind"`
		Category   string   `json:"category"`
		Name       string   `json:"name"`
		Domain     string   `json:"domain"`
		Country    string   `json:"country"`
		Sector     string   `json:"sector"`
		Industry   string   `json:"industry"`
		ISIN       string   `json:"isin"`
		Currency   string   `json:"currency"`
		MarketCap  *float64 `json:"market_cap"`
		Aliases    []string `json:"aliases"`
		File       string   `json:"file"`
		Artwork    string   `json:"artwork"`
		ArtworkURL string   `json:"artwork_url"`
		Licence    string   `json:"licence"`
		Author     string   `json:"author"`
	} `json:"logos"`
}

func buildWorldLogos(name string) func(b *Builder) error {
	return func(b *Builder) error {
		raw, err := os.ReadFile(b.Path(name, "metadata.json"))
		if err != nil {
			return err
		}
		var metadata worldLogoMetadata
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return err
		}

		// SVG files are checked and rendered once each, however many entries share them.
		svgByFile := map[string][]byte{}
		pngByFile := map[string][]byte{}
		var files []string
		for _, logo := range metadata.Logos {
			if logo.File == "" || svgByFile[logo.File] != nil || pngByFile[logo.File] != nil {
				continue
			}
			content, err := os.ReadFile(b.Path(name, filepath.FromSlash(logo.File)))
			if err != nil {
				return err
			}
			if strings.HasSuffix(logo.File, ".png") {
				pngByFile[logo.File] = content
				continue
			}
			if svgByFile[logo.File], err = prepareSVG(content); err != nil {
				return fmt.Errorf("%s: %w", logo.File, err)
			}
			files = append(files, logo.File)
		}
		if b.Resvg != "" && len(files) > 0 {
			svgs, pngs := make([][]byte, len(files)), make([][]byte, len(files))
			for i, file := range files {
				svgs[i] = svgByFile[file]
			}
			b.Logf("rendering %d PNG masters for %s with %s", len(files), name, b.Resvg)
			if err := renderPNGs(b.Resvg, svgs, pngs); err != nil {
				return err
			}
			for i, file := range files {
				pngByFile[file] = pngs[i]
			}
		}

		insert, err := b.Tx.Prepare(`INSERT INTO entries (id, symbol, exchange, kind, name, domain, country, sector,
  industry, isin, currency, market_cap, aliases, artwork, artwork_url, licence, author, svg, png)
  VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer insert.Close()
		key, err := b.Tx.Prepare(`INSERT INTO entry_keys (key, rank, id) VALUES (?,?,?)
  ON CONFLICT (key, id) DO UPDATE SET rank = min(rank, excluded.rank)`)
		if err != nil {
			return err
		}
		defer key.Close()

		for _, logo := range metadata.Logos {
			id, symbol, kind := logo.ID, logo.Symbol, logo.Kind
			if name == "brand-logos" {
				id, symbol, kind = "BRAND:"+logo.Slug, logo.Slug, logo.Category
			}
			if id == "" || symbol == "" || kind == "" {
				return fmt.Errorf("%s: an entry without id, symbol or kind: %+v", name, logo)
			}
			aliases := logo.Aliases
			if aliases == nil {
				aliases = []string{}
			}
			encoded, _ := json.Marshal(aliases)
			var svg, png any
			if content := svgByFile[logo.File]; content != nil {
				svg = content
			}
			if content := pngByFile[logo.File]; content != nil {
				png = content
				b.Add("png", 1)
			}
			if _, err := insert.Exec(id, symbol, nullable(logo.Exchange), kind, nullable(logo.Name), nullable(logo.Domain),
				nullable(logo.Country), nullable(logo.Sector), nullable(logo.Industry), nullable(logo.ISIN),
				nullable(logo.Currency), logo.MarketCap, string(encoded), nullable(logo.Artwork),
				nullable(logo.ArtworkURL), nullable(logo.Licence), nullable(logo.Author), svg, png); err != nil {
				return fmt.Errorf("%s: %w", id, err)
			}
			if logo.File != "" {
				b.Add("with_logo", 1)
			}
			b.Add("entries", 1)

			keys := map[string]int{}
			add := func(text string, rank int) {
				if k := LogoKey(text); k != "" {
					if old, ok := keys[k]; !ok || rank < old {
						keys[k] = rank
					}
				}
			}
			addLoose := func(text string, rank int) {
				if k := LooseLogoKey(text); len(k) >= 2 {
					if old, ok := keys[k]; !ok || rank < old {
						keys[k] = rank
					}
				}
			}
			add(symbol, 0)
			add(id, 0)
			addLoose(symbol, 2)
			for _, alias := range aliases {
				add(alias, 1)
				addLoose(alias, 2)
			}
			if logo.ISIN != "" {
				add(logo.ISIN, 0)
			}
			switch name {
			case "stock-logos":
				add(logo.Exchange+":"+symbol, 0)
				for _, suffix := range yahooSuffixes[logo.Exchange] {
					yahoo := symbol
					if logo.Exchange == "HKEX" && numericHKTiker.MatchString(symbol) {
						yahoo = fmt.Sprintf("%04s", symbol)
					}
					add(yahoo+"."+suffix, 1)
				}
				addLoose(shortCompanyName(logo.Name), 3)
			case "asset-logos":
				if kind == "crypto" {
					for _, quote := range []string{"USD", "USDT", "INR", "-USD", "-INR"} {
						add(symbol+quote, 1)
					}
				}
				if kind == "etf" && logo.Exchange != "" {
					add(logo.Exchange+":"+symbol, 0)
				}
				addLoose(logo.Name, 3)
			case "brand-logos":
				addLoose(logo.Name, 1)
				if logo.Domain != "" {
					add(strings.TrimPrefix(strings.ToLower(logo.Domain), "www."), 1)
				}
			}
			for k, rank := range keys {
				if _, err := key.Exec(k, rank, id); err != nil {
					return fmt.Errorf("%s: key %s: %w", id, k, err)
				}
				b.Add("keys", 1)
			}
		}
		return nil
	}
}
