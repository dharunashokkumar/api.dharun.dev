package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"
)

// runMissedLogos prints the logo symbols that matched no company, most missed first, from
// the file the API keeps in STATE_DIR (internal/api/missedlogos.go writes it).
func runMissedLogos(args []string) error {
	flags := flag.NewFlagSet("missed-logos", flag.ExitOnError)
	state := flags.String("state", envOr("STATE_DIR", "/var/lib/dharun-api"), "the API's state directory")
	limit := flags.Int("n", 100, "how many names to print (0 for all)")
	asJSON := flags.Bool("json", false, "print the list as JSON")
	flags.Parse(args)

	raw, err := os.ReadFile(filepath.Join(*state, "missed-logos.json"))
	if err != nil {
		return fmt.Errorf("%w (the API writes it within a minute of the first miss)", err)
	}
	var list []struct {
		Name  string    `json:"name"`
		Count uint64    `json:"count"`
		First time.Time `json:"first"`
		Last  time.Time `json:"last"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return err
	}
	if *limit > 0 && len(list) > *limit {
		list = list[:*limit]
	}
	if *asJSON {
		out := json.NewEncoder(os.Stdout)
		out.SetIndent("", "  ")
		return out.Encode(list)
	}
	table := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "NAME\tCOUNT\tFIRST SEEN\tLAST SEEN")
	for _, entry := range list {
		fmt.Fprintf(table, "%s\t%d\t%s\t%s\n", entry.Name, entry.Count, entry.First.Format("2006-01-02 15:04"), entry.Last.Format("2006-01-02 15:04"))
	}
	return table.Flush()
}
