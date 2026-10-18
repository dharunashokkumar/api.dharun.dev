package api

import "strings"

// The catalog of the endpoints that take files: OCR, images, PDF, documents, speech.

// upload says what an endpoint takes as its POST body.
type upload struct {
	// Accepts says what may be sent, for people.
	Accepts string
	// Types are the media types a raw body may have, for OpenAPI.
	Types []string
	// Sample is a file under /samples/ that the documented example posts; empty when the
	// body is JSON (the endpoint's ExampleBody).
	Sample string
	// Files above one: the endpoint takes several files as form fields.
	Files int
	// JSON: the body is a JSON document, not a file.
	JSON bool
}

var catalog = withRound5(withNetworkTools(withAITools(withFileTools(baseCatalog))))

const formNote = " Options go in the query string, or as form fields when the file is sent as multipart/form-data."

var (
	pictureTypes = []string{"image/jpeg", "image/png", "image/webp", "image/heic", "image/avif", "image/gif", "image/tiff"}
	pictureNote  = "A picture: JPEG, PNG, WebP, HEIC, AVIF, GIF, BMP or TIFF, up to 10 MB."
	pdfNote      = "A PDF, up to 10 MB."
	pagesParam   = param{Name: "pages", In: "query", Type: "string", Doc: "Pages: numbers and ranges such as 1-3,5,8- (8 to the end), odd, even or l (the last page)."}
)

func withFileTools(base []apiGroup) []apiGroup {
	out := make([]apiGroup, 0, len(base)+6)
	for _, group := range base {
		if group.ID == "codes" {
			group.Endpoints = append(append([]endpoint{}, group.Endpoints...), endpoint{
				Path: "/v1/barcode/read", Summary: "Read QR codes and barcodes from a picture", Private: true,
				Doc:     "Finds every QR code in a photo or screenshot, and Data Matrix, Aztec, EAN, UPC, Code 128, Code 39, Code 93, ITF and Codabar codes. Not PDF417. Returns each code's kind, text and position." + formNote,
				Example: "/v1/barcode/read",
				Upload:  &upload{Accepts: pictureNote, Types: pictureTypes, Sample: "codes.png"},
			})
		}
		out = append(out, group)
	}
	return append(out, fileGroups...)
}

