// Package subword reads a Hugging Face tokenizer.json and splits text into the token IDs
// a model was trained on: SentencePiece Unigram (XLM-RoBERTa and the models built on it),
// byte-pair encoding over SentencePiece pieces (IndicTrans2) and WordPiece (BERT). Only
// the parts those files use are implemented; Load refuses anything else, so a model
// cannot silently get different tokens from the ones it learnt.
//
// Every token keeps the byte span of the original text it came from, which name
// detection needs to point at a name.
package subword

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Tokenizer turns text into token IDs.
type Tokenizer struct {
	normalizers []normalizer
	pre         []preTokenizer
	model       model
	added       map[string]int64 // tokens matched whole, such as [CLS]
}

// Token is one token and the byte span of the original text it stands for.
type Token struct {
	ID         int64
	Start, End int
}

// span is text being worked on, with the original byte offset of every rune.
type span struct {
	runes   []rune
	offsets []int // the original start of each rune
	end     int   // the original end of the last rune
}

func (s span) String() string { return string(s.runes) }

func (s span) startOf(i int) int {
	if i < len(s.offsets) {
		return s.offsets[i]
	}
	return s.end
}

// spanEnd is where rune i ends in the original: where the next distinct original rune
// starts.
func (s span) endOf(i int) int {
	for j := i + 1; j < len(s.offsets); j++ {
		if s.offsets[j] != s.offsets[i] {
			return s.offsets[j]
		}
	}
	return s.end
}

func newSpan(text string) span {
	s := span{end: len(text)}
	for i, r := range text {
		s.runes = append(s.runes, r)
		s.offsets = append(s.offsets, i)
	}
	return s
}

type normalizer func(span) span
type preTokenizer func([]span) []span

type model interface {
	tokenize(word span) []Token
	id(token string) (int64, bool)
	each(func(token string, id int64))
}

// ID returns the ID of a token, such as a language tag or a special token.
func (t *Tokenizer) ID(token string) (int64, bool) {
	if id, ok := t.added[token]; ok {
		return id, true
	}
	return t.model.id(token)
}

// Encode tokenizes text without adding special tokens.
func (t *Tokenizer) Encode(text string) []Token {
	s := newSpan(text)
	for _, n := range t.normalizers {
		s = n(s)
	}
	if len(s.runes) == 0 {
		return nil
	}
	words := []span{s}
	for _, p := range t.pre {
		words = p(words)
	}
	var out []Token
	for _, w := range words {
		if len(w.runes) > 0 {
			out = append(out, t.model.tokenize(w)...)
		}
	}
	return out
}

// IDs tokenizes text and returns the IDs alone.
func (t *Tokenizer) IDs(text string) []int64 {
	tokens := t.Encode(text)
	ids := make([]int64, len(tokens))
	for i, tok := range tokens {
		ids[i] = tok.ID
	}
	return ids
}

// Vocabulary returns the token for every ID, for decoding.
func (t *Tokenizer) Vocabulary() []string {
	all := map[int64]string{}
	var size int64
	add := func(token string, id int64) {
		all[id] = token
		size = max(size, id+1)
	}
	t.model.each(add)
	for token, id := range t.added {
		add(token, id)
	}
	out := make([]string, size)
	for id, token := range all {
		out[id] = token
	}
	return out
}

// LoadFile reads a tokenizer.json file.
func LoadFile(path string) (*Tokenizer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Load(data)
}

type rawConfig struct {
	Normalizer   json.RawMessage `json:"normalizer"`
	PreTokenizer json.RawMessage `json:"pre_tokenizer"`
	Model        json.RawMessage `json:"model"`
	AddedTokens  []struct {
		ID      int64  `json:"id"`
		Content string `json:"content"`
	} `json:"added_tokens"`
}

// Load reads the contents of a tokenizer.json file.
func Load(data []byte) (*Tokenizer, error) {
	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	t := &Tokenizer{added: map[string]int64{}}
	var err error
	if t.normalizers, err = loadNormalizers(raw.Normalizer); err != nil {
		return nil, err
	}
	if t.pre, err = loadPre(raw.PreTokenizer); err != nil {
		return nil, err
	}
	if t.model, err = loadModel(raw.Model); err != nil {
		return nil, err
	}
	for _, added := range raw.AddedTokens {
		t.added[added.Content] = added.ID
	}
	return t, nil
}

