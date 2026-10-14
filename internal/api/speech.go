package api

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dharunashokkumar/api.dharun.dev/internal/dataset"
)

// Text to speech with Piper (MIT; the 2023 release, which brings espeak-ng, GPL-3.0, for
// pronunciation, all as its own program in the release) and the voices a server fetches
// (dataset "voices"). Denoising with DeepFilterNet (MIT or Apache-2.0, one program in the
// release). ffmpeg (installed on the server) turns audio into and out of the plain WAV
// these programs speak.

const (
	ttsMaxText    = 3000 // characters: about three minutes of speech; Hindi takes ~45 s on four CPUs
	denoiseMaxSec = 300
)

type voiceView struct {
	ID            string `json:"id"`
	Language      string `json:"language"`
	Name          string `json:"name"`
	Speaker       string `json:"speaker"`
	Licence       string `json:"licence"`
	LicenceURL    string `json:"licence_url"`
	Attribution   string `json:"attribution"`
	NonCommercial bool   `json:"non_commercial"`
}

func viewVoice(voice dataset.Voice) voiceView {
	return voiceView{ID: voice.ID, Language: voice.Language, Name: voice.Name, Speaker: voice.Speaker,
		Licence: voice.Licence, LicenceURL: voice.LicenceURL, Attribution: voice.Attribution, NonCommercial: voice.NonCommercial}
}

func (s *Server) handleVoices(w http.ResponseWriter, r *http.Request) {
	voices := make([]voiceView, 0, len(dataset.Voices))
	for _, voice := range dataset.Voices {
		voices = append(voices, viewVoice(voice))
	}
	total := len(voices)
	m := meta{Total: &total, Note: "Piper has no Tamil, Kannada, Bengali, Gujarati or working Marathi voice yet. Voices marked non_commercial may not be used for commercial work."}
	if source := s.source("voices"); source != nil {
		m.Source = source
	}
	s.ok(w, r, voices, m, cacheAPI)
}

// findVoice takes a voice ID, or a language (hi, te-IN, en-GB) for its first voice that
// may be used commercially.
func findVoice(value string) (dataset.Voice, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "en"
	}
	for _, voice := range dataset.Voices {
		if strings.EqualFold(voice.ID, value) {
			return voice, true
		}
	}
	for _, voice := range dataset.Voices {
		language := strings.ToLower(strings.ReplaceAll(value, "_", "-"))
		if !voice.NonCommercial && (strings.ToLower(voice.Language) == language || strings.HasPrefix(strings.ToLower(voice.Language), language+"-")) {
			return voice, true
		}
	}
	return dataset.Voice{}, false
}

// audioFormats are the formats speech and cleaned audio come back in, with ffmpeg's
// encoder settings. WAV is written here without ffmpeg.
var audioFormats = map[string]struct {
	mediaType, extension string
	args                 []string
}{
	"wav": {"audio/wav", ".wav", nil},
	"mp3": {"audio/mpeg", ".mp3", []string{"-c:a", "libmp3lame", "-q:a", "4", "-f", "mp3"}},
	"ogg": {"audio/ogg", ".ogg", []string{"-c:a", "libopus", "-b:a", "48k", "-f", "ogg"}},
}

// wavFile wraps 16-bit mono PCM in a WAV header.
func wavFile(pcm []byte, rate int) []byte {
	var out bytes.Buffer
	out.Grow(44 + len(pcm))
	write := func(v any) { binary.Write(&out, binary.LittleEndian, v) }
	out.WriteString("RIFF")
	write(uint32(36 + len(pcm)))
	out.WriteString("WAVEfmt ")
	write(uint32(16))
	write(uint16(1)) // PCM
	write(uint16(1)) // mono
	write(uint32(rate))
	write(uint32(rate * 2))
	write(uint16(2))
	write(uint16(16))
	out.WriteString("data")
	write(uint32(len(pcm)))
	out.Write(pcm)
	return out.Bytes()
}

// encodeAudio turns a WAV file into the format asked for.
func (s *Server) encodeAudio(r *http.Request, dir string, wav []byte, format string) ([]byte, error) {
	target := audioFormats[format]
	if target.args == nil {
		return wav, nil
	}
	args := append([]string{"-hide_banner", "-loglevel", "error", "-nostdin", "-f", "wav", "-i", "pipe:0", "-map_metadata", "-1"}, target.args...)
	return s.run(r.Context(), job{program: "ffmpeg", args: append(args, "pipe:1"), stdin: wav, dir: dir, timeout: 60 * time.Second})
}

