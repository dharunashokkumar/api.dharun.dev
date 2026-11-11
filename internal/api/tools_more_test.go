package api

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// Every tool page renders, names APIs that exist, and its form's address matches an
// endpoint in the catalog.
func TestToolPagesPointAtRealEndpoints(t *testing.T) {
	paths := map[string]bool{}
	for _, g := range catalog {
		for _, e := range g.Endpoints {
			paths[e.Path] = true
		}
	}
	get := regexp.MustCompile(`data-get="([^"]+)"`)
	for _, tl := range tools {
		for _, id := range tl.Uses {
			if _, ok := groupByID(id); !ok {
				t.Errorf("tool %s uses unknown API %s", tl.Slug, id)
			}
		}
		if m := get.FindStringSubmatch(tl.UI); m != nil && !paths[strings.SplitN(m[1], "?", 2)[0]] {
			t.Errorf("tool %s posts to %s, which is not in the catalog", tl.Slug, m[1])
		}
		got := expectStatus(t, "/tools/"+tl.Slug, http.StatusOK)
		if !strings.Contains(string(got.body), tl.Heading) {
			t.Errorf("tool %s page lacks its heading", tl.Slug)
		}
	}
	if len(tools) < 30 {
		t.Errorf("%d tools", len(tools))
	}
}
