package api

import (
	"embed"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// The client packages, india-api on npm and on PyPI, are generated from the catalog like
// the skill: one method per documented endpoint, named after its OpenAPI operation, and
// a plain request method for anything else. No dependencies in either. They are committed
// under clients/ (TestGeneratedFilesAreCurrent) and published from CI when they change;
// CI sets the version, so the committed one is a placeholder.

//go:embed clientfiles/*
var clientExtras embed.FS

const (
	clientPackage = "india-api"
	clientVersion = "1.0.0"
)

// clientMethod is one endpoint as the clients see it.
type clientMethod struct {
	op        string // OpenAPI operation ID, snake case
	group     apiGroup
	e         endpoint
	pathArgs  []param
	queryArgs []param
	kind      string // get, text (GET, or POST with a body), file, files, json
}

func clientMethods() []clientMethod {
	var methods []clientMethod
	seen := map[string]bool{}
	for _, group := range catalog {
		for _, e := range group.Endpoints {
			method := clientMethod{op: operationID(e.Path), group: group, e: e, kind: "get"}
			if seen[method.op] {
				continue
			}
			seen[method.op] = true
			byName := map[string]param{}
			for _, p := range e.Params {
				if p.In == "path" {
					byName[p.Name] = p
				} else {
					method.queryArgs = append(method.queryArgs, p)
				}
			}
			// Path arguments in the order they appear in the path.
			for _, match := range pathParamPattern.FindAllStringSubmatch(e.Path, -1) {
				p, ok := byName[match[1]]
				if !ok {
					p = param{Name: match[1], In: "path", Type: "string", Required: true}
				}
				method.pathArgs = append(method.pathArgs, p)
			}
			switch {
			case e.Upload != nil && e.Upload.JSON:
				method.kind = "json"
			case e.Upload != nil && e.Upload.Files > 1:
				method.kind = "files"
			case e.Upload != nil:
				method.kind = "file"
			case e.Post:
				method.kind = "text"
			}
			methods = append(methods, method)
		}
	}
	return methods
}

var pathParamPattern = regexp.MustCompile(`\{([^}.]+)(?:\.\.\.)?\}`)

func camelCase(snake string) string {
	parts := strings.FieldsFunc(snake, func(r rune) bool { return r == '_' || r == '-' })
	for i := 1; i < len(parts); i++ {
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}

func snakeCase(op string) string { return strings.ReplaceAll(op, "-", "_") }

// jsPath is the endpoint's path as a JavaScript template literal.
func (m clientMethod) jsPath() string {
	return "`" + pathParamPattern.ReplaceAllStringFunc(m.e.Path, func(match string) string {
		name := pathParamPattern.FindStringSubmatch(match)[1]
		return "${enc(" + jsIdent(name) + ")}"
	}) + "`"
}

func (m clientMethod) pyPath() string {
	if len(m.pathArgs) == 0 {
		return fmt.Sprintf("%q", m.e.Path)
	}
	return "f\"" + pathParamPattern.ReplaceAllStringFunc(m.e.Path, func(match string) string {
		name := pathParamPattern.FindStringSubmatch(match)[1]
		return "{_enc(" + pyIdent(name) + ")}"
	}) + "\""
}

var jsReserved = map[string]bool{"function": true, "default": true, "new": true, "class": true, "delete": true}

func jsIdent(name string) string {
	name = camelCase(strings.ReplaceAll(name, "-", "_"))
	if jsReserved[name] {
		return name + "_"
	}
	return name
}

var pyReserved = map[string]bool{"from": true, "type": true, "format": true, "id": true, "class": true, "in": true, "lambda": true}

func pyIdent(name string) string {
	name = strings.ReplaceAll(name, "-", "_")
	// camelCase parameters become snake_case in Python.
	var out strings.Builder
	for i, r := range name {
		if unicode.IsUpper(r) && i > 0 {
			out.WriteByte('_')
		}
		out.WriteRune(unicode.ToLower(r))
	}
	if pyReserved[out.String()] {
		return out.String() + "_"
	}
	return out.String()
}

// clientDoc is the first sentence or two of what an endpoint does, for a doc comment.
func clientDoc(m clientMethod) string {
	doc := m.e.Summary + "."
	if m.e.Doc != "" {
		doc += " " + m.e.Doc
	}
	doc = strings.Join(strings.Fields(doc), " ")
	if len(doc) > 600 {
		doc = doc[:strings.LastIndex(doc[:600], " ")] + " …"
	}
	return doc
}

func wrapText(text string, width int, prefix string) string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		if line != "" && len(line)+1+len(word) > width {
			lines = append(lines, prefix+line)
			line = word
			continue
		}
		if line == "" {
			line = word
		} else {
			line += " " + word
		}
	}
	if line != "" {
		lines = append(lines, prefix+line)
	}
	return strings.Join(lines, "\n")
}

