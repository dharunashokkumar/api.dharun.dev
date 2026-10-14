package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Server) registerRoutes() {
	// Data endpoints: counted against the per-client rate limit.
	s.handle("logo", costLight, "GET /v1/logos/{symbol}", s.handleLogo)
	s.handle("company", costLight, "GET /v1/companies/{symbol}", s.handleCompany)
	s.handle("company_search", costLight, "GET /v1/companies", s.handleCompanySearch)
	s.handle("pincode", costLight, "GET /v1/pincode/{pincode}", s.handlePincode)
	s.handle("pincode_search", costLight, "GET /v1/pincode", s.handlePincodeSearch)
	s.handle("ifsc", costLight, "GET /v1/ifsc/{code}", s.handleIFSC)
	s.handle("ifsc_search", costLight, "GET /v1/ifsc", s.handleIFSCSearch)
	s.handle("bank", costLight, "GET /v1/banks/{code}", s.handleBank)
	s.handle("bank_search", costLight, "GET /v1/banks", s.handleBankSearch)
	s.handle("rto", costLight, "GET /v1/rto/{code}", s.handleRTO)
	s.handle("rto_search", costLight, "GET /v1/rto", s.handleRTOSearch)
	s.handle("holidays", costLight, "GET /v1/holidays/{year}", s.handleHolidays)
	s.handle("words", costLight, "GET /v1/words/{number}", s.handleWords)
	s.handle("validate", costLight, "GET /v1/validate/{kind}/{value}", s.handleValidate)
	s.handle("states", costLight, "GET /v1/states", s.handleStates)
	s.handle("states", costLight, "GET /v1/states/{code}", s.handleState)
	s.handle("mcp", costLight, "/mcp", s.handleMCP)

	// Documentation and plumbing.
	s.handle("docs", costFree, "GET /v1", s.handleIndex)
	s.handle("docs", costFree, "GET /openapi.json", s.handleOpenAPI)
	s.handle("health", costFree, "GET /healthz", s.handleHealth)
	s.registerSite()

	s.registerMore()
	s.mux.HandleFunc("/", s.handleUnmatched)
}

// cached renders a document once per public origin.
func (s *Server) cached(r *http.Request, name string, build func(origin string) []byte) []byte {
	origin := s.origin(r)
	key := name + "|" + origin
	if value, ok := s.documents.Load(key); ok {
		return value.([]byte)
	}
	document := build(origin)
	// Only a configured origin is cached: a Host header chosen by the client must not
	// be able to fill memory.
	if s.cfg.PublicOrigin != "" {
		s.documents.Store(key, document)
	}
	return document
}

// ---- machine-readable index ----

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	origin := s.origin(r)
	type endpointView struct {
		Path    string `json:"path"`
		Summary string `json:"summary"`
		Example string `json:"example"`
	}
	type groupView struct {
		ID        string         `json:"id"`
		Title     string         `json:"title"`
		Summary   string         `json:"summary"`
		Docs      string         `json:"docs"`
		Source    *sourceRef     `json:"source,omitempty"`
		Endpoints []endpointView `json:"endpoints"`
	}
	groups := make([]groupView, 0, len(catalog))
	for _, group := range catalog {
		view := groupView{ID: group.ID, Title: group.Title, Summary: group.Tagline, Docs: origin + "/docs/" + group.ID, Source: s.source(group.SourceKey)}
		for _, e := range group.Endpoints {
			view.Endpoints = append(view.Endpoints, endpointView{Path: e.Path, Summary: e.Summary, Example: origin + e.Example})
		}
		groups = append(groups, view)
	}
	states := map[string]datasetState{}
	for _, state := range s.store.states() {
		states[state.Name] = state
	}
	data := map[string]any{
		"name":         productName,
		"summary":      siteTagline,
		"version":      s.cfg.Version,
		"datasets":     states,
		"holidayYears": s.holidayYears(),
		"apis":         groups,
		"links": map[string]string{
			"docs": origin + "/docs", "openapi": origin + "/openapi.json", "llms": origin + "/llms.txt", "mcp": origin + "/mcp",
			"ai": origin + "/ai", "source": repoURL,
		},
	}
	s.ok(w, r, data, meta{}, cacheAPI)
}

