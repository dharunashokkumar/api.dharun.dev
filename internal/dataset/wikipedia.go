package dataset

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
)

// wikipedia fetches the wikitext of an English Wikipedia article at a fixed revision, so
// the data can be credited to an exact version. The revision is im.Version when given,
// otherwise the latest. With im.File the wikitext is read from a file instead, and
// im.Version must name the revision it came from.
func (im *Importer) wikipedia(title string) (wikitext []byte, revision string, err error) {
	revision = im.Version
	if im.File != "" {
		if revision == "" {
			return nil, "", errors.New("-file needs -version <revision id> so the source can be credited exactly")
		}
		wikitext, err = os.ReadFile(im.File)
		return wikitext, revision, err
	}
	if revision == "" {
		raw, err := im.Get("https://en.wikipedia.org/w/api.php?action=query&prop=revisions&rvprop=ids&format=json&titles=" + url.QueryEscape(title))
		if err != nil {
			return nil, "", err
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
			return nil, "", err
		}
		for _, page := range result.Query.Pages {
			if len(page.Revisions) > 0 {
				revision = fmt.Sprint(page.Revisions[0].ID)
			}
		}
		if revision == "" {
			return nil, "", fmt.Errorf("could not find the latest revision of %q", title)
		}
	}
	wikitext, err = im.Get("https://en.wikipedia.org/w/index.php?action=raw&title=" + url.QueryEscape(title) + "&oldid=" + revision)
	return wikitext, revision, err
}

// wikipediaURL links to an article at a revision.
func wikipediaURL(title, revision string) string {
	return "https://en.wikipedia.org/w/index.php?title=" + url.QueryEscape(title) + "&oldid=" + revision
}
