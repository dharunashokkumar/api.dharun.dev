package api

// Typed response schemas for OpenAPI (growth plan Part 3 section 5), inferred from each
// endpoint's real sample answer (samples.go): the types and properties of its data. A
// value that is null in the sample stays untyped, so a schema never claims more than the
// sample showed; endpoints with no sample (live, files) keep the plain envelope.

import (
	"bytes"
	"encoding/json"
	"sort"
)

// inferSchema describes a decoded JSON value.
func inferSchema(v any) map[string]any {
	switch x := v.(type) {
	case map[string]any:
		props := map[string]any{}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			props[k] = inferSchema(x[k])
		}
		return map[string]any{"type": "object", "properties": props}
	case []any:
		schema := map[string]any{"type": "array"}
		if len(x) > 0 {
			items := inferSchema(x[0])
			// Objects in a list may not all carry the same keys: take every key seen.
			for _, item := range x[1:] {
				items = mergeSchemas(items, inferSchema(item))
			}
			schema["items"] = items
		}
		return schema
	case json.Number:
		if _, err := x.Int64(); err == nil {
			return map[string]any{"type": "integer"}
		}
		return map[string]any{"type": "number"}
	case float64:
		if x == float64(int64(x)) {
			return map[string]any{"type": "integer"}
		}
		return map[string]any{"type": "number"}
	case string:
		return map[string]any{"type": "string"}
	case bool:
		return map[string]any{"type": "boolean"}
	}
	return map[string]any{} // null: any type
}

func mergeSchemas(a, b map[string]any) map[string]any {
	if a["type"] == "integer" && b["type"] == "number" || a["type"] == "number" && b["type"] == "integer" {
		return map[string]any{"type": "number"}
	}
	if a["type"] != b["type"] {
		if len(a) == 0 {
			return b
		}
		if len(b) == 0 {
			return a
		}
		return map[string]any{}
	}
	if a["type"] == "object" {
		props := map[string]any{}
		for k, v := range a["properties"].(map[string]any) {
			props[k] = v
		}
		for k, v := range b["properties"].(map[string]any) {
			if old, ok := props[k]; ok {
				props[k] = mergeSchemas(old.(map[string]any), v.(map[string]any))
			} else {
				props[k] = v
			}
		}
		return map[string]any{"type": "object", "properties": props}
	}
	return a
}

// responseSchema is the envelope with the endpoint's data typed, or nil without a sample.
func (s *Server) responseSchema(host string, e endpoint) map[string]any {
	sample := s.sample(host, e)
	if sample == nil {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader([]byte(sample.JSON)))
	decoder.UseNumber()
	var body map[string]any
	if decoder.Decode(&body) != nil {
		return nil
	}
	data, ok := body["data"]
	if !ok {
		return nil
	}
	return map[string]any{"allOf": []any{
		map[string]any{"$ref": "#/components/schemas/Envelope"},
		map[string]any{"type": "object", "properties": map[string]any{"data": inferSchema(data)}},
	}}
}
