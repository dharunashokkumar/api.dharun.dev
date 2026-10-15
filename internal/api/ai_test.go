package api

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/dharunashokkumar/api.dharun.dev/internal/onnxwire"
	"github.com/dharunashokkumar/api.dharun.dev/internal/subword"
)

// Tests of the AI model endpoints. The logic around the models is tested everywhere; the
// models themselves only where MODELS_DIR offers them and onnx-run is built (Linux), or
// must be when REQUIRE_MODELS is set.

func init() {
	mcpTestCalls["translate_text"] = `{"text":"Good morning.","from":"en","to":"hi"}`
	mcpTestCalls["transliterate_roman_typing"] = `{"text":"vanakkam","lang":"ta"}`
	mcpTestCalls["moderate_text"] = `{"text":"Have a lovely day"}`
	mcpTestCalls["rerank_documents"] = `{"query":"capital of Tamil Nadu","documents":["Chennai is the capital of Tamil Nadu.","Mumbai is a big city."]}`
	speech, _ := sampleFile("speech.mp3")
	mcpTestCalls["transcribe_audio"] = `{"file":"` + base64.StdEncoding.EncodeToString(speech) + `","language":"en"}`
}

func requireTextModel(t *testing.T, dataset string) {
	t.Helper()
	requireProgram(t, "onnx-run")
	requireDataset(t, dataset)
}

func TestOnnxWireRoundTrip(t *testing.T) {
	clientIn, serverOut := io.Pipe()
	serverIn, clientOut := io.Pipe()
	go onnxwire.NewConn(serverIn, serverOut).Serve(echoModel{})
	conn := onnxwire.NewConn(clientIn, clientOut)
	out, err := conn.Run(onnxwire.RunRequest{Model: "m", Inputs: []onnxwire.Tensor{
		{Name: "x", Type: onnxwire.Float32, Shape: []int64{1, 2}, Floats: []float32{1.5, -2}},
		{Name: "y", Type: onnxwire.Int64, Shape: []int64{3}, Ints: []int64{7, 8, 9}},
	}})
	if err != nil || len(out) != 2 || out[0].Floats[1] != -2 || out[1].Ints[2] != 9 {
		t.Fatalf("run = %v, %v", out, err)
	}
	if _, err := conn.Run(onnxwire.RunRequest{Model: "fail"}); err == nil || !strings.Contains(err.Error(), "no such model") {
		t.Errorf("a failing run gave %v", err)
	}
	seqs, err := conn.Generate(onnxwire.GenerateRequest{Input: []int64{4, 5}, Beams: 2, Best: 2})
	if err != nil || len(seqs) != 1 || len(seqs[0].IDs) != 2 || seqs[0].Score != 0.5 {
		t.Fatalf("generate = %v, %v", seqs, err)
	}
	clientOut.Close()
}

// echoModel answers a run with its inputs and a generation with its input reversed.
type echoModel struct{}

func (echoModel) Run(req onnxwire.RunRequest) ([]onnxwire.Tensor, error) {
	if req.Model == "fail" {
		return nil, errors.New("no such model")
	}
	return req.Inputs, nil
}

func (echoModel) Generate(req onnxwire.GenerateRequest) ([]onnxwire.Sequence, error) {
	return []onnxwire.Sequence{{Score: 0.5, IDs: []int64{req.Input[1], req.Input[0]}}}, nil
}

func TestMeanPool(t *testing.T) {
	b := newBatch([][]int64{{0, 5, 2}, {0, 2}}, xlmrPad)
	if b.width != 3 || b.mask[5] != 0 || b.ids[5] != xlmrPad {
		t.Fatalf("batch %+v", b)
	}
	hidden := onnxwire.Tensor{Type: onnxwire.Float32, Shape: []int64{2, 3, 2}, Floats: []float32{
		1, 0, 3, 0, 2, 0, // first: mean (2, 0)
		0, 1, 0, 3, 9, 9, // second: the padding (9, 9) is ignored; mean (0, 2)
	}}
	vectors, err := meanPool(hidden, b)
	if err != nil || vectors[0][0] != 1 || vectors[0][1] != 0 || vectors[1][0] != 0 || vectors[1][1] != 1 {
		t.Fatalf("pooled %v, %v", vectors, err)
	}
}

