// Package devtools holds the small developer utilities: data format conversion, text
// and JSON differences, cron expressions, ids, hashes, token decoding, time zones,
// HTML and Markdown, colours and image placeholders. None of them touches the network.
package devtools

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

// Formats lists the data formats Convert reads and writes.
var Formats = []string{"json", "yaml", "toml", "csv", "xml"}

// Decode reads a document in the named format into plain Go values: maps with string
// keys, slices, strings, numbers, booleans and nil.
func Decode(format string, data []byte) (any, error) {
	switch format {
	case "json":
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		var v any
		if err := decoder.Decode(&v); err != nil {
			return nil, fmt.Errorf("JSON: %w", err)
		}
		if decoder.More() {
			return nil, errors.New("JSON: more than one value")
		}
		return normalise(v), nil
	case "yaml":
		var v any
		if err := yaml.Unmarshal(data, &v); err != nil {
			return nil, fmt.Errorf("YAML: %w", err)
		}
		return normalise(v), nil
	case "toml":
		var v map[string]any
		if err := toml.Unmarshal(data, &v); err != nil {
			return nil, fmt.Errorf("TOML: %w", err)
		}
		return normalise(v), nil
	case "csv":
		return decodeCSV(data)
	case "xml":
		return decodeXML(data)
	}
	return nil, errors.New("unknown format " + format)
}

// Encode writes plain values in the named format.
func Encode(format string, v any, indent bool) ([]byte, error) {
	switch format {
	case "json":
		var b bytes.Buffer
		e := json.NewEncoder(&b)
		e.SetEscapeHTML(false)
		if indent {
			e.SetIndent("", "  ")
		}
		err := e.Encode(v)
		return b.Bytes(), err
	case "yaml":
		var b bytes.Buffer
		e := yaml.NewEncoder(&b)
		e.SetIndent(2)
		if err := e.Encode(v); err != nil {
			return nil, err
		}
		return b.Bytes(), e.Close()
	case "toml":
		m, ok := v.(map[string]any)
		if !ok {
			return nil, errors.New("TOML needs a table (an object) at the top")
		}
		return toml.Marshal(m)
	case "csv":
		return encodeCSV(v)
	case "xml":
		return encodeXML(v)
	}
	return nil, errors.New("unknown format " + format)
}

// normalise turns the decoders' own types into the plain set.
func normalise(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, value := range t {
			t[k] = normalise(value)
		}
		return t
	case map[any]any:
		out := map[string]any{}
		for k, value := range t {
			out[fmt.Sprint(k)] = normalise(value)
		}
		return out
	case []any:
		for i := range t {
			t[i] = normalise(t[i])
		}
		return t
	case []map[string]any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = normalise(t[i])
		}
		return out
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		f, _ := t.Float64()
		return f
	}
	return v
}

