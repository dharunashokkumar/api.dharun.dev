// Command lockpdf writes a copy of a PDF protected by a password, for the sample file
// that /v1/pdf/unlock's example posts: go run ./tools/samples/lockpdf in.pdf out.pdf india
package main

import (
	"context"
	"log"
	"os"

	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func main() {
	if len(os.Args) != 4 {
		log.Fatal("usage: lockpdf in.pdf out.pdf password")
	}
	model.ConfigPath = "disable"
	in, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(os.Args[2])
	if err != nil {
		log.Fatal(err)
	}
	conf := model.NewAESConfiguration(os.Args[3], os.Args[3], 256)
	if err := pdfapi.Encrypt(context.Background(), in, out, conf); err != nil {
		log.Fatal(err)
	}
	if err := out.Close(); err != nil {
		log.Fatal(err)
	}
}
