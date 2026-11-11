package api

import (
	"net/http"
	"testing"
)

func init() {
	mcpTestCalls["list_index_constituents"] = `{"index":"nifty-bank"}`
}

func TestIndices(t *testing.T) {
	bank := expectStatus(t, "/v1/indices/nifty-bank", http.StatusOK).data(t)
	if bank["name"] != "Nifty Bank" || len(bank["members"].([]any)) < 10 {
		t.Errorf("bank %v", bank)
	}
	expectStatus(t, "/v1/indices/niftybank", http.StatusOK)
	of := expectStatus(t, "/v1/indices?symbol=AXISBANK", http.StatusOK).json(t)["data"].([]any)
	if len(of) != 2 {
		t.Errorf("AXISBANK is in %v", of)
	}
	expectStatus(t, "/v1/indices/nifty-moon", http.StatusNotFound)
	expectStatus(t, "/v1/indices?symbol=NOSUCH", http.StatusNotFound)
}
