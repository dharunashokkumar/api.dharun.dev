package dataset

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Mutual fund NAV history, for returns: each scheme's last NAV of every month for ten
// years, and every NAV of the last 45 days. From AMFI's NAV history download (the same
// publisher as the mutual-funds dataset; Dharun agreed on 2026-11-11). It exists only
// as a server's own copy: the first refresh fills the months forty at a time, later ones
// add the new days and the month that just ended, carrying the rest forward.

const (
	navHistoryMonths    = 120
	navHistoryDays      = 45
	navMonthsPerRefresh = 40
	amfiHistory         = "https://portal.amfiindia.com/DownloadNAVHistoryReport_Po.aspx?frmdt=%s&todt=%s"
)

func init() {
	register(&Spec{
		Name:   "nav-history",
		Schema: 1,
		DDL: `
CREATE TABLE navs (code INTEGER NOT NULL, date TEXT NOT NULL, nav REAL NOT NULL, month_end INTEGER NOT NULL, PRIMARY KEY (code, date)) WITHOUT ROWID;
CREATE TABLE months (month TEXT PRIMARY KEY);`,
		Build:  buildNAVHistory,
		Import: importNAVHistory,
		Every:  12 * time.Hour,
		Main:   "navs", MaxShrink: 0.2,
		NoSeed: true,
	})
}

