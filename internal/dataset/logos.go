package dataset

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// PNGMasterSize is the edge length of the pre-rendered PNG stored for each logo.
// Requests for smaller sizes are scaled down from it.
const PNGMasterSize = 512

func init() {
	register(&Spec{
		Name:   "logos",
		Schema: 1,
		DDL: `
CREATE TABLE logos (
  id TEXT PRIMARY KEY, exchange TEXT NOT NULL, ticker TEXT NOT NULL, company TEXT, sector TEXT,
  industry TEXT, one_line TEXT, isin TEXT, currency TEXT, market_cap REAL,
  is_index_constituent INTEGER NOT NULL DEFAULT 0, indexes TEXT NOT NULL DEFAULT '[]',
  svg BLOB NOT NULL, png BLOB
) WITHOUT ROWID;`,
		Build: buildLogos,
	})
}

// Anything executable or remote inside an SVG would be served to browsers, so it fails the build.
var unsafeSVG = []struct {
	pattern *regexp.Regexp
	label   string
}{
	{regexp.MustCompile(`(?i)<script`), "script element"},
	{regexp.MustCompile(`(?i)\son[a-z]+\s*=`), "event handler attribute"},
	{regexp.MustCompile(`(?i)javascript:`), "javascript: URL"},
	{regexp.MustCompile(`(?i)<(foreignObject|iframe|object|embed)\b`), "embedded document"},
	{regexp.MustCompile(`(?i)(href|src)\s*=\s*["']\s*(https?:)?//`), "external reference"},
}

var (
	svgOpenTag = regexp.MustCompile(`<svg\b[^>]*>`)
	svgViewBox = regexp.MustCompile(`\sviewBox\s*=`)
	svgWidth   = regexp.MustCompile(`\swidth="(\d+(?:\.\d+)?)(?:px)?"`)
	svgHeight  = regexp.MustCompile(`\sheight="(\d+(?:\.\d+)?)(?:px)?"`)
)

type logoMetadata struct {
	Logos []struct {
		ID                 string   `json:"id"`
		Exchange           string   `json:"exchange"`
		Ticker             string   `json:"ticker"`
		File               string   `json:"file"`
		Company            string   `json:"company_name"`
		Sector             string   `json:"sector"`
		Industry           string   `json:"industry"`
		OneLine            string   `json:"one_liner"`
		ISIN               string   `json:"isin"`
		Currency           string   `json:"currency"`
		MarketCap          *float64 `json:"market_cap"`
		IsIndexConstituent bool     `json:"is_index_constituent"`
		Indexes            []struct {
			Name string `json:"name"`
		} `json:"indexes"`
	} `json:"logos"`
}

func buildLogos(b *Builder) error {
	raw, err := os.ReadFile(b.Path("logo_metadata.full.json"))
	if err != nil {
		return err
	}
	var metadata logoMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return err
	}

	svgs := make([][]byte, len(metadata.Logos))
	for i, logo := range metadata.Logos {
		svg, err := os.ReadFile(filepath.Join(b.LogosDir, filepath.FromSlash(logo.File)))
		if err != nil {
			return err
		}
		svg, err = prepareSVG(svg)
		if err != nil {
			return fmt.Errorf("%s: %w", logo.File, err)
		}
		svgs[i] = svg
	}

	pngs := make([][]byte, len(svgs))
	if b.Resvg != "" {
		b.Logf("rendering %d PNG masters with %s", len(svgs), b.Resvg)
		if err := renderPNGs(b.Resvg, svgs, pngs); err != nil {
			return err
		}
	}

	insert, err := b.Tx.Prepare(`INSERT INTO logos VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	for i, logo := range metadata.Logos {
		names := []string{}
		for _, index := range logo.Indexes {
			if index.Name != "" {
				names = append(names, index.Name)
			}
		}
		indexes, _ := json.Marshal(names)
		var png any
		if pngs[i] != nil {
			png = pngs[i]
			b.Add("logos_png", 1)
		}
		if _, err := insert.Exec(logo.ID, logo.Exchange, logo.Ticker, logo.Company, logo.Sector, logo.Industry,
			logo.OneLine, nullable(logo.ISIN), logo.Currency, logo.MarketCap, logo.IsIndexConstituent, string(indexes), svgs[i], png); err != nil {
			return fmt.Errorf("%s: %w", logo.ID, err)
		}
		b.Add("logos", 1)
	}
	return nil
}

// prepareSVG rejects unsafe content and makes sure the image can scale.
func prepareSVG(svg []byte) ([]byte, error) {
	svg = []byte(strings.TrimSpace(string(svg)))
	if !strings.HasPrefix(string(svg), "<svg") {
		return nil, errors.New("does not start with <svg")
	}
	for _, check := range unsafeSVG {
		if check.pattern.Match(svg) {
			return nil, fmt.Errorf("contains %s", check.label)
		}
	}
	open := svgOpenTag.Find(svg)
	if open == nil {
		return nil, errors.New("no <svg> tag")
	}
	if svgViewBox.Match(open) {
		return svg, nil
	}
	width, height := svgWidth.FindSubmatch(open), svgHeight.FindSubmatch(open)
	if width == nil || height == nil {
		return nil, errors.New("has neither a viewBox nor a numeric width and height")
	}
	fixed := fmt.Sprintf(`<svg viewBox="0 0 %s %s"%s`, width[1], height[1], open[len("<svg"):])
	return append([]byte(fixed), svg[len(open):]...), nil
}

func renderPNGs(resvg string, svgs, pngs [][]byte) error {
	dir, err := os.MkdirTemp("", "logo-png-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	jobs := make(chan int)
	var firstErr error
	var once sync.Once
	var wg sync.WaitGroup
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			in := filepath.Join(dir, fmt.Sprintf("in-%d.svg", worker))
			out := filepath.Join(dir, fmt.Sprintf("out-%d.png", worker))
			for i := range jobs {
				err := os.WriteFile(in, svgs[i], 0o600)
				if err == nil {
					var output []byte
					output, err = exec.Command(resvg, "--width", strconv.Itoa(PNGMasterSize), in, out).CombinedOutput()
					if err != nil {
						err = fmt.Errorf("resvg failed on logo %d: %v: %s", i, err, strings.TrimSpace(string(output)))
					}
				}
				if err == nil {
					pngs[i], err = os.ReadFile(out)
				}
				if err != nil {
					once.Do(func() { firstErr = err })
				}
			}
		}(w)
	}
	for i := range svgs {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return firstErr
}
