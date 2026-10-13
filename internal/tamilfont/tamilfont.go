package tamilfont

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/encoding/charmap"
)

// Encoding is one 8-bit Tamil encoding.
type Encoding struct {
	ID    string
	Name  string
	About string
	// Fonts names well-known fonts that use the encoding, to help people recognise it.
	Fonts  []string
	decode func([]byte) (string, int)
	encode func(string) ([]byte, []string)
}

// Encodings lists what can be converted.
var Encodings = []Encoding{
	{
		ID: "tscii", Name: "TSCII 1.7",
		About:  "Tamil Script Code for Information Interchange: most Tamil web pages and mail from 1997 to the mid-2000s, and the Project Madurai library.",
		Fonts:  []string{"TSCu_Paranar", "TSCu_InaimathiTSC", "TSC_Avarangal", "Mylai-TSC", "TSCMaduram"},
		decode: decodeTSCII, encode: encodeTSCII,
	},
}

// Lookup finds an encoding by its ID.
func Lookup(id string) (Encoding, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, encoding := range Encodings {
		if encoding.ID == id {
			return encoding, true
		}
	}
	return Encoding{}, false
}

// ToUnicode converts bytes in the encoding to Unicode text. undefined counts bytes the
// encoding does not assign; each becomes U+FFFD.
func (e Encoding) ToUnicode(data []byte) (text string, undefined int) {
	return e.decode(data)
}

// FromUnicode converts Unicode text to bytes in the encoding. Characters the encoding
// has no code for become "?" and are listed, each once.
func (e Encoding) FromUnicode(text string) (data []byte, unmapped []string) {
	return e.encode(text)
}

// Text in an 8-bit Tamil encoding rarely travels as bytes. It is usually met as the
// characters those bytes stand for in Windows-1252, the way a browser or an editor shows
// it without the Tamil font: "¾Á¢ú" for தமிழ். View and Bytes go between the two.

// View shows bytes as Windows-1252 characters. The five bytes Windows-1252 leaves
// unassigned become the control characters of the same number, as browsers do.
func View(data []byte) string {
	var out strings.Builder
	out.Grow(len(data) * 2)
	for _, b := range data {
		r := charmap.Windows1252.DecodeByte(b)
		if r == unicode.ReplacementChar {
			r = rune(b)
		}
		out.WriteRune(r)
	}
	return out.String()
}

// Bytes turns text seen as Windows-1252 (or Latin-1) characters back into the bytes
// behind it. It fails on a character that neither has, which means the text is not in
// an 8-bit encoding at all: most often it is Unicode already.
func Bytes(view string) ([]byte, error) {
	out := make([]byte, 0, len(view))
	for _, r := range view {
		if b, ok := charmap.Windows1252.EncodeRune(r); ok {
			out = append(out, b)
		} else if r < 0x100 {
			out = append(out, byte(r)) // Latin-1, where the two differ
		} else {
			return nil, fmt.Errorf("%q (U+%04X) is not a character of an 8-bit encoding", r, r)
		}
	}
	return out, nil
}

// HasTamil reports whether text holds Unicode Tamil letters.
func HasTamil(text string) bool {
	for _, r := range text {
		if r >= 0x0B80 && r <= 0x0BFF {
			return true
		}
	}
	return false
}
