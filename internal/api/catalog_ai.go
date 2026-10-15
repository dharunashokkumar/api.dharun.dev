package api

import (
	"net/url"
	"strings"
)

// The catalog of the AI model endpoints (round 3): embeddings, reranking, speech to text,
// moderation, name detection, translation and transliteration from Roman typing. Most
// join the group they belong with; embeddings and translation get groups of their own.

const (
	embedExample     = "How do I renew my passport?"
	moderateExample  = "tu bahut bakwas aadmi hai"
	translateExample = "The train to Chennai leaves at 6 pm. Please reach the station early."
	xlitExample      = "vanakkam chennai"
	namesExample     = "Rahul Sharma called 98765 43210 from Pune."
	rerankBody       = `{"query": "capital of Tamil Nadu", "documents": ["Chennai is the capital of Tamil Nadu.", "Mumbai is the financial capital of India.", "சென்னை தமிழ்நாட்டின் தலைநகரம்."]}`
	embedBody        = `{"texts": ["How do I renew my passport?", "पासपोर्ट का नवीनीकरण कैसे करें?"], "type": "query"}`
)

func withAITools(base []apiGroup) []apiGroup {
	out := make([]apiGroup, 0, len(base)+2)
	for _, group := range base {
		endpoints := append([]endpoint{}, group.Endpoints...)
		switch group.ID {
		case "text":
			group.MoreSources = append(append([]string{}, group.MoreSources...), "names", "moderation")
			group.Tagline = "Chunk documents, redact identifiers and names, moderate, count tokens, detect language."
			for i, e := range endpoints {
				if e.Path == "/v1/text/redact" {
					e.Doc = strings.Replace(e.Doc, " Names and addresses are not detected.", " With names=true a model also masks people's names: IndicNER for Indian scripts, bert-base-NER for English. Street addresses are not detected.", 1)
					e.Params = append(append([]param{}, e.Params...), param{Name: "names", In: "query", Type: "string", Enum: []string{"true", "false"},
						Doc: "true also finds people's names with a model (slower; text up to 20,000 characters). Default false."})
					endpoints[i] = e
				}
			}
			endpoints = append(endpoints, endpoint{
				Path: "/v1/text/moderate", Summary: "Score text for abuse, Hinglish and Tanglish included", Post: true, Private: true,
				Doc: "Three classifiers each score the text from 0 to 1: a general multilingual one (toxic), one for Hindi typed in Roman letters (profanity, harassment, hate_speech, caste, communal, regional, misogyny) and one for Tamil typed in Roman letters (offensive). " +
					"flagged says whether any score reached the threshold. Up to 4,000 characters; the first 256 tokens are read. Scores are a model's guess: use them to sort or to ask a person, not to decide alone.",
				Body:        "The text.",
				Example:     "/v1/text/moderate?text=" + url.QueryEscape(moderateExample),
				ExampleBody: moderateExample,
				Params: []param{
					{Name: "text", In: "query", Type: "string", Doc: "The text (or POST it)."},
					{Name: "threshold", In: "query", Type: "string", Doc: "Score at which flagged becomes true, between 0 and 1. Default 0.5."},
					{Name: "models", In: "query", Type: "string", Doc: "Comma-separated: general, hinglish, tanglish. Default all three."},
				},
			})
		case "transliterate":
			group.MoreSources = append(append([]string{}, group.MoreSources...), "xlit")
			group.Tagline = "Rewrite text between Indian scripts and Roman letters, or turn Roman typing into an Indian script."
			group.Intro = strings.Replace(group.Intro, "Casual Roman typing (vanakkam) cannot be converted by rule and is not accepted as input.",
				"Casual Roman typing (vanakkam) cannot be converted by rule: /v1/transliterate/suggest guesses it with a model, as phone keyboards do.", 1)
			endpoints = append(endpoints, endpoint{
				Path: "/v1/transliterate/suggest", Summary: "Roman typing to an Indian script (type English, get Tamil)", Post: true, Private: true,
				Doc: "Guesses how words typed in Roman letters are written in an Indian script with IndicXlit (AI4Bharat), giving up to eight suggestions per word, best first, and the text with the best guess for each word. " +
					"21 languages. Up to 60 words; anything that is not Roman letters is kept as it is.",
				Body:        "The text in Roman letters.",
				Example:     "/v1/transliterate/suggest?lang=ta&text=" + url.QueryEscape(xlitExample),
				ExampleBody: xlitExample,
				Params: []param{
					{Name: "text", In: "query", Type: "string", Doc: "The text (or POST it)."},
					{Name: "lang", In: "query", Type: "string", Required: true, Doc: "as, bn, brx, gom, gu, hi, kn, ks, mai, ml, mni, mr, ne, or, pa, sa, sd, si, ta, te or ur."},
					{Name: "count", In: "query", Type: "integer", Doc: "Suggestions per word, 1 to 8. Default 4."},
				},
			})
		case "speech":
			group.MoreSources = append(append([]string{}, group.MoreSources...), "whisper")
			group.Title = "Speech"
			group.Tagline = "Speech to text, text to speech in Hindi, Telugu, Malayalam and English, and noise removal."
			group.Intro = "Turn a recording into text (and subtitles) with Whisper, turn text into natural speech with Piper voices, or clean background noise from a recording with DeepFilterNet. " +
				"Each voice keeps the licence of the recordings it learnt from: /v1/tts/voices lists them, and two Hindi voices are for non-commercial use only. There is no Tamil, Kannada, Bengali, Gujarati or Marathi voice yet. Nothing you send is kept or logged."
			endpoints = append([]endpoint{{
				Path: "/v1/stt", Summary: "Speech to text, with subtitles", Private: true,
				Doc: "Send a recording (MP3, WAV, Ogg, M4A, WebM, video files too) and get the text with timed segments, as JSON, SRT or WebVTT subtitles, or plain text. Whisper small recognises about a hundred languages, Indian ones included; " +
					"it is good on clear English and fair on Indian languages. task=translate gives English text whatever was spoken. The first five minutes are heard." + formNote,
				Example: "/v1/stt",
				Upload:  &upload{Accepts: "An audio or video file, up to 10 MB.", Types: []string{"audio/mpeg", "audio/wav", "audio/ogg", "audio/mp4", "audio/webm", "video/mp4"}, Sample: "speech.mp3"},
				Params: []param{
					{Name: "language", In: "query", Type: "string", Doc: "auto (default) or the language spoken: en, hi, ta, te, kn, ml, mr, bn, gu, pa, ur, as, ne, sa, sd, or, ar, fr, de, es, zh, ja. Naming it helps on short clips."},
					{Name: "task", In: "query", Type: "string", Enum: []string{"transcribe", "translate"}, Doc: "transcribe (default) writes what was said; translate writes it in English."},
					{Name: "format", In: "query", Type: "string", Enum: []string{"json", "srt", "vtt", "text"}, Doc: "Default json."},
				},
			}}, endpoints...)
		}
		group.Endpoints = endpoints
		out = append(out, group)
	}
	return append(out, aiGroups...)
}

