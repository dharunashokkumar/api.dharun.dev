package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"net/url"
)

// MCP tools for the rest of the API: making codes and images, the image, PDF and audio
// tools, text chunking, embeddings, Tamil legacy encodings and the states list. Pictures
// are asked for as PNG, which clients show; an SVG would arrive as a file.

// filesArgument decodes a list of base64 files, each up to mcpFileLimit.
func filesArgument(args map[string]any, name string, least, most int) ([][]byte, error) {
	list, _ := args[name].([]any)
	if len(list) < least || len(list) > most {
		return nil, fmt.Errorf("%s is required: %d to %d files, each base64-encoded.", name, least, most)
	}
	var files [][]byte
	for i, item := range list {
		data, err := fileArgument(map[string]any{name: item}, name)
		if err != nil {
			return nil, fmt.Errorf("file %d: %w", i+1, err)
		}
		files = append(files, data)
	}
	return files, nil
}

// multipartBody puts files in a multipart/form-data body, as the endpoints that take
// several files read them.
func multipartBody(files [][]byte, extension, kind string) ([]byte, string) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for i, data := range files {
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file%d"; filename="file%d.%s"`, i+1, i+1, extension))
		header.Set("Content-Type", kind)
		part, _ := form.CreatePart(header)
		part.Write(data)
	}
	form.Close()
	return body.Bytes(), form.FormDataContentType()
}

// uploadTool builds a tool that posts one file to a path, with the named arguments as its query.
func uploadTool(name, description, path string, options map[string]any, mapping map[string]string) mcpTool {
	properties := map[string]any{"file": map[string]any{"type": "string", "description": "The file, base64-encoded (a data: URL also works). Up to 5 MB."}}
	for key, value := range options {
		properties[key] = value
	}
	return mcpTool{
		Name: name, Description: description, InputSchema: schema([]string{"file"}, properties),
		send: func(args map[string]any) (toolRequest, error) {
			data, err := fileArgument(args, "file")
			return toolRequest{path: path + query(args, mapping), contentType: "application/octet-stream", body: data}, err
		},
	}
}

// textTool builds a tool that posts its text argument as text/plain.
func textTool(name, description, path, textArg, textDoc string, options map[string]any, mapping map[string]string) mcpTool {
	properties := map[string]any{textArg: text(textDoc)}
	for key, value := range options {
		properties[key] = value
	}
	return mcpTool{
		Name: name, Description: description, InputSchema: schema([]string{textArg}, properties),
		send: func(args map[string]any) (toolRequest, error) {
			words, err := required(args, textArg)
			return toolRequest{path: path + query(args, mapping), contentType: "text/plain", body: []byte(words)}, err
		},
	}
}

