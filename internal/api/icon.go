package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"golang.org/x/net/html"

	"github.com/dharunashokkumar/api.dharun.dev/internal/netguard"
)

// The icon a website gives itself (its favicon, Apple touch icon or web app manifest
// icon), fetched from the site and drawn again here as a square PNG, so what is served
// under this domain is always a fresh picture and never the site's own file. SVG icons
// are not used. A site without a usable icon gets the initials tile.

const (
	iconMinSize     = 16
	iconMaxSize     = 256
	iconDefaultSize = 64
	iconPageLimit   = 512 << 10
	iconFileLimit   = 1 << 20
	iconMaxPixels   = 2048
)

var errNoIcon = errors.New("the site has no usable icon")

type iconCandidate struct {
	URL  string
	Size int // the declared edge length; 0 when unknown
}

func (s *Server) handleIcon(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSuffix(r.PathValue("domain"), ".png")
	host, err := hostOf(raw)
	if _, notIP := netip.ParseAddr(host); err == nil && notIP == nil {
		err = errBadName
	}
	if err != nil {
		s.badRequest(w, r, "Give a domain name, such as example.co.in.")
		return
	}
	info(r).subject = host
	size := iconDefaultSize
	if text := strings.TrimSpace(r.URL.Query().Get("size")); text != "" {
		value, err := strconv.Atoi(text)
		if err != nil || value < iconMinSize || value > iconMaxSize {
			s.badRequest(w, r, "size is from "+itoa(iconMinSize)+" to "+itoa(iconMaxSize)+" pixels.")
			return
		}
		size = value
	}
	if format := r.URL.Query().Get("format"); format != "" && format != "png" {
		s.badRequest(w, r, "Icons come as PNG only.")
		return
	}

	master, cached := s.png.peek("icon:" + host)
	if !cached {
		if !s.chargeMore(w, r, costProbe-costLight) {
			return
		}
		release, ok := s.startCheck(w, r, host)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		source, picture, err := s.fetchIcon(ctx, host)
		cancel()
		release()
		switch {
		case errors.Is(err, errNoIcon):
			// Known to have none: remembered as an empty entry, so the site is not asked again.
			s.png.get("icon:"+host, func() ([]byte, error) { return []byte{}, nil })
		case err != nil:
			w.Header().Set("X-Icon-Error", "unreachable")
		default:
			master, _ = s.png.get("icon:"+host, func() ([]byte, error) { return iconMaster(picture) })
			w.Header().Set("X-Icon-Source", source)
		}
	}
	if len(master) == 0 {
		registered, _ := registeredDomain(host)
		label, _, _ := strings.Cut(registered, ".")
		s.serveMonogram(w, r, label, "png", size)
		return
	}
	sized, err := s.png.get("icon:"+host+"|"+itoa(size), func() ([]byte, error) { return resizePNG(master, size) })
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Could not draw the icon.")
		return
	}
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	writeBody(w, r, http.StatusOK, sized, typePNG, cacheLong, false)
}

// iconMaster draws the picture as a square PNG of at most iconMaxSize, centred on a
// transparent ground when it is not square.
func iconMaster(picture image.Image) ([]byte, error) {
	bounds := picture.Bounds()
	edge := min(max(bounds.Dx(), bounds.Dy()), iconMaxSize)
	scale := float64(edge) / float64(max(bounds.Dx(), bounds.Dy()))
	width, height := max(1, int(float64(bounds.Dx())*scale+0.5)), max(1, int(float64(bounds.Dy())*scale+0.5))
	square := image.NewNRGBA(image.Rect(0, 0, edge, edge))
	target := image.Rect((edge-width)/2, (edge-height)/2, (edge-width)/2+width, (edge-height)/2+height)
	xdraw.CatmullRom.Scale(square, target, picture, bounds, draw.Over, nil)
	return encodePNG(square)
}

// fetchIcon reads the site's home page for the icons it names, then tries them from
// the most promising, ending with /favicon.ico.
func (s *Server) fetchIcon(ctx context.Context, host string) (string, image.Image, error) {
	client := s.guard(80, 443).Client(netguard.ClientOptions{Timeout: 8 * time.Second, Roots: s.extras.network.roots, Redirects: 5, UserAgent: probeAgent})
	var page []byte
	var base *url.URL
	var lastErr error
	for _, scheme := range []string{"https", "http"} {
		// Icons are named in the head, so a long page is read only as far as the limit.
		body, final, err := iconGet(ctx, client, scheme+"://"+host+"/", iconPageLimit, true)
		if err == nil {
			page, base = body, final
			break
		}
		lastErr = err
	}
	if base == nil {
		// No readable home page (a refusal, an error page): /favicon.ico may still be there.
		favicon := "https://" + host + "/favicon.ico"
		if body, _, err := iconGet(ctx, client, favicon, iconFileLimit, false); err == nil {
			if picture, err := decodeIcon(body); err == nil {
				return favicon, picture, nil
			}
		}
		return "", nil, lastErr
	}
	candidates := iconLinks(page, base)
	for _, candidate := range candidates {
		if candidate.Manifest {
			if body, final, err := iconGet(ctx, client, candidate.URL, 64<<10, false); err == nil {
				candidates = append(candidates, manifestIcons(body, final)...)
			}
		}
	}
	favicon, _ := base.Parse("/favicon.ico")
	list := []iconCandidate{}
	for _, candidate := range candidates {
		if !candidate.Manifest && !strings.HasSuffix(strings.ToLower(candidate.path()), ".svg") && !candidate.SVG {
			list = append(list, iconCandidate{URL: candidate.URL, Size: candidate.Size})
		}
	}
	// The smallest icon at least as large as the largest size served, else the largest.
	sort.SliceStable(list, func(a, b int) bool {
		x, y := list[a], list[b]
		if (x.Size >= iconMaxSize) != (y.Size >= iconMaxSize) {
			return x.Size >= iconMaxSize
		}
		if x.Size >= iconMaxSize {
			return x.Size < y.Size
		}
		return x.Size > y.Size
	})
	list = append(list, iconCandidate{URL: favicon.String()})
	seen := map[string]bool{}
	tries := 0
	for _, candidate := range list {
		if seen[candidate.URL] || tries >= 4 {
			continue
		}
		seen[candidate.URL] = true
		tries++
		body, _, err := iconGet(ctx, client, candidate.URL, iconFileLimit, false)
		if err != nil {
			continue
		}
		if picture, err := decodeIcon(body); err == nil {
			return candidate.URL, picture, nil
		}
	}
	return "", nil, errNoIcon
}

