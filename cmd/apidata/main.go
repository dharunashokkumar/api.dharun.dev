// Command apidata maintains the datasets behind the API.
//
//	apidata import pincode -file pincode.csv   refresh one dataset from its source (run by hand)
//	apidata import ifsc
//	apidata import rto
//	apidata import bank-holidays -year 2026
//	apidata import market-holidays -year 2026
//	apidata build -out api.db                  compile data/ into the database the server reads
//
// Imports change files under data/ that are committed to the repository; build never
// touches the network.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

const userAgent = "api.dharun.dev data importer (https://github.com/dharunashokkumar/api.dharun.dev)"

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
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "apidata:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: apidata build [flags] | apidata import <pincode|ifsc|rto|bank-holidays|market-holidays> [flags]")
	os.Exit(2)
}

func runBuild(args []string) error {
	flags := flag.NewFlagSet("build", flag.ExitOnError)
	opts := dataset.BuildOptions{}
	flags.StringVar(&opts.DataDir, "data", "data", "directory holding the source files")
	flags.StringVar(&opts.LogosDir, "logos", "logos", "directory holding the nse/ and bse/ SVG folders")
	flags.StringVar(&opts.Out, "out", "api.db", "database file to write")
	flags.StringVar(&opts.Resvg, "resvg", "", "resvg executable for PNG logos (default: look on PATH; \"none\" to skip)")
	flags.BoolVar(&opts.Strict, "strict", false, "fail if any dataset or the PNG renderer is missing")
	flags.Parse(args)

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
	opts.Log = func(format string, args ...any) { fmt.Printf(format+"\n", args...) }

	started := time.Now()
	stats, err := dataset.Build(opts)
	if err != nil {
		return err
	}
	info, _ := os.Stat(opts.Out)
	fmt.Printf("built %s in %s (%.1f MB)\n", opts.Out, time.Since(started).Round(time.Millisecond), float64(info.Size())/1e6)
	fmt.Printf("  logos %d (%d with PNG) | pincodes %d (%d post offices) | banks %d, branches %d | rto %d | holidays %d\n",
		stats.Logos, stats.LogosWithPNG, stats.Pincodes, stats.PostOffices, stats.Banks, stats.Branches, stats.RTOs, stats.Holidays)
	return nil
}

func runImport(name string, args []string) error {
	flags := flag.NewFlagSet("import "+name, flag.ExitOnError)
	dataDir := flags.String("data", "data", "directory holding the source files")
	file := flags.String("file", "", "read the source from this local file instead of downloading it")
	version := flags.String("version", "", "source version to fetch (ifsc: release tag, rto: revision id); default latest")
	note := flags.String("note", "", "free-text note stored with the source record")
	year := flags.Int("year", time.Now().Year(), "year to fetch (bank-holidays and market-holidays)")
	flags.Parse(args)

	// Holiday imports rewrite part of data/holidays/<year>.json; their source record in
	// sources.json is maintained by hand because it names several publishers.
	if name == "bank-holidays" || name == "market-holidays" {
		fetch := importBankHolidays
		if name == "market-holidays" {
			fetch = importMarketHolidays
		}
		count, err := fetch(*dataDir, *year)
		if err != nil {
			return err
		}
		fmt.Printf("imported %s: %d entries for %d\n", name, count, *year)
		return nil
	}

	sources, err := dataset.ReadSources(*dataDir)
	if err != nil {
		return err
	}
	var source dataset.Source
	switch name {
	case "pincode":
		source, err = importPincode(*dataDir, *file)
	case "ifsc":
		source, err = importIFSC(*dataDir, *version)
	case "rto":
		source, err = importRTO(*dataDir, *file, *version)
	default:
		usage()
	}
	if err != nil {
		return err
	}
	source.Retrieved = time.Now().UTC().Format("2006-01-02")
	source.Notes = *note
	sources[name] = source
	if err := dataset.WriteSources(*dataDir, sources); err != nil {
		return err
	}
	fmt.Printf("imported %s: %s (%s)\n", name, source.Name, source.Version)
	return nil
}

const pincodeFileURL = "https://www.data.gov.in/files/ogdpv2dms/s3fs-public/dataurl03122020/pincode.csv"

func importPincode(dataDir, file string) (dataset.Source, error) {
	var raw []byte
	var err error
	if file != "" {
		raw, err = os.ReadFile(file)
	} else {
		raw, err = download(pincodeFileURL)
		if err != nil {
			err = fmt.Errorf("%w\nthe portal often refuses scripted downloads: save the CSV from "+
				"https://www.data.gov.in/resource/all-india-pincode-directory-till-last-month in a browser and pass it with -file", err)
		}
	}
	if err != nil {
		return dataset.Source{}, err
	}
	if !bytes.Contains(bytes.ToLower(raw[:min(len(raw), 400)]), []byte("pincode")) {
		return dataset.Source{}, errors.New("this does not look like the pincode directory: no Pincode column in the header")
	}
	if err := writeGzip(filepath.Join(dataDir, "pincode.csv.gz"), raw); err != nil {
		return dataset.Source{}, err
	}
	return dataset.Source{
		Name:        "All India Pincode Directory",
		Publisher:   "Department of Posts, Ministry of Communications, Government of India",
		URL:         "https://www.data.gov.in/resource/all-india-pincode-directory-till-last-month",
		Licence:     "Government Open Data License - India (GODL)",
		LicenceURL:  "https://www.data.gov.in/Godl",
		Version:     fmt.Sprintf("%d bytes", len(raw)),
		SHA256:      digest(raw),
		Attribution: "Contains data from the All India Pincode Directory, Department of Posts, published on data.gov.in under the Government Open Data License - India.",
	}, nil
}

