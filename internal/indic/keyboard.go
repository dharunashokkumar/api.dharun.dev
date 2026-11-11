package indic

import (
	"errors"
	"sort"
	"strings"
)

// Layouts lists the keyboard layouts Type and Keys know.
var Layouts = []string{"inscript-hindi", "tamil99"}

// inscript is the Hindi InScript layout (the Government of India's standard): the
// character each QWERTY key gives.
var inscript = map[rune]string{
	'`': "ॊ", '~': "ऒ", '1': "1", '2': "2", '3': "3", '4': "4", '5': "5", '6': "6", '7': "7", '8': "8", '9': "9", '0': "0",
	'!': "ऍ", '@': "ॅ", '#': "्र", '$': "र्", '%': "ज्ञ", '^': "त्र", '&': "क्ष", '*': "श्र", '(': "(", ')': ")",
	'-': "-", '_': "ः", '=': "ृ", '+': "ऋ", '\\': "ॉ", '|': "ऑ",
	'q': "ौ", 'w': "ै", 'e': "ा", 'r': "ी", 't': "ू", 'y': "ब", 'u': "ह", 'i': "ग", 'o': "द", 'p': "ज", '[': "ड", ']': "़",
	'Q': "औ", 'W': "ऐ", 'E': "आ", 'R': "ई", 'T': "ऊ", 'Y': "भ", 'U': "ङ", 'I': "घ", 'O': "ध", 'P': "झ", '{': "ढ", '}': "ञ",
	'a': "ो", 's': "े", 'd': "्", 'f': "ि", 'g': "ु", 'h': "प", 'j': "र", 'k': "क", 'l': "त", ';': "च", '\'': "ट",
	'A': "ओ", 'S': "ए", 'D': "अ", 'F': "इ", 'G': "उ", 'H': "फ", 'J': "ऱ", 'K': "ख", 'L': "थ", ':': "छ", '"': "ठ",
	'z': "ॆ", 'x': "ं", 'c': "म", 'v': "न", 'b': "व", 'n': "ल", 'm': "स", ',': ",", '.': ".", '/': "य",
	'Z': "ऎ", 'X': "ँ", 'C': "ण", 'N': "ळ", 'M': "श", '<': "ष", '>': "।", '?': "य़",
}

// Tamil99 (Tamil Nadu government standard, 1999). A consonant key gives the consonant
// with its inherent a; a vowel key straight after a consonant gives the vowel sign.
var tamil99Vowels = map[rune][2]string{ // the letter, and its sign after a consonant
	'a': {"அ", ""}, 'q': {"ஆ", "ா"}, 's': {"இ", "ி"}, 'w': {"ஈ", "ீ"}, 'd': {"உ", "ு"}, 'e': {"ஊ", "ூ"},
	'g': {"எ", "ெ"}, 't': {"ஏ", "ே"}, 'r': {"ஐ", "ை"}, 'c': {"ஒ", "ொ"}, 'x': {"ஓ", "ோ"}, 'z': {"ஔ", "ௌ"},
}

var tamil99Consonants = map[rune]string{
	'h': "க", 'b': "ங", '[': "ச", ']': "ஞ", 'o': "ட", 'p': "ண", 'l': "த", ';': "ந", 'i': "ன", 'j': "ப", 'k': "ம",
	'\'': "ய", 'm': "ர", 'u': "ற", 'n': "ல", 'y': "ள", '/': "ழ", 'v': "வ",
	'Q': "ஸ", 'W': "ஷ", 'E': "ஜ", 'R': "ஹ",
}

var tamil99Other = map[rune]string{'f': "்", 'F': "ஃ", 'T': "க்ஷ", 'Y': "ஸ்ரீ"}

// Pairs where Tamil99 puts the pulli on the first consonant by itself.
var tamil99Clusters = map[string]bool{"ஙக": true, "ஞச": true, "நத": true, "ணட": true, "மப": true, "னற": true}

