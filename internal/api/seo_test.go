package api

import "testing"

func TestSEOTextsNameRealPages(t *testing.T) {
	for id, text := range seoTexts {
		if _, ok := groupByID(id); !ok {
			t.Errorf("search text for unknown API %s", id)
		}
		if len(text.Description) > 160 || len(text.Description) < 70 {
			t.Errorf("%s: description is %d characters", id, len(text.Description))
		}
	}
}
