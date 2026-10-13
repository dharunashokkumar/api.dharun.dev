package api

import (
	"net/http"
	"testing"
)

func init() {
	mcpTestCalls["get_mutual_fund_nav"] = `{"code":"122639"}`
	mcpTestCalls["search_mutual_funds"] = `{"query":"parag flexi","plan":"direct"}`
}

func TestMutualFunds(t *testing.T) {
	scheme := expectStatus(t, "/v1/mutual-funds/122639", http.StatusOK).data(t)
	if scheme["name"] != "Parag Parikh Flexi Cap Fund" || scheme["fundHouse"] != "PPFAS Mutual Fund" || scheme["plan"] != "direct" ||
		scheme["option"] != "growth" || scheme["active"] != true || scheme["nav"].(float64) <= 0 || scheme["navDate"] == nil {
		t.Errorf("122639: %v", scheme)
	}
	isin, _ := scheme["isinGrowth"].(string)
	if byISIN := expectStatus(t, "/v1/mutual-funds/"+isin, http.StatusOK).data(t); byISIN["code"].(float64) != 122639 {
		t.Errorf("by ISIN %s: %v", isin, byISIN["code"])
	}

	found := expectStatus(t, "/v1/mutual-funds?q=flexi%20parag&plan=direct&option=growth", http.StatusOK).list(t)
	if len(found) != 1 || found[0].(map[string]any)["code"].(float64) != 122639 {
		t.Errorf("words in any order with filters: %v", found)
	}
	// Wound-up schemes only appear when asked for.
	if active := expectStatus(t, "/v1/mutual-funds?q=low%20duration", http.StatusOK).list(t); len(active) != 0 {
		t.Errorf("a wound-up scheme was listed as active: %v", active)
	}
	if all := expectStatus(t, "/v1/mutual-funds?q=low%20duration&all=true", http.StatusOK).list(t); len(all) == 0 || all[0].(map[string]any)["active"] != false {
		t.Errorf("all=true: %v", all)
	}
	houses := expectStatus(t, "/v1/mutual-funds/fund-houses", http.StatusOK).list(t)
	if len(houses) < 3 {
		t.Errorf("fund houses: %v", houses)
	}
	expectStatus(t, "/v1/mutual-funds/999999999", http.StatusNotFound)
	expectStatus(t, "/v1/mutual-funds/abc", http.StatusBadRequest)
	expectStatus(t, "/v1/mutual-funds", http.StatusBadRequest)
	expectStatus(t, "/v1/mutual-funds?q=hdfc&plan=both", http.StatusBadRequest)
}
