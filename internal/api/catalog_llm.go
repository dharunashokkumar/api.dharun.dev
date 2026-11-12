package api

// The small hosted LLM (llm.go): English task endpoints and an OpenAI-compatible chat.

const (
	llmSummariseBody = `{"text": "The Reserve Bank of India kept the repo rate unchanged at 5.5 per cent on Wednesday. The Monetary Policy Committee voted unanimously and said inflation was within the target band, while growth stayed strong.", "style": "headline"}`
	llmClassifyBody  = `{"text": "The delivery came two days late and the box was torn.", "labels": ["positive", "negative", "neutral"]}`
	llmExtractBody   = `{"text": "Hi, I am Ravi Kumar from Coimbatore, call me on 98765 43210 about the 2 BHK flat.", "fields": ["name", "city", "phone", "requirement"]}`
	llmRewriteBody   = `{"text": "pls send the documents asap we are waiting since monday", "style": "polite"}`
	llmChatBody      = `{"model": "qwen2.5-1.5b-instruct", "messages": [{"role": "user", "content": "Write a two-line welcome message for a food delivery app."}], "max_tokens": 80}`
)

func withLLM(groups []apiGroup) []apiGroup {
	keyNote := " Needs a free API key; each key has 100,000 tokens a day (prompt and answer)."
	json := func() *upload {
		return &upload{Accepts: "A JSON document as below.", Types: []string{"application/json"}, JSON: true}
	}
	return append(groups, apiGroup{
		ID: "llm", Category: "language", Title: "Small LLM (English)",
		Tagline:   "Summarise, classify, extract to JSON and rewrite English text with a small open model, and an OpenAI-compatible chat.",
		Intro:     "A small language model (Qwen 2.5 1.5B Instruct, Apache-2.0) that runs on this server, for narrow tasks where its answer is forced into shape: a summary, a label from your list, fields as JSON that match your schema, a rewrite. And /v1/chat/completions in OpenAI's format, so code and tools written for that API work by changing the base URL to https://api.dharun.dev/v1. English only: it was measured and is not good enough in Hindi or Tamil. It can be wrong; check what matters. Nothing you send is logged or kept." + keyNote,
		SourceKey: "llm",
		Endpoints: []endpoint{
			{Path: "/v1/llm/summarise", Summary: "Summarise English text", Live: true, Private: true, Key: true, KeyOnly: true, Upload: json(),
				Example: "/v1/llm/summarise", ExampleBody: llmSummariseBody,
				Doc: "Send {text, style, words}: style paragraph (default), bullets or headline; words the length, 10 to 300 (default 60). Up to 12,000 characters. Answers {summary, style, truncated, usage}."},
			{Path: "/v1/llm/classify", Summary: "Pick a label for a text from your list", Live: true, Private: true, Key: true, KeyOnly: true, Upload: json(),
				Example: "/v1/llm/classify", ExampleBody: llmClassifyBody,
				Doc: "Send {text, labels, multi, instructions}: 2 to 30 labels; the answer is always one of them (label), or with multi true every one that fits (labels). instructions may say what the labels mean."},
			{Path: "/v1/llm/extract", Summary: "Extract fields from text as JSON", Live: true, Private: true, Key: true, KeyOnly: true, Upload: json(),
				Example: "/v1/llm/extract", ExampleBody: llmExtractBody,
				Doc: "Send {text, fields} (up to 40 names, each answered as a string) or {text, schema} (a JSON Schema of type object, up to 8 KB). The answer's data always matches the schema: decoding is constrained to it. Anything the text does not say comes back empty."},
			{Path: "/v1/llm/rewrite", Summary: "Rewrite text in another tone", Live: true, Private: true, Key: true, KeyOnly: true, Upload: json(),
				Example: "/v1/llm/rewrite", ExampleBody: llmRewriteBody,
				Doc: "Send {text, style}: style formal, simple, polite, shorter, friendly or grammar (the default: fix grammar and spelling only). Up to 4,000 characters."},
			{Path: "/v1/chat/completions", Summary: "Chat, OpenAI-compatible", Live: true, Private: true, Key: true, KeyOnly: true, Upload: json(),
				Example: "/v1/chat/completions", ExampleBody: llmChatBody, Output: "openai",
				Doc: "OpenAI's chat completions request and answer (not this API's envelope): messages, max_tokens up to 1,024 (default 512), temperature, top_p, stop, seed, response_format (json_object or json_schema) and stream (server-sent events). The window is 4,096 tokens; messages up to 16,000 characters. No tools, images or audio. Any model name is accepted."},
			{Path: "/v1/models", Summary: "The model list, for OpenAI clients", Example: "/v1/models", Reference: true, Output: "openai",
				Doc: "OpenAI's model list with the one model, for clients that ask before they chat."},
		},
	})
}
