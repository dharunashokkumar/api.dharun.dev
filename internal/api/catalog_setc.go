package api

import "strconv"

// The catalog of set C (setc.go, setc_text.go).

func withSetC(groups []apiGroup) []apiGroup {
	return append(groups,
		apiGroup{
			ID: "festivals", Category: "dates", Title: "Festivals",
			Tagline: "Hindu, Tamil, Sikh, Jain, Buddhist, Islamic and Christian festival dates for any year, computed.",
			Intro:   "Festival dates for 1950 to 2100, worked out from the panchang by each festival's own rule (Diwali by the amavasya at sunset, Ganesh Chaturthi by the chaturthi at midday, Karthigai Deepam by the evening nakshatra), with the rule given beside each date. Islamic dates come from the tabular calendar and are marked expected. Also as a calendar feed. For official holidays, see Holidays.",
			Endpoints: []endpoint{
				{Path: "/v1/festivals/{year}", Summary: "Festivals in a year", Reference: true,
					Example: "/v1/festivals/2026?month=11",
					Params: []param{{Name: "year", In: "path", Type: "integer", Required: true, Doc: strconv.Itoa(1950) + " to 2100."},
						{Name: "kind", In: "query", Type: "string", Enum: festivalKinds, Doc: "Only one kind."},
						{Name: "state", In: "query", Type: "string", Doc: "Leave out regional festivals not kept in this state."},
						{Name: "month", In: "query", Type: "integer", Doc: "1 to 12."}}},
				{Path: "/v1/festivals/{year}.ics", Summary: "Festivals as a calendar to subscribe to", Output: "calendar", Reference: true,
					Example: "/v1/festivals/2026.ics?state=TN",
					Params: []param{{Name: "year", In: "path", Type: "integer", Required: true, Doc: "The year, followed by .ics."},
						{Name: "kind", In: "query", Type: "string", Enum: festivalKinds, Doc: "Only one kind."}, stateFilter}},
			},
		},
		apiGroup{
			ID: "indic", Category: "language", Title: "Indic text tools",
			Tagline: "Letters as readers count them, dictionary order, look-alike fixes, braille and keyboard layouts.",
			Intro:   "For text in Indian scripts: split into aksharas and count them, sort words in a language's dictionary order, repair sequences that only look like a letter, write Bharati braille, and convert between QWERTY keys and the InScript and Tamil99 layouts.",
			Endpoints: []endpoint{
				{Path: "/v1/indic/aksharas", Summary: "Split into letters and count them", Post: true, Reference: true,
					Body: "The text, as text/plain or JSON {\"text\": \"...\"}.", Example: "/v1/indic/aksharas?text=%E0%A4%95%E0%A5%8D%E0%A4%B7%E0%A4%A4%E0%A5%8D%E0%A4%B0%E0%A4%BF%E0%A4%AF", ExampleBody: "தமிழ்நாடு",
					Params: []param{{Name: "text", In: "query", Type: "string", Doc: "The text."}}},
				{Path: "/v1/indic/sort", Summary: "Dictionary order", Post: true, Reference: true,
					Body: "JSON {\"words\": [...], \"lang\": \"ta\"}.", Example: "/v1/indic/sort?lang=ta&words=%E0%AE%95%E0%AE%9F%E0%AE%B2%E0%AF%8D,%E0%AE%85%E0%AE%AE%E0%AF%8D%E0%AE%AE%E0%AE%BE,%E0%AE%86%E0%AE%B1%E0%AF%81",
					ExampleBody: `{"words": ["नमक", "आम", "कल"], "lang": "hi"}`,
					Params:      []param{{Name: "words", In: "query", Type: "string", Doc: "Words separated by commas."}, {Name: "lang", In: "query", Type: "string", Doc: "hi, ta, bn, te, kn, ml, mr, gu, pa, or or en. Default hi."}}},
				{Path: "/v1/indic/normalise", Summary: "Fix look-alike letters", Post: true, Reference: true,
					Body: "The text, as text/plain or JSON {\"text\": \"...\", \"strip\": \"true\"}.", Example: "/v1/indic/normalise?text=%E0%A4%85%E0%A4%BE%E0%A4%AE", ExampleBody: "अाम",
					Params: []param{{Name: "text", In: "query", Type: "string", Doc: "The text."}, {Name: "strip", In: "query", Type: "boolean", Doc: "Also remove zero-width characters."}}},
				{Path: "/v1/braille", Summary: "Bharati braille", Post: true, Reference: true,
					Doc:  "Text in any Indian script (and digits) as Unicode braille cells, Grade 1. Check with a braille reader before printing for others.",
					Body: "The text, as text/plain or JSON {\"text\": \"...\"}.", Example: "/v1/braille?text=%E0%A4%A8%E0%A4%AE%E0%A4%B8%E0%A5%8D%E0%A4%A4%E0%A5%87", ExampleBody: "வணக்கம்",
					Params: []param{{Name: "text", In: "query", Type: "string", Doc: "The text, up to 5,000 characters."}}},
				{Path: "/v1/keyboard/convert", Summary: "QWERTY keys and Indian layouts", Post: true, Reference: true,
					Doc:  "type: what pressing those QWERTY keys gives on the layout (to fix text typed with the wrong keyboard on). keys: the keys to press to type a text.",
					Body: "The text, as text/plain or JSON {\"text\": \"...\", \"layout\": \"tamil99\", \"direction\": \"type\"}.", Example: "/v1/keyboard/convert?layout=tamil99&text=vphhkf", ExampleBody: "kf;",
					Params: []param{{Name: "text", In: "query", Type: "string", Doc: "Keys, or text."}, {Name: "layout", In: "query", Type: "string", Enum: []string{"inscript-hindi", "tamil99"}, Doc: "Default inscript-hindi."},
						{Name: "direction", In: "query", Type: "string", Enum: []string{"type", "keys"}, Doc: "Default type."}}},
			},
		},
		apiGroup{
			ID: "helplines", Category: "places", Title: "Helplines",
			Tagline: "National emergency and helpline numbers.",
			Intro:   "112 for any emergency, and the national helplines for women, children, senior citizens, cyber fraud, mental health, railways, highways, consumers and more, with who runs each.",
			Endpoints: []endpoint{{Path: "/v1/helplines", Summary: "National helplines", Reference: true, Example: "/v1/helplines?q=cyber",
				Params: []param{{Name: "q", In: "query", Type: "string", Doc: "Search the names and purposes."}}}},
		},
	)
}
