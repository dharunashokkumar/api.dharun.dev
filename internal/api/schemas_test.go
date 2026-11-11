package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestOpenAPIHasTypedResponses(t *testing.T) {
	got := expectStatus(t, "/openapi.json", http.StatusOK)
	var doc map[string]any
	if err := json.Unmarshal(got.body, &doc); err != nil {
		t.Fatal(err)
	}
	op := doc["paths"].(map[string]any)["/v1/pincode/{pincode}"].(map[string]any)["get"].(map[string]any)
	schema := op["responses"].(map[string]any)["200"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	all, ok := schema["allOf"].([]any)
	if !ok || len(all) != 2 {
		t.Fatalf("pincode response is not typed: %v", schema)
	}
	data := all[1].(map[string]any)["properties"].(map[string]any)["data"].(map[string]any)
	props := data["properties"].(map[string]any)
	if props["district"].(map[string]any)["type"] != "string" || props["offices"].(map[string]any)["type"] != "array" {
		t.Errorf("pincode data schema: %v", props)
	}
	typed, total := 0, 0
	for _, item := range doc["paths"].(map[string]any) {
		for _, raw := range item.(map[string]any) {
			op := raw.(map[string]any)
			content, _ := op["responses"].(map[string]any)["200"].(map[string]any)["content"].(map[string]any)
			if js, ok := content["application/json"].(map[string]any); ok {
				total++
				if _, ok := js["schema"].(map[string]any)["allOf"]; ok {
					typed++
				}
			}
		}
	}
	if typed*2 < total {
		t.Errorf("only %d of %d JSON responses typed", typed, total)
	}
	t.Logf("%d of %d JSON responses typed", typed, total)
}

func TestInferSchemaMergesListItems(t *testing.T) {
	var v any
	json.Unmarshal([]byte(`[{"a":1,"b":null},{"a":2.5,"c":"x"}]`), &v)
	s := inferSchema(v)
	props := s["items"].(map[string]any)["properties"].(map[string]any)
	if props["a"].(map[string]any)["type"] != "number" || props["c"].(map[string]any)["type"] != "string" || len(props["b"].(map[string]any)) != 0 {
		t.Errorf("merged %v", props)
	}
}
