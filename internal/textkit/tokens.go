package textkit

import (
	"bytes"
	"container/heap"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/pkoukk/tiktoken-go-loader/assets"
)

// Token counts for OpenAI's byte-pair encodings, computed the way tiktoken does: split
// the text with the encoding's pattern, then merge the bytes of each piece by rank.
// Only counting is needed, so only the merge ranks are kept, a vocabulary is loaded on
// first use, and it is dropped again after it has been idle for a while: a server that
// is rarely asked for token counts pays for them only while it is asked.

// Encoding is one tokenizer vocabulary.
type Encoding struct {
	Name    string
	Models  []string
	file    string
	pattern string
}

// Encodings lists the vocabularies on offer. The vocabulary files are OpenAI's
// (MIT, from the tiktoken project), embedded through tiktoken-go-loader.
var Encodings = []Encoding{
	{
		Name: "o200k_base", file: "o200k_base.tiktoken",
		Models: []string{"gpt-4o", "gpt-4o-mini", "gpt-4.1", "gpt-4.5", "o1", "o3", "o4-mini"},
		pattern: strings.Join([]string{
			`[^\r\n\p{L}\p{N}]?[\p{Lu}\p{Lt}\p{Lm}\p{Lo}\p{M}]*[\p{Ll}\p{Lm}\p{Lo}\p{M}]+(?i:'s|'t|'re|'ve|'m|'ll|'d)?`,
			`[^\r\n\p{L}\p{N}]?[\p{Lu}\p{Lt}\p{Lm}\p{Lo}\p{M}]+[\p{Ll}\p{Lm}\p{Lo}\p{M}]*(?i:'s|'t|'re|'ve|'m|'ll|'d)?`,
			`\p{N}{1,3}`, ` ?[^\s\p{L}\p{N}]+[\r\n/]*`, `\s*[\r\n]+`, `\s+(?!\S)`, `\s+`,
		}, "|"),
	},
	{
		Name: "cl100k_base", file: "cl100k_base.tiktoken",
		Models:  []string{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo", "text-embedding-3-small", "text-embedding-3-large", "text-embedding-ada-002"},
		pattern: `(?i:'s|'t|'re|'ve|'m|'ll|'d)|[^\r\n\p{L}\p{N}]?\p{L}+|\p{N}{1,3}| ?[^\s\p{L}\p{N}]+[\r\n]*|\s*[\r\n]+|\s+(?!\S)|\s+`,
	},
	{
		Name: "p50k_base", file: "p50k_base.tiktoken",
		Models:  []string{"text-davinci-003", "code-davinci-002"},
		pattern: `'s|'t|'re|'ve|'m|'ll|'d| ?\p{L}+| ?\p{N}+| ?[^\s\p{L}\p{N}]+|\s+(?!\S)|\s+`,
	},
}

// EncodingNames lists the vocabulary names.
func EncodingNames() []string {
	names := make([]string, len(Encodings))
	for i, encoding := range Encodings {
		names[i] = encoding.Name
	}
	return names
}

// EncodingFor finds the vocabulary for an encoding or model name.
func EncodingFor(name string) (Encoding, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, encoding := range Encodings {
		if encoding.Name == name {
			return encoding, true
		}
		for _, model := range encoding.Models {
			if model == name {
				return encoding, true
			}
		}
	}
	return Encoding{}, false
}

type vocabulary struct {
	ranks    map[string]uint32
	pattern  *regexp2.Regexp
	lastUsed time.Time
}

// Tokenizer loads vocabularies on demand. It is safe for concurrent use.
type Tokenizer struct {
	mu     sync.Mutex
	loaded map[string]*vocabulary
}

// NewTokenizer returns a tokenizer with nothing loaded.
func NewTokenizer() *Tokenizer { return &Tokenizer{loaded: map[string]*vocabulary{}} }

func (t *Tokenizer) vocabulary(encoding Encoding) (*vocabulary, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if v, ok := t.loaded[encoding.Name]; ok {
		v.lastUsed = time.Now()
		return v, nil
	}
	raw, err := assets.Assets.ReadFile(encoding.file)
	if err != nil {
		return nil, err
	}
	ranks := make(map[string]uint32, bytes.Count(raw, []byte("\n"))+1)
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		space := bytes.IndexByte(line, ' ')
		if space < 0 {
			return nil, fmt.Errorf("%s: bad line", encoding.file)
		}
		token := make([]byte, base64.StdEncoding.DecodedLen(space))
		n, err := base64.StdEncoding.Decode(token, line[:space])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", encoding.file, err)
		}
		rank, err := strconv.ParseUint(string(line[space+1:]), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", encoding.file, err)
		}
		ranks[string(token[:n])] = uint32(rank)
	}
	pattern, err := regexp2.Compile(encoding.pattern, regexp2.None)
	if err != nil {
		return nil, err
	}
	pattern.MatchTimeout = 10 * time.Second
	v := &vocabulary{ranks: ranks, pattern: pattern, lastUsed: time.Now()}
	t.loaded[encoding.Name] = v
	return v, nil
}