func (s *Server) handleTTS(w http.ResponseWriter, r *http.Request) {
	in, ok := s.readText(w, r)
	if !ok {
		return
	}
	text := strings.Join(strings.Fields(in.Text), " ")
	if utf8.RuneCountInString(text) > ttsMaxText {
		s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "Speech is made from at most "+itoa(ttsMaxText)+" characters at a time.")
		return
	}
	voice, ok := findVoice(in.option(r, "voice"))
	if !ok {
		s.badRequest(w, r, "No such voice. /v1/tts/voices lists them; a language code such as hi or te picks its first voice.")
		return
	}
	format := strings.ToLower(in.option(r, "format"))
	if format == "" {
		format = "mp3"
	}
	if _, ok := audioFormats[format]; !ok {
		s.badRequest(w, r, "format must be mp3, ogg or wav.")
		return
	}
	speed := 1.0
	if raw := in.option(r, "speed"); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || value < 0.5 || value > 2 {
			s.badRequest(w, r, "speed must be from 0.5 to 2.")
			return
		}
		speed = value
	}
	set := s.need(w, r, "voices")
	if set == nil {
		return
	}
	modelName, configName := dataset.VoiceFiles(voice.ID)
	model, config := set.attachment(modelName), set.attachment(configName)
	if model == "" || config == "" {
		s.fail(w, r, http.StatusServiceUnavailable, "dataset_unavailable", "This server does not hold that voice yet.")
		return
	}
	settings, err := os.ReadFile(config)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	var described struct {
		Audio struct {
			SampleRate int `json:"sample_rate"`
		} `json:"audio"`
	}
	if json.Unmarshal(settings, &described) != nil || described.Audio.SampleRate == 0 {
		described.Audio.SampleRate = 22050
	}

	release, ok := s.takeSlot(w, r, s.models, modelWait)
	if !ok {
		return
	}
	defer release()
	allowTime(w, uploadReadTime, uploadWriteTime)
	dir, done, err := workspace()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer done()
	args := []string{"--model", model, "--config", config, "--output-raw", "--length_scale", strconv.FormatFloat(1/speed, 'f', 3, 64), "--quiet"}
	// One line: Piper speaks each line of its input separately.
	pcm, err := s.run(r.Context(), job{program: "piper", args: args, stdin: []byte(text + "\n"), dir: dir, timeout: 75 * time.Second})
	if err != nil {
		s.toolFailed(w, r, err, "make speech")
		return
	}
	if len(pcm) == 0 {
		s.badRequest(w, r, "The text gave no speech: it may hold only symbols.")
		return
	}
	audio, err := s.encodeAudio(r, dir, wavFile(pcm, described.Audio.SampleRate), format)
	if err != nil {
		s.toolFailed(w, r, err, "encode the audio")
		return
	}
	// The licence of a voice travels with what it said.
	w.Header().Set("X-Voice", voice.ID)
	w.Header().Set("X-Voice-Licence", voice.Licence)
	sendFile(w, r, audio, audioFormats[format].mediaType, "speech"+audioFormats[format].extension)
}

func (s *Server) handleDenoise(w http.ResponseWriter, r *http.Request, in uploadInput) {
	format := strings.ToLower(in.option(r, "format"))
	if format == "" {
		format = "mp3"
	}
	if _, ok := audioFormats[format]; !ok {
		s.badRequest(w, r, "format must be mp3, ogg or wav.")
		return
	}
	limit := 100
	if raw := in.option(r, "limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 6 || value > 100 {
			s.badRequest(w, r, "limit is the most noise is lowered by, in decibels, from 6 to 100.")
			return
		}
		limit = value
	}
	dir, path, done, err := writeUpload(in.Files[0].Data, "in")
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	defer done()
	// DeepFilterNet works on 48 kHz audio; anything ffmpeg can read is turned into that.
	clean := filepath.Join(dir, "speech.wav")
	_, err = s.run(r.Context(), job{program: "ffmpeg", args: []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-i", path,
		"-vn", "-ac", "1", "-ar", "48000", "-c:a", "pcm_s16le", "-t", itoa(denoiseMaxSec), "-map_metadata", "-1", clean}, dir: dir, timeout: 30 * time.Second})
	if err != nil {
		if errorsIsNoProgram(err) {
			s.toolFailed(w, r, err, "read audio")
			return
		}
		s.fail(w, r, http.StatusUnprocessableEntity, "unprocessable", "The file is not audio this server can read.")
		return
	}
	outDir := filepath.Join(dir, "out")
	if err := os.Mkdir(outDir, 0o700); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal error.")
		return
	}
	if _, err := s.run(r.Context(), job{program: "deep-filter", args: []string{"--atten-lim-db", itoa(limit), "--output-dir", outDir, clean}, dir: dir, timeout: 85 * time.Second}); err != nil {
		s.toolFailed(w, r, err, "clean the audio")
		return
	}
	wav, err := os.ReadFile(filepath.Join(outDir, "speech.wav"))
	if err != nil {
		s.toolFailed(w, r, err, "clean the audio")
		return
	}
	audio, err := s.encodeAudio(r, dir, wav, format)
	if err != nil {
		s.toolFailed(w, r, err, "encode the audio")
		return
	}
	sendFile(w, r, audio, audioFormats[format].mediaType, "clean"+audioFormats[format].extension)
}
