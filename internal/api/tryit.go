package api

// The try-it panel and the playground (growth plan Part 3) are generated from the
// catalog: each endpoint becomes a spec the scripts (assets/try.js, assets/play.js)
// build a form from, filled with the endpoint's own Example.

import (
	"html/template"
	"net/url"
	"strings"
)

type trySpec struct {
	ID          string     `json:"id"`
	Group       string     `json:"group"`
	GroupTitle  string     `json:"groupTitle"`
	Summary     string     `json:"summary"`
	Path        string     `json:"path"`
	Method      string     `json:"method"`
	Post        bool       `json:"post,omitempty"`
	Params      []tryParam `json:"params"`
	Example     string     `json:"example"`
	ExampleBody string     `json:"exampleBody,omitempty"`
	Upload      *tryUpload `json:"upload,omitempty"`
	Kind        string     `json:"kind,omitempty"` // image, PDF, audio
	Key         bool       `json:"key,omitempty"`
	Private     bool       `json:"private,omitempty"`
	Live        bool       `json:"live,omitempty"`
	Cost        int        `json:"cost"`
}

type tryParam struct {
	Name     string   `json:"name"`
	In       string   `json:"in"`
	Type     string   `json:"type"`
	Required bool     `json:"required,omitempty"`
	Enum     []string `json:"enum,omitempty"`
	Doc      string   `json:"doc"`
	Value    string   `json:"value,omitempty"` // from the Example
}

type tryUpload struct {
	Accepts string `json:"accepts"`
	Sample  string `json:"sample,omitempty"`
	Files   int    `json:"files,omitempty"`
	JSON    bool   `json:"json,omitempty"`
}

// exampleValues reads the path and query values out of an endpoint's Example.
func exampleValues(e endpoint) map[string]string {
	values := map[string]string{}
	example, err := url.Parse(e.Example)
	if err != nil {
		return values
	}
	template := strings.Split(strings.Trim(e.Path, "/"), "/")
	actual := strings.Split(strings.Trim(example.EscapedPath(), "/"), "/")
	for i, part := range template {
		if i < len(actual) && strings.HasPrefix(part, "{") {
			name, suffix, _ := strings.Cut(strings.TrimPrefix(part, "{"), "}")
			if value, err := url.PathUnescape(strings.TrimSuffix(actual[i], suffix)); err == nil {
				values[strings.TrimSuffix(name, "...")] = value
			}
		}
	}
	for name, list := range example.Query() {
		if len(list) > 0 {
			values[name] = list[0]
		}
	}
	return values
}

func specOf(group apiGroup, e endpoint, cost int) trySpec {
	values := exampleValues(e)
	spec := trySpec{ID: group.ID + "-" + anchor(e.Summary), Group: group.ID, GroupTitle: group.Title, Summary: e.Summary, Path: e.Path,
		Method: e.Method(), Post: e.Post && e.Upload == nil, Example: e.Example, ExampleBody: e.ExampleBody, Kind: e.Kind(),
		Key: e.Key, Private: e.Private, Live: e.Live, Cost: cost}
	for _, p := range e.Params {
		spec.Params = append(spec.Params, tryParam{Name: p.Name, In: p.In, Type: p.Type, Required: p.Required, Enum: p.Enum, Doc: p.Doc, Value: values[p.Name]})
	}
	if e.Upload != nil {
		spec.Upload = &tryUpload{Accepts: e.Upload.Accepts, Sample: e.Upload.Sample, Files: e.Upload.Files, JSON: e.Upload.JSON}
	}
	return spec
}

// allSpecs lists every endpoint, for the playground.
func (s *Server) allSpecs() []trySpec {
	var specs []trySpec
	for _, group := range catalog {
		for _, e := range group.Endpoints {
			specs = append(specs, specOf(group, e, s.costs[strings.TrimSuffix(e.Path, ".ics")]))
		}
	}
	return specs
}

// tryItPanel is the panel under an endpoint on its API page. Without JavaScript it is
// a closed box saying what it would do; the example address above still works.
func (s *Server) tryItPanel(group apiGroup, e endpoint) template.HTML {
	data := pageData(specOf(group, e, s.costs[e.Path]))
	return template.HTML(`<details class="tryit" data-spec="` + template.HTMLEscapeString(string(data)) + `"><summary>Try it here</summary><div class="inner"><p class="note">Change the values and send the request from your browser. Needs JavaScript.</p></div></details>`)
}
