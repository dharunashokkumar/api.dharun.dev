package api

// The catalog of set E, AI and agent tools (sete.go).

func withSetE(groups []apiGroup) []apiGroup {
	return append(groups,
		apiGroup{
			ID: "rag", Category: "language", Title: "Search your own documents",
			Tagline: "Upload documents, ask a question, get the best passages back: retrieval for your own AI app.",
			Intro:   "Make a collection, add documents (plain text or Markdown; take a PDF's text with POST /v1/pdf/text first), and query it: the documents are cut into passages, embedded with the multilingual model on this server, and each query's nearest passages are reranked by the cross-encoder. Collections are private to their secret, kept in memory for a day, up to 1,000 passages. The collection demo (secret demo) holds a few facts about India, for trying." + keyNote,
			Endpoints: []endpoint{
				{Path: "/v1/rag", Summary: "Make a collection", Private: true, Key: true,
					Upload: &upload{Accepts: "Nothing, or {} .", Types: []string{"application/json"}, JSON: true}, Example: "/v1/rag", ExampleBody: "{}"},
				{Path: "/v1/rag/{id}/documents", Summary: "Add documents", Private: true, Key: true,
					Upload:      &upload{Accepts: `JSON {"documents": [{"name": "...", "text": "..."}], "size": 800}: up to 2 MB of text.`, Types: []string{"application/json"}, JSON: true},
					Example:     "/v1/rag/demo/documents?secret=demo",
					ExampleBody: `{"documents": [{"name": "shop rules", "text": "Returns are accepted within 7 days with the bill."}]}`},
				{Path: "/v1/rag/{id}/query", Summary: "Ask a question", Private: true, Key: true,
					Upload:      &upload{Accepts: `JSON {"query": "...", "top": 5, "rerank": true}.`, Types: []string{"application/json"}, JSON: true},
					Example:     "/v1/rag/demo/query?secret=demo",
					ExampleBody: `{"query": "How many digits does a pincode have?", "top": 2}`},
				{Path: "/v1/rag/{id}/delete", Summary: "Delete a collection", Private: true,
					Upload: &upload{Accepts: "Nothing, or {} .", Types: []string{"application/json"}, JSON: true}, Example: "/v1/rag/demo/delete?secret=demo", ExampleBody: "{}"},
			},
		},
		apiGroup{
			ID: "kv", Category: "developer", Title: "Agent notes",
			Tagline: "A small private key-value store, for agents and scripts that must remember between runs.",
			Intro:   "Make a store, then write and read notes by name over HTTP: plain text or JSON, up to 16 KB a note and 1 MB a store, kept 30 days after the last write (they survive restarts). Private to the secret, sent as X-Secret. The store demo (secret demo) is read-only." + keyNote,
			Endpoints: []endpoint{
				{Path: "/v1/kv", Summary: "Make a store", Private: true, Key: true,
					Upload: &upload{Accepts: "Nothing, or {} .", Types: []string{"application/json"}, JSON: true}, Example: "/v1/kv", ExampleBody: "{}"},
				{Path: "/v1/kv/{id}", Summary: "List notes", Private: true, Example: "/v1/kv/demo?secret=demo",
					Params: []param{{Name: "id", In: "path", Type: "string", Required: true, Doc: "The store."}, {Name: "prefix", In: "query", Type: "string", Doc: "Only names starting with this."}}},
				{Path: "/v1/kv/{id}/{key}", Summary: "Read or write a note", Private: true, Post: true,
					Doc:  "GET reads the note; POST writes the body as the note.",
					Body: "The note: text or JSON, up to 16 KB.", Example: "/v1/kv/demo/greeting?secret=demo", ExampleBody: `{"step": 2}`,
					Params: []param{{Name: "id", In: "path", Type: "string", Required: true, Doc: "The store."}, {Name: "key", In: "path", Type: "string", Required: true, Doc: "The note's name, up to 200 characters."}}},
				{Path: "/v1/kv/{id}/{key}/delete", Summary: "Delete a note", Private: true,
					Upload: &upload{Accepts: "Nothing, or {} .", Types: []string{"application/json"}, JSON: true}, Example: "/v1/kv/demo/greeting/delete?secret=demo", ExampleBody: "{}"},
			},
		},
	)
}