func init() {
	for _, name := range []string{
		"make_qr_code", "make_barcode", "make_code_image", "chunk_text", "embed_texts", "convert_tamil_legacy_text",
		"convert_image", "get_image_info", "remove_image_background", "hash_image", "compare_images",
		"read_pdf_text", "get_pdf_info", "render_pdf_page", "select_pdf_pages", "rotate_pdf_pages", "compress_pdf", "merge_pdfs",
		"remove_audio_noise", "probe_url_from_india",
	} {
		privateTools[name] = true
	}
	pages := text("Pages, such as 1-3,5 or 2,1 (a page may repeat).")
	pixels := func(description string) map[string]any {
		return map[string]any{"type": "integer", "description": description}
	}
	colours := map[string]any{"fg": text("Dark colour, hex such as 000000."), "bg": text("Light colour, hex, or transparent.")}

	qr := textTool("make_qr_code", "Make a QR code image (PNG) for any text or link.", "/v1/qr", "text", "What to encode, up to about 2,900 bytes.",
		map[string]any{"size": pixels("Width in pixels, 32 to 2048. Default 256."), "ecc": oneOf("Error correction. Default M.", "L", "M", "Q", "H"), "fg": colours["fg"], "bg": colours["bg"]},
		map[string]string{"size": "size", "ecc": "ecc", "fg": "fg", "bg": "bg", "format": "format"})
	barcode := mcpTool{
		Name:        "make_barcode",
		Description: "Make a barcode image (PNG): Code 128, Code 39, Code 93, EAN-13, EAN-8, ITF, Codabar, Data Matrix, PDF417 or Aztec.",
		InputSchema: schema([]string{"type", "text"}, map[string]any{
			"type":   oneOf("Kind of barcode.", barcodeKindIDs()...),
			"text":   text("What to encode."),
			"scale":  pixels("Linear codes: pixels per bar, 1 to 10. Default 2."),
			"height": pixels("Linear codes: bar height in pixels, 10 to 600. Default 80."),
			"size":   pixels("2D codes: width in pixels, 32 to 2048. Default 256."),
		}),
		send: func(args map[string]any) (toolRequest, error) {
			kind, err := required(args, "type")
			if err != nil {
				return toolRequest{}, err
			}
			words, err := required(args, "text")
			args["format"] = "png"
			return toolRequest{path: "/v1/barcode/" + url.PathEscape(kind) + query(args, map[string]string{"scale": "scale", "height": "height", "size": "size", "format": "format"}), contentType: "text/plain", body: []byte(words)}, err
		},
	}
	pngFirst := func(tool mcpTool) mcpTool {
		send := tool.send
		tool.send = func(args map[string]any) (toolRequest, error) {
			args["format"] = "png"
			return send(args)
		}
		return tool
	}
	codeImage := textTool("make_code_image", "Draw a code snippet as a PNG picture of an editor window with syntax colouring, for slides, posts and READMEs. Up to 150 lines of 140 characters.", "/v1/code-image", "code", "The code.",
		map[string]any{"language": text("Language name or file extension, such as go, python, js, sql. Guessed when left out."), "title": text("File name shown in the title bar."), "theme": oneOf("Default dark.", "dark", "light")},
		map[string]string{"language": "language", "title": "title", "theme": "theme", "format": "format"})

	mcpTools = append(mcpTools,
		pngFirst(qr), barcode, pngFirst(codeImage),
		mcpTool{
			Name:        "make_social_preview_image",
			Description: "Make a 1200 by 630 link preview (Open Graph) image as PNG from a title, with an optional subtitle, site name and tag. Latin and nine Indian scripts are set correctly. The same arguments always give the same picture.",
			InputSchema: schema([]string{"title"}, map[string]any{
				"title":    text("Up to 200 characters, in any of the supported scripts."),
				"subtitle": text("Up to 300 characters; two lines are shown."),
				"site":     text("Site name, bottom left."),
				"tag":      text("Short label, bottom right."),
				"theme":    oneOf("Default dark.", "dark", "light"),
			}),
			path: func(args map[string]any) (string, error) {
				_, err := required(args, "title")
				return "/v1/og-image" + query(args, map[string]string{"title": "title", "subtitle": "subtitle", "site": "site", "tag": "tag", "theme": "theme"}), err
			},
		},
		textTool("chunk_text", "Split a long document (plain text or Markdown) into readable chunks for retrieval (RAG): at headings, paragraphs, sentences and words in that order; Markdown chunks keep their heading path. Each chunk carries its character offsets.", "/v1/text/chunk", "text", "The document.",
			map[string]any{"size": pixels("Largest chunk in characters, 100 to 20000. Default 1000."), "overlap": pixels("Characters repeated from the previous chunk. Default a tenth of the size."), "format": oneOf("Default auto.", "auto", "markdown", "text")},
			map[string]string{"size": "size", "overlap": "overlap", "format": "format"}),
		mcpTool{
			Name:        "embed_texts",
			Description: "Turn texts into 384-number vectors with multilingual-e5-small (Indian languages and English), scaled to length 1 so a dot product is the cosine similarity. For storing in a vector database or comparing meaning; use rerank_documents to order a few passages for a question instead.",
			InputSchema: schema([]string{"texts"}, map[string]any{
				"texts": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Up to 64 texts."},
				"type":  oneOf("query (default) for questions, passage for the documents searched.", "query", "passage"),
			}),
			send: func(args map[string]any) (toolRequest, error) {
				texts, ok := args["texts"].([]any)
				if !ok || len(texts) == 0 {
					return toolRequest{}, errors.New("texts is required: a list of texts.")
				}
				body := map[string]any{"texts": texts}
				if kind := arg(args, "type"); kind != "" {
					body["type"] = kind
				}
				encoded, _ := json.Marshal(body)
				return toolRequest{path: "/v1/embed", contentType: "application/json", body: encoded}, nil
			},
		},
		textTool("convert_tamil_legacy_text", "Convert Tamil text in an old 8-bit encoding (TSCII), which shows as accented Latin letters such as ¾Á¢ú without its font, to Unicode Tamil, or Unicode back to it.", "/v1/tamil-font/convert", "text", "The text as it looks without its font, or Unicode Tamil.",
			map[string]any{"from": oneOf("What the text is in.", append([]string{"unicode"}, tamilEncodingIDs()...)...), "to": oneOf("What to convert to. Default unicode.", append([]string{"unicode"}, tamilEncodingIDs()...)...)},
			map[string]string{"from": "from", "to": "to"}),
		mcpTool{
			Name:        "get_indian_states",
			Description: "List India's 28 states and 8 union territories, or get one, with its two-letter code, GST state code and vehicle registration prefixes.",
			InputSchema: schema([]string{}, map[string]any{"state": text("State code, vehicle prefix or name. Leave out for all.")}),
			path: func(args map[string]any) (string, error) {
				if state := arg(args, "state"); state != "" {
					return "/v1/states/" + url.PathEscape(state), nil
				}
				return "/v1/states", nil
			},
		},
		uploadTool("convert_image", "Resize a picture or convert it (JPEG, PNG, WebP, AVIF, GIF, TIFF out; HEIC and AVIF read too). Turned upright, all metadata such as the GPS position removed.", "/v1/image/convert",
			map[string]any{"format": oneOf("Output format. Default webp.", imageFormatNames...), "width": pixels("Width in pixels, 1 to 8000."), "height": pixels("Height in pixels, 1 to 8000."), "fit": oneOf("Default inside: within width and height, never enlarged.", "inside", "cover", "fill"), "quality": pixels("JPEG, WebP and AVIF quality, 1 to 100. Default 80.")},
			map[string]string{"format": "format", "width": "width", "height": "height", "fit": "fit", "quality": "quality"}),
		uploadTool("get_image_info", "Format, size, colour space, orientation and camera of a picture, and whether it carries EXIF data or a GPS position.", "/v1/image/info", nil, nil),
		uploadTool("remove_image_background", "Cut the subject out of a picture (a product, a person, an animal) and return it as a PNG with a transparent background, with the IS-Net model.", "/v1/image/remove-background",
			map[string]any{"format": oneOf("png (default), webp, or mask: the cut-out mask alone.", "png", "webp", "mask")}, map[string]string{"format": "format"}),
		uploadTool("hash_image", "Perceptual fingerprints of a picture (average, difference and DCT hashes) that stay nearly the same after resizing or recompression, for finding duplicates.", "/v1/image/hash", nil, nil),
		mcpTool{
			Name:        "compare_images",
			Description: "Compare two pictures by how they look: the bits by which their perceptual hashes differ, a score from 0 to 1, and whether they look like the same picture.",
			InputSchema: schema([]string{"files"}, map[string]any{"files": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "The two pictures, base64-encoded, up to 5 MB together."}}),
			send: func(args map[string]any) (toolRequest, error) {
				files, err := filesArgument(args, "files", 2, 2)
				if err != nil {
					return toolRequest{}, err
				}
				body, kind := multipartBody(files, "jpg", "application/octet-stream")
				return toolRequest{path: "/v1/image/compare", contentType: kind, body: body}, nil
			},
		},
		uploadTool("read_pdf_text", "Read the text of a PDF, page by page. needs_ocr is true when the pages are scanned pictures; send those to read_text_in_image.", "/v1/pdf/text",
			map[string]any{"first": pixels("First page. Default 1."), "last": pixels("Last page."), "layout": oneOf("true keeps the page layout with spaces, for tables.", "true", "false")},
			map[string]string{"first": "first", "last": "last", "layout": "layout"}),
		uploadTool("get_pdf_info", "Page count, PDF version, title, author, dates and page sizes of a PDF, and whether it has forms, signatures or attachments.", "/v1/pdf/info", nil, nil),
		uploadTool("render_pdf_page", "Draw one page of a PDF as a picture, for a preview.", "/v1/pdf/render",
			map[string]any{"page": pixels("Page number. Default 1."), "width": pixels("Width in pixels, 50 to 3000. Default 1000."), "format": oneOf("Default png.", "png", "jpeg")},
			map[string]string{"page": "page", "width": "width", "format": "format"}),
		mcpTool{
			Name:        "select_pdf_pages",
			Description: "Make a new PDF from some pages of one, in the order named: to pick, reorder or split pages.",
			InputSchema: schema([]string{"file", "pages"}, map[string]any{"file": map[string]any{"type": "string", "description": "The PDF, base64-encoded. Up to 5 MB."}, "pages": pages}),
			send: func(args map[string]any) (toolRequest, error) {
				if _, err := required(args, "pages"); err != nil {
					return toolRequest{}, err
				}
				data, err := fileArgument(args, "file")
				return toolRequest{path: "/v1/pdf/pages" + query(args, map[string]string{"pages": "pages"}), contentType: "application/octet-stream", body: data}, err
			},
		},
		uploadTool("rotate_pdf_pages", "Turn the pages of a PDF clockwise by a multiple of 90 degrees.", "/v1/pdf/rotate",
			map[string]any{"angle": oneOf("Degrees, clockwise. Default 90.", "90", "180", "270", "-90", "-180", "-270"), "pages": text("Pages to turn, such as 1-3,5. Default all.")},
			map[string]string{"angle": "angle", "pages": "pages"}),
		uploadTool("compress_pdf", "Make a PDF smaller by removing duplicate fonts and images and packing its structure. Scanned PDFs shrink little.", "/v1/pdf/compress", nil, nil),
		mcpTool{
			Name:        "merge_pdfs",
			Description: "Join PDFs into one, in the order given.",
			InputSchema: schema([]string{"files"}, map[string]any{"files": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Two to twenty PDFs, base64-encoded, up to 5 MB together."}}),
			send: func(args map[string]any) (toolRequest, error) {
				files, err := filesArgument(args, "files", 2, pdfMaxFiles)
				if err != nil {
					return toolRequest{}, err
				}
				body, kind := multipartBody(files, "pdf", "application/pdf")
				return toolRequest{path: "/v1/pdf/merge", contentType: kind, body: body}, nil
			},
		},
		uploadTool("remove_audio_noise", "Lower background noise (fans, traffic, hum) in a recording with DeepFilterNet; returns MP3. The first five minutes are cleaned.", "/v1/audio/denoise",
			map[string]any{"limit": pixels("The most noise is lowered by, in decibels, 6 to 100. Lower sounds more natural. Default 100.")},
			map[string]string{"limit": "limit"}),
	)
}
