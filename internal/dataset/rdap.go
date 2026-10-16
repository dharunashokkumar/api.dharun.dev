package dataset

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

// Which RDAP server answers for a top-level domain, an IP network or a range of AS
// numbers, from IANA's bootstrap registries (RFC 9224). IANA dedicates its protocol
// registries to the public domain (CC0 1.0).

func init() {
	register(&Spec{
		Name:   "rdap",
		Schema: 1,
		DDL: `CREATE TABLE tlds (tld TEXT NOT NULL, url TEXT NOT NULL, PRIMARY KEY (tld, url)) WITHOUT ROWID;
CREATE TABLE networks (version INTEGER NOT NULL, prefix TEXT NOT NULL, url TEXT NOT NULL, PRIMARY KEY (prefix, url)) WITHOUT ROWID;
CREATE TABLE asns (first INTEGER NOT NULL, last INTEGER NOT NULL, url TEXT NOT NULL, PRIMARY KEY (first, url)) WITHOUT ROWID;`,
		Build:  buildRDAP,
		Import: importRDAP,
		Every:  7 * 24 * time.Hour,
		Main:   "tlds", MaxShrink: 0.1,
	})
}

// rdapFiles are the four bootstrap registries, as IANA names them.
var rdapFiles = []string{"dns", "ipv4", "ipv6", "asn"}

type rdapBootstrap struct {
	Publication string       `json:"publication"`
	Services    [][][]string `json:"services"`
}

// rdapServer picks the HTTPS address among a service's servers; RFC 9224 lets a
// registry list plain HTTP ones too, which are not used.
func rdapServer(urls []string) string {
	for _, address := range urls {
		if strings.HasPrefix(address, "https://") {
			if !strings.HasSuffix(address, "/") {
				address += "/"
			}
			return address
		}
	}
	return ""
}

func readRDAPBootstrap(b *Builder, name string) (rdapBootstrap, error) {
	var bootstrap rdapBootstrap
	raw, err := os.ReadFile(b.Path("rdap", name+".json"))
	if err != nil {
		return bootstrap, err
	}
	if err := json.Unmarshal(raw, &bootstrap); err != nil {
		return bootstrap, fmt.Errorf("%s.json: %w", name, err)
	}
	for _, service := range bootstrap.Services {
		if len(service) != 2 {
			return bootstrap, fmt.Errorf("%s.json: a service has %d parts, not 2", name, len(service))
		}
	}
	return bootstrap, nil
}

func buildRDAP(b *Builder) error {
	for _, name := range rdapFiles {
		bootstrap, err := readRDAPBootstrap(b, name)
		if err != nil {
			return err
		}
		for _, service := range bootstrap.Services {
			server := rdapServer(service[1])
			if server == "" {
				continue
			}
			for _, key := range service[0] {
				key = strings.ToLower(strings.TrimSpace(key))
				switch name {
				case "dns":
					if _, err := b.Tx.Exec(`INSERT OR IGNORE INTO tlds VALUES (?, ?)`, key, server); err != nil {
						return err
					}
					b.Add("tlds", 1)
				case "ipv4", "ipv6":
					prefix, err := netip.ParsePrefix(key)
					if err != nil {
						return fmt.Errorf("%s.json: %q is not a network", name, key)
					}
					if _, err := b.Tx.Exec(`INSERT OR IGNORE INTO networks VALUES (?, ?, ?)`, map[bool]int{true: 4, false: 6}[prefix.Addr().Is4()], prefix.Masked().String(), server); err != nil {
						return err
					}
					b.Add("networks", 1)
				case "asn":
					first, last, ok := strings.Cut(key, "-")
					if !ok {
						last = first
					}
					low, err1 := strconv.ParseUint(first, 10, 32)
					high, err2 := strconv.ParseUint(last, 10, 32)
					if err1 != nil || err2 != nil || high < low {
						return fmt.Errorf("asn.json: %q is not a range of AS numbers", key)
					}
					if _, err := b.Tx.Exec(`INSERT OR IGNORE INTO asns VALUES (?, ?, ?)`, low, high, server); err != nil {
						return err
					}
					b.Add("asns", 1)
				}
			}
		}
	}
	return nil
}

const rdapBootstrapBase = "https://data.iana.org/rdap/"

func importRDAP(im *Importer) (Source, error) {
	files := map[string][]byte{}
	var versions []string
	for _, name := range rdapFiles {
		raw, err := im.Get(rdapBootstrapBase + name + ".json")
		if err != nil {
			return Source{}, err
		}
		var bootstrap rdapBootstrap
		if err := json.Unmarshal(raw, &bootstrap); err != nil || len(bootstrap.Services) == 0 {
			return Source{}, fmt.Errorf("%s.json is not an RDAP bootstrap file", name)
		}
		files[name] = raw
		versions = append(versions, digest(raw)[:8])
	}
	if dns := files["dns"]; len(dns) < 10_000 {
		return Source{}, fmt.Errorf("dns.json has only %d bytes, expected tens of thousands", len(dns))
	}
	version := strings.Join(versions, "-")
	if im.CurrentVersion() == version {
		return Source{}, ErrNotChanged
	}
	for _, name := range rdapFiles {
		if err := im.Write("rdap/"+name+".json", files[name]); err != nil {
			return Source{}, err
		}
	}
	return Source{
		Name:        "RDAP bootstrap registries",
		Publisher:   "Internet Assigned Numbers Authority (IANA)",
		URL:         "https://www.iana.org/assignments/rdap-dns/",
		Licence:     "CC0 1.0",
		LicenceURL:  "https://www.iana.org/help/licensing-terms",
		Version:     version,
		Attribution: "Which registry answers for each domain, network and AS number, from IANA's RDAP bootstrap registries (CC0). Registration details come live from that registry.",
	}, nil
}