// Normalizers ---------------------------------------------------------------------

type rawNormalizer struct {
	Type        string            `json:"type"`
	Normalizers []json.RawMessage `json:"normalizers"`
	Charsmap    string            `json:"precompiled_charsmap"`
	Pattern     struct {
		Regex  *string `json:"Regex"`
		String *string `json:"String"`
	} `json:"pattern"`
	Content       string `json:"content"`
	StripLeft     bool   `json:"strip_left"`
	StripRight    bool   `json:"strip_right"`
	CleanText     bool   `json:"clean_text"`
	ChineseChars  bool   `json:"handle_chinese_chars"`
	StripAccents  *bool  `json:"strip_accents"`
	Lowercase     bool   `json:"lowercase"`
	PrependString string `json:"prepend"`
}

func loadNormalizers(data json.RawMessage) ([]normalizer, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	var raw rawNormalizer
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	switch raw.Type {
	case "Sequence":
		var out []normalizer
		for _, one := range raw.Normalizers {
			more, err := loadNormalizers(one)
			if err != nil {
				return nil, err
			}
			out = append(out, more...)
		}
		return out, nil
	case "Precompiled":
		blob, err := base64.StdEncoding.DecodeString(raw.Charsmap)
		if err != nil {
			return nil, err
		}
		charsmap, err := newCharsmap(blob)
		if err != nil {
			return nil, err
		}
		return []normalizer{charsmap.normalize}, nil
	case "Replace":
		var pattern *regexp.Regexp
		switch {
		case raw.Pattern.Regex != nil:
			p, err := regexp.Compile(*raw.Pattern.Regex)
			if err != nil {
				return nil, err
			}
			pattern = p
		case raw.Pattern.String != nil:
			pattern = regexp.MustCompile(regexp.QuoteMeta(*raw.Pattern.String))
		default:
			return nil, errors.New("subword: Replace without a pattern")
		}
		return []normalizer{replacer(pattern, raw.Content)}, nil
	case "Strip":
		return []normalizer{stripper(raw.StripLeft, raw.StripRight)}, nil
	case "BertNormalizer":
		strip := raw.Lowercase
		if raw.StripAccents != nil {
			strip = *raw.StripAccents
		}
		return []normalizer{bertNormalizer(raw.CleanText, raw.ChineseChars, strip, raw.Lowercase)}, nil
	case "NFKC":
		return []normalizer{unicodeForm(norm.NFKC)}, nil
	case "NFC":
		return []normalizer{unicodeForm(norm.NFC)}, nil
	case "Lowercase":
		return []normalizer{mapRunes(func(r rune) []rune { return []rune(strings.ToLower(string(r))) })}, nil
	}
	return nil, fmt.Errorf("subword: normalizer %q is not supported", raw.Type)
}

// mapRunes rewrites each rune on its own, keeping its offset for what it becomes.
func mapRunes(f func(rune) []rune) normalizer {
	return func(s span) span {
		out := span{end: s.end}
		for i, r := range s.runes {
			for _, n := range f(r) {
				out.runes = append(out.runes, n)
				out.offsets = append(out.offsets, s.offsets[i])
			}
		}
		return out
	}
}

func unicodeForm(form norm.Form) normalizer {
	return mapRunes(func(r rune) []rune { return []rune(form.String(string(r))) })
}

func replacer(pattern *regexp.Regexp, content string) normalizer {
	return func(s span) span {
		text := string(s.runes)
		matches := pattern.FindAllStringIndex(text, -1)
		if matches == nil {
			return s
		}
		// Byte positions in text to rune positions.
		runeAt := make([]int, len(text)+1)
		n := 0
		for i := range text {
			runeAt[i] = n
			n++
		}
		runeAt[len(text)] = n
		out := span{end: s.end}
		last := 0
		for _, m := range matches {
			from, to := runeAt[m[0]], runeAt[m[1]]
			out.runes = append(out.runes, s.runes[last:from]...)
			out.offsets = append(out.offsets, s.offsets[last:from]...)
			for _, r := range content {
				out.runes = append(out.runes, r)
				out.offsets = append(out.offsets, s.startOf(from))
			}
			last = to
		}
		out.runes = append(out.runes, s.runes[last:]...)
		out.offsets = append(out.offsets, s.offsets[last:]...)
		return out
	}
}

