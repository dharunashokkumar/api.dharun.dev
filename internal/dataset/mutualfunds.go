package dataset

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The net asset value of every mutual fund scheme, from the daily file the Association
// of Mutual Funds in India publishes. AMFI's site terms restrict reuse; Dharun chose on
// 2026-10-13 to publish it regardless (see ROADMAP.md).

func init() {
	register(&Spec{
		Name:   "mutual-funds",
		Schema: 1,
		DDL: `
CREATE TABLE schemes (
  code INTEGER PRIMARY KEY, name TEXT NOT NULL, amc TEXT NOT NULL, scheme_type TEXT, category TEXT,
  plan TEXT, option TEXT, plan_label TEXT, option_label TEXT,
  isin_growth TEXT, isin_reinvest TEXT, nav REAL, nav_date TEXT,
  active INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX schemes_amc ON schemes(amc);
CREATE INDEX schemes_isin_growth ON schemes(isin_growth) WHERE isin_growth IS NOT NULL;
CREATE INDEX schemes_isin_reinvest ON schemes(isin_reinvest) WHERE isin_reinvest IS NOT NULL;`,
		Build:  buildMutualFunds,
		Import: importMutualFunds,
		// AMFI publishes each evening; a check every few hours picks it up the same night.
		Every: 4 * time.Hour,
		Main:  "schemes", MaxShrink: 0.1,
	})
}

var (
	schemeHeading = regexp.MustCompile(`^(Open Ended|Close Ended|Interval Fund)\s*Schemes?\s*\((.*)\)\s*$`)
	isinShape     = regexp.MustCompile(`^IN[A-Z0-9]{10}$`)
)

// normalPlan and normalOption read AMFI's many spellings of plan and option.
func normalPlan(label, name string) string {
	text := strings.ToLower(label + " " + name)
	switch {
	case strings.Contains(text, "direct"):
		return "direct"
	case strings.Contains(text, "regular"):
		return "regular"
	}
	return ""
}

func normalOption(label, name string) string {
	text := strings.ToLower(label + " " + name)
	switch {
	case strings.Contains(text, "growth"):
		return "growth"
	case strings.Contains(text, "idcw"), strings.Contains(text, "dividend"), strings.Contains(text, "income distribution"):
		return "idcw"
	case strings.Contains(text, "bonus"):
		return "bonus"
	}
	return ""
}

func buildMutualFunds(b *Builder) error {
	file, err := os.Open(b.Path("mutual-funds", "NAVAll.txt.gz"))
	if err != nil {
		return err
	}
	defer file.Close()
	unzipped, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	insert, err := b.Tx.Prepare(`INSERT OR REPLACE INTO schemes VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,0)`)
	if err != nil {
		return err
	}
	defer insert.Close()

	amcs := map[string]bool{}
	var schemeType, category, amc string
	newest := ""
	scanner := bufio.NewScanner(unzipped)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || lineNumber == 1 {
			continue
		}
		if !strings.Contains(line, ";") {
			if match := schemeHeading.FindStringSubmatch(line); match != nil {
				// The fund house carries over: the file sometimes repeats a heading for the
				// same fund house without naming it again.
				schemeType, category = match[1], strings.TrimSpace(match[2])
			} else {
				amc = line
			}
			continue
		}
		fields := strings.Split(line, ";")
		var code, isin1, isin2, name, planLabel, optionLabel, nav, date string
		switch len(fields) {
		case 8:
			code, isin1, isin2, name, planLabel, optionLabel, nav, date = fields[0], fields[1], fields[2], fields[3], fields[4], fields[5], fields[6], fields[7]
		case 6:
			code, isin1, isin2, name, nav, date = fields[0], fields[1], fields[2], fields[3], fields[4], fields[5]
		default:
			b.Warnf("line %d has %d fields", lineNumber, len(fields))
			continue
		}
		number, err := strconv.Atoi(strings.TrimSpace(code))
		if err != nil {
			b.Warnf("line %d: scheme code %q", lineNumber, code)
			continue
		}
		if amc == "" {
			return fmt.Errorf("line %d: a scheme before any fund house name", lineNumber)
		}
		var value any
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(nav), 64); err == nil && parsed > 0 {
			value = parsed
		}
		var navDate any
		if parsed, err := time.Parse("02-Jan-2006", strings.TrimSpace(date)); err == nil {
			navDate = parsed.Format("2006-01-02")
			if navDate.(string) > newest {
				newest = navDate.(string)
			}
		}
		isin := func(value string) any {
			value = strings.TrimSpace(value)
			if isinShape.MatchString(value) {
				return value
			}
			return nil
		}
		name = strings.TrimSpace(name)
		if _, err := insert.Exec(number, name, amc, nullable(schemeType), nullable(category),
			nullable(normalPlan(planLabel, name)), nullable(normalOption(optionLabel, name)),
			nullable(strings.TrimSpace(planLabel)), nullable(strings.TrimSpace(optionLabel)),
			isin(isin1), isin(isin2), value, navDate); err != nil {
			return fmt.Errorf("scheme %d: %w", number, err)
		}
		amcs[amc] = true
		b.Add("rows", 1)
	}
	if err := countRows(b, "schemes", "schemes", "rows"); err != nil {
		return err
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	// AMFI keeps listing wound-up and matured schemes with their last NAV. A scheme
	// counts as active when its NAV is no more than 30 days older than the newest.
	result, err := b.Tx.Exec(`UPDATE schemes SET active = 1 WHERE nav IS NOT NULL AND nav_date >= date(?, '-30 day')`, newest)
	if err != nil {
		return err
	}
	active, _ := result.RowsAffected()
	b.Count("active_schemes", int(active))
	b.Count("fund_houses", len(amcs))
	b.Logf("mutual-funds: newest NAV dated %s", newest)
	return nil
}

const amfiNAVs = "https://portal.amfiindia.com/spages/NAVAll.txt"

func importMutualFunds(im *Importer) (Source, error) {
	raw, err := im.Browse(amfiNAVs)
	if err != nil {
		return Source{}, err
	}
	if !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("Scheme Code;")) {
		return Source{}, errors.New("the NAV file does not start with its header: AMFI may have changed it")
	}
	if bytes.Count(raw, []byte(";")) < 50000 {
		return Source{}, errors.New("the NAV file lists far fewer schemes than usual: not publishing it")
	}
	version := digest(raw)[:12]
	if im.CurrentVersion() == version {
		return Source{}, ErrNotChanged
	}
	if err := im.WriteGzip("mutual-funds/NAVAll.txt.gz", raw); err != nil {
		return Source{}, err
	}
	return Source{
		Name:        "Net asset values of mutual fund schemes",
		Publisher:   "Association of Mutual Funds in India (AMFI)",
		URL:         "https://www.amfiindia.com/net-asset-value",
		Licence:     "Published by AMFI under SEBI's disclosure rules",
		Version:     version,
		SHA256:      digest(raw),
		Attribution: "Net asset values as published by the Association of Mutual Funds in India.",
	}, nil
}