var aiGroups = []apiGroup{
	{
		ID:        "embeddings",
		Category:  "language",
		Title:     "Embeddings and reranking",
		Tagline:   "Vectors for semantic search in Indian languages and English, and a reranker to order results.",
		Intro:     "For retrieval (RAG) and search without an API key: turn text into 384 numbers with multilingual-e5-small, so texts that mean the same land close together across about a hundred languages, Hindi, Tamil and the other Indian languages among them; then let bge-reranker-v2-m3 read the query with each candidate and order them. Nothing you send is kept or logged.",
		SourceKey: "embeddings", MoreSources: []string{"reranker"},
		Endpoints: []endpoint{
			{
				Path: "/v1/embed", Summary: "Text embeddings", Post: true, Private: true,
				Doc: "Up to 64 texts a request (GET: one or more text parameters; POST: JSON {\"texts\": [...], \"type\": ...}). Each comes back as 384 numbers scaled to length 1, so a dot product is the cosine similarity. " +
					"Embed questions with type=query and the documents searched with type=passage, as the model was trained. Each text is read up to 512 tokens.",
				Body:        `JSON: {"texts": ["..."], "type": "query" or "passage"}, or {"text": "..."}.`,
				Example:     "/v1/embed?type=query&text=" + url.QueryEscape(embedExample),
				ExampleBody: embedBody,
				Params: []param{
					{Name: "text", In: "query", Type: "string", Doc: "A text; repeat for several (or POST them)."},
					{Name: "type", In: "query", Type: "string", Enum: []string{"query", "passage"}, Doc: "query (default) for questions, passage for the documents searched."},
				},
			},
			{
				Path: "/v1/rerank", Summary: "Rerank documents for a query", Post: true, Private: true,
				Doc: "Scores how well each document answers the query, from 0 to 1, and returns them best first with their place in the request. Up to 100 documents of up to 8,000 characters; each pair is read up to 512 tokens. " +
					"GET takes query and repeated document parameters; POST takes JSON.",
				Body:        `JSON: {"query": "...", "documents": ["...", "..."], "top": 3}.`,
				Example:     "/v1/rerank?query=" + url.QueryEscape("capital of Tamil Nadu") + "&document=" + url.QueryEscape("Chennai is the capital of Tamil Nadu.") + "&document=" + url.QueryEscape("Mumbai is the financial capital of India."),
				ExampleBody: rerankBody,
				Params: []param{
					{Name: "query", In: "query", Type: "string", Required: true, Doc: "The question, up to 2,000 characters."},
					{Name: "document", In: "query", Type: "string", Doc: "A candidate; repeat for each (or POST them)."},
					{Name: "top", In: "query", Type: "integer", Doc: "Return only the best few. Default all."},
				},
			},
		},
	},
	{
		ID:        "translate",
		Category:  "language",
		Title:     "Translation",
		Tagline:   "Translate between English and 22 Indian languages with IndicTrans2.",
		Intro:     "Machine translation by AI4Bharat's IndicTrans2 (distilled models), between English and Assamese, Bengali, Bodo, Dogri, Gujarati, Hindi, Kannada, Kashmiri, Konkani, Maithili, Malayalam, Manipuri, Marathi, Nepali, Odia, Punjabi, Sanskrit, Santali, Sindhi, Tamil, Telugu and Urdu. Between two Indian languages the text goes through English. Web and e-mail addresses and long numbers are kept as they are. Nothing you send is kept or logged.",
		SourceKey: "translation",
		Endpoints: []endpoint{
			{
				Path: "/v1/translate", Summary: "Translate text", Post: true, Private: true,
				Doc:         "Translates a sentence at a time and returns the whole translation and each sentence. Up to 2,000 characters and 30 sentences. Write Indian languages in their own script.",
				Body:        "The text to translate.",
				Example:     "/v1/translate?from=en&to=ta&text=" + url.QueryEscape(translateExample),
				ExampleBody: translateExample,
				Params: []param{
					{Name: "text", In: "query", Type: "string", Doc: "The text (or POST it)."},
					{Name: "from", In: "query", Type: "string", Required: true, Doc: "The language of the text: en, or a code from /v1/translate/languages such as hi or ta."},
					{Name: "to", In: "query", Type: "string", Required: true, Doc: "The language to translate into."},
				},
			},
			{
				Path: "/v1/translate/languages", Summary: "Languages", Reference: true,
				Doc:     "Every language with its short code and FLORES-200 code; either can be used as from and to.",
				Example: "/v1/translate/languages",
			},
		},
	},
}
