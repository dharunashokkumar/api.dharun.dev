package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

// fakeLlama answers like llama-server's chat endpoint: JSON that fits the schema when
// one is asked for, a fixed sentence otherwise, and a stream when asked.
func fakeLlama(t *testing.T, seen *[]map[string]any) string {
	socket := filepath.Join(t.TempDir(), "s")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		*seen = append(*seen, body)
		messages := body["messages"].([]any)
		last := messages[len(messages)-1].(map[string]any)["content"].(string)
		if strings.Contains(last, "too long") {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"code":400,"message":"the request exceeds the available context size","type":"exceed_context_size_error"}}`)
			return
		}
		content := "Repo rate kept at 5.5 per cent"
		if format, ok := body["response_format"].(map[string]any); ok {
			schema := format["json_schema"].(map[string]any)["schema"].(map[string]any)
			props := schema["properties"].(map[string]any)
			out := map[string]any{}
			for name := range props {
				switch name {
				case "label":
					out[name] = "negative"
				case "labels":
					out[name] = []string{"negative"}
				default:
					out[name] = "x"
				}
			}
			encoded, _ := json.Marshal(out)
			content = string(encoded)
		}
		if body["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			for _, piece := range []string{"Hello", " there"} {
				fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":%q}}],\"model\":\"x.gguf\"}\n\n", piece)
			}
			fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":20,\"completion_tokens\":2,\"total_tokens\":22},\"model\":\"x.gguf\"}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		answer, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "finish_reason": "stop",
			"message": map[string]any{"role": "assistant", "content": content}}},
			"model": "x.gguf", "object": "chat.completion", "timings": map[string]any{"predicted_n": 10},
			"usage": map[string]any{"prompt_tokens": 50, "completion_tokens": 10, "total_tokens": 60}})
		w.Write(answer)
	})}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	return socket
}

func llmTestServer(t *testing.T, tokens int) (*Server, string, *[]map[string]any) {
	db, good, _, _ := fakeAccountsDB(t)
	live, files := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(files, "model.gguf"), []byte("not a real model"), 0o644); err != nil {
		t.Fatal(err)
	}
	buildLiveFixture(t, live, "llm", files, dataset.Source{Name: "fake model", Licence: "test"})
	server := newTestServer(Config{Version: "t", AccountsDB: db, DataDir: live, StateDir: t.TempDir(), LLMTokensPerDay: tokens})
	t.Cleanup(server.Close)
	var seen []map[string]any
	socket := fakeLlama(t, &seen)
	server.extras.models.startLLM = func(kind modelKind, model string) (*modelWorker, error) {
		if !strings.HasSuffix(model, ".gguf") {
			t.Errorf("model file %q", model)
		}
		return &modelWorker{kind: kind, web: unixClient(socket)}, nil
	}
	return server, good, &seen
}

func TestLLMTasks(t *testing.T) {
	server, key, seen := llmTestServer(t, 0)
	post := func(path, body string, headers ...string) map[string]any {
		t.Helper()
		got := send(server, "POST", path, body, append([]string{"Content-Type", "application/json", "X-API-Key", key}, headers...)...)
		if got.Code != http.StatusOK {
			t.Fatalf("%s = %d %s", path, got.Code, got.Body)
		}
		var out map[string]any
		json.Unmarshal(got.Body.Bytes(), &out)
		return out["data"].(map[string]any)
	}
	if got := post("/v1/llm/summarise", llmSummariseBody); got["summary"] != "Repo rate kept at 5.5 per cent" || got["style"] != "headline" {
		t.Errorf("summarise: %v", got)
	}
	if got := post("/v1/llm/classify", llmClassifyBody); got["label"] != "negative" {
		t.Errorf("classify: %v", got)
	}
	// The labels reach the model as an enum it must keep to.
	format := (*seen)[len(*seen)-1]["response_format"].(map[string]any)
	enum := format["json_schema"].(map[string]any)["schema"].(map[string]any)["properties"].(map[string]any)["label"].(map[string]any)["enum"]
	if fmt.Sprint(enum) != "[positive negative neutral]" {
		t.Errorf("classify schema: %v", enum)
	}
	if got := post("/v1/llm/extract", llmExtractBody); len(got["data"].(map[string]any)) != 4 {
		t.Errorf("extract: %v", got)
	}
	if got := post("/v1/llm/extract", `{"text":"Ravi, Coimbatore","schema":{"type":"object","properties":{"name":{"type":"string"}}}}`); got["data"].(map[string]any)["name"] != "x" {
		t.Errorf("extract with schema: %v", got)
	}
	if got := post("/v1/llm/rewrite", llmRewriteBody); got["text"] == "" || got["usage"].(map[string]any)["prompt_tokens"] != float64(50) {
		t.Errorf("rewrite: %v", got)
	}
	// The text is wrapped as data, never as instructions.
	if last := (*seen)[len(*seen)-1]["messages"].([]any)[1].(map[string]any)["content"].(string); !strings.HasPrefix(last, "<text>") {
		t.Errorf("text not wrapped: %q", last)
	}
	if used := server.usage.tokensUsed("key1goodxxxxxxx"); used != 300 {
		t.Errorf("tokens counted: %d", used)
	}
	for path, body := range map[string]string{
		"/v1/llm/summarise": `{"text":""}`,
		"/v1/llm/classify":  `{"text":"x","labels":["only one"]}`,
		"/v1/llm/extract":   `{"text":"x"}`,
		"/v1/llm/rewrite":   `{"text":"x","style":"pirate"}`,
	} {
		if got := send(server, "POST", path, body, "X-API-Key", key); got.Code != http.StatusBadRequest {
			t.Errorf("%s %s = %d", path, body, got.Code)
		}
	}
	if got := send(server, "POST", "/v1/llm/summarise", `{"text":"too long"}`, "X-API-Key", key); got.Code != http.StatusBadRequest || errorCode(got) != "llm_refused" {
		t.Errorf("context refusal = %d %s", got.Code, got.Body)
	}
	// Without a key: 401, notice period or not.
	if got := send(server, "POST", "/v1/llm/summarise", llmSummariseBody); got.Code != http.StatusUnauthorized || errorCode(got) != "key_required" {
		t.Errorf("keyless = %d %s", got.Code, got.Body)
	}
}

func TestLLMTokenAllowance(t *testing.T) {
	server, key, _ := llmTestServer(t, 100)
	first := send(server, "POST", "/v1/llm/rewrite", llmRewriteBody, "X-API-Key", key)
	if first.Code != http.StatusOK || first.Header().Get("X-Token-Remaining") != "100" {
		t.Fatalf("first = %d %s %s", first.Code, first.Header().Get("X-Token-Remaining"), first.Body)
	}
	send(server, "POST", "/v1/llm/rewrite", llmRewriteBody, "X-API-Key", key) // 120 used now
	if got := send(server, "POST", "/v1/llm/rewrite", llmRewriteBody, "X-API-Key", key); got.Code != http.StatusTooManyRequests || errorCode(got) != "token_quota_exceeded" {
		t.Errorf("over the allowance = %d %s", got.Code, got.Body)
	}
}

func TestChatCompletions(t *testing.T) {
	server, key, seen := llmTestServer(t, 0)
	got := send(server, "POST", "/v1/chat/completions", llmChatBody, "Authorization", "Bearer "+key)
	if got.Code != http.StatusOK {
		t.Fatalf("chat = %d %s", got.Code, got.Body)
	}
	var answer map[string]any
	json.Unmarshal(got.Body.Bytes(), &answer)
	if answer["model"] != llmModelName || answer["timings"] != nil || answer["object"] != "chat.completion" || answer["data"] != nil {
		t.Errorf("chat answer: %v", answer)
	}
	if sent := (*seen)[len(*seen)-1]; sent["max_tokens"] != float64(80) || sent["model"] != nil {
		t.Errorf("passed on: %v", sent)
	}
	stream := send(server, "POST", "/v1/chat/completions", `{"messages":[{"role":"user","content":"hi"}],"stream":true}`, "X-API-Key", key)
	body := stream.Body.String()
	if stream.Header().Get("Content-Type") != "text/event-stream" || !strings.Contains(body, `"content":"Hello"`) ||
		!strings.Contains(body, `"model":"`+llmModelName+`"`) || !strings.HasSuffix(body, "data: [DONE]\n\n") {
		t.Errorf("stream: %q", body)
	}
	if used := server.usage.tokensUsed("key1goodxxxxxxx"); used != 82 {
		t.Errorf("tokens counted: %d", used)
	}
	for _, bad := range []string{`{}`, `{"messages":[{"role":"tool","content":"x"}]}`, `{"messages":[{"role":"user","content":"x"}],"tools":[]}`,
		`{"messages":[{"role":"user","content":"x"}],"max_tokens":5000}`, `{"messages":[{"role":"user","content":[{"type":"image_url"}]}]}`} {
		if got := send(server, "POST", "/v1/chat/completions", bad, "X-API-Key", key); got.Code != http.StatusBadRequest {
			t.Errorf("%s = %d", bad, got.Code)
		}
	}
	if got := send(server, "GET", "/v1/models", ""); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), llmModelName) {
		t.Errorf("models = %d %s", got.Code, got.Body)
	}
}