// Type returns what typing keys (QWERTY letters) on the layout produces.
func Type(keys, layout string) (string, error) {
	switch layout {
	case "inscript-hindi", "inscript":
		var out strings.Builder
		for _, k := range keys {
			if text, ok := inscript[k]; ok {
				out.WriteString(text)
			} else {
				out.WriteRune(k)
			}
		}
		return out.String(), nil
	case "tamil99":
		var out strings.Builder
		previous := "" // the consonant just typed, still bare
		for _, k := range keys {
			if v, ok := tamil99Vowels[k]; ok {
				if previous != "" {
					out.WriteString(v[1])
				} else {
					out.WriteString(v[0])
				}
				previous = ""
				continue
			}
			if c, ok := tamil99Consonants[k]; ok {
				if previous != "" && (previous == c || tamil99Clusters[previous+c]) {
					out.WriteString("்")
				}
				out.WriteString(c)
				previous = c
				continue
			}
			if o, ok := tamil99Other[k]; ok {
				out.WriteString(o)
			} else {
				out.WriteRune(k)
			}
			previous = ""
		}
		return out.String(), nil
	}
	return "", errors.New("unknown layout; use " + strings.Join(Layouts, " or "))
}

// Keys returns the keys to press on the layout to type text: the reverse of Type.
func Keys(text, layout string) (string, []string, error) {
	var table map[string]string
	switch layout {
	case "inscript-hindi", "inscript":
		table = map[string]string{}
		for k, v := range inscript {
			if _, taken := table[v]; !taken || strings.ContainsRune("1234567890(),.-", k) {
				table[v] = string(k)
			}
		}
	case "tamil99":
		table = map[string]string{}
		for k, v := range tamil99Consonants {
			table[v] = string(k)
		}
		for k, v := range tamil99Other {
			table[v] = string(k)
		}
		for k, v := range tamil99Vowels {
			table[v[0]] = string(k)
			if v[1] != "" {
				table["\x00"+v[1]] = string(k) // a sign after a consonant
			}
		}
	default:
		return "", nil, errors.New("unknown layout; use " + strings.Join(Layouts, " or "))
	}
	// Longest match first, so ज्ञ is one key rather than three.
	pieces := make([]string, 0, len(table))
	for piece := range table {
		pieces = append(pieces, piece)
	}
	sort.Slice(pieces, func(i, j int) bool { return len(pieces[i]) > len(pieces[j]) })
	var out strings.Builder
	missing := map[string]bool{}
	afterConsonant := false
	for len(text) > 0 {
		matched := false
		if layout == "tamil99" && afterConsonant {
			for _, sign := range []string{"ா", "ி", "ீ", "ு", "ூ", "ெ", "ே", "ை", "ொ", "ோ", "ௌ"} {
				if strings.HasPrefix(text, sign) {
					out.WriteString(table["\x00"+sign])
					text, matched, afterConsonant = text[len(sign):], true, false
					break
				}
			}
			if matched {
				continue
			}
			if strings.HasPrefix(text, "்") {
				// Typing the same consonant again, or a cluster pair, adds the pulli.
				rest := text[len("்"):]
				out.WriteString("f")
				text = rest
				afterConsonant = false
				continue
			}
		}
		for _, piece := range pieces {
			if piece[0] != 0 && strings.HasPrefix(text, piece) {
				out.WriteString(table[piece])
				_, afterConsonant = tamil99ConsonantSet[piece]
				text, matched = text[len(piece):], true
				break
			}
		}
		if !matched {
			r := []rune(text)[0]
			if r >= 0x0900 {
				missing[string(r)] = true
			}
			out.WriteRune(r)
			text = text[len(string(r)):]
			afterConsonant = false
		}
	}
	list := []string{}
	for m := range missing {
		list = append(list, m)
	}
	sort.Strings(list)
	return out.String(), list, nil
}

var tamil99ConsonantSet = func() map[string]bool {
	set := map[string]bool{}
	for _, c := range tamil99Consonants {
		set[c] = true
	}
	return set
}()
