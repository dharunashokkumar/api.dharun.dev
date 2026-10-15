package subword

import (
	"slices"
	"testing"
)

func TestWordPiece(t *testing.T) {
	tok, err := Load([]byte(`{
		"normalizer": {"type": "BertNormalizer", "clean_text": true, "handle_chinese_chars": true, "strip_accents": null, "lowercase": true},
		"pre_tokenizer": {"type": "BertPreTokenizer"},
		"model": {"type": "WordPiece", "unk_token": "[UNK]", "continuing_subword_prefix": "##", "max_input_chars_per_word": 100,
			"vocab": {"[UNK]": 0, "[CLS]": 1, "[SEP]": 2, "rahul": 3, "met": 4, "san": 5, "##ja": 6, "##y": 7, ",": 8}},
		"added_tokens": [{"id": 1, "content": "[CLS]"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	tokens := tok.Encode("Rahul met Sanjay, Zed")
	var ids []int64
	for _, token := range tokens {
		ids = append(ids, token.ID)
	}
	if !slices.Equal(ids, []int64{3, 4, 5, 6, 7, 8, 0}) {
		t.Errorf("ids %v", ids)
	}
	// Offsets point into the original text, before lowercasing.
	if tokens[2].Start != 10 || tokens[4].End != 16 || tokens[6].Start != 18 || tokens[6].End != 21 {
		t.Errorf("offsets %+v", tokens)
	}
	if id, ok := tok.ID("[CLS]"); !ok || id != 1 {
		t.Errorf("[CLS] = %d", id)
	}
}

func TestUnigramAndBPE(t *testing.T) {
	unigram, err := Load([]byte(`{
		"normalizer": {"type": "Replace", "pattern": {"Regex": " {2,}"}, "content": " "},
		"pre_tokenizer": {"type": "Metaspace", "replacement": "▁", "add_prefix_space": true},
		"model": {"type": "Unigram", "unk_id": 0, "vocab": [["<unk>", 0], ["▁", -2], ["▁hello", -3], ["▁he", -2], ["llo", -2], ["▁world", -3], ["w", -5]]}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	// ▁hello (-3) beats ▁he + llo (-4); two unknown letters become one unknown.
	if ids := unigram.IDs("hello   world qq"); !slices.Equal(ids, []int64{2, 5, 1, 0}) {
		t.Errorf("unigram %v", ids)
	}
	bpe, err := Load([]byte(`{
		"pre_tokenizer": {"type": "Metaspace", "replacement": "▁", "prepend_scheme": "always", "split": true},
		"model": {"type": "BPE", "unk_token": "<unk>", "fuse_unk": true,
			"vocab": {"<unk>": 0, "▁": 1, "c": 2, "a": 3, "t": 4, "▁c": 5, "at": 6, "▁cat": 7},
			"merges": [["a", "t"], ["▁", "c"], ["▁c", "at"]]}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if ids := bpe.IDs("cat tac"); !slices.Equal(ids, []int64{7, 1, 4, 3, 2}) {
		t.Errorf("bpe %v", ids)
	}
}
