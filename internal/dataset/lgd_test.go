package dataset

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// Records the platform sends pretty-printed must reach the build one per line.
func TestEachLineReadsCompactedRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.jsonl.gz")
	file, _ := os.Create(path)
	zipped := gzip.NewWriter(file)
	zipped.Write([]byte("{\"villageCode\":636547,\"villageNameEnglish\":\"Pallipalayam\"}\n{\"villageCode\":\"12\"}\n"))
	zipped.Close()
	file.Close()
	var codes []int
	if err := eachLine(path, func(r map[string]any) error { codes = append(codes, lgdCode(r, "villageCode")); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(codes) != 2 || codes[0] != 636547 || codes[1] != 12 {
		t.Errorf("codes %v", codes)
	}
}