var fileGroups = []apiGroup{
	{
		ID:       "ocr",
		Category: "files",
		Title:    "OCR",
		Tagline:  "Text from photos, scans and scanned PDFs, in English and ten Indian languages.",
		Intro:    "Send a picture or a PDF and get its text back, with Tesseract 5. Name the languages the text uses: English, Hindi, Tamil, Telugu, Kannada, Malayalam, Bengali, Gujarati, Punjabi, Odia and Marathi, up to three at once. Files are read and deleted; nothing is kept or logged.",
		Endpoints: []endpoint{
			{
				Path: "/v1/ocr", Summary: "Read the text in a picture or PDF", Private: true,
				Doc:     "Returns the text, Tesseract's confidence and, with words=true, every word with its box. A PDF is read up to 10 pages at a time at 300 dpi; a PDF that already holds text is read faster by /v1/pdf/text." + formNote,
				Example: "/v1/ocr?lang=eng+tam",
				Upload:  &upload{Accepts: pictureNote + " Or a PDF.", Types: append(append([]string{}, pictureTypes...), "application/pdf"), Sample: "receipt.png"},
				Params: []param{
					{Name: "lang", In: "query", Type: "string", Doc: "Languages, joined by + or a comma: " + strings.Join(ocrLanguageCodes, ", ") + " (or en, ta, hi...). Default eng."},
					{Name: "words", In: "query", Type: "string", Enum: []string{"true", "false"}, Doc: "Also return each word with its confidence and box."},
					{Name: "first", In: "query", Type: "integer", Doc: "PDF: first page to read. Default 1."},
					{Name: "last", In: "query", Type: "integer", Doc: "PDF: last page to read; at most 10 pages per request."},
				},
			},
		},
	},
	{
		ID:       "image-tools",
		Category: "files",
		Title:    "Image tools",
		// The background removal model; the other image tools use no data.
		MoreSources: []string{"matting"},
		Tagline:     "Resize and convert pictures, remove backgrounds, and compare pictures by how they look.",
		Intro:       "Upload a picture and get it back resized or in another format (WebP, AVIF, JPEG, PNG), with its background removed, or as a fingerprint for finding duplicates. HEIC photos from iPhones are read too. Metadata such as the GPS position is always removed. Pictures are processed and deleted; nothing is kept or logged.",
		Endpoints: []endpoint{
			{
				Path: "/v1/image/convert", Summary: "Resize and convert a picture", Private: true, Image: true,
				Doc:     "Returns the picture itself, turned upright from its EXIF orientation, without metadata. fit=inside keeps the whole picture within width and height and never enlarges it; cover fills the box and crops the middle; fill stretches." + formNote,
				Example: "/v1/image/convert?format=webp&width=400",
				Upload:  &upload{Accepts: pictureNote, Types: pictureTypes, Sample: "photo.jpg"},
				Params: []param{
					{Name: "format", In: "query", Type: "string", Enum: imageFormatNames, Doc: "Output format. Default webp."},
					{Name: "width", In: "query", Type: "integer", Doc: "Width in pixels, 1 to 8000."},
					{Name: "height", In: "query", Type: "integer", Doc: "Height in pixels, 1 to 8000."},
					{Name: "fit", In: "query", Type: "string", Enum: []string{"inside", "cover", "fill"}, Doc: "How the picture fits width and height. Default inside."},
					{Name: "quality", In: "query", Type: "integer", Doc: "JPEG, WebP and AVIF quality, 1 to 100. Default 80."},
				},
			},
			{
				Path: "/v1/image/info", Summary: "Size, format and metadata of a picture", Private: true,
				Doc:     "Format, size, colour space, pages, orientation, the camera, and whether the picture carries EXIF data or a GPS position.",
				Example: "/v1/image/info",
				Upload:  &upload{Accepts: pictureNote, Types: pictureTypes, Sample: "photo.jpg"},
			},
			{
				Path: "/v1/image/remove-background", Summary: "Remove the background", Private: true, Image: true,
				Doc:     "Returns the picture as a PNG with the background transparent, cut out by the IS-Net model. Works best on one clear subject: a product, a person, an animal. Takes a few seconds; pictures larger than 4096 pixels are made smaller." + formNote,
				Example: "/v1/image/remove-background",
				Upload:  &upload{Accepts: pictureNote, Types: pictureTypes, Sample: "photo.jpg"},
				Params: []param{
					{Name: "format", In: "query", Type: "string", Enum: []string{"png", "webp", "mask"}, Doc: "png (default), webp, or mask: the cut-out mask alone, white where the subject is."},
				},
			},
			{
				Path: "/v1/image/hash", Summary: "Perceptual hash of a picture", Private: true,
				Doc:     "Three 64-bit fingerprints (average, difference and DCT-based perceptual hashes) that stay nearly the same when a picture is resized, recompressed or slightly edited. Pictures whose perceptual hashes differ by 10 bits or fewer look alike.",
				Example: "/v1/image/hash",
				Upload:  &upload{Accepts: pictureNote, Types: pictureTypes, Sample: "photo.jpg"},
			},
			{
				Path: "/v1/image/compare", Summary: "Compare two pictures", Private: true,
				Doc:     "Send two pictures as form fields. Returns the bits by which each hash differs, a score from 0 to 1, and whether they look like the same picture.",
				Example: "/v1/image/compare",
				Upload:  &upload{Accepts: "Two pictures, as multipart/form-data, up to 10 MB each.", Types: pictureTypes, Sample: "photo.jpg", Files: 2},
			},
		},
	},
	{
		ID:       "pdf",
		Category: "files",
		Title:    "PDF tools",
		Tagline:  "Merge, split, rotate and compress PDFs, read their text and draw their pages.",
		Intro:    "Upload a PDF and get a new one back: merged with others, cut down to some pages, rotated, or made smaller. Or read its details and text, or draw a page as a picture. Password-protected PDFs are refused. Files are processed and deleted; nothing is kept or logged.",
		Endpoints: []endpoint{
			{
				Path: "/v1/pdf/info", Summary: "Pages, size and details of a PDF", Private: true,
				Doc:     "Page count, PDF version, title, author, dates, page sizes (named when A4, Letter and so on), and whether it has forms, signatures or attachments.",
				Example: "/v1/pdf/info",
				Upload:  &upload{Accepts: pdfNote, Types: []string{"application/pdf"}, Sample: "document.pdf"},
			},
			{
				Path: "/v1/pdf/text", Summary: "Text of a PDF, page by page", Private: true,
				Doc:     "The text of each page, in reading order, or laid out as on the page with layout=true. needs_ocr is true when the pages hold almost no text, as in a scan: send those to /v1/ocr." + formNote,
				Example: "/v1/pdf/text",
				Upload:  &upload{Accepts: pdfNote, Types: []string{"application/pdf"}, Sample: "document.pdf"},
				Params: []param{
					{Name: "first", In: "query", Type: "integer", Doc: "First page. Default 1."},
					{Name: "last", In: "query", Type: "integer", Doc: "Last page; at most 500 pages per request."},
					{Name: "layout", In: "query", Type: "string", Enum: []string{"true", "false"}, Doc: "Keep the page's layout with spaces, for tables."},
				},
			},
			{
				Path: "/v1/pdf/merge", Summary: "Merge PDFs into one", Private: true, Output: "pdf",
				Doc:     "Send two to twenty PDFs as form fields, in order; up to 25 MB together. Returns one PDF.",
				Example: "/v1/pdf/merge",
				Upload:  &upload{Accepts: "Two to twenty PDFs as multipart/form-data, up to 25 MB together.", Types: []string{"application/pdf"}, Sample: "document.pdf", Files: 2},
			},
			{
				Path: "/v1/pdf/pages", Summary: "Pick, reorder or split pages", Private: true, Output: "pdf",
				Doc:     "Returns a PDF of the pages named, in the order named; a page may be named twice. To split a PDF, ask for each part in turn." + formNote,
				Example: "/v1/pdf/pages?pages=2,1",
				Upload:  &upload{Accepts: pdfNote, Types: []string{"application/pdf"}, Sample: "document.pdf"},
				Params:  []param{{Name: "pages", In: "query", Type: "string", Required: true, Doc: pagesParam.Doc}},
			},
			{
				Path: "/v1/pdf/rotate", Summary: "Rotate pages", Private: true, Output: "pdf",
				Doc:     "Turns pages clockwise by a multiple of 90 degrees." + formNote,
				Example: "/v1/pdf/rotate?angle=90&pages=1",
				Upload:  &upload{Accepts: pdfNote, Types: []string{"application/pdf"}, Sample: "document.pdf"},
				Params: []param{
					{Name: "angle", In: "query", Type: "integer", Enum: []string{"90", "180", "270", "-90", "-180", "-270"}, Doc: "Degrees, clockwise. Default 90."},
					pagesParam,
				},
			},
			{
				Path: "/v1/pdf/compress", Summary: "Make a PDF smaller", Private: true, Output: "pdf",
				Doc:     "Removes duplicate fonts and images and packs the file's structure. Pictures are not re-compressed, so a scanned PDF shrinks little. The original size is in the X-Original-Bytes header; when nothing was gained, the original comes back.",
				Example: "/v1/pdf/compress",
				Upload:  &upload{Accepts: pdfNote, Types: []string{"application/pdf"}, Sample: "document.pdf"},
			},
			{
				Path: "/v1/pdf/render", Summary: "Draw a page as a picture", Private: true, Image: true,
				Doc:     "Returns one page as PNG or JPEG, for previews and thumbnails." + formNote,
				Example: "/v1/pdf/render?page=1&width=600",
				Upload:  &upload{Accepts: pdfNote, Types: []string{"application/pdf"}, Sample: "document.pdf"},
				Params: []param{
					{Name: "page", In: "query", Type: "integer", Doc: "Page number. Default 1."},
					{Name: "width", In: "query", Type: "integer", Doc: "Width in pixels, 50 to 3000. Default 1000."},
					{Name: "format", In: "query", Type: "string", Enum: []string{"png", "jpeg"}, Doc: "Default png."},
				},
			},
		},
	},
	{
		ID:       "invoice",
		Category: "money",
		Title:    "GST invoice",
		Tagline:  "A GST tax invoice as a PDF from JSON, with the tax worked out.",
		Intro:    "Send the seller, the buyer and the items with their HSN codes and GST rates. The API works out each line's taxable value, CGST and SGST (UTGST in union territories without a legislature) for a sale within the state, or IGST across states, from the GSTINs or the place of supply; adds the HSN-wise tax summary, rounding and the amount in words; and sets it on an A4 page with an optional UPI QR code for the exact amount. Nothing you send is kept or logged.",
		Endpoints: []endpoint{
			{
				Path: "/v1/invoice", Summary: "Make a GST tax invoice", Private: true, Output: "pdf",
				Doc: "Fields: number (required, up to 16 letters, digits, - and /), date and due_date (YYYY-MM-DD, date defaults to today), title (default Tax Invoice), place_of_supply (state code, name or GST code; defaults to the buyer's state from the GSTIN or state), reverse_charge, " +
					"seller and buyer (name, address, gstin, state, phone, email; the seller's GSTIN is required), ship_to, items (description, hsn, quantity (default 1), unit (default Nos), rate before tax, discount as an amount, gst_rate in percent), bank (name, account, ifsc, branch, holder), upi, notes and terms. " +
					"Unknown fields are refused, so a misspelt field is caught. format=json returns the worked-out invoice instead of a document; format=png the first page as a picture.",
				Example:     "/v1/invoice",
				Upload:      &upload{Accepts: "The invoice as JSON, up to 256 KB.", Types: []string{"application/json"}, JSON: true},
				ExampleBody: invoiceExample,
				Params: []param{
					{Name: "format", In: "query", Type: "string", Enum: []string{"pdf", "png", "json"}, Doc: "pdf (default), png (the first page) or json (the amounts only)."},
				},
			},
		},
	},
	{
		ID:       "typst",
		Category: "files",
		Title:    "Typst to PDF",
		Tagline:  "Turn a Typst document into a PDF, with Indian scripts set correctly.",
		Intro:    "Typst is a modern markup language for documents, quicker to write than LaTeX. Send a document and get a PDF, or one page as PNG or SVG. Noto fonts for Latin and nine Indian scripts are available; packages, other files and the network are not, so a document must be self-contained. Each document has 20 seconds.",
		Endpoints: []endpoint{
			{
				Path: "/v1/typst", Summary: "Compile Typst to PDF", Private: true, Output: "pdf", Post: true,
				Doc:         "Returns the PDF itself, or one page as an image. A mistake in the document returns 422 with Typst's message, line and column.",
				Body:        "The Typst document.",
				Example:     "/v1/typst?text=" + typstExampleQuery,
				ExampleBody: typstExample,
				Params: []param{
					{Name: "text", In: "query", Type: "string", Doc: "The document (or POST it)."},
					{Name: "format", In: "query", Type: "string", Enum: []string{"pdf", "png", "svg"}, Doc: "Default pdf."},
					{Name: "page", In: "query", Type: "integer", Doc: "png and svg: which page. Default 1."},
					{Name: "ppi", In: "query", Type: "integer", Doc: "png: pixels per inch, 36 to 300. Default 144."},
				},
			},
		},
	},
	{
		ID:        "speech",
		Category:  "language",
		Title:     "Speech",
		Tagline:   "Text to speech in Hindi, Telugu, Malayalam and English, and noise removal for recordings.",
		Intro:     "Turn text into natural speech with Piper voices, as MP3, Ogg Opus or WAV, or clean background noise from a recording with DeepFilterNet. Each voice keeps the licence of the recordings it learnt from: /v1/tts/voices lists them, and two Hindi voices are for non-commercial use only. There is no Tamil, Kannada, Bengali, Gujarati or Marathi voice yet. Nothing you send is kept or logged.",
		SourceKey: "voices",
		Endpoints: []endpoint{
			{
				Path: "/v1/tts", Summary: "Text to speech", Private: true, Output: "audio", Post: true,
				Doc:         "Returns the audio itself; X-Voice and X-Voice-Licence headers name the voice and its licence. Up to 3,000 characters; split longer text. Write the text in its own script: Hindi in Devanagari, Telugu in Telugu, Malayalam in Malayalam.",
				Body:        "The text to speak.",
				Example:     "/v1/tts?text=Namaste%2C%20welcome%20to%20India%20API.&voice=en_GB-cori-medium",
				ExampleBody: "Namaste, welcome to India API.",
				Params: []param{
					{Name: "text", In: "query", Type: "string", Doc: "What to say (or POST it)."},
					{Name: "voice", In: "query", Type: "string", Doc: "A voice ID from /v1/tts/voices, or a language (hi, te, ml, en, en-GB) for its first voice that may be used commercially. Default en."},
					{Name: "format", In: "query", Type: "string", Enum: []string{"mp3", "ogg", "wav"}, Doc: "Default mp3."},
					{Name: "speed", In: "query", Type: "string", Doc: "0.5 to 2. Default 1."},
				},
			},
			{
				Path: "/v1/tts/voices", Summary: "Voices", Reference: true,
				Doc:     "Every voice with its language, speaker, licence and the credit the licence asks for.",
				Example: "/v1/tts/voices",
			},
			{
				Path: "/v1/audio/denoise", Summary: "Remove background noise", Private: true, Output: "audio",
				Doc:     "Send a recording (MP3, WAV, Ogg, M4A, WebM, video files too) and get it back with fans, traffic and hum lowered, as mono 48 kHz. The first five minutes are cleaned." + formNote,
				Example: "/v1/audio/denoise",
				Upload:  &upload{Accepts: "An audio or video file, up to 10 MB.", Types: []string{"audio/mpeg", "audio/wav", "audio/ogg", "audio/mp4", "audio/webm", "video/mp4"}, Sample: "noisy.mp3"},
				Params: []param{
					{Name: "format", In: "query", Type: "string", Enum: []string{"mp3", "ogg", "wav"}, Doc: "Default mp3."},
					{Name: "limit", In: "query", Type: "integer", Doc: "The most noise is lowered by, in decibels, 6 to 100. Lower values sound more natural. Default 100."},
				},
			},
		},
	},
}

