package dataset

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

// IP address to country, region, city and network, from DB-IP's free "Lite" databases
// (CC BY 4.0, a credit link to DB-IP is required). They are about 140 MB together and
// change monthly, so they are not kept in the repository: a server downloads them. The
// files stay in MaxMind DB format next to the dataset file, and the API reads them
// through a memory map instead of loading them.

func init() {
	register(&Spec{
		Name:   "ip",
		Schema: 1,
		Build:  buildIP,
		Import: importIP,
		// DB-IP publishes a new month's file early in the month; a weekly look finds it
		// within days, and an unchanged month costs one small request.
		Every: 7 * 24 * time.Hour,
		Main:  "networks", MaxShrink: 0.2,
		NoSeed: true,
	})
}

var ipDatabases = []struct{ kind, file, wantType string }{
	{"city", "dbip-city-lite", "City"},
	{"asn", "dbip-asn-lite", "ASN"},
}

func buildIP(b *Builder) error {
	for _, database := range ipDatabases {
		path := b.Path("ip", database.kind+".mmdb")
		reader, err := maxminddb.Open(path)
		if err != nil {
			return fmt.Errorf("%s: %w", database.kind, err)
		}
		metadata := reader.Metadata
		reader.Close()
		if !strings.Contains(metadata.DatabaseType, database.wantType) {
			return fmt.Errorf("%s: the file is a %q database", database.kind, metadata.DatabaseType)
		}
		if database.kind == "city" {
			b.Count("networks", int(metadata.NodeCount))
		}
		if err := b.Attach(path, database.kind); err != nil {
			return err
		}
	}
	return nil
}

const dbipDownloads = "https://download.db-ip.com/free/"

func importIP(im *Importer) (Source, error) {
	now := im.Now
	if now.IsZero() {
		now = time.Now()
	}
	// This month's file, or last month's while this month's is not out yet.
	for _, month := range []time.Time{now, now.AddDate(0, -1, 0)} {
		version := month.Format("2006-01")
		if im.Version != "" && im.Version != version {
			continue
		}
		if im.CurrentVersion() == version {
			return Source{}, ErrNotChanged
		}
		var sha string
		missing := false
		for _, database := range ipDatabases {
			zipped, err := im.Download(dbipDownloads+database.file+"-"+version+".mmdb.gz", "ip/"+database.kind+".mmdb.gz")
			if err != nil && strings.Contains(err.Error(), "404") {
				missing = true
				break
			}
			if err != nil {
				return Source{}, err
			}
			if err := gunzipFile(zipped, im.Path("ip", database.kind+".mmdb")); err != nil {
				return Source{}, fmt.Errorf("%s: %w", database.kind, err)
			}
			if database.kind == "city" {
				sha, _ = fileDigest(zipped)
			}
			os.Remove(zipped)
		}
		if missing {
			im.Logf("ip: DB-IP has not published %s yet", version)
			continue
		}
		return Source{
			Name:        "DB-IP Lite: IP to City and IP to ASN",
			Publisher:   "DB-IP.com",
			URL:         "https://db-ip.com/db/lite.php",
			Licence:     "CC BY 4.0",
			LicenceURL:  "https://creativecommons.org/licenses/by/4.0/",
			Version:     version,
			SHA256:      sha,
			Attribution: "IP Geolocation by DB-IP (https://db-ip.com), licensed under CC BY 4.0.",
		}, nil
	}
	return Source{}, errors.New("DB-IP has no Lite database for this month or the last")
}

func gunzipFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	unzipped, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	// Decompressed, the city file is about 130 MB; a limit keeps a hostile file finite.
	written, err := io.Copy(out, io.LimitReader(unzipped, 2*maxDownload+1))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil && written > 2*maxDownload {
		err = errors.New("decompresses to more than 1 GB")
	}
	return err
}

// fileDigest hashes a file without reading it all into memory.
func fileDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