func clientFiles() map[string][]byte {
	methods := clientMethods()
	files := map[string][]byte{}
	js := renderJSClient(methods)
	types := renderJSTypes(methods)
	py := renderPyClient(methods)
	hash := contentHash(map[string][]byte{"js": js, "d.ts": types, "py": py}, "", nil)
	stamp := func(body []byte) []byte { return []byte(strings.ReplaceAll(string(body), "@CATALOG@", hash)) }

	readme := func(language string) []byte {
		return stamp(renderMarkdown("client-readme-"+language, map[string]any{
			"Package": clientPackage, "Origin": publicOrigin, "Repo": repoURL, "Count": len(methods), "Catalog": "@CATALOG@",
			"Limit": defaultRateLimit,
		}))
	}
	files["clients/js/index.js"] = stamp(js)
	files["clients/js/index.d.ts"] = stamp(types)
	files["clients/js/README.md"] = readme("js")
	files["clients/js/LICENSE"] = []byte(mit0)
	files["clients/js/bridge.js"], _ = clientExtras.ReadFile("clientfiles/bridge.js")
	files["clients/python/src/india_api/bridge.py"], _ = clientExtras.ReadFile("clientfiles/bridge.py")
	files["clients/js/package.json"] = indentedJSON(map[string]any{
		"name":        clientPackage,
		"version":     clientVersion,
		"description": "Client for the India API (api.dharun.dev): free Indian data, Indian-language AI and document tools. No key, no dependencies.",
		"type":        "module",
		"main":        "./index.js",
		"types":       "./index.d.ts",
		"exports":     map[string]any{".": map[string]string{"types": "./index.d.ts", "default": "./index.js"}},
		"files":       []string{"index.js", "index.d.ts", "bridge.js", "README.md", "LICENSE"},
		"bin":         map[string]string{clientPackage: "./bridge.js"},
		"engines":     map[string]string{"node": ">=18"},
		"keywords":    pluginKeywords,
		"homepage":    publicOrigin + "/docs",
		"repository":  map[string]string{"type": "git", "url": "git+" + repoURL + ".git", "directory": "clients/js"},
		"bugs":        repoURL + "/issues",
		"author":      authorName + " (" + authorURL + ")",
		"license":     "MIT-0",
		"sideEffects": false,
	})
	files["clients/python/src/india_api/__init__.py"] = stamp(py)
	files["clients/python/src/india_api/py.typed"] = []byte{}
	files["clients/python/README.md"] = readme("python")
	files["clients/python/LICENSE"] = []byte(mit0)
	files["clients/python/pyproject.toml"] = []byte(`[build-system]
requires = ["hatchling>=1.27"]
build-backend = "hatchling.build"

[project]
name = "` + clientPackage + `"
version = "` + clientVersion + `"
description = "Client for the India API (api.dharun.dev): free Indian data, Indian-language AI and document tools. No key, no dependencies."
readme = "README.md"
license = "MIT-0"
license-files = ["LICENSE"]
requires-python = ">=3.9"
authors = [{ name = "` + authorName + `" }]
keywords = [` + quotedList(pluginKeywords) + `]
classifiers = [
    "Programming Language :: Python :: 3",
    "Operating System :: OS Independent",
    "Intended Audience :: Developers",
    "Topic :: Software Development :: Libraries",
]

[project.urls]
Homepage = "` + publicOrigin + `/docs"
Source = "` + repoURL + `"
Issues = "` + repoURL + `/issues"

[tool.hatch.build.targets.wheel]
packages = ["src/india_api"]
`)
	return files
}

