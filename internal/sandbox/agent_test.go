package sandbox

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestFrames(t *testing.T) {
	var buffer bytes.Buffer
	WriteFrame(&buffer, FrameStdout, []byte("hello"))
	WriteFrame(&buffer, FrameExit, nil)
	kind, payload, err := ReadFrame(&buffer)
	if err != nil || kind != FrameStdout || string(payload) != "hello" {
		t.Fatalf("first frame: %c %q %v", kind, payload, err)
	}
	if kind, payload, err = ReadFrame(&buffer); err != nil || kind != FrameExit || len(payload) != 0 {
		t.Fatalf("second frame: %c %q %v", kind, payload, err)
	}
	// A frame claiming more than any frame may hold is refused before anything is allocated.
	header := []byte{FrameStdin, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(header[1:], maxFrame+1)
	if _, _, err := ReadFrame(bytes.NewReader(header)); !errors.Is(err, ErrFrameTooLarge) {
		t.Errorf("oversized frame: %v", err)
	}
}

func TestLanguagesAndAliases(t *testing.T) {
	for alias, name := range LanguageAliases {
		if _, ok := Languages[name]; !ok {
			t.Errorf("alias %s points at unknown language %s", alias, name)
		}
	}
	for name, language := range Languages {
		if language.File == "" || len(language.Command) == 0 {
			t.Errorf("%s is incomplete", name)
		}
	}
}