func importIFSC(dataDir, version string) (dataset.Source, error) {
	if version == "" {
		raw, err := download("https://api.github.com/repos/razorpay/ifsc/releases/latest")
		if err != nil {
			return dataset.Source{}, err
		}
		var release struct {
			Tag string `json:"tag_name"`
		}
		if err := json.Unmarshal(raw, &release); err != nil || release.Tag == "" {
			return dataset.Source{}, fmt.Errorf("could not read the latest release tag: %v", err)
		}
		version = release.Tag
	}
	base := "https://github.com/razorpay/ifsc/releases/download/" + version + "/"
	branches, err := download(base + "IFSC.csv")
	if err != nil {
		return dataset.Source{}, err
	}
	banks, err := download(base + "banks.json")
	if err != nil {
		return dataset.Source{}, err
	}
	names, err := download("https://raw.githubusercontent.com/razorpay/ifsc/" + version + "/src/banknames.json")
	if err != nil {
		return dataset.Source{}, err
	}
	if !json.Valid(banks) || !json.Valid(names) {
		return dataset.Source{}, errors.New("banks.json or banknames.json is not valid JSON")
	}
	if err := writeGzip(filepath.Join(dataDir, "ifsc.csv.gz"), branches); err != nil {
		return dataset.Source{}, err
	}
	if err := os.WriteFile(filepath.Join(dataDir, "banks.json"), banks, 0o644); err != nil {
		return dataset.Source{}, err
	}
	if err := os.WriteFile(filepath.Join(dataDir, "banknames.json"), names, 0o644); err != nil {
		return dataset.Source{}, err
	}
	return dataset.Source{
		Name:        "IFSC dataset",
		Publisher:   "Razorpay, compiled from Reserve Bank of India and NPCI lists",
		URL:         "https://github.com/razorpay/ifsc/releases/tag/" + version,
		Licence:     "Public domain (dataset), as stated by the publisher",
		LicenceURL:  "https://github.com/razorpay/ifsc#license",
		Version:     version,
		SHA256:      digest(branches),
		Attribution: "IFSC data from the open razorpay/ifsc dataset, compiled from Reserve Bank of India and NPCI publications.",
	}, nil
}

const rtoPage = "List_of_Regional_Transport_Office_districts_in_India"

func importRTO(dataDir, file, revision string) (dataset.Source, error) {
	var wikitext []byte
	var err error
	switch {
	case file != "":
		if revision == "" {
			return dataset.Source{}, errors.New("-file needs -version <revision id> so the source can be credited exactly")
		}
		wikitext, err = os.ReadFile(file)
	default:
		if revision == "" {
			raw, err := download("https://en.wikipedia.org/w/api.php?action=query&prop=revisions&rvprop=ids&format=json&titles=" + rtoPage)
			if err != nil {
				return dataset.Source{}, err
			}
			var result struct {
				Query struct {
					Pages map[string]struct {
						Revisions []struct {
							ID int64 `json:"revid"`
						} `json:"revisions"`
					} `json:"pages"`
				} `json:"query"`
			}
			if err := json.Unmarshal(raw, &result); err != nil {
				return dataset.Source{}, err
			}
			for _, page := range result.Query.Pages {
				if len(page.Revisions) > 0 {
					revision = fmt.Sprint(page.Revisions[0].ID)
				}
			}
			if revision == "" {
				return dataset.Source{}, errors.New("could not find the latest revision of the Wikipedia page")
			}
		}
		wikitext, err = download("https://en.wikipedia.org/w/index.php?action=raw&title=" + rtoPage + "&oldid=" + revision)
	}
	if err != nil {
		return dataset.Source{}, err
	}

	offices, err := parseRTO(string(wikitext))
	if err != nil {
		return dataset.Source{}, err
	}
	if len(offices) < 1000 {
		return dataset.Source{}, fmt.Errorf("parsed only %d codes, expected well over 1,000: refusing to overwrite rto.json", len(offices))
	}
	encoded, err := json.MarshalIndent(offices, "", " ")
	if err != nil {
		return dataset.Source{}, err
	}
	if err := os.WriteFile(filepath.Join(dataDir, "rto.json"), append(encoded, '\n'), 0o644); err != nil {
		return dataset.Source{}, err
	}
	return dataset.Source{
		Name:        "List of Regional Transport Office districts in India",
		Publisher:   "Wikipedia contributors",
		URL:         "https://en.wikipedia.org/w/index.php?title=" + rtoPage + "&oldid=" + revision,
		Licence:     "CC BY-SA 4.0",
		LicenceURL:  "https://creativecommons.org/licenses/by-sa/4.0/",
		Version:     "revision " + revision,
		SHA256:      digest(wikitext),
		Attribution: "RTO codes adapted from the Wikipedia article \"List of Regional Transport Office districts in India\", licensed CC BY-SA 4.0. This dataset is shared under the same licence.",
	}, nil
}

func download(url string) ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", userAgent)
	client := &http.Client{Timeout: 5 * time.Minute}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, response.Status)
	}
	return io.ReadAll(response.Body)
}

func writeGzip(path string, raw []byte) error {
	var buffer bytes.Buffer
	// No name or timestamp in the header, so the same input always gives the same file.
	writer, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return err
	}
	if _, err := writer.Write(raw); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buffer.Bytes(), 0o644)
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