func quotedList(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = fmt.Sprintf("%q", value)
	}
	return strings.Join(quoted, ", ")
}

func renderJSClient(methods []clientMethod) []byte {
	var b strings.Builder
	b.WriteString(`// Client for the India API, ` + publicOrigin + `/docs
// Generated from the API's catalog; do not edit. MIT-0: copy it into anything.
// ` + repoURL + `/tree/main/clients/js

export const CATALOG = "@CATALOG@";

const isNode = typeof process !== "undefined" && !!(process.versions && process.versions.node);

/** An answer other than 2xx. code is the API's error code, such as not_found or rate_limited. */
export class IndiaAPIError extends Error {
  constructor(status, code, message) {
    super(message || "HTTP " + status);
    this.name = "IndiaAPIError";
    this.status = status;
    this.code = code;
  }
}

const enc = (value) => encodeURIComponent(String(value));

async function fileBody(file) {
  if (typeof file === "string") {
    if (!isNode) throw new TypeError("A file path works only in Node; pass a Blob, File, Uint8Array or ArrayBuffer.");
    const { readFile } = await import("node:fs/promises");
    const { basename } = await import("node:path");
    return { blob: new Blob([await readFile(file)]), name: basename(file) };
  }
  if (typeof Blob !== "undefined" && file instanceof Blob) return { blob: file, name: file.name || "file" };
  if (file instanceof ArrayBuffer || ArrayBuffer.isView(file)) return { blob: new Blob([file]), name: "file" };
  throw new TypeError("Pass a file as a path (Node), Blob, File, Uint8Array or ArrayBuffer.");
}

export class IndiaAPI {
  /**
   * @param {object} [options]
   * @param {string} [options.baseUrl] Default ` + publicOrigin + `.
   * @param {string} [options.token] A sandbox token (only the sandbox services need one).
   * @param {Record<string,string>} [options.headers] Sent with every request.
   * @param {typeof fetch} [options.fetch] A fetch to use instead of the global one.
   */
  constructor(options = {}) {
    this.baseUrl = (options.baseUrl || "` + publicOrigin + `").replace(/\/+$/, "");
    this.fetch = options.fetch || globalThis.fetch.bind(globalThis);
    this.headers = { ...(options.headers || {}) };
    if (options.token) this.headers.Authorization = "Bearer " + options.token;
  }

  /**
   * Any request. JSON answers resolve to the { data, meta } envelope; files (images,
   * PDFs, audio) to { contentType, bytes, headers }. Other than 2xx throws IndiaAPIError.
   */
  async request(method, path, { query, body, contentType, headers } = {}) {
    const url = new URL(this.baseUrl + path);
    for (const [name, value] of Object.entries(query || {})) {
      if (value !== undefined && value !== null) url.searchParams.set(name, String(value));
    }
    const init = { method, headers: { Accept: "application/json, */*", ...this.headers, ...(headers || {}) } };
    if (isNode) init.headers["User-Agent"] = "india-api-js/" + CATALOG;
    if (body !== undefined && body !== null) {
      if (typeof FormData !== "undefined" && body instanceof FormData) {
        init.body = body;
      } else if (typeof body === "string") {
        init.body = body;
        init.headers["Content-Type"] = contentType || "text/plain; charset=utf-8";
      } else if ((typeof Blob !== "undefined" && body instanceof Blob) || body instanceof ArrayBuffer || ArrayBuffer.isView(body)) {
        init.body = body;
        init.headers["Content-Type"] = contentType || body.type || "application/octet-stream";
      } else {
        init.body = JSON.stringify(body);
        init.headers["Content-Type"] = "application/json";
      }
    }
    const response = await this.fetch(url, init);
    const type = response.headers.get("content-type") || "";
    if (type.startsWith("application/json")) {
      const decoded = await response.json();
      if (!response.ok) {
        const error = (decoded && decoded.error) || {};
        throw new IndiaAPIError(response.status, error.code, error.message);
      }
      return decoded;
    }
    if (!response.ok) throw new IndiaAPIError(response.status, "http_" + response.status, await response.text());
    return { contentType: type, bytes: new Uint8Array(await response.arrayBuffer()), headers: response.headers };
  }

  /** GET any path, such as "/v1/pincode/600001". */
  get(path, query) {
    return this.request("GET", path, { query });
  }

  /** POST to any path: a string as text, an object as JSON, bytes or a Blob as they are. */
  post(path, body, query) {
    return this.request("POST", path, { body, query });
  }

  async _upload(path, file, query) {
    const { blob, name } = await fileBody(file);
    const form = new FormData();
    form.append("file", blob, name);
    return this.request("POST", path, { body: form, query });
  }

  async _uploadMany(path, files, query) {
    const form = new FormData();
    for (const file of files) {
      const { blob, name } = await fileBody(file);
      form.append("file", blob, name);
    }
    return this.request("POST", path, { body: form, query });
  }
`)
	for _, m := range methods {
		args := []string{}
		for _, p := range m.pathArgs {
			args = append(args, jsIdent(p.Name))
		}
		var call string
		switch m.kind {
		case "json":
			args = append(args, "document = {}", "params = {}")
			call = "this.request(\"POST\", " + m.jsPath() + ", { body: document, query: params })"
		case "file":
			args = append(args, "file", "params = {}")
			call = "this._upload(" + m.jsPath() + ", file, params)"
		case "files":
			args = append(args, "files", "params = {}")
			call = "this._uploadMany(" + m.jsPath() + ", files, params)"
		case "text":
			args = append(args, "params = {}", "{ body } = {}")
			call = "body === undefined ? this.request(\"GET\", " + m.jsPath() + ", { query: params }) : this.request(\"POST\", " + m.jsPath() + ", { body, query: params })"
		default:
			args = append(args, "params = {}")
			call = "this.request(\"GET\", " + m.jsPath() + ", { query: params })"
		}
		b.WriteString("\n  /**\n" + wrapText(clientDoc(m), 86, "   * ") + "\n   * " + publicOrigin + "/docs/" + m.group.ID + "\n   */\n")
		b.WriteString("  " + camelCase(m.op) + "(" + strings.Join(args, ", ") + ") {\n    return " + call + ";\n  }\n")
	}
	b.WriteString("}\n\nexport default IndiaAPI;\n")
	return []byte(b.String())
}