const typstExample = "#set page(width: 12cm, height: auto, margin: 1cm)\n= வணக்கம், India API\nTypst sets *Tamil*, हिन्दी and English on one page."

// typstExampleQuery is the same document, as it goes in a URL.
var typstExampleQuery = strings.NewReplacer("\n", "%0A", " ", "%20", "#", "%23", ",", "%2C", "*", "%2A", "(", "%28", ")", "%29", ":", "%3A").Replace(typstExample)

const invoiceExample = `{
  "number": "INV-2026-0042",
  "date": "2026-10-14",
  "seller": {"name": "Sri Murugan Stores", "address": "12 Car Street, Namakkal 637001", "gstin": "33AABCS1429B1Z1"},
  "buyer": {"name": "Kaveri Traders", "address": "4 MG Road, Bengaluru 560001", "gstin": "29AAACK6812C1ZS"},
  "items": [
    {"description": "Basmati rice, 25 kg bag", "hsn": "1006", "quantity": 4, "unit": "Bag", "rate": 2150, "gst_rate": 5},
    {"description": "Steel tiffin box", "hsn": "7323", "quantity": 10, "rate": 349.5, "discount": 120, "gst_rate": 18}
  ],
  "bank": {"name": "State Bank of India", "account": "12345678901", "ifsc": "SBIN0000001", "holder": "Sri Murugan Stores"},
  "upi": "srimurugan@okaxis",
  "terms": "Goods once sold will not be taken back."
}`
