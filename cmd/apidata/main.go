// Command apidata maintains the datasets behind the API.
//
//	apidata build                      build the repository's copy of every dataset into seed/
//	apidata import ifsc                refresh the repository's copy of one dataset from its source
//	apidata refresh                    on a server: fetch and rebuild every dataset that is due
//	apidata status                     on a server: which copy of each dataset is in use
//
// build never touches the network. import changes files under data/ that are committed.
// refresh writes only to the server's own data directory.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "build":
		err = runBuild(os.Args[2:])
	case "import":
		if len(os.Args) < 3 {
			usage()
		}
		err = runImport(os.Args[2], os.Args[3:])
	case "refresh":
		err = runRefresh(os.Args[2:])
	case "status":
		err = runStatus(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "apidata:", err)
		os.Exit(1)
	}
}

func usage() {
	var fetchable []string
	for _, spec := range dataset.Specs() {
		if spec.Import != nil {
			fetchable = append(fetchable, spec.Name)
		}
	}
	fmt.Fprintln(os.Stderr, `usage:
  apidata build   [-data data] [-logos logos] [-out seed] [-resvg path|none] [-strict] [dataset ...]
  apidata import  <dataset> [-data data] [-file F] [-version V] [-year Y] [-note text]
  apidata refresh [-seed seed] [-live dir] [-force] [dataset ...]
  apidata status  [-seed seed] [-live dir]

datasets with a source that can be fetched: `+strings.Join(fetchable, ", "))
	os.Exit(2)
}

func logf(format string, args ...any) { fmt.Printf(format+"\n", args...) }

func runBuild(args []string) error {
	flags := flag.NewFlagSet("build", flag.ExitOnError)
	opts := dataset.SeedOptions{Log: logf}
	flags.StringVar(&opts.DataDir, "data", "data", "directory holding the source files")
	flags.StringVar(&opts.LogosDir, "logos", "logos", "directory holding the nse/ and bse/ SVG folders")
	flags.StringVar(&opts.OutDir, "out", "seed", "directory to write the dataset files into")
	flags.StringVar(&opts.Resvg, "resvg", "", "resvg executable for PNG logos (default: look on PATH; \"none\" to skip)")
	flags.BoolVar(&opts.Strict, "strict", false, "fail if any dataset or the PNG renderer is missing")
	flags.Parse(args)
	opts.Only = flags.Args()

	switch opts.Resvg {
	case "none":
		opts.Resvg = ""
	case "":
		if path, err := exec.LookPath("resvg"); err == nil {
			opts.Resvg = path
		}
	}
	if opts.Resvg == "" {
		if opts.Strict {
			return errors.New("resvg not found: PNG logos cannot be built (install it or drop -strict)")
		}
		fmt.Println("warning: resvg not found, PNG logo output will be unavailable in this build")
	}

	started := time.Now()
	built, _, err := dataset.BuildSeeds(opts)
	if err != nil {
		return err
	}
	var size int64
	for _, meta := range built {
		if info, err := os.Stat(dataset.SeedPath(opts.OutDir, meta.Dataset)); err == nil {
			size += info.Size()
		}
		fmt.Printf("  %-18s %s\n", meta.Dataset, describeCounts(meta.Counts))
	}
	fmt.Printf("built %d datasets into %s in %s (%.1f MB)\n", len(built), opts.OutDir, time.Since(started).Round(time.Millisecond), float64(size)/1e6)
	return nil
}

func describeCounts(counts map[string]int) string {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, len(names))
	for i, name := range names {
		parts[i] = fmt.Sprintf("%s %d", name, counts[name])
	}
	return strings.Join(parts, ", ")
}

// Earlier names of two import commands, kept so old notes and habits still work.
var importAliases = map[string]string{"bank-holidays": "holidays-bank", "market-holidays": "holidays-market"}