func jsType(p param) string {
	switch {
	case len(p.Enum) > 0:
		quoted := make([]string, len(p.Enum))
		for i, value := range p.Enum {
			quoted[i] = fmt.Sprintf("%q", value)
		}
		return strings.Join(quoted, " | ")
	case p.Type == "integer":
		return "number"
	}
	return "string"
}

func renderJSTypes(methods []clientMethod) []byte {
	var b strings.Builder
	b.WriteString(`// Types for the India API client. Generated from the catalog; do not edit. MIT-0.

export declare const CATALOG: string;

export interface Envelope<T = any> {
  data: T;
  meta: {
    total?: number;
    limit?: number;
    offset?: number;
    note?: string;
    source?: { name: string; url: string; licence: string; version?: string; retrieved?: string; attribution?: string };
  };
}

/** A file the API answered with: an image, a PDF or audio. */
export interface FileAnswer {
  contentType: string;
  bytes: Uint8Array;
  headers: Headers;
}

export type Answer<T = any> = Envelope<T> | FileAnswer;
export type FileInput = string | Blob | Uint8Array | ArrayBuffer;
type Query = Record<string, string | number | boolean | undefined | null>;

export declare class IndiaAPIError extends Error {
  status: number;
  code?: string;
}

export interface IndiaAPIOptions {
  baseUrl?: string;
  token?: string;
  headers?: Record<string, string>;
  fetch?: typeof fetch;
}

export declare class IndiaAPI {
  constructor(options?: IndiaAPIOptions);
  request(method: string, path: string, options?: { query?: Query; body?: unknown; contentType?: string; headers?: Record<string, string> }): Promise<Answer>;
  get(path: string, query?: Query): Promise<Answer>;
  post(path: string, body?: unknown, query?: Query): Promise<Answer>;
`)
	for _, m := range methods {
		args := []string{}
		for _, p := range m.pathArgs {
			args = append(args, jsIdent(p.Name)+": string")
		}
		fields := []string{}
		for _, p := range m.queryArgs {
			optional := "?"
			if p.Required && m.kind == "get" {
				optional = ""
			}
			fields = append(fields, fmt.Sprintf("%q%s: %s", p.Name, optional, jsType(p)))
		}
		paramsType := "{ " + strings.Join(append(fields, "[name: string]: unknown"), "; ") + " }"
		switch m.kind {
		case "json":
			args = append(args, "document?: object", "params?: "+paramsType)
		case "file":
			args = append(args, "file: FileInput", "params?: "+paramsType)
		case "files":
			args = append(args, "files: FileInput[]", "params?: "+paramsType)
		case "text":
			args = append(args, "params?: "+paramsType, "options?: { body?: string }")
		default:
			args = append(args, "params?: "+paramsType)
		}
		b.WriteString("  /** " + strings.ReplaceAll(m.e.Summary, "*/", "") + ". " + publicOrigin + "/docs/" + m.group.ID + " */\n")
		b.WriteString("  " + camelCase(m.op) + "(" + strings.Join(args, ", ") + "): Promise<Answer>;\n")
	}
	b.WriteString("}\n\nexport default IndiaAPI;\n")
	return []byte(b.String())
}