func stripper(left, right bool) normalizer {
	return func(s span) span {
		from, to := 0, len(s.runes)
		if left {
			for from < to && unicode.IsSpace(s.runes[from]) {
				from++
			}
		}
		if right {
			for to > from && unicode.IsSpace(s.runes[to-1]) {
				to--
			}
		}
		return span{runes: s.runes[from:to], offsets: s.offsets[from:to], end: s.endOf(max(to-1, 0))}
	}
}

func isControl(r rune) bool {
	if r == '\t' || r == '\n' || r == '\r' {
		return false
	}
	return unicode.In(r, unicode.Cc, unicode.Cf, unicode.Co, unicode.Cs)
}

func isChinese(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) || (r >= 0x20000 && r <= 0x2A6DF) ||
		(r >= 0x2A700 && r <= 0x2B73F) || (r >= 0x2B740 && r <= 0x2B81F) || (r >= 0x2B820 && r <= 0x2CEAF) ||
		(r >= 0xF900 && r <= 0xFAFF) || (r >= 0x2F800 && r <= 0x2FA1F)
}

// bertNormalizer is BERT's: drop control characters, space out Chinese characters, strip
// accents (what Unicode decomposes into marks) and lowercase.
func bertNormalizer(clean, chinese, stripAccents, lower bool) normalizer {
	return mapRunes(func(r rune) []rune {
		if clean {
			if r == 0 || r == 0xFFFD || isControl(r) {
				return nil
			}
			if unicode.IsSpace(r) {
				return []rune{' '}
			}
		}
		if chinese && isChinese(r) {
			return []rune{' ', r, ' '}
		}
		out := []rune{r}
		if stripAccents {
			out = out[:0]
			for _, d := range norm.NFD.String(string(r)) {
				if !unicode.Is(unicode.Mn, d) {
					out = append(out, d)
				}
			}
		}
		if lower {
			out = []rune(strings.ToLower(string(out)))
		}
		return out
	})
}

// charsmap is SentencePiece's precompiled normalization: a double-array trie (darts-clone)
// from byte strings to their replacements.
type charsmap struct {
	trie        []uint32
	normalized  []byte
	longestPart int
}

func newCharsmap(blob []byte) (*charsmap, error) {
	if len(blob) < 4 {
		return nil, errors.New("subword: charsmap too short")
	}
	size := int(uint32(blob[0]) | uint32(blob[1])<<8 | uint32(blob[2])<<16 | uint32(blob[3])<<24)
	if size%4 != 0 || 4+size > len(blob) {
		return nil, errors.New("subword: charsmap damaged")
	}
	c := &charsmap{trie: make([]uint32, size/4), normalized: blob[4+size:]}
	for i := range c.trie {
		b := blob[4+4*i:]
		c.trie[i] = uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
	}
	return c, nil
}

// longest finds the longest prefix of key that the trie maps, and its replacement.
func (c *charsmap) longest(key []byte) (int, string) {
	offset := func(u uint32) uint32 { return (u >> 10) << ((u & (1 << 9)) >> 6) }
	pos := uint32(0)
	unit := c.trie[0]
	pos ^= offset(unit)
	bestLength, bestValue := 0, -1
	for i, b := range key {
		pos ^= uint32(b)
		if int(pos) >= len(c.trie) {
			break
		}
		unit = c.trie[pos]
		if unit&((1<<31)|0xFF) != uint32(b) {
			break
		}
		pos ^= offset(unit)
		if int(pos) >= len(c.trie) {
			break
		}
		if (unit>>8)&1 == 1 {
			bestLength, bestValue = i+1, int(c.trie[pos]&((1<<31)-1))
		}
	}
	if bestValue < 0 || bestValue >= len(c.normalized) {
		return 0, ""
	}
	end := bestValue
	for end < len(c.normalized) && c.normalized[end] != 0 {
		end++
	}
	return bestLength, string(c.normalized[bestValue:end])
}

