package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

// MCP tools for the endpoints that take or make files. A file goes in as base64 and a
// file comes out as MCP content: an image or audio block, or a PDF as an embedded
// resource.

// privateTools carry what may be personal: their calls are kept out of the log.
var privateTools = map[string]bool{
	"make_gst_invoice": true, "compile_typst": true, "text_to_speech": true,
	"read_text_in_image": true, "read_barcodes": true,
}

// fileContent wraps a file a tool made.
func fileContent(data []byte, kind string) map[string]any {
	kind, _, _ = strings.Cut(kind, ";")
	encoded := base64.StdEncoding.EncodeToString(data)
	var block map[string]any
	switch {
	case strings.HasPrefix(kind, "image/") && kind != "image/svg+xml":
		block = map[string]any{"type": "image", "data": encoded, "mimeType": kind}
	case strings.HasPrefix(kind, "audio/"):
		block = map[string]any{"type": "audio", "data": encoded, "mimeType": kind}
	default:
		name := "file"
		if kind == "application/pdf" {
			name = "document.pdf"
		}
		block = map[string]any{"type": "resource", "resource": map[string]any{"uri": "data:" + name, "mimeType": kind, "blob": encoded}}
	}
	return map[string]any{"content": []any{block}}
}

// fileArgument decodes a base64 file argument, with or without a data: prefix.
func fileArgument(args map[string]any, name string) ([]byte, error) {
	raw := arg(args, name)
	if raw == "" {
		return nil, errors.New(name + " is required: the file, base64-encoded.")
	}
	if strings.HasPrefix(raw, "data:") {
		if _, after, ok := strings.Cut(raw, ","); ok {
			raw = after
		}
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, errors.New(name + " is not valid base64.")
	}
	if len(data) > mcpFileLimit {
		return nil, errors.New("The file is larger than 5 MB; use the REST endpoint for larger files.")
	}
	return data, nil
}

func init() {
	fileArg := map[string]any{"type": "string", "description": "The file, base64-encoded (a data: URL also works). Up to 5 MB."}
	mcpTools = append(mcpTools,
		mcpTool{
			Name:        "make_gst_invoice",
			Description: "Make an Indian GST tax invoice as a PDF from seller, buyer and items. Works out each line's taxable value, CGST and SGST (or UTGST) within a state or IGST across states, the HSN-wise summary, rounding and the amount in words. format=json returns the worked-out amounts only.",
			InputSchema: schema([]string{"invoice"}, map[string]any{
				"invoice": map[string]any{"type": "object", "description": "number, date (YYYY-MM-DD), seller {name, address, gstin, state}, buyer {name, address, gstin or state}, items [{description, hsn, quantity, unit, rate (before tax), discount, gst_rate (percent)}], optional place_of_supply, ship_to, bank {name, account, ifsc}, upi, notes, terms."},
				"format":  oneOf("pdf (default), png or json.", "pdf", "png", "json"),
			}),
			send: func(args map[string]any) (toolRequest, error) {
				invoice, ok := args["invoice"].(map[string]any)
				if !ok {
					return toolRequest{}, errors.New("invoice is required, as an object.")
				}
				body, _ := json.Marshal(invoice)
				return toolRequest{path: "/v1/invoice" + query(args, map[string]string{"format": "format"}), contentType: "application/json", body: body}, nil
			},
		},
		mcpTool{
			Name:        "compile_typst",
			Description: "Compile a Typst document to PDF (or one page as PNG). Noto fonts for Latin and nine Indian scripts are available; packages and files are not.",
			InputSchema: schema([]string{"source"}, map[string]any{
				"source": text("The Typst document."),
				"format": oneOf("pdf (default) or png.", "pdf", "png"),
				"page":   map[string]any{"type": "integer", "description": "png: which page. Default 1."},
			}),
			send: func(args map[string]any) (toolRequest, error) {
				source, err := required(args, "source")
				return toolRequest{path: "/v1/typst" + query(args, map[string]string{"format": "format", "page": "page"}), contentType: "text/plain", body: []byte(source)}, err
			},
		},
		mcpTool{
			Name:        "text_to_speech",
			Description: "Speak text aloud in Hindi, Telugu, Malayalam or English with Piper voices; returns MP3 audio. Write the text in its own script. Some Hindi voices are for non-commercial use only.",
			InputSchema: schema([]string{"text"}, map[string]any{
				"text":  text("What to say, up to 3,000 characters."),
				"voice": text("A voice ID (hi_IN-rohan-medium, te_IN-padmavathi-medium, ml_IN-meera-medium, en_GB-cori-medium...) or a language code: hi, te, ml, en."),
				"speed": map[string]any{"type": "number", "minimum": 0.5, "maximum": 2},
			}),
			send: func(args map[string]any) (toolRequest, error) {
				words, err := required(args, "text")
				return toolRequest{path: "/v1/tts" + query(args, map[string]string{"voice": "voice", "speed": "speed"}), contentType: "text/plain", body: []byte(words)}, err
			},
		},
		mcpTool{
			Name:        "read_text_in_image",
			Description: "OCR: read the text in a photo, scan or scanned PDF, in English and Indian languages (Hindi, Tamil, Telugu, Kannada, Malayalam, Bengali, Gujarati, Punjabi, Odia, Marathi).",
			InputSchema: schema([]string{"file"}, map[string]any{
				"file": fileArg,
				"lang": text("Languages the text uses, joined by +: eng, hin, tam, tel, kan, mal, ben, guj, pan, ori, mar. Up to three. Default eng."),
			}),
			send: func(args map[string]any) (toolRequest, error) {
				data, err := fileArgument(args, "file")
				return toolRequest{path: "/v1/ocr" + query(args, map[string]string{"lang": "lang"}), contentType: "application/octet-stream", body: data}, err
			},
		},
		mcpTool{
			Name:        "read_barcodes",
			Description: "Find and read every QR code and barcode (Data Matrix, Aztec, EAN, UPC, Code 128, Code 39, ITF, Codabar) in a picture.",
			InputSchema: schema([]string{"file"}, map[string]any{"file": fileArg}),
			send: func(args map[string]any) (toolRequest, error) {
				data, err := fileArgument(args, "file")
				return toolRequest{path: "/v1/barcode/read", contentType: "application/octet-stream", body: data}, err
			},
		},
	)
}