func renderPyClient(methods []clientMethod) []byte {
	var b strings.Builder
	b.WriteString(`"""Client for the India API, ` + publicOrigin + `/docs

Generated from the API's catalog; do not edit. MIT-0: copy it into anything.
` + repoURL + `/tree/main/clients/python
"""

from __future__ import annotations

import json as _json
import mimetypes as _mimetypes
import os as _os
import uuid as _uuid
from typing import Any, Dict, Iterable, Optional, Union
from urllib import error as _error
from urllib import parse as _parse
from urllib import request as _request

CATALOG = "@CATALOG@"

__all__ = ["IndiaAPI", "IndiaAPIError", "FileAnswer", "CATALOG"]

FileInput = Union[str, "_os.PathLike[str]", bytes, bytearray]


class IndiaAPIError(Exception):
    """An answer other than 2xx. code is the API's error code, such as not_found or rate_limited."""

    def __init__(self, status: int, code: Optional[str], message: str):
        super().__init__(message or f"HTTP {status}")
        self.status = status
        self.code = code


class FileAnswer:
    """A file the API answered with: an image, a PDF or audio."""

    def __init__(self, content_type: str, data: bytes, headers: Dict[str, str]):
        self.content_type = content_type
        self.data = data
        self.headers = headers

    def save(self, path: Union[str, "_os.PathLike[str]"]) -> None:
        with open(path, "wb") as out:
            out.write(self.data)

    def __repr__(self) -> str:
        return f"FileAnswer({self.content_type!r}, {len(self.data)} bytes)"


def _enc(value: Any) -> str:
    return _parse.quote(str(value), safe="")


def _read_file(file: FileInput) -> tuple:
    if isinstance(file, (bytes, bytearray)):
        return bytes(file), "file"
    path = _os.fspath(file)
    with open(path, "rb") as handle:
        return handle.read(), _os.path.basename(path)


class IndiaAPI:
    """The India API. Every method returns the {"data", "meta"} envelope as a dict, or a
    FileAnswer for images, PDFs and audio, and raises IndiaAPIError otherwise."""

    def __init__(self, base_url: str = "` + publicOrigin + `", token: Optional[str] = None,
                 headers: Optional[Dict[str, str]] = None, timeout: float = 120):
        self.base_url = base_url.rstrip("/")
        self.headers = dict(headers or {})
        if token:
            self.headers["Authorization"] = "Bearer " + token
        self.timeout = timeout

    def request(self, method: str, path: str, query: Optional[Dict[str, Any]] = None,
                body: Any = None, content_type: Optional[str] = None) -> Any:
        """Any request. A str body is sent as text, bytes as they are, anything else as JSON."""
        url = self.base_url + path
        pairs = {name: _value(value) for name, value in (query or {}).items() if value is not None}
        if pairs:
            url += "?" + _parse.urlencode(pairs)
        headers = {"Accept": "application/json, */*", "User-Agent": "india-api-python/" + CATALOG}
        headers.update(self.headers)
        data = None
        if body is not None:
            if isinstance(body, str):
                data = body.encode("utf-8")
                headers["Content-Type"] = content_type or "text/plain; charset=utf-8"
            elif isinstance(body, (bytes, bytearray)):
                data = bytes(body)
                headers["Content-Type"] = content_type or "application/octet-stream"
            else:
                data = _json.dumps(body).encode("utf-8")
                headers["Content-Type"] = "application/json"
        req = _request.Request(url, data=data, method=method, headers=headers)
        try:
            with _request.urlopen(req, timeout=self.timeout) as response:
                return _answer(response.status, dict(response.headers), response.read())
        except _error.HTTPError as failed:
            return _answer(failed.code, dict(failed.headers), failed.read())

    def get(self, path: str, **query: Any) -> Any:
        """GET any path, such as "/v1/pincode/600001"."""
        return self.request("GET", path, query)

    def post(self, path: str, body: Any = None, **query: Any) -> Any:
        """POST to any path: a str as text, a dict as JSON, bytes as they are."""
        return self.request("POST", path, query, body)

    def _upload(self, path: str, files: Iterable[FileInput], query: Dict[str, Any]) -> Any:
        boundary = "india-api-" + _uuid.uuid4().hex
        chunks = []
        for file in files:
            data, name = _read_file(file)
            kind = _mimetypes.guess_type(name)[0] or "application/octet-stream"
            chunks.append(
                f"--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; filename=\"{name}\"\r\n"
                f"Content-Type: {kind}\r\n\r\n".encode("utf-8") + data + b"\r\n")
        chunks.append(f"--{boundary}--\r\n".encode("utf-8"))
        return self.request("POST", path, query, b"".join(chunks), "multipart/form-data; boundary=" + boundary)
`)
	for _, m := range methods {
		args := []string{"self"}
		for _, p := range m.pathArgs {
			args = append(args, pyIdent(p.Name)+": str")
		}
		var call string
		switch m.kind {
		case "json":
			args = append(args, "document: Optional[Dict[str, Any]] = None", "**params: Any")
			call = "self.request(\"POST\", " + m.pyPath() + ", params, document if document is not None else {})"
		case "file":
			args = append(args, "file: FileInput", "**params: Any")
			call = "self._upload(" + m.pyPath() + ", [file], params)"
		case "files":
			args = append(args, "files: Iterable[FileInput]", "**params: Any")
			call = "self._upload(" + m.pyPath() + ", files, params)"
		case "text":
			args = append(args, "body: Optional[str] = None", "**params: Any")
			call = "self.request(\"POST\" if body is not None else \"GET\", " + m.pyPath() + ", params, body)"
		default:
			args = append(args, "**params: Any")
			call = "self.request(\"GET\", " + m.pyPath() + ", params)"
		}
		doc := strings.ReplaceAll(clientDoc(m), `\`, `\\`)
		doc = strings.ReplaceAll(doc, `"""`, `'''`)
		b.WriteString("\n    def " + snakeCase(m.op) + "(" + strings.Join(args, ", ") + ") -> Any:\n")
		b.WriteString("        \"\"\"" + strings.TrimSpace(wrapText(doc, 84, "        ")) + "\n\n        " + publicOrigin + "/docs/" + m.group.ID + "\n        \"\"\"\n")
		b.WriteString("        return " + call + "\n")
	}
	b.WriteString(`

def _value(value: Any) -> str:
    if isinstance(value, bool):
        return "true" if value else "false"
    return str(value)


def _answer(status: int, headers: Dict[str, str], raw: bytes) -> Any:
    lowered = {name.lower(): value for name, value in headers.items()}
    kind = lowered.get("content-type", "")
    if kind.startswith("application/json"):
        decoded = _json.loads(raw.decode("utf-8")) if raw else {}
        if status >= 300:
            error = decoded.get("error") or {}
            raise IndiaAPIError(status, error.get("code"), error.get("message", ""))
        return decoded
    if status >= 300:
        raise IndiaAPIError(status, f"http_{status}", raw.decode("utf-8", "replace"))
    return FileAnswer(kind, raw, headers)
`)
	return []byte(b.String())
}