func runImport(name string, args []string) error {
	if alias, ok := importAliases[name]; ok {
		name = alias
	}
	spec, ok := dataset.Lookup(name)
	if !ok || spec.Import == nil {
		usage()
	}
	flags := flag.NewFlagSet("import "+name, flag.ExitOnError)
	dataDir := flags.String("data", "data", "directory holding the source files")
	file := flags.String("file", "", "read the source from this local file instead of downloading it")
	version := flags.String("version", "", "source version to fetch (a release tag or revision id); default latest")
	note := flags.String("note", "", "free-text note stored with the source record")
	year := flags.Int("year", 0, "year to fetch, for holiday lists; default the current year")
	flags.Parse(args)

	sources, err := dataset.ReadSources(*dataDir)
	if err != nil {
		return err
	}
	source, err := spec.Import(&dataset.Importer{Dir: *dataDir, File: *file, Version: *version, Year: *year, Now: time.Now(), Log: logf})
	if err != nil {
		return err
	}
	source.Retrieved = time.Now().UTC().Format("2006-01-02")
	if *note != "" {
		source.Notes = *note
	} else if source.Notes == "" {
		source.Notes = sources[name].Notes
	}
	sources[name] = source
	if err := dataset.WriteSources(*dataDir, sources); err != nil {
		return err
	}
	fmt.Printf("imported %s: %s (%s)\n", name, source.Name, source.Version)
	return nil
}

func runRefresh(args []string) error {
	flags := flag.NewFlagSet("refresh", flag.ExitOnError)
	opts := dataset.RefreshOptions{Log: logf}
	flags.StringVar(&opts.SeedDir, "seed", envOr("SEED_DIR", "seed"), "directory holding the copies that came with the release")
	flags.StringVar(&opts.LiveDir, "live", os.Getenv("DATA_DIR"), "directory this server keeps its own copies in")
	flags.BoolVar(&opts.Force, "force", false, "refresh even if the copy in use is recent, and accept a result that would be held back")
	flags.Parse(args)
	opts.Only = flags.Args()
	if opts.LiveDir == "" {
		return errors.New("no data directory: pass -live or set DATA_DIR")
	}

	results, err := dataset.Refresh(opts)
	if err != nil {
		return err
	}
	failed := 0
	for _, result := range results {
		if result.Outcome == "failed" || result.Outcome == "held" {
			failed++
		}
	}
	// A source being down is an expected event, not a reason for the timer unit to show
	// as failed every hour; the status file and the metrics carry it. A run where nothing
	// that was asked for worked is different.
	if len(opts.Only) > 0 && failed == len(results) && failed > 0 {
		return fmt.Errorf("%d of %d datasets did not refresh", failed, len(results))
	}
	return nil
}

func runStatus(args []string) error {
	flags := flag.NewFlagSet("status", flag.ExitOnError)
	seedDir := flags.String("seed", envOr("SEED_DIR", "seed"), "directory holding the copies that came with the release")
	liveDir := flags.String("live", os.Getenv("DATA_DIR"), "directory this server keeps its own copies in")
	flags.Parse(args)

	status := dataset.ReadStatus(*liveDir)
	fmt.Printf("%-18s %-6s %-22s %-9s %s\n", "DATASET", "COPY", "SOURCE FETCHED", "REFRESH", "RECORDS")
	for _, spec := range dataset.Specs() {
		_, meta, live, ok := dataset.Current(spec, *seedDir, *liveDir)
		origin, fetched, records := "none", "-", "-"
		if ok {
			origin, fetched, records = "seed", meta.Fetched(), describeCounts(meta.Counts)
			if live {
				origin = "server"
			}
		}
		refresh := "by hand"
		if spec.Every > 0 {
			refresh = fmt.Sprintf("every %dd", int(spec.Every.Hours()/24))
		}
		fmt.Printf("%-18s %-6s %-22s %-9s %s\n", spec.Name, origin, fetched, refresh, records)
		if entry := status.Datasets[spec.Name]; entry.Result == "failed" || entry.Result == "held" {
			fmt.Printf("%-18s last attempt %s %s: %s\n", "", entry.LastAttempt, entry.Result, entry.Error)
		}
	}
	return nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
