//go:build !cgo

// Command onnx-run needs cgo to link ONNX Runtime. Built without it, it only says so;
// the release is built on Linux with cgo (tools/build-release.sh).
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "onnx-run: built without cgo, so it cannot run models")
	os.Exit(1)
}
