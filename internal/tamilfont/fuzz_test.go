package tamilfont

import (
	"bytes"
	"testing"
)

// View and Bytes are inverses for every byte string, and each encoding decodes any bytes
// and encodes any text without panicking.
func FuzzEncodings(f *testing.F) {
	f.Add([]byte("\xbe\xc1\xa2\xfa"), "தமிழ்")
	f.Add([]byte{0x81, 0x8d, 0x8f, 0x90, 0x9d}, "")
	f.Fuzz(func(t *testing.T, data []byte, text string) {
		back, err := Bytes(View(data))
		if err != nil || !bytes.Equal(back, data) {
			t.Errorf("Bytes(View(%x)) = %x, %v", data, back, err)
		}
		Bytes(text)
		HasTamil(text)
		for _, encoding := range Encodings {
			encoding.ToUnicode(data)
			encoding.FromUnicode(text)
		}
	})
}