// navs.csv.gz holds lines "M|D,code,date,nav"; months.txt the months filled.
func buildNAVHistory(b *Builder) error {
	file, err := os.Open(b.Path("nav-history", "navs.csv.gz"))
	if err != nil {
		return err
	}
	defer file.Close()
	unzipped, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	insert, err := b.Tx.Prepare(`INSERT OR REPLACE INTO navs VALUES (?,?,?,?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	scanner := bufio.NewScanner(unzipped)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), ",")
		if len(parts) != 4 {
			continue
		}
		code, err1 := strconv.Atoi(parts[1])
		nav, err2 := strconv.ParseFloat(parts[3], 64)
		if err1 != nil || err2 != nil {
			continue
		}
		monthEnd := 0
		if parts[0] == "M" {
			monthEnd = 1
		}
		if _, err := insert.Exec(code, parts[2], nav, monthEnd); err != nil {
			return err
		}
		b.Add("navs", 1)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	months, err := os.ReadFile(b.Path("nav-history", "months.txt"))
	if err != nil {
		return err
	}
	for _, m := range strings.Fields(string(months)) {
		if _, err := b.Tx.Exec(`INSERT OR IGNORE INTO months VALUES (?)`, m); err != nil {
			return err
		}
		b.Add("months", 1)
	}
	return nil
}

type navPoint struct {
	date string
	nav  float64
}

// readHistoryWindow reads AMFI's history file for a date range: code -> NAVs by date.
func readHistoryWindow(raw []byte) map[int][]navPoint {
	out := map[int][]navPoint{}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ";")
		if len(fields) < 8 {
			continue
		}
		code, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil {
			continue
		}
		nav, err := strconv.ParseFloat(strings.TrimSpace(fields[6]), 64)
		if err != nil || nav <= 0 {
			continue
		}
		date, err := time.Parse("02-Jan-2006", strings.TrimSpace(fields[7]))
		if err != nil {
			continue
		}
		out[code] = append(out[code], navPoint{date.Format(time.DateOnly), nav})
	}
	return out
}

func amfiDate(t time.Time) string { return t.Format("02-Jan-2006") }

func importNAVHistory(im *Importer) (Source, error) {
	now := im.Now
	if now.IsZero() {
		now = time.Now()
	}
	type key struct {
		code int
		date string
	}
	monthEnds := map[key]float64{}
	filled := map[string]bool{}
	if im.Current != nil {
		rows, err := im.Current.Query(`SELECT code, date, nav FROM navs WHERE month_end = 1`)
		if err == nil {
			for rows.Next() {
				var k key
				var nav float64
				if rows.Scan(&k.code, &k.date, &nav) == nil {
					monthEnds[k] = nav
				}
			}
			rows.Close()
		}
		if rows, err := im.Current.Query(`SELECT month FROM months`); err == nil {
			for rows.Next() {
				var m string
				if rows.Scan(&m) == nil {
					filled[m] = true
				}
			}
			rows.Close()
		}
	}
	// The month-ends still missing, newest first, a few dozen per run.
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	fetchedMonths := 0
	for i := 1; i <= navHistoryMonths && fetchedMonths < navMonthsPerRefresh; i++ {
		first := thisMonth.AddDate(0, -i, 0)
		month := first.Format("2006-01")
		if filled[month] {
			continue
		}
		last := first.AddDate(0, 1, -1)
		raw, err := im.Browse(fmt.Sprintf(amfiHistory, amfiDate(last.AddDate(0, 0, -6)), amfiDate(last)))
		if err != nil {
			im.Logf("nav-history: %s: %v", month, err)
			break
		}
		window := readHistoryWindow(raw)
		if len(window) < 500 {
			im.Logf("nav-history: %s has only %d schemes; stopping here", month, len(window))
			break
		}
		for code, points := range window {
			sort.Slice(points, func(a, b int) bool { return points[a].date < points[b].date })
			p := points[len(points)-1]
			monthEnds[key{code, p.date}] = p.nav
		}
		filled[month] = true
		fetchedMonths++
	}
	// The last 45 days, every NAV, in windows of 15 days.
	daily := map[key]float64{}
	newest := ""
	for end := now; end.After(now.AddDate(0, 0, -navHistoryDays)); end = end.AddDate(0, 0, -15) {
		raw, err := im.Browse(fmt.Sprintf(amfiHistory, amfiDate(end.AddDate(0, 0, -14)), amfiDate(end)))
		if err != nil {
			return Source{}, fmt.Errorf("recent NAVs: %w", err)
		}
		for code, points := range readHistoryWindow(raw) {
			for _, p := range points {
				daily[key{code, p.date}] = p.nav
				if p.date > newest {
					newest = p.date
				}
			}
		}
	}
	if len(daily) < 10000 {
		return Source{}, errors.New("AMFI's recent NAV history is far smaller than usual: not publishing it")
	}
	version := newest + "-" + strconv.Itoa(len(filled))
	if im.CurrentVersion() == version {
		return Source{}, ErrNotChanged
	}
	var out bytes.Buffer
	zipped := gzip.NewWriter(&out)
	write := func(w io.Writer, kind string, k key, nav float64) {
		fmt.Fprintf(w, "%s,%d,%s,%s\n", kind, k.code, k.date, strconv.FormatFloat(nav, 'f', -1, 64))
	}
	for k, nav := range monthEnds {
		write(zipped, "M", k, nav)
	}
	for k, nav := range daily {
		if _, isMonthEnd := monthEnds[k]; !isMonthEnd {
			write(zipped, "D", k, nav)
		}
	}
	if err := zipped.Close(); err != nil {
		return Source{}, err
	}
	if err := im.Write("nav-history/navs.csv.gz", out.Bytes()); err != nil {
		return Source{}, err
	}
	months := make([]string, 0, len(filled))
	for m := range filled {
		months = append(months, m)
	}
	sort.Strings(months)
	if err := im.Write("nav-history/months.txt", []byte(strings.Join(months, "\n")+"\n")); err != nil {
		return Source{}, err
	}
	im.Logf("nav-history: %d month-ends (%d months), %d recent NAVs to %s", len(monthEnds), len(filled), len(daily), newest)
	return Source{
		Name:        "Mutual fund NAV history",
		Publisher:   "Association of Mutual Funds in India (AMFI)",
		URL:         "https://www.amfiindia.com/net-asset-value-history",
		Licence:     "Published by AMFI under SEBI's disclosure rules",
		Version:     version,
		Attribution: "Net asset values as published by the Association of Mutual Funds in India.",
	}, nil
}
