package api

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

func init() {
	mcpTestCalls["lookup_ip_address"] = `{"ip":"49.204.0.1"}`
}

// The records copy what DB-IP's real files say about these ranges (October 2026).
var ipFixture = []struct {
	network, city, region, country, countryName string
	lat, lng                                    float64
	asn                                         uint32
	organization                                string
}{
	{"49.204.0.0/16", "Hyderabad (NBT Nagar)", "Telangana", "IN", "India", 17.4089, 78.4346, 55577, "Atria Convergence Technologies Pvt. Ltd.,"},
	{"8.8.8.0/24", "Mountain View", "California", "US", "United States", 37.422, -122.085, 15169, "Google LLC"},
	{"2401:4900::/32", "Patna (Police Colony)", "Bihar", "IN", "India", 25.5819, 85.0818, 0, ""},
}

func writeMMDB(t interface{ Fatal(...any) }, path, kind string) {
	tree, err := mmdbwriter.New(mmdbwriter.Options{DatabaseType: map[string]string{"city": "DBIP-City-Lite", "asn": "DBIP-ASN-Lite"}[kind], RecordSize: 28})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range ipFixture {
		_, network, _ := net.ParseCIDR(f.network)
		var record mmdbtype.Map
		if kind == "city" {
			record = mmdbtype.Map{
				"city":         mmdbtype.Map{"names": mmdbtype.Map{"en": mmdbtype.String(f.city)}},
				"continent":    mmdbtype.Map{"code": mmdbtype.String(map[string]string{"IN": "AS", "US": "NA"}[f.country])},
				"country":      mmdbtype.Map{"iso_code": mmdbtype.String(f.country), "names": mmdbtype.Map{"en": mmdbtype.String(f.countryName)}},
				"location":     mmdbtype.Map{"latitude": mmdbtype.Float64(f.lat), "longitude": mmdbtype.Float64(f.lng)},
				"subdivisions": mmdbtype.Slice{mmdbtype.Map{"names": mmdbtype.Map{"en": mmdbtype.String(f.region)}}},
			}
		} else if f.asn != 0 {
			record = mmdbtype.Map{"autonomous_system_number": mmdbtype.Uint32(f.asn), "autonomous_system_organization": mmdbtype.String(f.organization)}
		} else {
			continue
		}
		if err := tree.Insert(network, record); err != nil {
			t.Fatal(err)
		}
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := tree.WriteTo(file); err != nil {
		t.Fatal(err)
	}
}

type panicker struct{}

func (panicker) Fatal(args ...any) { panic(args) }

// buildIPDataset writes the fixture as a server would after a refresh.
func buildIPDataset(t interface{ Fatal(...any) }, liveDir string) {
	source, err := os.MkdirTemp("", "ip-source-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(source)
	writeMMDB(t, filepath.Join(source, "ip", "city.mmdb"), "city")
	writeMMDB(t, filepath.Join(source, "ip", "asn.mmdb"), "asn")
	spec, _ := dataset.Lookup("ip")
	if _, _, err := dataset.Build(spec, dataset.BuildOptions{
		Dir: source, Out: filepath.Join(liveDir, dataset.LiveName("ip", time.Now())),
		Source: dataset.Source{Name: "DB-IP Lite (test)", Licence: "CC BY 4.0", Version: "2026-10", Retrieved: time.Now().UTC().Format(time.RFC3339),
			Attribution: "IP Geolocation by DB-IP (https://db-ip.com), licensed under CC BY 4.0."},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestIPLookup(t *testing.T) {
	hyderabad := expectStatus(t, "/v1/ip/49.204.0.1", http.StatusOK)
	data := hyderabad.data(t)
	if data["stateCode"] != "TS" || data["region"] != "Telangana" || data["country"].(map[string]any)["code"] != "IN" || data["version"].(float64) != 4 {
		t.Errorf("49.204.0.1: %v", data)
	}
	if asn := data["asn"].(map[string]any); asn["number"].(float64) != 55577 || strings.HasSuffix(asn["organization"].(string), ",") {
		t.Errorf("network: %v", asn)
	}
	if hyderabad.header.Get("Cache-Control") != "private, no-store" {
		t.Errorf("an IP lookup must not be shared-cached: %q", hyderabad.header.Get("Cache-Control"))
	}
	if source := hyderabad.json(t)["meta"].(map[string]any)["source"].(map[string]any); !strings.Contains(source["attribution"].(string), "DB-IP") {
		t.Errorf("CC BY needs the DB-IP credit: %v", source)
	}

	google := expectStatus(t, "/v1/ip/8.8.8.8", http.StatusOK).data(t)
	if google["stateCode"] != nil || google["country"].(map[string]any)["code"] != "US" {
		t.Errorf("8.8.8.8: %v", google)
	}
	v6 := expectStatus(t, "/v1/ip/2401:4900::1", http.StatusOK).data(t)
	if v6["version"].(float64) != 6 || v6["stateCode"] != "BR" || v6["asn"] != nil {
		t.Errorf("2401:4900::1: %v", v6)
	}
	if mapped := expectStatus(t, "/v1/ip/::ffff:49.204.0.1", http.StatusOK).data(t); mapped["ip"] != "49.204.0.1" {
		t.Errorf("a mapped IPv4 address should be read as IPv4: %v", mapped["ip"])
	}

	for _, private := range []string{"10.1.2.3", "192.168.1.250", "127.0.0.1", "100.64.0.1", "fe80::1", "::1", "203.0.113.9"} {
		expectStatus(t, "/v1/ip/"+private, http.StatusUnprocessableEntity)
	}
	expectStatus(t, "/v1/ip/999.1.1.1", http.StatusBadRequest)
	expectStatus(t, "/v1/ip/1.1.1.1", http.StatusNotFound) // not in the fixture

	// Your own address: behind the trusted proxy it is the one the proxy reports.
	if mine := request(t, http.MethodGet, "/v1/ip", "", "CF-Connecting-IP", "8.8.8.8"); mine.status != http.StatusOK || mine.data(t)["ip"] != "8.8.8.8" {
		t.Errorf("own address: %d %s", mine.status, mine.body)
	}
	expectStatus(t, "/v1/ip", http.StatusUnprocessableEntity) // the test client is on loopback
}

func TestIPWithoutTheDataset(t *testing.T) {
	server := newTestServer(Config{Version: "t", SeedDir: copySeed(t, "rto")})
	defer server.Close()
	if recorder := serve(server, http.MethodGet, "/v1/ip/8.8.8.8"); recorder.Code != http.StatusServiceUnavailable {
		t.Errorf("no ip dataset: %d", recorder.Code)
	}
}