// ---- OpenAPI ----

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	body := s.cached(r, "openapi", func(origin string) []byte {
		paths := map[string]any{}
		for _, group := range catalog {
			for _, e := range group.Endpoints {
				parameters := []any{}
				for _, p := range e.Params {
					schema := map[string]any{"type": p.Type}
					if len(p.Enum) > 0 {
						schema["enum"] = p.Enum
					}
					parameters = append(parameters, map[string]any{
						"name": p.Name, "in": p.In, "required": p.Required, "description": p.Doc, "schema": schema,
					})
				}
				success := map[string]any{
					"description": "Success",
					"content":     map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/Envelope"}}},
				}
				if e.Image {
					success["content"] = map[string]any{"image/svg+xml": map[string]any{}, "image/png": map[string]any{}}
				}
				if types := outputTypes(e); types != nil {
					success["content"] = types
				}
				errorResponse := map[string]any{"$ref": "#/components/responses/Error"}
				operation := func(method string) map[string]any {
					return map[string]any{
						"operationId": operationID(e.Path) + method,
						"tags":        []string{group.Title},
						"summary":     e.Summary,
						"description": e.Doc,
						"parameters":  parameters,
						"responses":   map[string]any{"200": success, "400": errorResponse, "404": errorResponse, "429": errorResponse},
					}
				}
				if e.Upload != nil {
					post := operation("")
					post["requestBody"] = uploadBody(e)
					post["responses"].(map[string]any)["413"] = errorResponse
					paths[e.Path] = map[string]any{"post": post}
					continue
				}
				item := map[string]any{"get": operation("")}
				if e.Post {
					post := operation("_post")
					post["requestBody"] = map[string]any{
						"description": e.Body,
						"required":    true,
						"content": map[string]any{
							"text/plain": map[string]any{"schema": map[string]any{"type": "string"}},
							"application/json": map[string]any{"schema": map[string]any{
								"type": "object", "required": []string{"text"},
								"properties": map[string]any{"text": map[string]any{"type": "string"}},
							}},
						},
					}
					item["post"] = post
				}
				paths[e.Path] = item
			}
		}
		document := map[string]any{
			"openapi": "3.1.0",
			"info": map[string]any{
				"title":       productName + ": " + siteTagline,
				"version":     s.cfg.Version,
				"description": siteSummary,
				"license":     map[string]string{"name": "AGPL-3.0 (server code); data under each source's licence, named in meta.source", "identifier": "AGPL-3.0-only"},
				"contact":     map[string]string{"name": authorName, "url": repoURL + "/issues", "email": "me@dharun.dev"},
			},
			"externalDocs": map[string]string{"description": "Documentation, guides and AI setup", "url": origin + "/docs"},
			"servers":      []any{map[string]any{"url": origin}},
			"paths":        paths,
			"components": map[string]any{
				"schemas": map[string]any{
					"Envelope": map[string]any{
						"type":     "object",
						"required": []string{"data", "meta"},
						"properties": map[string]any{
							"data": map[string]any{"description": "The result: an object, or an array for list endpoints."},
							"meta": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"total":  map[string]any{"type": "integer"},
									"limit":  map[string]any{"type": "integer"},
									"offset": map[string]any{"type": "integer"},
									"note":   map[string]any{"type": "string"},
									"source": map[string]any{
										"type":        "object",
										"description": "Where the data came from and the credit its licence asks for.",
										"properties": map[string]any{
											"name": map[string]any{"type": "string"}, "url": map[string]any{"type": "string"},
											"licence": map[string]any{"type": "string"}, "version": map[string]any{"type": "string"},
											"retrieved": map[string]any{"type": "string"}, "attribution": map[string]any{"type": "string"},
										},
									},
								},
							},
						},
					},
					"Error": map[string]any{
						"type": "object",
						"properties": map[string]any{"error": map[string]any{
							"type":       "object",
							"properties": map[string]any{"code": map[string]any{"type": "string"}, "message": map[string]any{"type": "string"}},
						}},
					},
				},
				"responses": map[string]any{"Error": map[string]any{
					"description": "Error",
					"content":     map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/Error"}}},
				}},
			},
		}
		encoded, _ := json.MarshalIndent(document, "", "  ")
		return encoded
	})
	writeBody(w, r, http.StatusOK, body, typeJSON, cacheAPI, true)
}

func operationID(path string) string {
	replacer := strings.NewReplacer("/v1/", "", "{", "by_", "}", "", "/", "_")
	return replacer.Replace(path)
}