func TestScriptRunsAndPersonSpans(t *testing.T) {
	text := "Rahul met ராஜேஷ் குமார் in Chennai."
	runs, indic := scriptRuns(text)
	if len(runs) != 3 || indic[0] || !indic[1] || indic[2] || text[runs[1][0]:runs[1][1]] != "ராஜேஷ் குமார் " {
		t.Fatalf("runs %v %v", runs, indic)
	}
	// "Rah" "##ul" marked on the second piece only still covers the word.
	tokens := []subword.Token{{ID: 1, Start: 0, End: 3}, {ID: 2, Start: 3, End: 5}, {ID: 3, Start: 6, End: 9}}
	spans := personSpans("Rahul met", tokens, []bool{false, true, false})
	if len(spans) != 1 || spans[0] != [2]int{0, 5} {
		t.Errorf("spans %v", spans)
	}
}

func TestSplitRunsAndSubtitles(t *testing.T) {
	if got := splitRuns("vanakkam, chennai!", func(r rune) bool { return r >= 'a' && r <= 'z' }); strings.Join(got, "|") != "vanakkam|, |chennai|!" {
		t.Errorf("runs %q", got)
	}
	view, err := readWhisper([]byte(`{"result":{"language":"en"},"transcription":[{"offsets":{"from":0,"to":2500},"text":" Hello there."},{"offsets":{"from":2500,"to":61040},"text":" Bye."}]}`))
	if err != nil || view.Text != "Hello there. Bye." || view.Language != "en" {
		t.Fatalf("whisper %+v %v", view, err)
	}
	if srt := subtitles(view.Segments, "srt"); !strings.Contains(srt, "2\n00:00:02,500 --> 00:01:01,040\nBye.") {
		t.Errorf("srt %q", srt)
	}
	if vtt := subtitles(view.Segments, "vtt"); !strings.HasPrefix(vtt, "WEBVTT\n\n00:00:00.000 --> 00:00:02.500\nHello there.") {
		t.Errorf("vtt %q", vtt)
	}
}

func TestAIInputChecks(t *testing.T) {
	for path, want := range map[string]int{
		"/v1/embed":                                    http.StatusBadRequest,
		"/v1/embed?text=hello&type=document":           http.StatusBadRequest,
		"/v1/rerank?document=a":                        http.StatusBadRequest,
		"/v1/rerank?query=q&document=a&top=5":          http.StatusBadRequest,
		"/v1/transliterate/suggest?text=vanakkam":      http.StatusBadRequest,
		"/v1/transliterate/suggest?text=123&lang=ta":   http.StatusBadRequest,
		"/v1/translate?text=hello&from=en":             http.StatusBadRequest,
		"/v1/translate?text=hello&from=en&to=en":       http.StatusBadRequest,
		"/v1/translate?text=hello&from=en&to=xx":       http.StatusBadRequest,
		"/v1/text/moderate?text=hi&threshold=2":        http.StatusBadRequest,
		"/v1/text/moderate?text=hi&models=other":       http.StatusBadRequest,
		"/v1/text/redact?text=hi&names=maybe":          http.StatusBadRequest,
		"/v1/translate/languages":                      http.StatusOK,
		"/v1/text/redact?text=" + url.QueryEscape("x"): http.StatusOK,
	} {
		if got := get(t, path); got.status != want {
			t.Errorf("%s = %d, want %d: %s", path, got.status, want, clip(string(got.body), 200))
		}
	}
	if got := postBody(t, "/v1/stt?format=docx", "audio/mpeg", []byte("x")); got.status != http.StatusBadRequest {
		t.Errorf("stt format = %d", got.status)
	}
}

func TestEmbedAndRerank(t *testing.T) {
	requireTextModel(t, "embeddings")
	got := request(t, http.MethodPost, "/v1/embed", `{"texts":["How do I renew my passport?","पासपोर्ट का नवीनीकरण कैसे करें?","Best biryani in Hyderabad"]}`, "Content-Type", "application/json")
	if got.status != http.StatusOK {
		t.Fatalf("embed = %d: %s", got.status, got.body)
	}
	vectors := got.data(t)["embeddings"].([]any)
	dot := func(a, b int) (sum float64) {
		for i, v := range vectors[a].([]any) {
			sum += v.(float64) * vectors[b].([]any)[i].(float64)
		}
		return sum
	}
	if len(vectors) != 3 || len(vectors[0].([]any)) != embedDimensions || dot(0, 1) <= dot(0, 2) {
		t.Errorf("the Hindi question should be closer than biryani: %.3f vs %.3f", dot(0, 1), dot(0, 2))
	}
	requireTextModel(t, "reranker")
	got = request(t, http.MethodPost, "/v1/rerank", rerankBody, "Content-Type", "application/json")
	if got.status != http.StatusOK {
		t.Fatalf("rerank = %d: %s", got.status, got.body)
	}
	results := got.data(t)["results"].([]any)
	if first := results[0].(map[string]any); first["index"].(float64) == 1 {
		t.Errorf("Mumbai ranked first: %s", got.body)
	}
}