func (c *charsmap) normalize(s span) span {
	out := span{end: s.end}
	text := []byte(string(s.runes))
	// Byte position to rune index.
	runeIndex := make([]int, 0, len(text))
	for i, r := range s.runes {
		for range utf8.RuneLen(r) {
			runeIndex = append(runeIndex, i)
		}
	}
	for at := 0; at < len(text); {
		length, value := c.longest(text[at:])
		origin := s.offsets[runeIndex[at]]
		if length > 0 {
			for _, r := range value {
				out.runes = append(out.runes, r)
				out.offsets = append(out.offsets, origin)
			}
			at += length
			// A match ends inside a rune only if the table is broken; step to the next.
			for at < len(text) && !utf8.RuneStart(text[at]) {
				at++
			}
			continue
		}
		r, size := utf8.DecodeRune(text[at:])
		out.runes = append(out.runes, r)
		out.offsets = append(out.offsets, origin)
		at += size
	}
	return out
}

// Pre-tokenizers ------------------------------------------------------------------

type rawPre struct {
	Type          string            `json:"type"`
	PreTokenizers []json.RawMessage `json:"pretokenizers"`
	Replacement   string            `json:"replacement"`
	AddPrefix     *bool             `json:"add_prefix_space"`
	PrependScheme string            `json:"prepend_scheme"`
	Split         *bool             `json:"split"`
}

func loadPre(data json.RawMessage) ([]preTokenizer, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	var raw rawPre
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	switch raw.Type {
	case "Sequence":
		var out []preTokenizer
		for _, one := range raw.PreTokenizers {
			more, err := loadPre(one)
			if err != nil {
				return nil, err
			}
			out = append(out, more...)
		}
		return out, nil
	case "Metaspace":
		replacement, _ := utf8.DecodeRuneInString(raw.Replacement)
		prepend := raw.PrependScheme == "always" || raw.PrependScheme == "first"
		if raw.AddPrefix != nil {
			prepend = *raw.AddPrefix
		}
		if raw.PrependScheme == "never" {
			prepend = false
		}
		split := raw.Split == nil || *raw.Split
		return []preTokenizer{metaspace(replacement, prepend, split)}, nil
	case "BertPreTokenizer":
		return []preTokenizer{bertPre}, nil
	case "WhitespaceSplit":
		return []preTokenizer{whitespaceSplit}, nil
	}
	return nil, fmt.Errorf("subword: pre-tokenizer %q is not supported", raw.Type)
}

// metaspace turns spaces into the replacement character, puts one in front, and splits
// the text before each, so every word starts with it.
func metaspace(replacement rune, prepend, split bool) preTokenizer {
	return func(words []span) []span {
		var out []span
		for _, w := range words {
			s := span{end: w.end}
			for i, r := range w.runes {
				if r == ' ' {
					r = replacement
				}
				s.runes = append(s.runes, r)
				s.offsets = append(s.offsets, w.offsets[i])
			}
			if prepend && (len(s.runes) == 0 || s.runes[0] != replacement) {
				first := w.end
				if len(w.offsets) > 0 {
					first = w.offsets[0]
				}
				s.runes = append([]rune{replacement}, s.runes...)
				s.offsets = append([]int{first}, s.offsets...)
			}
			if !split {
				out = append(out, s)
				continue
			}
			start := 0
			for i := 1; i <= len(s.runes); i++ {
				if i == len(s.runes) || s.runes[i] == replacement {
					out = append(out, span{runes: s.runes[start:i], offsets: s.offsets[start:i], end: s.endOf(i - 1)})
					start = i
				}
			}
		}
		return out
	}
}

func isPunctuation(r rune) bool {
	if (r >= 33 && r <= 47) || (r >= 58 && r <= 64) || (r >= 91 && r <= 96) || (r >= 123 && r <= 126) {
		return true
	}
	return unicode.IsPunct(r)
}

// bertPre splits on white space and around every punctuation character.
func bertPre(words []span) []span {
	var out []span
	for _, w := range words {
		start := -1
		flush := func(i int) {
			if start >= 0 {
				out = append(out, span{runes: w.runes[start:i], offsets: w.offsets[start:i], end: w.endOf(i - 1)})
				start = -1
			}
		}
		for i, r := range w.runes {
			switch {
			case unicode.IsSpace(r):
				flush(i)
			case isPunctuation(r):
				flush(i)
				out = append(out, span{runes: w.runes[i : i+1], offsets: w.offsets[i : i+1], end: w.endOf(i)})
			default:
				if start < 0 {
					start = i
				}
			}
		}
		flush(len(w.runes))
	}
	return out
}

