package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Speech to text with OpenAI's Whisper (small, 5-bit, dataset "whisper") through
// whisper.cpp, which the release carries. ffmpeg first turns whatever was sent into the
// 16 kHz mono audio Whisper reads; only the first five minutes are heard. Whisper can
// also translate what it hears into English.

const sttMaxSec = 300

// sttLanguages are the languages a caller may name; Whisper knows about a hundred, and
// "auto" lets it decide from the first thirty seconds.
var sttLanguages = map[string]string{
	"auto": "detect", "en": "English", "hi": "Hindi", "ta": "Tamil", "te": "Telugu", "kn": "Kannada",
	"ml": "Malayalam", "mr": "Marathi", "bn": "Bengali", "gu": "Gujarati", "pa": "Punjabi", "ur": "Urdu",
	"as": "Assamese", "ne": "Nepali", "sa": "Sanskrit", "sd": "Sindhi", "or": "Odia",
	"ar": "Arabic", "fr": "French", "de": "German", "es": "Spanish", "zh": "Chinese", "ja": "Japanese",
}

type sttSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

type sttView struct {
	Language string       `json:"language"`
	Task     string       `json:"task"`
	Heard    float64      `json:"heard"`
	Cut      bool         `json:"cut"`
	Text     string       `json:"text"`
	Segments []sttSegment `json:"segments"`
}

func (s *Server) handleSTT(w http.ResponseWriter, r *http.Request, in uploadInput) {
	language := strings.ToLower(strings.TrimSpace(in.option(r, "language")))
	if language == "" {
		language = "auto"
	}
	if _, ok := sttLanguages[language]; !ok {
		s.badRequest(w, r, "language must be auto or one of: en, hi, ta, te, kn, ml, mr, bn, gu, pa, ur, as, ne, sa, sd, or, ar, fr, de, es, zh, ja.")
		return
	}
	task := strings.ToLower(strings.TrimSpace(in.option(r, "task")))
	if task == "" {
		task = "transcribe"
	}
	if task != "transcribe" && task != "translate" {
		s.badRequest(w, r, "task must be transcribe (text in the language spoken) or translate (into English).")
		return
	}
	format := strings.ToLower(strings.TrimSpace(in.option(r, "format")))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "srt" && format != "vtt" && format != "text" {
		s.badRequest(w, r, "format must be json, srt, vtt or text.")
		return
	}
	set := s.need(w, r, "whisper")
	if set == nil {
		return
	}
	dir, path, done, err := writeUpload(in.Files[0].Data, "in")
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer done()
	// Raw samples, a second more than is heard, to tell whether the audio goes on.
	pcm, err := s.run(r.Context(), job{program: "ffmpeg", args: []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-i", path,
		"-vn", "-ac", "1", "-ar", "16000", "-f", "s16le", "-t", itoa(sttMaxSec + 1), "-"}, dir: dir, timeout: 30 * time.Second})
	if err != nil {
		if errorsIsNoProgram(err) {
			s.toolFailed(w, r, err, "read audio")
			return
		}
		s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "The file is not audio this server can read.")
		return
	}
	if len(pcm) <= 3200 {
		s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "The file holds no audio, or less than a tenth of a second.")
		return
	}
	cut := len(pcm) > sttMaxSec*32000
	if cut {
		pcm = pcm[:sttMaxSec*32000]
	}
	heard := float64(len(pcm)) / 32000
	wav := filepath.Join(dir, "speech.wav")
	if err := os.WriteFile(wav, wavFile(pcm, 16000), 0o600); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	args := []string{"-m", set.attachment("model"), "-f", wav, "-l", language, "-oj", "-of", filepath.Join(dir, "out"), "-np",
		"-t", itoa(min(runtime.NumCPU(), 8))}
	if task == "translate" {
		args = append(args, "-tr")
	}
	if _, err := s.run(r.Context(), job{program: "whisper-cli", args: args, dir: dir, timeout: 95 * time.Second}); err != nil {
		s.toolFailed(w, r, err, "recognise speech")
		return
	}
	raw, err := os.ReadFile(filepath.Join(dir, "out.json"))
	if err != nil {
		s.toolFailed(w, r, err, "recognise speech")
		return
	}
	view, err := readWhisper(raw)
	if err != nil {
		s.toolFailed(w, r, err, "recognise speech")
		return
	}
	view.Task, view.Heard, view.Cut = task, round2(heard), cut
	switch format {
	case "srt", "vtt":
		sendFile(w, r, []byte(subtitles(view.Segments, format)), map[string]string{"srt": "application/x-subrip", "vtt": "text/vtt"}[format]+"; charset=utf-8", "speech."+format)
		return
	case "text":
		sendFile(w, r, []byte(view.Text+"\n"), "text/plain; charset=utf-8", "speech.txt")
		return
	}
	note := "Whisper small: good on clear English and fair on Indian languages; check names and numbers."
	if cut {
		note += fmt.Sprintf(" Only the first %d seconds were heard.", sttMaxSec)
	}
	s.ok(w, r, view, meta{Source: s.source("whisper"), Note: note}, "private, no-store")
}

// readWhisper reads the JSON whisper.cpp writes.
func readWhisper(raw []byte) (sttView, error) {
	var parsed struct {
		Result struct {
			Language string `json:"language"`
		} `json:"result"`
		Transcription []struct {
			Offsets struct {
				From int64 `json:"from"`
				To   int64 `json:"to"`
			} `json:"offsets"`
			Text string `json:"text"`
		} `json:"transcription"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return sttView{}, err
	}
	view := sttView{Language: parsed.Result.Language, Segments: []sttSegment{}}
	var text []string
	for _, seg := range parsed.Transcription {
		cleaned := strings.TrimSpace(seg.Text)
		if cleaned == "" {
			continue
		}
		view.Segments = append(view.Segments, sttSegment{Start: float64(seg.Offsets.From) / 1000, End: float64(seg.Offsets.To) / 1000, Text: cleaned})
		text = append(text, cleaned)
	}
	view.Text = strings.Join(text, " ")
	return view, nil
}

// subtitles writes segments as SubRip or WebVTT.
func subtitles(segments []sttSegment, format string) string {
	var b strings.Builder
	stamp := func(seconds float64) string {
		ms := int64(seconds*1000 + 0.5)
		separator := ","
		if format == "vtt" {
			separator = "."
		}
		return fmt.Sprintf("%02d:%02d:%02d%s%03d", ms/3600000, ms/60000%60, ms/1000%60, separator, ms%1000)
	}
	if format == "vtt" {
		b.WriteString("WEBVTT\n\n")
	}
	for i, seg := range segments {
		if format == "srt" {
			fmt.Fprintf(&b, "%d\n", i+1)
		}
		fmt.Fprintf(&b, "%s --> %s\n%s\n\n", stamp(seg.Start), stamp(seg.End), seg.Text)
	}
	return b.String()
}
