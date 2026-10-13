// Package textkit holds the text tools behind /v1/text: chunking, redaction and
// language detection. Nothing here keeps state or touches the network.
package textkit

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Chunk is one piece of a document. Text is exactly Source[Start:End] counted in
// characters (Unicode code points), so a caller can point back into the original.
type Chunk struct {
	Index   int    `json:"index"`
	Text    string `json:"text"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Chars   int    `json:"chars"`
	Heading string `json:"heading,omitempty"`
}

// ChunkOptions controls chunking. Size and Overlap are in characters.
type ChunkOptions struct {
	Size     int
	Overlap  int
	Markdown bool
}

type span struct{ start, end int } // byte offsets

var (
	headingLine = regexp.MustCompile(`^(#{1,6})[ \t]+(.+?)[ \t#]*$`)
	fenceLine   = regexp.MustCompile("^(```|~~~)")
	// Sentence ends: . ! ? and the Devanagari danda, followed by space.
	sentenceEnd = regexp.MustCompile(`[.!?।॥][\p{Pe}"'”’]*\s+`)
)

// LooksLikeMarkdown reports whether text has Markdown headings or code fences.
func LooksLikeMarkdown(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		if headingLine.MatchString(line) || fenceLine.MatchString(line) {
			return true
		}
	}
	return false
}

// section is a run of the document under one heading.
type section struct {
	span
	heading string
}

// sections splits Markdown at its headings, outside code fences, and names each part by
// the path of headings above it ("Setup > Install").
func sections(text string, markdown bool) []section {
	if !markdown {
		return []section{{span: span{0, len(text)}}}
	}
	var out []section
	var stack []string // heading text by level, 1-based
	current := section{span: span{0, 0}}
	offset := 0
	inFence := false
	for _, line := range strings.SplitAfter(text, "\n") {
		trimmed := strings.TrimRight(line, "\r\n")
		if fenceLine.MatchString(trimmed) {
			inFence = !inFence
		}
		if match := headingLine.FindStringSubmatch(trimmed); match != nil && !inFence {
			if offset > current.start {
				current.end = offset
				out = append(out, current)
			}
			level := len(match[1])
			for len(stack) < level {
				stack = append(stack, "")
			}
			stack = append(stack[:level-1], strings.TrimSpace(match[2]))
			var path []string
			for _, name := range stack {
				if name != "" {
					path = append(path, name)
				}
			}
			current = section{span: span{offset, 0}, heading: strings.Join(path, " > ")}
		}
		offset += len(line)
	}
	current.end = len(text)
	if current.end > current.start || len(out) == 0 {
		out = append(out, current)
	}
	return out
}

func runes(text string, s span) int { return utf8.RuneCountInString(text[s.start:s.end]) }

// pieces cuts a span into contiguous parts of at most size characters, at the most
// natural boundary available: paragraphs, then lines, then sentences, then words, and
// only as a last resort in the middle of a word.
func pieces(text string, whole span, size int, level int) []span {
	if runes(text, whole) <= size {
		return []span{whole}
	}
	var cuts []int // byte offsets after which the span may be cut
	part := text[whole.start:whole.end]
	switch level {
	case 0: // paragraphs, keeping code fences whole
		inFence := false
		offset := 0
		lines := strings.SplitAfter(part, "\n")
		for i, line := range lines {
			if fenceLine.MatchString(strings.TrimSpace(line)) {
				inFence = !inFence
			}
			offset += len(line)
			if !inFence && strings.TrimSpace(line) == "" && i > 0 {
				cuts = append(cuts, offset)
			}
		}
	case 1: // lines
		for offset := strings.Index(part, "\n"); offset >= 0; {
			cuts = append(cuts, offset+1)
			next := strings.Index(part[offset+1:], "\n")
			if next < 0 {
				break
			}
			offset += next + 1
		}
	case 2: // sentences
		for _, match := range sentenceEnd.FindAllStringIndex(part, -1) {
			cuts = append(cuts, match[1])
		}
	case 3: // words
		for i, r := range part {
			if unicode.IsSpace(r) {
				cuts = append(cuts, i+utf8.RuneLen(r))
			}
		}
	default: // characters
		count := 0
		for i := range part {
			if count > 0 && count%size == 0 {
				cuts = append(cuts, i)
			}
			count++
		}
	}
	if len(cuts) == 0 {
		return pieces(text, whole, size, level+1)
	}
	var out []span
	from := whole.start
	for _, cut := range append(cuts, len(part)) {
		end := whole.start + cut
		if end <= from {
			continue
		}
		out = append(out, pieces(text, span{from, end}, size, level+1)...)
		from = end
	}
	return out
}

// ChunkText splits text into chunks of at most opts.Size characters that never cross a
// Markdown heading, each starting with up to opts.Overlap characters of the one before
// it in the same section. The overlap counts towards the size. Overlap must be less than
// Size.
func ChunkText(text string, opts ChunkOptions) []Chunk {
	limit := opts.Size - opts.Overlap // room left for the chunk's own text
	// Byte offset to character offset, for the answer.
	charAt := func(byteOffset int) int { return utf8.RuneCountInString(text[:byteOffset]) }
	var chunks []Chunk
	for _, sec := range sections(text, opts.Markdown) {
		parts := pieces(text, sec.span, limit, 0)
		var spans []span
		for _, p := range parts {
			// Merge into the current chunk while it still fits.
			if n := len(spans); n > 0 && runes(text, span{spans[n-1].start, p.end}) <= limit {
				spans[n-1].end = p.end
				continue
			}
			spans = append(spans, p)
		}
		for i, s := range spans {
			if i > 0 && opts.Overlap > 0 {
				s.start = overlapStart(text, spans[i-1], opts.Overlap, sec.start)
			}
			s = trim(text, s)
			if s.end <= s.start {
				continue
			}
			chunks = append(chunks, Chunk{
				Index: len(chunks), Text: text[s.start:s.end], Start: charAt(s.start), End: charAt(s.end),
				Chars: runes(text, s), Heading: sec.heading,
			})
		}
	}
	return chunks
}

// overlapStart moves back from the end of the previous chunk by up to n characters,
// to the start of a word.
func overlapStart(text string, previous span, n, floor int) int {
	start := previous.end
	for count := 0; count < n && start > previous.start && start > floor; count++ {
		_, size := utf8.DecodeLastRuneInString(text[:start])
		start -= size
	}
	// Do not begin halfway through a word.
	for start < previous.end {
		r, size := utf8.DecodeRuneInString(text[start:])
		if start == floor || unicode.IsSpace(r) {
			break
		}
		before, _ := utf8.DecodeLastRuneInString(text[:start])
		if unicode.IsSpace(before) {
			break
		}
		start += size
	}
	return start
}

func trim(text string, s span) span {
	for s.start < s.end {
		r, size := utf8.DecodeRuneInString(text[s.start:])
		if !unicode.IsSpace(r) {
			break
		}
		s.start += size
	}
	for s.end > s.start {
		r, size := utf8.DecodeLastRuneInString(text[:s.end])
		if !unicode.IsSpace(r) {
			break
		}
		s.end -= size
	}
	return s
}