func decodeCSV(data []byte) (any, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	if first, _, _ := bytes.Cut(data, []byte("\n")); bytes.Count(first, []byte(";")) > bytes.Count(first, []byte(",")) {
		r.Comma = ';'
	} else if bytes.Count(first, []byte("\t")) > bytes.Count(first, []byte(",")) {
		r.Comma = '\t'
	}
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV: %w", err)
	}
	if len(records) == 0 {
		return []any{}, nil
	}
	header := records[0]
	out := make([]any, 0, len(records)-1)
	for _, record := range records[1:] {
		row := map[string]any{}
		for i, name := range header {
			if i < len(record) {
				row[name] = record[i]
			} else {
				row[name] = ""
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func scalar(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case map[string]any, []any:
		b, _ := json.Marshal(t)
		return string(b)
	}
	return fmt.Sprint(v)
}

func encodeCSV(v any) ([]byte, error) {
	rows, ok := v.([]any)
	if !ok {
		if m, isMap := v.(map[string]any); isMap {
			rows = []any{m}
		} else {
			return nil, errors.New("CSV needs a list of objects")
		}
	}
	var columns []string
	seen := map[string]bool{}
	for _, row := range rows {
		m, ok := row.(map[string]any)
		if !ok {
			return nil, errors.New("CSV needs a list of objects")
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if !seen[k] {
				seen[k] = true
				columns = append(columns, k)
			}
		}
	}
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.Write(columns)
	for _, row := range rows {
		m := row.(map[string]any)
		record := make([]string, len(columns))
		for i, c := range columns {
			record[i] = scalar(m[c])
		}
		w.Write(record)
	}
	w.Flush()
	return b.Bytes(), w.Error()
}

// XML: elements become keys, repeated elements lists, attributes keys starting with @,
// and text next to children or attributes the key #text.

func decodeXML(data []byte) (any, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	for {
		token, err := d.Token()
		if err == io.EOF {
			return nil, errors.New("XML: no element")
		}
		if err != nil {
			return nil, fmt.Errorf("XML: %w", err)
		}
		if start, ok := token.(xml.StartElement); ok {
			value, err := xmlElement(d, start, 0)
			if err != nil {
				return nil, fmt.Errorf("XML: %w", err)
			}
			return map[string]any{start.Name.Local: value}, nil
		}
	}
}

func xmlElement(d *xml.Decoder, start xml.StartElement, depth int) (any, error) {
	if depth > 100 {
		return nil, errors.New("nested too deeply")
	}
	node := map[string]any{}
	for _, a := range start.Attr {
		node["@"+a.Name.Local] = a.Value
	}
	var text strings.Builder
	for {
		token, err := d.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			child, err := xmlElement(d, t, depth+1)
			if err != nil {
				return nil, err
			}
			name := t.Name.Local
			if existing, ok := node[name]; ok {
				if list, isList := existing.([]any); isList {
					node[name] = append(list, child)
				} else {
					node[name] = []any{existing, child}
				}
			} else {
				node[name] = child
			}
		case xml.CharData:
			text.Write(t)
		case xml.EndElement:
			content := strings.TrimSpace(text.String())
			if len(node) == 0 {
				return content, nil
			}
			if content != "" {
				node["#text"] = content
			}
			return node, nil
		}
	}
}

func encodeXML(v any) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(xml.Header)
	m, ok := v.(map[string]any)
	if !ok || len(m) != 1 {
		m = map[string]any{"root": v}
	}
	for name, value := range m {
		writeXML(&b, name, value, 0)
	}
	return b.Bytes(), nil
}

func xmlName(name string) string {
	var out strings.Builder
	for i, r := range name {
		ok := r == '_' || r == '-' || r == '.' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r > 127
		if !ok || (i == 0 && (r >= '0' && r <= '9' || r == '-' || r == '.')) {
			out.WriteByte('_')
			if ok {
				out.WriteRune(r)
			}
			continue
		}
		out.WriteRune(r)
	}
	if out.Len() == 0 {
		return "item"
	}
	return out.String()
}

func writeXML(b *bytes.Buffer, name string, v any, depth int) {
	name = xmlName(name)
	pad := strings.Repeat("  ", depth)
	switch t := v.(type) {
	case []any:
		for _, item := range t {
			writeXML(b, name, item, depth)
		}
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString(pad + "<" + name)
		for _, k := range keys {
			if strings.HasPrefix(k, "@") {
				b.WriteString(" " + xmlName(k[1:]) + `="`)
				xml.EscapeText(b, []byte(scalar(t[k])))
				b.WriteString(`"`)
			}
		}
		b.WriteString(">")
		if text, ok := t["#text"]; ok {
			xml.EscapeText(b, []byte(scalar(text)))
		}
		children := false
		for _, k := range keys {
			if strings.HasPrefix(k, "@") || k == "#text" {
				continue
			}
			if !children {
				b.WriteString("\n")
				children = true
			}
			writeXML(b, k, t[k], depth+1)
		}
		if children {
			b.WriteString(pad)
		}
		b.WriteString("</" + name + ">\n")
	default:
		b.WriteString(pad + "<" + name + ">")
		xml.EscapeText(b, []byte(scalar(t)))
		b.WriteString("</" + name + ">\n")
	}
}
