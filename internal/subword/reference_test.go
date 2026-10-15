package subword

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestAgainstReference compares the tokens with those of the Hugging Face tokenizers
// library for real tokenizer files, which are too large to keep in the repository. Set
// TOKENIZER_REFERENCE to a directory holding them and expect.json, written by the
// library: {"<file name without .json>": [{"text": ..., "ids": [...]}, ...]}, from
// Tokenizer.from_file(f).encode(text, add_special_tokens=False).ids.
func TestAgainstReference(t *testing.T) {
	dir := os.Getenv("TOKENIZER_REFERENCE")
	if dir == "" {
		t.Skip("TOKENIZER_REFERENCE not set")
	}
	data, err := os.ReadFile(filepath.Join(dir, "expect.json"))
	if err != nil {
		t.Fatal(err)
	}
	var expect map[string][]struct {
		Text    string   `json:"text"`
		IDs     []int64  `json:"ids"`
		Offsets [][2]int `json:"offsets"`
	}
	if err := json.Unmarshal(data, &expect); err != nil {
		t.Fatal(err)
	}
	for name, rows := range expect {
		tok, err := LoadFile(filepath.Join(dir, name+".json"))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		for _, row := range rows {
			got := tok.IDs(row.Text)
			if !slices.Equal(got, row.IDs) {
				t.Errorf("%s %q:\n got  %v\n want %v", name, row.Text, got, row.IDs)
			}
		}
	}
}
