// Package tamilfont converts Tamil text between Unicode and the 8-bit encodings that
// came before it, which are usually met as "legacy fonts": text that shows as Tamil
// only in one particular font and as accented Latin letters everywhere else.
//
// The tables are written here from the published standards, not taken from another
// converter.
package tamilfont

// TSCII 1.7, the Tamil Script Code for Information Interchange: the encoding of most
// Tamil text on the internet between 1997 and the mid-2000s, including the Project
// Madurai library. The table follows the TSCII 1.7 chart as registered with IANA and
// described in Unicode Technical Note 15; it agrees, byte for byte, with the table in
// the GNU C library.
//
// TSCII stores text in the order it is written, not the order it is spoken: the vowel
// signs that stand to the left of a consonant (e, ee, ai) come before it, and o, oo and
// au are a left part, the consonant, and a right part. Unicode stores every vowel sign
// after its consonant, so converting means moving them.

const (
	tsciiAA       = 0xA1 // the right part of o and oo
	tsciiU        = 0xA4
	tsciiUU       = 0xA5
	tsciiE        = 0xA6 // written before the consonant
	tsciiEE       = 0xA7 // written before the consonant
	tsciiAI       = 0xA8 // written before the consonant
	tsciiAULength = 0xAA // the right part of au
)

// tsciiTable gives the Unicode text for each byte from 0x80. Bytes below 0x80 are ASCII.
var tsciiTable = [128]string{
	0x80 - 0x80: "௦", 0x81 - 0x80: "௧",
	0x82 - 0x80: "ஸ்ரீ",
	0x83 - 0x80: "ஜ", 0x84 - 0x80: "ஷ", 0x85 - 0x80: "ஸ", 0x86 - 0x80: "ஹ", 0x87 - 0x80: "க்ஷ",
	0x88 - 0x80: "ஜ்", 0x89 - 0x80: "ஷ்", 0x8A - 0x80: "ஸ்", 0x8B - 0x80: "ஹ்", 0x8C - 0x80: "க்ஷ்",
	0x8D - 0x80: "௨", 0x8E - 0x80: "௩", 0x8F - 0x80: "௪", 0x90 - 0x80: "௫",
	0x91 - 0x80: "‘", 0x92 - 0x80: "’", 0x93 - 0x80: "“", 0x94 - 0x80: "”",
	0x95 - 0x80: "௬", 0x96 - 0x80: "௭", 0x97 - 0x80: "௮", 0x98 - 0x80: "௯",
	0x99 - 0x80: "ஙு", 0x9A - 0x80: "ஞு", 0x9B - 0x80: "ஙூ", 0x9C - 0x80: "ஞூ",
	0x9D - 0x80: "௰", 0x9E - 0x80: "௱", 0x9F - 0x80: "௲",
	0xA0 - 0x80: " ",
	0xA1 - 0x80: "ா", 0xA2 - 0x80: "ி", 0xA3 - 0x80: "ீ", 0xA4 - 0x80: "ு", 0xA5 - 0x80: "ூ",
	0xA6 - 0x80: "ெ", 0xA7 - 0x80: "ே", 0xA8 - 0x80: "ை",
	0xA9 - 0x80: "©", 0xAA - 0x80: "ௗ",
	0xAB - 0x80: "அ", 0xAC - 0x80: "ஆ",
	0xAD - 0x80: "இ", // where TSCII 1.6 had it; 1.7 moved it to 0xFE
	0xAE - 0x80: "ஈ", 0xAF - 0x80: "உ", 0xB0 - 0x80: "ஊ", 0xB1 - 0x80: "எ", 0xB2 - 0x80: "ஏ",
	0xB3 - 0x80: "ஐ", 0xB4 - 0x80: "ஒ", 0xB5 - 0x80: "ஓ", 0xB6 - 0x80: "ஔ", 0xB7 - 0x80: "ஃ",
	0xB8 - 0x80: "க", 0xB9 - 0x80: "ங", 0xBA - 0x80: "ச", 0xBB - 0x80: "ஞ", 0xBC - 0x80: "ட", 0xBD - 0x80: "ண",
	0xBE - 0x80: "த", 0xBF - 0x80: "ந", 0xC0 - 0x80: "ப", 0xC1 - 0x80: "ம", 0xC2 - 0x80: "ய", 0xC3 - 0x80: "ர",
	0xC4 - 0x80: "ல", 0xC5 - 0x80: "வ", 0xC6 - 0x80: "ழ", 0xC7 - 0x80: "ள", 0xC8 - 0x80: "ற", 0xC9 - 0x80: "ன",
	0xCA - 0x80: "டி", 0xCB - 0x80: "டீ",
	0xCC - 0x80: "கு", 0xCD - 0x80: "சு", 0xCE - 0x80: "டு", 0xCF - 0x80: "ணு", 0xD0 - 0x80: "து", 0xD1 - 0x80: "நு",
	0xD2 - 0x80: "பு", 0xD3 - 0x80: "மு", 0xD4 - 0x80: "யு", 0xD5 - 0x80: "ரு", 0xD6 - 0x80: "லு", 0xD7 - 0x80: "வு",
	0xD8 - 0x80: "ழு", 0xD9 - 0x80: "ளு", 0xDA - 0x80: "று", 0xDB - 0x80: "னு",
	0xDC - 0x80: "கூ", 0xDD - 0x80: "சூ", 0xDE - 0x80: "டூ", 0xDF - 0x80: "ணூ", 0xE0 - 0x80: "தூ", 0xE1 - 0x80: "நூ",
	0xE2 - 0x80: "பூ", 0xE3 - 0x80: "மூ", 0xE4 - 0x80: "யூ", 0xE5 - 0x80: "ரூ", 0xE6 - 0x80: "லூ", 0xE7 - 0x80: "வூ",
	0xE8 - 0x80: "ழூ", 0xE9 - 0x80: "ளூ", 0xEA - 0x80: "றூ", 0xEB - 0x80: "னூ",
	0xEC - 0x80: "க்", 0xED - 0x80: "ங்", 0xEE - 0x80: "ச்", 0xEF - 0x80: "ஞ்", 0xF0 - 0x80: "ட்", 0xF1 - 0x80: "ண்",
	0xF2 - 0x80: "த்", 0xF3 - 0x80: "ந்", 0xF4 - 0x80: "ப்", 0xF5 - 0x80: "ம்", 0xF6 - 0x80: "ய்", 0xF7 - 0x80: "ர்",
	0xF8 - 0x80: "ல்", 0xF9 - 0x80: "வ்", 0xFA - 0x80: "ழ்", 0xFB - 0x80: "ள்", 0xFC - 0x80: "ற்", 0xFD - 0x80: "ன்",
	0xFE - 0x80: "இ",
	// 0xFF is not assigned.
}