func TestTransliterateSuggest(t *testing.T) {
	requireTextModel(t, "xlit")
	got := get(t, "/v1/transliterate/suggest?lang=ta&text="+url.QueryEscape("vanakkam chennai!"))
	if got.status != http.StatusOK {
		t.Fatalf("suggest = %d: %s", got.status, got.body)
	}
	if text := got.data(t)["text"]; text != "வணக்கம் சென்னை!" {
		t.Errorf("text %q", text)
	}
}

func TestModerate(t *testing.T) {
	requireTextModel(t, "moderation")
	kind := get(t, "/v1/text/moderate?text="+url.QueryEscape("Have a lovely day, see you tomorrow"))
	abuse := get(t, "/v1/text/moderate?text="+url.QueryEscape("tu bahut bakwas aadmi hai saala kamina"))
	if kind.status != http.StatusOK || abuse.status != http.StatusOK {
		t.Fatalf("moderate = %d, %d: %s %s", kind.status, abuse.status, kind.body, abuse.body)
	}
	if kind.data(t)["flagged"] != false || abuse.data(t)["flagged"] != true {
		t.Errorf("kind %s\nabuse %s", kind.body, abuse.body)
	}
}

func TestRedactNames(t *testing.T) {
	requireTextModel(t, "names")
	got := request(t, http.MethodPost, "/v1/text/redact", `{"text":"Rahul Gandhi met Sachin Tendulkar in Mumbai. ராஜேஷ் குமார் சென்னையில் வசிக்கிறார்.","names":"true"}`, "Content-Type", "application/json")
	if got.status != http.StatusOK {
		t.Fatalf("redact = %d: %s", got.status, got.body)
	}
	text := got.data(t)["text"].(string)
	if strings.Contains(text, "Rahul") || strings.Contains(text, "Sachin") || strings.Contains(text, "ராஜேஷ்") || !strings.Contains(text, "Mumbai") {
		t.Errorf("redacted %q", text)
	}
}

func TestTranslate(t *testing.T) {
	requireTextModel(t, "translation")
	for path, want := range map[string]string{
		"/v1/translate?from=en&to=hi&text=" + url.QueryEscape("My name is Rahul."):           "राहुल",
		"/v1/translate?from=hi&to=en&text=" + url.QueryEscape("मेरा नाम राहुल है।"):          "Rahul",
		"/v1/translate?from=ta&to=hi&text=" + url.QueryEscape("வணக்கம், எப்படி இருக்கீங்க?"): "",
	} {
		got := get(t, path)
		if got.status != http.StatusOK {
			t.Fatalf("%s = %d: %s", path, got.status, got.body)
		}
		if translation := got.data(t)["translation"].(string); translation == "" || !strings.Contains(translation, want) {
			t.Errorf("%s: %q", path, translation)
		}
	}
}

func TestSpeechToText(t *testing.T) {
	requireProgram(t, "whisper-cli", "ffmpeg")
	requireDataset(t, "whisper")
	got := postBody(t, "/v1/stt?language=en", "audio/mpeg", sampleOf(t, "speech.mp3"))
	if got.status != http.StatusOK {
		t.Fatalf("stt = %d: %s", got.status, got.body)
	}
	if text := strings.ToLower(got.data(t)["text"].(string)); !strings.Contains(text, "spoken by a computer") {
		t.Errorf("text %q", text)
	}
	if srt := postBody(t, "/v1/stt?format=srt", "audio/mpeg", sampleOf(t, "speech.mp3")); srt.status != http.StatusOK || !bytes.Contains(srt.body, []byte(" --> ")) {
		t.Errorf("srt = %d: %s", srt.status, srt.body)
	}
}