func whitespaceSplit(words []span) []span {
	var out []span
	for _, w := range words {
		start := -1
		for i := 0; i <= len(w.runes); i++ {
			if i == len(w.runes) || unicode.IsSpace(w.runes[i]) {
				if start >= 0 {
					out = append(out, span{runes: w.runes[start:i], offsets: w.offsets[start:i], end: w.endOf(i - 1)})
					start = -1
				}
			} else if start < 0 {
				start = i
			}
		}
	}
	return out
}

// Models --------------------------------------------------------------------------

type rawModel struct {
	Type         string          `json:"type"`
	Vocab        json.RawMessage `json:"vocab"`
	Merges       json.RawMessage `json:"merges"`
	UnkID        *int64          `json:"unk_id"`
	UnkToken     string          `json:"unk_token"`
	Prefix       string          `json:"continuing_subword_prefix"`
	MaxChars     int             `json:"max_input_chars_per_word"`
	FuseUnk      bool            `json:"fuse_unk"`
	ByteFallback bool            `json:"byte_fallback"`
}

func loadModel(data json.RawMessage) (model, error) {
	var raw rawModel
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if raw.ByteFallback {
		return nil, errors.New("subword: byte fallback is not supported")
	}
	switch raw.Type {
	case "Unigram":
		var vocab [][2]any
		if err := json.Unmarshal(raw.Vocab, &vocab); err != nil {
			return nil, err
		}
		u := &unigram{pieces: make(map[string]unigramPiece, len(vocab))}
		lowest := math.Inf(1)
		for i, entry := range vocab {
			piece, _ := entry[0].(string)
			score, _ := entry[1].(float64)
			u.pieces[piece] = unigramPiece{int64(i), score}
			lowest = min(lowest, score)
			u.longest = max(u.longest, utf8.RuneCountInString(piece))
		}
		if raw.UnkID == nil {
			return nil, errors.New("subword: Unigram without unk_id")
		}
		u.unk, u.unkScore = *raw.UnkID, lowest-10
		return u, nil
	case "BPE":
		vocab := map[string]int64{}
		if err := json.Unmarshal(raw.Vocab, &vocab); err != nil {
			return nil, err
		}
		b := &bpe{vocab: vocab, ranks: map[[2]string]int{}, fuseUnk: raw.FuseUnk}
		unk, ok := vocab[raw.UnkToken]
		if !ok {
			return nil, errors.New("subword: BPE unk_token is not in the vocabulary")
		}
		b.unk = unk
		var asPairs [][2]string
		if err := json.Unmarshal(raw.Merges, &asPairs); err != nil {
			var asText []string
			if err := json.Unmarshal(raw.Merges, &asText); err != nil {
				return nil, err
			}
			for _, line := range asText {
				left, right, ok := strings.Cut(line, " ")
				if !ok {
					return nil, fmt.Errorf("subword: bad merge %q", line)
				}
				asPairs = append(asPairs, [2]string{left, right})
			}
		}
		for rank, pair := range asPairs {
			if _, seen := b.ranks[pair]; !seen {
				b.ranks[pair] = rank
			}
		}
		return b, nil
	case "WordPiece":
		vocab := map[string]int64{}
		if err := json.Unmarshal(raw.Vocab, &vocab); err != nil {
			return nil, err
		}
		unk, ok := vocab[raw.UnkToken]
		if !ok {
			return nil, errors.New("subword: WordPiece unk_token is not in the vocabulary")
		}
		if raw.MaxChars == 0 {
			raw.MaxChars = 100
		}
		return &wordPiece{vocab: vocab, unk: unk, prefix: raw.Prefix, maxChars: raw.MaxChars}, nil
	}
	return nil, fmt.Errorf("subword: model %q is not supported", raw.Type)
}

func (u *unigram) id(token string) (int64, bool) {
	piece, ok := u.pieces[token]
	return piece.id, ok
}

func (u *unigram) each(f func(string, int64)) {
	for token, piece := range u.pieces {
		f(token, piece.id)
	}
}

func (b *bpe) id(token string) (int64, bool) {
	id, ok := b.vocab[token]
	return id, ok
}

func (b *bpe) each(f func(string, int64)) {
	for token, id := range b.vocab {
		f(token, id)
	}
}

func (w *wordPiece) id(token string) (int64, bool) {
	id, ok := w.vocab[token]
	return id, ok
}

func (w *wordPiece) each(f func(string, int64)) {
	for token, id := range w.vocab {
		f(token, id)
	}
}

