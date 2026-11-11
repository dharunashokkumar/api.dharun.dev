package api

import (
	"net/http"
	"testing"
)

func init() {
	mcpTestCalls["find_law_section"] = `{"law":"ipc","section":"302"}`
}

func TestLaw(t *testing.T) {
	got := expectStatus(t, "/v1/law/ipc/302", http.StatusOK).json(t)["data"].([]any)
	if len(got) != 1 || got[0].(map[string]any)["reference"] != "BNS 103(1)" {
		t.Errorf("IPC 302: %v", got)
	}
	murder := expectStatus(t, "/v1/law/bns/103", http.StatusOK).json(t)["data"].([]any)
	if len(murder) != 2 || murder[1].(map[string]any)["new_provision"] != true {
		t.Errorf("BNS 103: %v", murder)
	}
	if got := expectStatus(t, "/v1/law/ipc/420", http.StatusOK).json(t)["data"].([]any); got[0].(map[string]any)["reference"] != "BNS 318(4)" {
		t.Errorf("IPC 420: %v", got)
	}
	if got := expectStatus(t, "/v1/law/crpc/154", http.StatusOK).json(t)["data"].([]any); got[0].(map[string]any)["reference"] != "BNSS 173" {
		t.Errorf("CrPC 154: %v", got)
	}
	if got := expectStatus(t, "/v1/law/iea/65b", http.StatusOK).json(t)["data"].([]any); got[0].(map[string]any)["reference"] != "BSA 63" {
		t.Errorf("IEA 65B: %v", got)
	}
	if got := expectStatus(t, "/v1/law/bns/111", http.StatusOK).json(t)["data"].([]any); got[0].(map[string]any)["title"] != "Organized crime" {
		t.Errorf("BNS 111: %v", got)
	}
	search := expectStatus(t, "/v1/law/bns?q=murder", http.StatusOK).json(t)["data"].([]any)
	if len(search) < 3 {
		t.Errorf("search murder: %v", search)
	}
	expectStatus(t, "/v1/law/bns/999", http.StatusNotFound)
	expectStatus(t, "/v1/law/xyz/1", http.StatusNotFound)
	expectStatus(t, "/v1/law/bns/abc", http.StatusBadRequest)
	expectStatus(t, "/v1/law/bns?q=a", http.StatusBadRequest)
}
