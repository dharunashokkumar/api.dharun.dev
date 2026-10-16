// Command traffic reports who uses the API: people, AI assistants, programs, crawlers
// and our own checks, which sources led them here and which of our links they followed.
//
//	go run ./cmd/traffic -days 7                 # Cloudflare analytics: by user agent only
//	go run ./cmd/traffic -logs requests.jsonl    # the server's own log lines: everything
//
// Cloudflare's analytics on this plan carry neither the query string nor the referring
// page, so that report sorts by user agent alone. The server's log has every label; a
// file of its lines can come from journalctl -o cat or a Loki export ("-" reads stdin).
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/traffic"
)

const host = "api.dharun.dev"

func main() {
	days := flag.Int("days", 7, "days of Cloudflare analytics to read, up to 30")
	logs := flag.String("logs", "", "read the server's JSON log lines from this file (- for stdin) instead of Cloudflare")
	envFile := flag.String("env", ".cache/cloudflare.env", "file with CLOUDFLARE_API_TOKEN and CLOUDFLARE_ZONE_ID, when they are not in the environment")
	top := flag.Int("top", 25, "how many sources to list")
	agents := flag.Bool("agents", false, "also list the user agents behind each source, to see what a rule should match")
	flag.Parse()

	var (
		report *tally
		err    error
	)
	if *logs != "" {
		report, err = fromLogs(*logs)
	} else {
		report, err = fromCloudflare(*days, *envFile)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "traffic:", err)
		os.Exit(1)
	}
	report.print(os.Stdout, *top)
	if *agents {
		report.printAgents(os.Stdout, *top)
	}
}

// tally counts requests by each label.
type tally struct {
	title   string
	total   int
	kinds   map[string]int
	sources map[string]int // kind/source
	media   map[string]int // kind/medium
	agents  map[string]int // kind/source <tab> user agent
	noMedia bool           // the input carries no medium at all
}

func newTally(title string) *tally {
	return &tally{title: title, kinds: map[string]int{}, sources: map[string]int{}, media: map[string]int{}, agents: map[string]int{}}
}

func (t *tally) add(o traffic.Origin, userAgent string, count int) {
	source := o.Source
	if source == "" {
		source = "(none)"
	}
	medium := o.Medium
	if medium == "" {
		medium = "(none)"
	}
	t.total += count
	t.kinds[o.Kind] += count
	t.sources[o.Kind+"/"+source] += count
	t.media[o.Kind+"/"+medium] += count
	t.agents[o.Kind+"/"+source+"\t"+userAgent] += count
}

// printAgents lists the most frequent user agents with the label each got.
func (t *tally) printAgents(out io.Writer, top int) {
	keys := make([]string, 0, len(t.agents))
	for key := range t.agents {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return t.agents[keys[i]] > t.agents[keys[j]] })
	fmt.Fprintln(out, "\nUser agents (kind/source, count, user agent)")
	for i, key := range keys {
		if i == top*4 {
			break
		}
		label, agent, _ := strings.Cut(key, "\t")
		fmt.Fprintf(out, "  %-28s %7d  %s\n", label, t.agents[key], agent)
	}
}

func (t *tally) print(out io.Writer, top int) {
	fmt.Fprintf(out, "%s\n%d requests (health checks left out)\n\n", t.title, t.total)
	if t.total == 0 {
		return
	}
	fmt.Fprintln(out, "By kind")
	for _, kind := range traffic.Kinds {
		if n := t.kinds[kind]; n > 0 {
			fmt.Fprintf(out, "  %-16s %8d  %5.1f%%\n", kind, n, 100*float64(n)/float64(t.total))
		}
	}
	section := func(title string, counts map[string]int, limit int) {
		fmt.Fprintf(out, "\n%s\n", title)
		keys := make([]string, 0, len(counts))
		for key := range counts {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			if counts[keys[i]] != counts[keys[j]] {
				return counts[keys[i]] > counts[keys[j]]
			}
			return keys[i] < keys[j]
		})
		for i, key := range keys {
			if limit > 0 && i == limit {
				fmt.Fprintf(out, "  ... %d more\n", len(keys)-limit)
				break
			}
			fmt.Fprintf(out, "  %-40s %8d  %5.1f%%\n", key, counts[key], 100*float64(counts[key])/float64(t.total))
		}
	}
	section("Top sources (kind/source)", t.sources, top)
	if !t.noMedia {
		section("Links followed (kind/medium)", t.media, 0)
	}
}

// ---- the server's log ----