// iconGet fetches one address, refusing anything larger than limit, or with cut, keeping
// only the first limit bytes.
func iconGet(ctx context.Context, client *http.Client, address string, limit int64, cut bool) ([]byte, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, errors.New("status " + strconv.Itoa(resp.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, nil, err
	}
	if int64(len(body)) > limit {
		if !cut {
			return nil, nil, errors.New("too large")
		}
		body = body[:limit]
	}
	return body, resp.Request.URL, nil
}

// decodeIcon decodes a raster icon after checking its dimensions, so a small file that
// claims an enormous picture is not unpacked.
func decodeIcon(body []byte) (image.Image, error) {
	if isICO(body) {
		return decodeICO(body)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > iconMaxPixels || config.Height > iconMaxPixels {
		return nil, errors.New("picture too large")
	}
	picture, _, err := image.Decode(bytes.NewReader(body))
	return picture, err
}

type iconLink struct {
	URL      string
	Size     int
	SVG      bool
	Manifest bool
}

func (l iconLink) path() string {
	if parsed, err := url.Parse(l.URL); err == nil {
		return parsed.Path
	}
	return l.URL
}

// iconLinks lists the icons and the manifest a page names in its <link> elements.
func iconLinks(page []byte, base *url.URL) []iconLink {
	var links []iconLink
	tokens := html.NewTokenizer(bytes.NewReader(page))
	for {
		kind := tokens.Next()
		if kind == html.ErrorToken {
			return links
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		name, hasAttr := tokens.TagName()
		if string(name) == "body" {
			return links
		}
		if string(name) != "link" || !hasAttr {
			continue
		}
		attrs := map[string]string{}
		for {
			key, value, more := tokens.TagAttr()
			attrs[strings.ToLower(string(key))] = strings.TrimSpace(string(value))
			if !more {
				break
			}
		}
		rel := " " + strings.ToLower(attrs["rel"]) + " "
		href, err := base.Parse(attrs["href"])
		if attrs["href"] == "" || err != nil || (href.Scheme != "http" && href.Scheme != "https") {
			continue
		}
		switch {
		case strings.Contains(rel, " manifest "):
			links = append(links, iconLink{URL: href.String(), Manifest: true})
		case strings.Contains(rel, " mask-icon "):
		case strings.Contains(rel, "icon"):
			size := largestSize(attrs["sizes"])
			if size == 0 && strings.Contains(rel, "apple-touch-icon") {
				size = 180 // the size Apple asks for when none is given
			}
			links = append(links, iconLink{URL: href.String(), Size: size, SVG: strings.Contains(attrs["type"], "svg")})
		}
	}
}

// manifestIcons lists the icons of a web app manifest.
func manifestIcons(body []byte, base *url.URL) []iconLink {
	var manifest struct {
		Icons []struct {
			Src     string `json:"src"`
			Sizes   string `json:"sizes"`
			Type    string `json:"type"`
			Purpose string `json:"purpose"`
		} `json:"icons"`
	}
	if json.Unmarshal(body, &manifest) != nil {
		return nil
	}
	var links []iconLink
	for _, icon := range manifest.Icons {
		if strings.Contains(icon.Purpose, "monochrome") {
			continue
		}
		href, err := base.Parse(icon.Src)
		if icon.Src == "" || err != nil || (href.Scheme != "http" && href.Scheme != "https") {
			continue
		}
		links = append(links, iconLink{URL: href.String(), Size: largestSize(icon.Sizes), SVG: strings.Contains(icon.Type, "svg")})
	}
	return links
}

// largestSize reads a sizes attribute such as "16x16 32x32" and returns the largest edge.
func largestSize(sizes string) int {
	largest := 0
	for _, size := range strings.Fields(strings.ToLower(sizes)) {
		width, height, ok := strings.Cut(size, "x")
		w, err1 := strconv.Atoi(width)
		h, err2 := strconv.Atoi(height)
		if ok && err1 == nil && err2 == nil {
			largest = max(largest, min(w, h))
		}
	}
	return largest
}

func init() {
	mcpTools = append(mcpTools, mcpTool{
		Name:        "domain_icon",
		Description: "Get the icon of any website as a square PNG, from its favicon, Apple touch icon or web app manifest, redrawn at the size asked. A site without a usable icon gets an initials tile.",
		InputSchema: schema([]string{"domain"}, map[string]any{
			"domain": text("Domain name, such as github.com."),
			"size":   map[string]any{"type": "integer", "description": "Edge length in pixels, 16 to 256. Default 64."},
		}),
		path: func(args map[string]any) (string, error) {
			domain, err := required(args, "domain")
			return "/v1/icon/" + url.PathEscape(domain) + query(args, map[string]string{"size": "size"}), err
		},
	})
}