// tsciiConsonant reports the bytes that are a bare consonant, which a vowel sign written
// to the left belongs to: the eighteen Tamil consonants, the Grantha four, and ksha.
func tsciiConsonant(b byte) bool {
	return (b >= 0xB8 && b <= 0xC9) || (b >= 0x83 && b <= 0x87)
}

// tsciiCodes is the table the other way round: Unicode text to the byte for it. Where
// two bytes stand for the same text, the one TSCII 1.7 prefers wins.
var tsciiCodes = func() map[string]byte {
	codes := map[string]byte{}
	for i, text := range tsciiTable {
		if text != "" && i != 0xAD-0x80 {
			codes[text] = byte(0x80 + i)
		}
	}
	return codes
}()

func decodeTSCII(data []byte) (string, int) {
	out := make([]rune, 0, len(data)*2)
	undefined := 0
	var pending rune // a vowel sign read before its consonant
	for i := 0; i < len(data); i++ {
		b := data[i]
		if b == tsciiE || b == tsciiEE || b == tsciiAI {
			if pending != 0 {
				out = append(out, pending) // two in a row: the first had no consonant
			}
			pending = []rune(tsciiTable[b-0x80])[0]
			continue
		}
		if pending != 0 && tsciiConsonant(b) {
			out = append(out, []rune(tsciiTable[b-0x80])...)
			sign := pending
			if i+1 < len(data) {
				switch next := data[i+1]; {
				case next == tsciiAA && pending == 'ெ':
					sign, i = 'ொ', i+1
				case next == tsciiAA && pending == 'ே':
					sign, i = 'ோ', i+1
				// au is written e, consonant, length mark; the GNU C library writes it
				// with ee, and both are read.
				case next == tsciiAULength && pending != 'ை':
					sign, i = 'ௌ', i+1
				}
			}
			out = append(out, sign)
			pending = 0
			continue
		}
		if pending != 0 {
			out = append(out, pending) // a sign with no consonant after it stays where it was
			pending = 0
		}
		switch {
		case b < 0x80:
			out = append(out, rune(b))
		case tsciiTable[b-0x80] == "":
			out = append(out, '�')
			undefined++
		default:
			out = append(out, []rune(tsciiTable[b-0x80])...)
		}
	}
	if pending != 0 {
		out = append(out, pending)
	}
	return string(out), undefined
}