func fromLogs(path string) (*tally, error) {
	in := os.Stdin
	if path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		in = file
	}
	report := newTally("Server log: " + path)
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		// journalctl and Loki exports may put a timestamp or a label before the JSON.
		start := bytes.IndexByte(line, '{')
		if start < 0 {
			continue
		}
		var entry struct {
			Msg     string `json:"msg"`
			Route   string `json:"route"`
			Path    string `json:"path"`
			UA      string `json:"ua"`
			Referer string `json:"referer"`
			Kind    string `json:"kind"`
			Src     string `json:"src"`
			Medium  string `json:"medium"`
		}
		if json.Unmarshal(line[start:], &entry) != nil || entry.Msg != "request" || entry.Route == "health" {
			continue
		}
		var o traffic.Origin
		if entry.Kind != "" {
			o = traffic.Origin{Kind: entry.Kind, Source: entry.Src, Medium: entry.Medium}
		} else {
			// A line from before the server labelled requests: label it the same way now.
			query := url.Values{}
			if _, raw, ok := strings.Cut(entry.Path, "?"); ok {
				query, _ = url.ParseQuery(raw)
			}
			o = traffic.Classify(traffic.Request{UserAgent: entry.UA, Referer: entry.Referer,
				UTMSource: query.Get("utm_source"), UTMMedium: query.Get("utm_medium"), MCP: entry.Route == "mcp"})
		}
		report.add(o, entry.UA, 1)
	}
	return report, scanner.Err()
}

// ---- Cloudflare ----

func fromCloudflare(days int, envFile string) (*tally, error) {
	if days < 1 || days > 30 {
		return nil, fmt.Errorf("-days must be from 1 to 30")
	}
	token, zone := os.Getenv("CLOUDFLARE_API_TOKEN"), os.Getenv("CLOUDFLARE_ZONE_ID")
	if token == "" || zone == "" {
		values, err := readEnv(envFile)
		if err != nil {
			return nil, fmt.Errorf("no CLOUDFLARE_API_TOKEN and CLOUDFLARE_ZONE_ID in the environment, and %v", err)
		}
		token, zone = values["CLOUDFLARE_API_TOKEN"], values["CLOUDFLARE_ZONE_ID"]
	}
	until := time.Now().UTC()
	since := until.AddDate(0, 0, -days)
	query := `query($zone: String!, $since: Time!, $until: Time!, $host: String!) {
  viewer { zones(filter: {zoneTag: $zone}) {
    httpRequestsAdaptiveGroups(limit: 10000, filter: {datetime_geq: $since, datetime_lt: $until,
        clientRequestHTTPHost: $host, clientRequestPath_neq: "/healthz"}) {
      count
      dimensions { userAgent verifiedBotCategory }
    }
  } }
}`
	body, _ := json.Marshal(map[string]any{"query": query, "variables": map[string]string{
		"zone": zone, "host": host, "since": since.Format(time.RFC3339), "until": until.Format(time.RFC3339)}})
	request, _ := http.NewRequest(http.MethodPost, "https://api.cloudflare.com/client/v4/graphql", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 60 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var answer struct {
		Data struct {
			Viewer struct {
				Zones []struct {
					Groups []struct {
						Count      int `json:"count"`
						Dimensions struct {
							UserAgent   string `json:"userAgent"`
							BotCategory string `json:"verifiedBotCategory"`
						} `json:"dimensions"`
					} `json:"httpRequestsAdaptiveGroups"`
				} `json:"zones"`
			} `json:"viewer"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(response.Body).Decode(&answer); err != nil {
		return nil, fmt.Errorf("reading Cloudflare's answer: %v", err)
	}
	if len(answer.Errors) > 0 {
		return nil, fmt.Errorf("Cloudflare: %s", answer.Errors[0].Message)
	}
	if len(answer.Data.Viewer.Zones) == 0 {
		return nil, fmt.Errorf("Cloudflare returned no zone; check CLOUDFLARE_ZONE_ID")
	}
	report := newTally(fmt.Sprintf("Cloudflare, %s to %s, by user agent (Cloudflare has no query string or referer on this plan)",
		since.Format("2006-01-02 15:04"), until.Format("2006-01-02 15:04 UTC")))
	report.noMedia = true
	groups := answer.Data.Viewer.Zones[0].Groups
	for _, group := range groups {
		o := traffic.Classify(traffic.Request{UserAgent: group.Dimensions.UserAgent})
		// Cloudflare knows some bots by their addresses, whatever their user agent says.
		if category := group.Dimensions.BotCategory; category != "" && (o.Kind == traffic.Person || o.Kind == traffic.Unknown) {
			o.Kind, o.Source = traffic.Bot, traffic.Clean(category)
			if strings.Contains(strings.ToLower(category), "ai") {
				o.Kind = traffic.AICrawler
			} else if strings.Contains(strings.ToLower(category), "search") {
				o.Kind = traffic.SearchCrawler
			}
		}
		report.add(o, group.Dimensions.UserAgent, group.Count)
	}
	if len(groups) == 10000 {
		fmt.Fprintln(os.Stderr, "traffic: Cloudflare returned its limit of 10,000 user agents; use fewer -days for a complete count")
	}
	return report, nil
}

// readEnv reads KEY=value lines, ignoring comments and quotes.
func readEnv(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "export "))
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.HasPrefix(key, "#") {
			continue
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return values, nil
}