type unigramPiece struct {
	id    int64
	score float64
}

type unigram struct {
	pieces   map[string]unigramPiece
	longest  int // in runes
	unk      int64
	unkScore float64
}

// tokenize finds the split with the highest total score (Viterbi). A character no piece
// covers becomes the unknown token, and unknown characters next to each other become one.
func (u *unigram) tokenize(word span) []Token {
	n := len(word.runes)
	best := make([]float64, n+1)
	from := make([]int, n+1)
	id := make([]int64, n+1)
	for i := 1; i <= n; i++ {
		best[i] = math.Inf(-1)
	}
	for start := 0; start < n; start++ {
		if math.IsInf(best[start], -1) {
			continue
		}
		matched := false
		for length := 1; length <= u.longest && start+length <= n; length++ {
			piece, ok := u.pieces[string(word.runes[start:start+length])]
			if !ok {
				continue
			}
			if length == 1 {
				matched = true
			}
			if score := best[start] + piece.score; score > best[start+length] {
				best[start+length], from[start+length], id[start+length] = score, start, piece.id
			}
		}
		if !matched {
			if score := best[start] + u.unkScore; score > best[start+1] {
				best[start+1], from[start+1], id[start+1] = score, start, u.unk
			}
		}
	}
	var out []Token
	for end := n; end > 0; end = from[end] {
		start := from[end]
		out = append(out, Token{ID: id[end], Start: word.startOf(start), End: word.endOf(end - 1)})
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	// Fuse runs of unknowns.
	fused := out[:0]
	for _, tok := range out {
		if tok.ID == u.unk && len(fused) > 0 && fused[len(fused)-1].ID == u.unk {
			fused[len(fused)-1].End = tok.End
			continue
		}
		fused = append(fused, tok)
	}
	return fused
}

type bpe struct {
	vocab   map[string]int64
	ranks   map[[2]string]int
	unk     int64
	fuseUnk bool
}

func (b *bpe) tokenize(word span) []Token {
	type symbol struct {
		text       string
		start, end int // rune positions in word
	}
	symbols := make([]symbol, len(word.runes))
	for i, r := range word.runes {
		symbols[i] = symbol{string(r), i, i + 1}
	}
	for len(symbols) > 1 {
		bestRank, at := math.MaxInt, -1
		for i := 0; i+1 < len(symbols); i++ {
			if rank, ok := b.ranks[[2]string{symbols[i].text, symbols[i+1].text}]; ok && rank < bestRank {
				bestRank, at = rank, i
			}
		}
		if at < 0 {
			break
		}
		merged := symbol{symbols[at].text + symbols[at+1].text, symbols[at].start, symbols[at+1].end}
		symbols = append(symbols[:at], append([]symbol{merged}, symbols[at+2:]...)...)
	}
	var out []Token
	for _, s := range symbols {
		id, ok := b.vocab[s.text]
		if !ok {
			id = b.unk
		}
		tok := Token{ID: id, Start: word.startOf(s.start), End: word.endOf(s.end - 1)}
		if id == b.unk && b.fuseUnk && len(out) > 0 && out[len(out)-1].ID == b.unk {
			out[len(out)-1].End = tok.End
			continue
		}
		out = append(out, tok)
	}
	return out
}

type wordPiece struct {
	vocab    map[string]int64
	unk      int64
	prefix   string
	maxChars int
}

// tokenize takes the longest piece in the vocabulary from the start of the word, then
// from where it ended, with the continuation prefix; a word that cannot be split so is
// the unknown token.
func (w *wordPiece) tokenize(word span) []Token {
	if len(word.runes) > w.maxChars {
		return []Token{{ID: w.unk, Start: word.startOf(0), End: word.end}}
	}
	var out []Token
	for start := 0; start < len(word.runes); {
		end := len(word.runes)
		found := false
		for ; end > start; end-- {
			piece := string(word.runes[start:end])
			if start > 0 {
				piece = w.prefix + piece
			}
			if id, ok := w.vocab[piece]; ok {
				out = append(out, Token{ID: id, Start: word.startOf(start), End: word.endOf(end - 1)})
				found = true
				break
			}
		}
		if !found {
			return []Token{{ID: w.unk, Start: word.startOf(0), End: word.end}}
		}
		start = end
	}
	return out
}
