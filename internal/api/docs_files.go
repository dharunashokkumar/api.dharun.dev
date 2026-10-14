package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// How the documentation shows the endpoints that take files or return them.

// method is how the endpoint is called in its example.
func (e endpoint) Method() string {
	if e.Upload != nil {
		return "POST"
	}
	return "GET"
}

// Returns says in a few words what comes back when it is not JSON.
func (e endpoint) Returns() string {
	switch {
	case e.Output == "pdf":
		return "returns the PDF itself"
	case e.Output == "audio":
		return "returns the audio itself"
	case e.Image:
		return "returns the image itself"
	}
	return ""
}

// SampleURL is where the example's sample file can be downloaded.
func (e endpoint) SampleURL(origin string) string {
	if e.Upload == nil || e.Upload.Sample == "" {
		return ""
	}
	return origin + "/samples/" + e.Upload.Sample
}

// outputName is the file name the examples save a binary answer under.
func outputName(e endpoint, address string) string {
	format := ""
	if parsed, err := url.Parse(address); err == nil {
		format = parsed.Query().Get("format")
	}
	switch {
	case e.Output == "pdf" && (format == "" || format == "pdf"):
		if strings.Contains(e.Path, "invoice") {
			return "invoice.pdf"
		}
		return "out.pdf"
	case e.Output == "audio":
		if format == "" {
			format = "mp3"
		}
		if strings.Contains(e.Path, "denoise") {
			return "clean." + format
		}
		return "speech." + format
	case format != "":
		if format == "jpeg" {
			format = "jpg"
		}
		return "out." + format
	case strings.Contains(e.Path, "convert"):
		return "out.webp"
	}
	return "out.png"
}

func fileSamples(e endpoint, address string) []codeSample {
	origin, _, _ := strings.Cut(address, "/v1/")
	binary := e.Image || e.Output != ""
	name := outputName(e, address)
	save := ""
	if binary {
		save = " -o " + name
	}
	if e.Upload == nil {
		// Text in, a file out: an ordinary GET works.
		samples := []codeSample{
			{"curl", "curl", fmt.Sprintf("curl%s \"%s\"", save, address)},
			{"python", "Python", fmt.Sprintf("import requests\n\nresponse = requests.get(\"%s\", timeout=60)\nresponse.raise_for_status()\nopen(\"%s\", \"wb\").write(response.content)", address, name)},
		}
		if e.Output == "audio" {
			samples = append(samples, codeSample{"html", "HTML", fmt.Sprintf("<audio controls src=\"%s\"></audio>", strings.ReplaceAll(address, "&", "&amp;"))})
		}
		return samples
	}

	if e.Upload.JSON {
		return []codeSample{
			{"curl", "curl", fmt.Sprintf("curl -H \"Content-Type: application/json\" --data @invoice.json%s \"%s\"", save, address)},
			{"js", "JavaScript", fmt.Sprintf("const response = await fetch(\"%s\", {\n  method: \"POST\",\n  headers: { \"Content-Type\": \"application/json\" },\n  body: JSON.stringify(invoice),\n});\nif (!response.ok) throw new Error((await response.json()).error.message);\nconst pdf = await response.blob();", address)},
			{"python", "Python", fmt.Sprintf("import json, requests\n\ninvoice = json.load(open(\"invoice.json\"))\nresponse = requests.post(\"%s\", json=invoice, timeout=60)\nif not response.ok:\n    raise RuntimeError(response.json()[\"error\"][\"message\"])\nopen(\"%s\", \"wb\").write(response.content)", address, name)},
		}
	}

	file := e.Upload.Sample
	files := []string{file}
	if e.Upload.Files > 1 {
		files = []string{file, file}
		if strings.Contains(file, ".") {
			stem, extension, _ := strings.Cut(file, ".")
			files = []string{stem + "-1." + extension, stem + "-2." + extension}
		}
	}
	var curlFields, pyFiles, jsAppend []string
	for i, f := range files {
		curlFields = append(curlFields, "-F \"file=@"+f+"\"")
		pyFiles = append(pyFiles, fmt.Sprintf("(\"file\", open(\"%s\", \"rb\"))", f))
		jsAppend = append(jsAppend, fmt.Sprintf("form.append(\"file\", input.files[%d]);", i))
	}
	download := fmt.Sprintf("# a sample to try: %s/samples/%s\n", origin, file)
	curl := download + fmt.Sprintf("curl %s%s \"%s\"", strings.Join(curlFields, " "), save, address)
	result := "const body = await response.json();\nif (!response.ok) throw new Error(body.error.message);\nconsole.log(body.data);"
	python := "body = response.json()\nif not response.ok:\n    raise RuntimeError(body[\"error\"][\"message\"])\nprint(body[\"data\"])"
	if binary {
		result = "if (!response.ok) throw new Error((await response.json()).error.message);\nconst file = await response.blob();"
		python = fmt.Sprintf("if not response.ok:\n    raise RuntimeError(response.json()[\"error\"][\"message\"])\nopen(\"%s\", \"wb\").write(response.content)", name)
	}
	return []codeSample{
		{"curl", "curl", curl},
		{"js", "JavaScript", fmt.Sprintf("// input is an <input type=\"file\"> element\nconst form = new FormData();\n%s\nconst response = await fetch(\"%s\", { method: \"POST\", body: form });\n%s", strings.Join(jsAppend, "\n"), address, result)},
		{"python", "Python", fmt.Sprintf("import requests\n\nfiles = [%s]\nresponse = requests.post(\"%s\", files=files, timeout=90)\n%s", strings.Join(pyFiles, ", "), address, python)},
	}
}

// outputTypes lists the media types a file-returning endpoint answers with, for OpenAPI.
func outputTypes(e endpoint) map[string]any {
	binary := map[string]any{"schema": map[string]any{"type": "string", "format": "binary"}}
	var types []string
	switch {
	case e.Output == "pdf":
		types = []string{"application/pdf", "image/png"}
		if e.Path == "/v1/typst" {
			types = append(types, "image/svg+xml")
		}
	case e.Output == "audio":
		types = []string{"audio/mpeg", "audio/ogg", "audio/wav"}
	case e.Image && e.Upload != nil:
		types = []string{"image/png", "image/webp", "image/jpeg", "image/avif", "image/gif", "image/tiff"}
	default:
		return nil
	}
	content := map[string]any{}
	for _, kind := range types {
		content[kind] = binary
	}
	if e.Path == "/v1/invoice" {
		content["application/json"] = map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/Envelope"}}
	}
	return content
}

// uploadBody describes what an upload endpoint takes, for OpenAPI.
func uploadBody(e endpoint) map[string]any {
	content := map[string]any{}
	if e.Upload.JSON {
		content["application/json"] = map[string]any{"schema": map[string]any{"type": "object"}, "example": json.RawMessage(e.ExampleBody)}
		return map[string]any{"description": e.Upload.Accepts, "required": true, "content": content}
	}
	file := map[string]any{"type": "string", "format": "binary"}
	field := any(file)
	if e.Upload.Files > 1 {
		field = map[string]any{"type": "array", "items": file, "minItems": 2}
	}
	content["multipart/form-data"] = map[string]any{"schema": map[string]any{
		"type": "object", "required": []string{"file"},
		"properties": map[string]any{"file": field},
	}}
	if e.Upload.Files <= 1 {
		for _, kind := range e.Upload.Types {
			content[kind] = map[string]any{"schema": file}
		}
	}
	return map[string]any{"description": e.Upload.Accepts, "required": true, "content": content}
}
