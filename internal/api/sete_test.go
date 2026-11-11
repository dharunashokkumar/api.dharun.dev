package api

import (
	"net/http"
	"strings"
	"testing"
)

func init() {
	mcpTestCalls["agent_notes"] = `{"store":"demo","secret":"demo","key":"greeting"}`
}

func TestAgentNotes(t *testing.T) {
	made := request(t, http.MethodPost, "/v1/kv", "{}", "Content-Type", "application/json")
	if made.status != http.StatusOK {
		t.Fatalf("make %d %s", made.status, made.body)
	}
	store := made.json(t)["data"].(map[string]any)
	id, secret := store["id"].(string), store["secret"].(string)
	if got := request(t, http.MethodPost, "/v1/kv/"+id+"/plan", `{"step":3}`, "X-Secret", secret); got.status != http.StatusOK {
		t.Fatalf("write %d %s", got.status, got.body)
	}
	read := request(t, http.MethodGet, "/v1/kv/"+id+"/plan", "", "X-Secret", secret)
	if !strings.Contains(string(read.body), `"value":"{\"step\":3}"`) {
		t.Errorf("read %s", read.body)
	}
	if got := request(t, http.MethodGet, "/v1/kv/"+id, "", "X-Secret", "nope"); got.status != http.StatusNotFound {
		t.Errorf("wrong secret = %d", got.status)
	}
	list := request(t, http.MethodGet, "/v1/kv/"+id, "", "X-Secret", secret)
	if !strings.Contains(string(list.body), `"keys":["plan"]`) {
		t.Errorf("list %s", list.body)
	}
	request(t, http.MethodPost, "/v1/kv/"+id+"/plan/delete", "{}", "X-Secret", secret)
	if got := request(t, http.MethodGet, "/v1/kv/"+id+"/plan", "", "X-Secret", secret); got.status != http.StatusNotFound {
		t.Errorf("deleted note = %d", got.status)
	}
	if got := request(t, http.MethodPost, "/v1/kv/demo/greeting?secret=demo", "changed"); !strings.Contains(string(got.body), `"saved":false`) {
		t.Errorf("demo took a write: %s", got.body)
	}
}

func TestRAGWithoutModels(t *testing.T) {
	made := request(t, http.MethodPost, "/v1/rag", "{}", "Content-Type", "application/json")
	if made.status != http.StatusOK {
		t.Fatalf("make %d", made.status)
	}
	c := made.json(t)["data"].(map[string]any)
	got := request(t, http.MethodPost, "/v1/rag/"+c["id"].(string)+"/documents", `{"documents":[{"name":"a","text":"Pincodes are six digits."}]}`, "X-Secret", c["secret"].(string))
	// Without the embedding model the server says so; with it, the passage is added.
	if got.status != http.StatusOK && !missingTool(got) {
		t.Errorf("add %d %s", got.status, got.body)
	}
	if got := request(t, http.MethodPost, "/v1/rag/"+c["id"].(string)+"/query", `{"query":""}`, "X-Secret", c["secret"].(string)); got.status != http.StatusBadRequest {
		t.Errorf("empty query = %d", got.status)
	}
	request(t, http.MethodPost, "/v1/rag/"+c["id"].(string)+"/delete", "{}", "X-Secret", c["secret"].(string))
}