// DropIdle forgets vocabularies not used for the given time, so their memory can be
// reclaimed. It returns the names it dropped.
func (t *Tokenizer) DropIdle(idle time.Duration) []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var dropped []string
	for name, v := range t.loaded {
		if time.Since(v.lastUsed) >= idle {
			delete(t.loaded, name)
			dropped = append(dropped, name)
		}
	}
	return dropped
}

// Loaded lists the vocabularies currently in memory.
func (t *Tokenizer) Loaded() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var names []string
	for name := range t.loaded {
		names = append(names, name)
	}
	return names
}

// ErrTooSlow means the pattern match ran out of time, which only a hostile input does.
var ErrTooSlow = errors.New("the text took too long to split")

// Count returns the number of tokens text encodes to, treating special tokens such as
// <|endoftext|> as ordinary text.
func (t *Tokenizer) Count(encoding Encoding, text string) (int, error) {
	v, err := t.vocabulary(encoding)
	if err != nil {
		return 0, err
	}
	count := 0
	match, err := v.pattern.FindStringMatch(text)
	for match != nil && err == nil {
		count += v.pieceTokens([]byte(match.String()))
		match, err = v.pattern.FindNextMatch(match)
	}
	if err != nil {
		return 0, ErrTooSlow
	}
	return count, nil
}

// longPiece is the length from which a piece is merged through a heap. The plain merge
// is quadratic in the length of a piece, and only machine-made text (a base64 blob, a
// minified line) has a single "word" this long; both give the same tokens.
const longPiece = 4096

func (v *vocabulary) pieceTokens(piece []byte) int {
	if _, ok := v.ranks[string(piece)]; ok {
		return 1
	}
	if len(piece) > longPiece {
		return v.mergeLong(piece)
	}
	return v.merge(piece)
}

// merge is tiktoken's byte-pair merge: repeatedly join the adjacent pair with the lowest
// rank until no joinable pair is left. The pieces left are the tokens.
func (v *vocabulary) merge(piece []byte) int {
	if len(piece) <= 1 {
		return len(piece)
	}
	type part struct {
		start int
		rank  uint32
	}
	const none = math.MaxUint32
	rankOf := func(b []byte) uint32 {
		if rank, ok := v.ranks[string(b)]; ok {
			return rank
		}
		return none
	}
	parts := make([]part, 0, len(piece)+1)
	for i := 0; i < len(piece)-1; i++ {
		parts = append(parts, part{i, rankOf(piece[i : i+2])})
	}
	parts = append(parts, part{len(piece) - 1, none}, part{len(piece), none})
	rankAt := func(i int) uint32 {
		if i+3 < len(parts) {
			return rankOf(piece[parts[i].start:parts[i+3].start])
		}
		return none
	}
	for {
		lowest, at := uint32(none), -1
		for i := 0; i < len(parts)-1; i++ {
			if parts[i].rank < lowest {
				lowest, at = parts[i].rank, i
			}
		}
		if at < 0 {
			break
		}
		if at > 0 {
			parts[at-1].rank = rankAt(at - 1)
		}
		parts[at].rank = rankAt(at)
		parts = append(parts[:at+1], parts[at+2:]...)
	}
	return len(parts) - 1
}

// mergeLong is the same merge in O(n log n). Every adjacent pair that has a rank waits
// in a heap ordered by rank and then position, so the pair taken is the one the plain
// loop takes: the lowest rank, the leftmost first. A merge makes the entries for the
// pairs it touched stale; they are skipped when they come up.
func (v *vocabulary) mergeLong(piece []byte) int {
	n := len(piece)
	// The parts alive form a list by start offset: next[i] is where the part at i ends,
	// prev[i] where the part before it starts.
	next, prev, alive := make([]int, n), make([]int, n), make([]bool, n)
	for i := range n {
		next[i], prev[i], alive[i] = i+1, i-1, true
	}
	pairs := &pairHeap{}
	push := func(i int) {
		if i < 0 || next[i] >= n {
			return
		}
		end := next[next[i]]
		if rank, ok := v.ranks[string(piece[i:end])]; ok {
			heap.Push(pairs, pair{rank, i, end})
		}
	}
	for i := 0; i < n-1; i++ {
		push(i)
	}
	count := n
	for pairs.Len() > 0 {
		p := heap.Pop(pairs).(pair)
		i := p.start
		if !alive[i] || next[i] >= n || next[next[i]] != p.end {
			continue // a merge since this entry was made changed one of its parts
		}
		joined := next[i]
		alive[joined] = false
		next[i] = next[joined]
		if next[i] < n {
			prev[next[i]] = i
		}
		count--
		push(prev[i])
		push(i)
	}
	return count
}

type pair struct {
	rank       uint32
	start, end int
}

type pairHeap []pair

func (h pairHeap) Len() int { return len(h) }
func (h pairHeap) Less(a, b int) bool {
	return h[a].rank < h[b].rank || (h[a].rank == h[b].rank && h[a].start < h[b].start)
}
func (h pairHeap) Swap(a, b int) { h[a], h[b] = h[b], h[a] }
func (h *pairHeap) Push(x any)   { *h = append(*h, x.(pair)) }
func (h *pairHeap) Pop() any {
	old := *h
	last := old[len(old)-1]
	*h = old[:len(old)-1]
	return last
}