func encodeTSCII(text string) ([]byte, []string) {
	runes := []rune(text)
	out := make([]byte, 0, len(runes))
	var unmapped []string
	seen := map[rune]bool{}
	at := func(i int) rune {
		if i < len(runes) {
			return runes[i]
		}
		return 0
	}
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r < 0x80 {
			out = append(out, byte(r))
			continue
		}
		// The two conjuncts that have a code of their own.
		if r == 'ஸ' && at(i+1) == '்' && at(i+2) == 'ர' && at(i+3) == 'ீ' {
			out = append(out, tsciiCodes["ஸ்ரீ"])
			i += 3
			continue
		}
		base, isConsonant := tsciiCodes[string(r)]
		isConsonant = isConsonant && tsciiConsonant(base)
		letter := string(r)
		if r == 'க' && at(i+1) == '்' && at(i+2) == 'ஷ' {
			base, isConsonant, letter = tsciiCodes["க்ஷ"], true, "க்ஷ"
			i += 2
		}
		if !isConsonant {
			if code, ok := tsciiCodes[string(r)]; ok {
				out = append(out, code)
			} else {
				out = append(out, '?')
				if !seen[r] {
					seen[r] = true
					unmapped = append(unmapped, string(r))
				}
			}
			continue
		}
		sign := at(i + 1)
		// Unicode also allows o, oo and au spelt as their two parts.
		if (sign == 'ெ' || sign == 'ே') && at(i+2) == 'ா' {
			sign, i = sign+4, i+1 // ெ to ொ, ே to ோ
		} else if sign == 'ெ' && at(i+2) == 'ௗ' {
			sign, i = 'ௌ', i+1
		}
		switch sign {
		case '்', 'ு', 'ூ', 'ி', 'ீ':
			if code, ok := tsciiCodes[letter+string(sign)]; ok {
				out = append(out, code) // one code for the pair
			} else {
				out = append(out, base, tsciiCodes[string(sign)])
			}
		case 'ா':
			out = append(out, base, tsciiAA)
		case 'ெ':
			out = append(out, tsciiE, base)
		case 'ே':
			out = append(out, tsciiEE, base)
		case 'ை':
			out = append(out, tsciiAI, base)
		case 'ொ':
			out = append(out, tsciiE, base, tsciiAA)
		case 'ோ':
			out = append(out, tsciiEE, base, tsciiAA)
		case 'ௌ':
			out = append(out, tsciiE, base, tsciiAULength)
		default:
			out = append(out, base)
			continue // no sign: the next character is looked at on its own
		}
		i++
	}
	return out, unmapped
}
