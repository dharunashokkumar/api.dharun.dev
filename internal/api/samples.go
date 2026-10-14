package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"unicode/utf8"
)

// Documentation pages show what each example really returns. The example runs through
// the router in memory, as an MCP tool call does, and the JSON is shortened for reading:
// long lists keep their first items and long strings are cut. Key order is kept, so the
// sample reads like the real response.

const (
	sampleItems   = 3   // items kept from a list
	sampleRunes   = 160 // characters kept from a string
	docsSampleKey = "docs-samples"
)

type sample struct {
	JSON      string
	Shortened bool // some list or string was cut
}

// sample returns what the endpoint's example answers, or nil when there is nothing to
// show: images, endpoints marked Live, and examples that fail on this server.
func (s *Server) sample(host string, e endpoint) *sample {
	if e.Image || e.Live || e.Output != "" {
		return nil
	}
	// Charged to an account of its own, so building the pages never uses up a visitor's
	// allowance.
	ctx := context.WithValue(context.Background(), infoKey{}, &requestInfo{internal: true, client: docsSampleKey})
	inner, err := exampleRequest(ctx, e)
	if err != nil {
		return nil
	}
	inner.Host = host
	recorder := &memoryWriter{header: http.Header{}}
	s.mux.ServeHTTP(recorder, inner)
	if recorder.status != http.StatusOK {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(recorder.body.Bytes()))
	decoder.UseNumber()
	tree, err := readNode(decoder)
	if err != nil {
		return nil
	}
	var out strings.Builder
	shortened := tree.write(&out, "")
	return &sample{JSON: out.String(), Shortened: shortened}
}

// node is a JSON value that remembers the order of its keys.
type node struct {
	keys   []string // object keys, in order
	items  []*node  // object values or array items
	array  bool
	object bool
	scalar any
}

func readNode(decoder *json.Decoder) (*node, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch token {
	case json.Delim('{'):
		n := &node{object: true}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			value, err := readNode(decoder)
			if err != nil {
				return nil, err
			}
			n.keys = append(n.keys, key.(string))
			n.items = append(n.items, value)
		}
		_, err := decoder.Token()
		return n, err
	case json.Delim('['):
		n := &node{array: true}
		for decoder.More() {
			value, err := readNode(decoder)
			if err != nil {
				return nil, err
			}
			n.items = append(n.items, value)
		}
		_, err := decoder.Token()
		return n, err
	}
	return &node{scalar: token}, nil
}

// write prints the value indented by two spaces and reports whether anything was cut.
func (n *node) write(out io.StringWriter, indent string) bool {
	shortened := false
	inner := indent + "  "
	switch {
	case n.object:
		if len(n.keys) == 0 {
			out.WriteString("{}")
			return false
		}
		out.WriteString("{\n")
		for i, key := range n.keys {
			out.WriteString(inner + scalarJSON(key) + ": ")
			shortened = n.items[i].write(out, inner) || shortened
			if i < len(n.keys)-1 {
				out.WriteString(",")
			}
			out.WriteString("\n")
		}
		out.WriteString(indent + "}")
	case n.array:
		if len(n.items) == 0 {
			out.WriteString("[]")
			return false
		}
		items := n.items
		if len(items) > sampleItems {
			items, shortened = items[:sampleItems], true
		}
		out.WriteString("[\n")
		for i, item := range items {
			out.WriteString(inner)
			shortened = item.write(out, inner) || shortened
			if i < len(items)-1 {
				out.WriteString(",")
			}
			out.WriteString("\n")
		}
		out.WriteString(indent + "]")
	default:
		value := n.scalar
		if text, ok := value.(string); ok && utf8.RuneCountInString(text) > sampleRunes {
			value, shortened = string([]rune(text)[:sampleRunes])+"…", true
		}
		out.WriteString(scalarJSON(value))
	}
	return shortened
}

func scalarJSON(value any) string {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.Encode(value)
	return strings.TrimSuffix(buffer.String(), "\n")
}

// exampleRequest builds the request an endpoint's documented example makes: a GET, or for
// an endpoint that takes files, a POST of its sample file (twice for one that takes two).
func exampleRequest(ctx context.Context, e endpoint) (*http.Request, error) {
	if e.Upload == nil {
		return http.NewRequestWithContext(ctx, http.MethodGet, e.Example, nil)
	}
	if e.Upload.JSON {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, e.Example, strings.NewReader(e.ExampleBody))
		if err == nil {
			request.Header.Set("Content-Type", "application/json")
		}
		return request, err
	}
	data, ok := sampleFile(e.Upload.Sample)
	if !ok {
		return nil, errors.New("no sample file " + e.Upload.Sample)
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for i := 0; i < max(1, e.Upload.Files); i++ {
		part, err := form.CreateFormFile("file", e.Upload.Sample)
		if err != nil {
			return nil, err
		}
		part.Write(data)
	}
	form.Close()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, e.Example, &body)
	if err == nil {
		request.Header.Set("Content-Type", form.FormDataContentType())
	}
	return request, err
}
