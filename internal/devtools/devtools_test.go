package devtools

import (
	"image"
	"image/color"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestConvert(t *testing.T) {
	v, err := Decode("yaml", []byte("name: India API\nport: 8080\ntags: [a, b]\n"))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := Encode("json", v, false)
	if string(out) != `{"name":"India API","port":8080,"tags":["a","b"]}`+"\n" {
		t.Errorf("yaml to json %s", out)
	}
	toml, err := Encode("toml", v, false)
	if err != nil || !strings.Contains(string(toml), "port = 8080") {
		t.Errorf("toml %s %v", toml, err)
	}
	rows, _ := Decode("csv", []byte("pincode,district\n637001,Namakkal\n600001,Chennai\n"))
	if got, _ := Encode("json", rows, false); !strings.Contains(string(got), `{"district":"Namakkal","pincode":"637001"}`) {
		t.Errorf("csv %s", got)
	}
	back, _ := Encode("csv", rows, false)
	if string(back) != "district,pincode\nNamakkal,637001\nChennai,600001\n" {
		t.Errorf("csv out %q", back)
	}
	x, err := Decode("xml", []byte(`<shop id="7"><item>rice</item><item>dal</item><city>Salem</city></shop>`))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := Encode("json", x, false); string(got) != `{"shop":{"@id":"7","city":"Salem","item":["rice","dal"]}}`+"\n" {
		t.Errorf("xml %s", got)
	}
	if xml, _ := Encode("xml", x, false); !strings.Contains(string(xml), `<shop id="7">`) || strings.Count(string(xml), "<item>") != 2 {
		t.Errorf("xml out %s", xml)
	}
	if _, err := Decode("json", []byte("{")); err == nil {
		t.Error("broken JSON decoded")
	}
}

func TestDiff(t *testing.T) {
	d, err := DiffText("a\nb\nc\nd\n", "a\nB\nc\nd\ne\n", 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if d.Added != 2 || d.Removed != 1 || !strings.Contains(d.Patch, "-b\n+B\n") || !strings.Contains(d.Patch, "+e\n") {
		t.Errorf("%+v", d)
	}
	same, _ := DiffText("x", "x", 3, false)
	if !same.Same {
		t.Error("same texts differ")
	}
	changes := DiffJSON(map[string]any{"a": 1.0, "b": []any{1.0, 2.0}, "c/d": "x"}, map[string]any{"a": 2.0, "b": []any{1.0}, "e": true, "c/d": "x"})
	want := []string{"replace /a", "remove /b/1", "add /e"}
	if len(changes) != len(want) {
		t.Fatalf("%+v", changes)
	}
	for i, c := range changes {
		if c.Op+" "+c.Path != want[i] {
			t.Errorf("change %d = %s %s", i, c.Op, c.Path)
		}
	}
}

func TestCron(t *testing.T) {
	ist, _ := time.LoadLocation("Asia/Kolkata")
	c, err := ParseCron("30 9 * * 1-5")
	if err != nil {
		t.Fatal(err)
	}
	if c.Describe() != "At 09:30, on Monday, Tuesday, Wednesday, Thursday and Friday." {
		t.Errorf("describe %q", c.Describe())
	}
	from := time.Date(2026, 11, 13, 10, 0, 0, 0, ist) // a Friday after 9:30
	next := c.Next(from, 2)
	if len(next) != 2 || next[0].Format(time.RFC3339) != "2026-11-16T09:30:00+05:30" || next[1].Format(time.RFC3339) != "2026-11-17T09:30:00+05:30" {
		t.Errorf("next %v", next)
	}
	every, _ := ParseCron("*/15 * * * *")
	if every.Describe() != "Every 15 minutes." {
		t.Errorf("%q", every.Describe())
	}
	both, _ := ParseCron("0 0 13 * 5") // the 13th or any Friday
	if got := both.Next(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), 2); got[0].Day() != 6 || got[1].Day() != 13 {
		t.Errorf("either day %v", got)
	}
	if m, _ := ParseCron("@daily"); m.Describe() != "At 00:00." {
		t.Errorf("macro %q", m.Describe())
	}
	for _, bad := range []string{"* * *", "60 * * * *", "* * * * * *", "5-1 * * * *"} {
		if _, err := ParseCron(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

func TestIDsAndHashes(t *testing.T) {
	now := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	for _, kind := range IDKinds {
		id, err := NewID(kind, now)
		if err != nil || id == "" {
			t.Fatalf("%s: %v", kind, err)
		}
		if kind == "uuid7" || kind == "ulid" {
			info, err := ReadID(id)
			if err != nil || info.Time != "2026-11-11T10:00:00Z" {
				t.Errorf("%s %s: %+v %v", kind, id, info, err)
			}
		}
	}
	if info, _ := ReadID("c232ab00-9414-11ec-b3c8-9f6bdeced846"); info.Version != 1 || info.Time != "2022-02-22T19:22:22Z" {
		t.Errorf("uuid1 %+v", info)
	}
	sums, err := Hash([]byte("abc"), []string{"sha256", "md5"}, nil)
	if err != nil || sums["sha256"].Hex != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" || sums["md5"].Hex != "900150983cd24fb0d6963f7d28e17f72" {
		t.Errorf("%v %v", sums, err)
	}
	mac, _ := Hash([]byte("The quick brown fox jumps over the lazy dog"), []string{"sha256"}, []byte("key"))
	if mac["sha256"].Hex != "f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8" {
		t.Errorf("hmac %v", mac)
	}
}

func TestJWT(t *testing.T) {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJleHAiOjE1MTYyMzk5MjJ9.signature"
	j, err := DecodeJWT("Bearer "+token, time.Now())
	if err != nil || j.Header["alg"] != "HS256" || j.Payload["name"] != "John Doe" || j.Expired == nil || !*j.Expired || j.Times["iat"] != "2018-01-18T01:30:22Z" {
		t.Errorf("%+v %v", j, err)
	}
	if _, err := DecodeJWT("abc", time.Now()); err == nil {
		t.Error("abc decoded")
	}
}

func TestTimeZones(t *testing.T) {
	ist, err := Zone("IST")
	if err != nil {
		t.Fatal(err)
	}
	at, err := ParseTime("2026-11-11 15:00", ist, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	berlin, _ := Zone("Europe/Berlin")
	got := In(at, berlin, "Europe/Berlin", at)
	if got.Local != "Wed 11 Nov 2026, 10:30" || got.Offset != "+01:00" {
		t.Errorf("%+v", got)
	}
	if z, err := Zone("+05:45"); err != nil || In(at, z, "+05:45", at).Local != "Wed 11 Nov 2026, 15:15" {
		t.Errorf("offset zone %v", err)
	}
	if _, err := Zone("Mars/Base"); err == nil {
		t.Error("unknown zone found")
	}
}

func TestMarkdown(t *testing.T) {
	page, err := HTMLToMarkdown(`<html lang="en"><head><title>Rice</title><meta name="description" content="About rice"></head><body>
<nav><a href="/">Home</a></nav><main><h1>Rice  prices</h1><p>Sona <b>masoori</b> is <a href="/sona">here</a>.</p>
<ul><li>One</li><li>Two<ul><li>Two a</li></ul></li></ul><pre><code class="language-go">x := 1
</code></pre><table><tr><th>Kind</th><th>Rs</th></tr><tr><td>Ponni</td><td>60</td></tr></table></main><footer>c</footer></body></html>`, mustURL("https://example.com/a/"), true)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Rice prices\n\nSona **masoori** is [here](https://example.com/sona).\n\n- One\n- Two\n  - Two a\n\n```go\nx := 1\n```\n\n| Kind | Rs |\n|---|---|\n| Ponni | 60 |"
	if page.Markdown != want || page.Title != "Rice" || page.Description != "About rice" || page.Language != "en" {
		t.Errorf("got\n%s\n%+v", page.Markdown, page)
	}
	html, _ := MarkdownToHTML("# Hi\n\n<script>x</script>\n\n| a |\n|---|\n| 1 |")
	if strings.Contains(html, "<script>") || !strings.Contains(html, "<table>") {
		t.Errorf("html %s", html)
	}
}

func TestColours(t *testing.T) {
	c, err := ParseColor("#2563eb")
	if err != nil {
		t.Fatal(err)
	}
	if h, s, l := c.HSL(); h != 221.2 || s != 83.2 || l != 53.3 {
		t.Errorf("hsl %v %v %v", h, s, l)
	}
	if got, _ := ParseColor("hsl(221.2, 83.2%, 53.3%)"); got.Hex() != "#2563eb" {
		t.Errorf("from hsl %s", got.Hex())
	}
	white, _ := ParseColor("white")
	black, _ := ParseColor("#000")
	if r := ContrastOf(black, white); r.Ratio != 21 || !r.Passes["AAA"] {
		t.Errorf("%+v", r)
	}
	grey, _ := ParseColor("#999999")
	if r := ContrastOf(grey, white); r.Passes["AA"] || r.Suggestion == "" {
		t.Errorf("grey %+v", r)
	}
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			if x < 30 {
				img.Set(x, y, color.RGBA{255, 153, 51, 255})
			} else {
				img.Set(x, y, color.RGBA{19, 136, 8, 255})
			}
		}
	}
	p := Palette(img, 4)
	if p[0].Hex != "#ff9933" || p[0].Share != 0.75 || p[1].Hex != "#138808" {
		t.Errorf("palette %+v", p)
	}
}

func TestThumbHash(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := range img.Pix {
		if i%4 == 0 || i%4 == 3 {
			img.Pix[i] = 255
		}
	}
	h, err := ThumbHash(img)
	if err != nil {
		t.Fatal(err)
	}
	// Solid red: l = 1/3, p = 1/2, q = 1, and no detail.
	header24 := 21 | 47<<6 | 63<<12
	if h[0] != byte(header24) || h[1] != byte(header24>>8) || h[2] != byte(header24>>16) {
		t.Errorf("header % x", h[:3])
	}
	if _, err := ThumbHash(image.NewRGBA(image.Rect(0, 0, 101, 5))); err == nil {
		t.Error("a large picture was hashed")
	}
}

func mustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
