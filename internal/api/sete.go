package api

// Set E of the idea catalogue (growth plan, 05): AI and agent tools. Search over your
// own documents joins chunking, embeddings and reranking, which already exist, by
// calling them in memory with the caller's own key; agent notes are a small key-value
// store per secret, kept in STATE_DIR so they outlive a restart.

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/dharunashokkumar/api.dharun.dev/internal/textkit"
)

const (
	ragMaxCollections = 100
	ragMaxChunks      = 1000 // per collection
	ragMaxText        = 2 << 20
	kvMaxStores       = 2000
	kvMaxBytes        = 1 << 20 // per store
	kvMaxValue        = 16 << 10
	kvLifetime        = 30 * 24 * time.Hour // since the last write
)

func (s *Server) registerSetE() {
	s.handle("rag", costLight, "POST /v1/rag", s.handleRAGCreate)
	s.handle("rag", costHeavy, "POST /v1/rag/{id}/documents", s.handleRAGAdd)
	s.handle("rag", costHeavy, "POST /v1/rag/{id}/query", s.handleRAGQuery)
	s.handle("rag", costLight, "POST /v1/rag/{id}/delete", s.handleRAGDelete)
	s.handle("kv", costLight, "POST /v1/kv", s.handleKVCreate)
	s.handle("kv", costLight, "GET /v1/kv/{id}", s.handleKVList)
	s.handle("kv", costLight, "GET /v1/kv/{id}/{key}", s.handleKVGet)
	s.handle("kv", costLight, "POST /v1/kv/{id}/{key}", s.handleKVSet)
	s.handle("kv", costLight, "POST /v1/kv/{id}/{key}/delete", s.handleKVDelete)
}

// innerPost runs a POST to one of this API's own addresses in memory, as the caller.
func (s *Server) innerPost(r *http.Request, address string, body any, into any) (int, string, string) {
	outer := info(r)
	ctx := context.WithValue(r.Context(), infoKey{}, &requestInfo{internal: true, client: outer.bucket, cred: outer.cred})
	encoded, _ := json.Marshal(body)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(encoded))
	if err != nil {
		return http.StatusInternalServerError, "internal_error", "Internal error."
	}
	request.Header.Set("Content-Type", "application/json")
	request.Host = r.Host
	recorder := &memoryWriter{header: http.Header{}}
	s.mux.ServeHTTP(recorder, request)
	var envelope struct {
		Data  json.RawMessage `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(recorder.body.Bytes(), &envelope)
	if recorder.status != http.StatusOK {
		return recorder.status, envelope.Error.Code, envelope.Error.Message
	}
	if json.Unmarshal(envelope.Data, into) != nil {
		return http.StatusInternalServerError, "internal_error", "Internal error."
	}
	return http.StatusOK, "", ""
}

// ---- search over your own documents ----

type ragChunk struct {
	Document string
	Index    int
	Heading  string
	Text     string
	Vector   []float32
}

type ragCollection struct {
	ID      string    `json:"id"`
	Secret  string    `json:"secret,omitempty"`
	Created time.Time `json:"created"`
	Expires time.Time `json:"expires"`
	owner   string
	mu      sync.Mutex
	chunks  []ragChunk
}

type ragStore struct {
	mu          sync.Mutex
	collections map[string]*ragCollection
}

func (s *Server) handleRAGCreate(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	st := &s.rag
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.collections == nil {
		st.collections = map[string]*ragCollection{}
	}
	now := time.Now()
	owner, mine := ownerOf(r), 0
	for id, c := range st.collections {
		if now.After(c.Expires) {
			delete(st.collections, id)
		} else if c.owner == owner {
			mine++
		}
	}
	if mine >= ownedPerCaller {
		s.fail(w, r, http.StatusTooManyRequests, "too_many", "You have 5 document collections already. Delete one, or wait a day.")
		return
	}
	if len(st.collections) >= ragMaxCollections {
		w.Header().Set("Retry-After", "3600")
		s.fail(w, r, http.StatusServiceUnavailable, "busy", "The server holds as many document collections as it can. Try again later.")
		return
	}
	c := &ragCollection{ID: randomToken(10), Secret: randomToken(16), Created: now.UTC(), Expires: now.Add(ownedLifetime).UTC(), owner: owner}
	st.collections[c.ID] = c
	s.ok(w, r, map[string]any{"id": c.ID, "secret": c.Secret, "created": c.Created, "expires": c.Expires,
		"documents": s.origin(r) + "/v1/rag/" + c.ID + "/documents", "query": s.origin(r) + "/v1/rag/" + c.ID + "/query"},
		meta{Note: "A private collection for a day, up to 1,000 passages, in memory only. Send the secret as X-Secret with every call; it is shown once."}, "private, no-store")
}

// demoPassages fill the public demo collection: a few facts written for this API.
var demoPassages = []string{
	"Pincodes are six digits. The first digit is the postal region, the first two the postal circle, and the first three the sorting district. Tamil Nadu's pincodes begin with 6.",
	"An IFSC is eleven characters: four letters for the bank, a zero, and six characters for the branch. NEFT, RTGS and IMPS transfers use it to find the branch.",
	"A GSTIN is fifteen characters: the two-digit state code, the holder's PAN, an entity number, the letter Z by default, and a check character.",
	"Diwali falls on the amavasya at the end of the month of Ashvina in the amanta reckoning, and in most years comes in October or November.",
	"The Reserve Bank of India's policy repo rate is the rate at which it lends to banks overnight; the standing deposit facility rate is the floor of the corridor.",
	"Thiruvalluvar's Thirukkural has 1,330 couplets in 133 chapters, in three books: on virtue, on wealth and on love.",
}

func (s *Server) demoCollection() *ragCollection {
	if s.rag.collections == nil {
		s.rag.collections = map[string]*ragCollection{}
	}
	c, ok := s.rag.collections[demoID]
	if !ok {
		c = &ragCollection{ID: demoID, Secret: demoID, Created: time.Now().UTC(), Expires: demoForever}
		for i, text := range demoPassages {
			c.chunks = append(c.chunks, ragChunk{Document: "India API facts", Index: i, Text: text})
		}
		s.rag.collections[demoID] = c
	}
	return c
}

func (s *Server) ragCollection(w http.ResponseWriter, r *http.Request) *ragCollection {
	info(r).private = true
	s.rag.mu.Lock()
	s.demoCollection()
	c, ok := s.rag.collections[r.PathValue("id")]
	s.rag.mu.Unlock()
	if !ok || time.Now().After(c.Expires) || !sameSecret(c.Secret, secretOf(r)) {
		s.notFound(w, r, "No document collection with that id and secret (send the secret as X-Secret).")
		return nil
	}
	return c
}

func (s *Server) handleRAGAdd(w http.ResponseWriter, r *http.Request) {
	c := s.ragCollection(w, r)
	if c == nil {
		return
	}
	if c.ID == demoID {
		s.ok(w, r, map[string]any{"added": 0, "passages": len(c.chunks)}, meta{Note: "The demo collection is read-only. Make your own with POST /v1/rag."}, "private, no-store")
		return
	}
	raw, err := readAllLimited(w, r, ragMaxText)
	if err != nil {
		s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "At most 2 MB of text in one call.")
		return
	}
	var body struct {
		Documents []struct {
			Name string `json:"name"`
			Text string `json:"text"`
		} `json:"documents"`
		Size int `json:"size"`
	}
	if json.Unmarshal(raw, &body) != nil || len(body.Documents) == 0 {
		s.badRequest(w, r, `Send JSON {"documents": [{"name": "...", "text": "..."}]}. For a PDF, take its text with POST /v1/pdf/text first.`)
		return
	}
	size := body.Size
	if size == 0 {
		size = 800
	}
	if size < 200 || size > 4000 {
		s.badRequest(w, r, "size is 200 to 4000 characters a passage.")
		return
	}
	var fresh []ragChunk
	for i, d := range body.Documents {
		if !utf8.ValidString(d.Text) || strings.TrimSpace(d.Text) == "" {
			s.badRequest(w, r, "Each document needs text, as UTF-8.")
			return
		}
		name := d.Name
		if name == "" {
			name = "document " + itoa(i+1)
		}
		for _, chunk := range textkit.ChunkText(d.Text, textkit.ChunkOptions{Size: size, Overlap: size / 8, Markdown: textkit.LooksLikeMarkdown(d.Text)}) {
			fresh = append(fresh, ragChunk{Document: name, Index: chunk.Index, Heading: chunk.Heading, Text: chunk.Text})
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.chunks)+len(fresh) > ragMaxChunks {
		s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "A collection holds at most 1,000 passages; this would make "+itoa(len(c.chunks)+len(fresh))+".")
		return
	}
	for from := 0; from < len(fresh); from += embedMaxTexts {
		part := fresh[from:min(from+embedMaxTexts, len(fresh))]
		texts := make([]string, len(part))
		for i, chunk := range part {
			texts[i] = chunk.Text
			if chunk.Heading != "" {
				texts[i] = chunk.Heading + "\n" + chunk.Text
			}
		}
		var view embedView
		if status, code, message := s.innerPost(r, "/v1/embed", map[string]any{"texts": texts, "type": "passage"}, &view); status != http.StatusOK {
			s.fail(w, r, status, code, "The passages could not be embedded: "+message)
			return
		}
		for i := range part {
			part[i].Vector = normalised(view.Embeddings[i])
		}
	}
	c.chunks = append(c.chunks, fresh...)
	s.ok(w, r, map[string]any{"added": len(fresh), "passages": len(c.chunks)}, meta{Note: "Each document was cut into passages and embedded. Query with POST /v1/rag/{id}/query."}, "private, no-store")
}

func normalised(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	n := float32(math.Sqrt(sum))
	if n == 0 {
		return v
	}
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = x / n
	}
	return out
}

func (s *Server) handleRAGQuery(w http.ResponseWriter, r *http.Request) {
	c := s.ragCollection(w, r)
	if c == nil {
		return
	}
	var body struct {
		Query  string `json:"query"`
		Top    int    `json:"top"`
		Rerank *bool  `json:"rerank"`
	}
	raw, ok := s.readJSONBody(w, r)
	if !ok {
		return
	}
	if json.Unmarshal(raw, &body) != nil || strings.TrimSpace(body.Query) == "" {
		s.badRequest(w, r, `Send JSON {"query": "...", "top": 5, "rerank": true}.`)
		return
	}
	if body.Top == 0 {
		body.Top = 5
	}
	if body.Top < 1 || body.Top > 20 {
		s.badRequest(w, r, "top is 1 to 20.")
		return
	}
	var view embedView
	if status, code, message := s.innerPost(r, "/v1/embed", map[string]any{"text": body.Query, "type": "query"}, &view); status != http.StatusOK {
		s.fail(w, r, status, code, "The query could not be embedded: "+message)
		return
	}
	q := normalised(view.Embeddings[0])
	c.mu.Lock()
	if c.ID == demoID && len(c.chunks) > 0 && c.chunks[0].Vector == nil {
		texts := make([]string, len(c.chunks))
		for i := range c.chunks {
			texts[i] = c.chunks[i].Text
		}
		var passages embedView
		if status, code, message := s.innerPost(r, "/v1/embed", map[string]any{"texts": texts, "type": "passage"}, &passages); status != http.StatusOK {
			c.mu.Unlock()
			s.fail(w, r, status, code, "The demo passages could not be embedded: "+message)
			return
		}
		for i := range c.chunks {
			c.chunks[i].Vector = normalised(passages.Embeddings[i])
		}
	}
	type scored struct {
		chunk ragChunk
		score float64
	}
	all := make([]scored, 0, len(c.chunks))
	for _, chunk := range c.chunks {
		var dot float64
		for i := range q {
			dot += float64(q[i]) * float64(chunk.Vector[i])
		}
		all = append(all, scored{chunk, dot})
	}
	c.mu.Unlock()
	if len(all) == 0 {
		s.badRequest(w, r, "The collection has no documents yet: add some with POST /v1/rag/{id}/documents.")
		return
	}
	sort.Slice(all, func(i, j int) bool { return all[i].score > all[j].score })
	candidates := all[:min(len(all), 30)]
	reranked := body.Rerank == nil || *body.Rerank
	type passage struct {
		Document   string  `json:"document"`
		Passage    int     `json:"passage"`
		Heading    string  `json:"heading,omitempty"`
		Text       string  `json:"text"`
		Similarity float64 `json:"similarity"`
		Score      float64 `json:"rerankScore,omitempty"`
	}
	out := []passage{}
	if reranked && len(candidates) > 1 {
		docs := make([]string, len(candidates))
		for i, cand := range candidates {
			docs[i] = cand.chunk.Text
		}
		var rv rerankView
		if status, _, _ := s.innerPost(r, "/v1/rerank", rerankRequest{Query: body.Query, Documents: docs, Top: min(body.Top, len(docs))}, &rv); status == http.StatusOK {
			for _, res := range rv.Results {
				cand := candidates[res.Index]
				out = append(out, passage{cand.chunk.Document, cand.chunk.Index, cand.chunk.Heading, cand.chunk.Text, roundCents(cand.score*100) / 100, res.Score})
			}
		} else {
			reranked = false
		}
	}
	if len(out) == 0 {
		reranked = false
		for _, cand := range candidates[:min(body.Top, len(candidates))] {
			out = append(out, passage{cand.chunk.Document, cand.chunk.Index, cand.chunk.Heading, cand.chunk.Text, roundCents(cand.score*100) / 100, 0})
		}
	}
	note := "The passages nearest the query by embedding, then reranked by the cross-encoder."
	if !reranked {
		note = "The passages nearest the query by embedding (cosine similarity)."
	}
	s.ok(w, r, map[string]any{"query": body.Query, "reranked": reranked, "passages": out}, meta{Note: note + " Give these to a language model as context, with their document names."}, "private, no-store")
}

func (s *Server) handleRAGDelete(w http.ResponseWriter, r *http.Request) {
	c := s.ragCollection(w, r)
	if c == nil {
		return
	}
	if c.ID == demoID {
		s.ok(w, r, map[string]any{"deleted": false, "id": demoID}, meta{Note: "The demo collection is never deleted."}, "private, no-store")
		return
	}
	s.rag.mu.Lock()
	delete(s.rag.collections, c.ID)
	s.rag.mu.Unlock()
	s.ok(w, r, map[string]any{"deleted": c.ID}, meta{}, "private, no-store")
}

// ---- agent notes ----

type kvStore struct {
	ID      string            `json:"id"`
	Secret  string            `json:"secret"`
	Owner   string            `json:"owner"`
	Created time.Time         `json:"created"`
	Written time.Time         `json:"written"`
	Values  map[string]string `json:"values"`
}

type kvStores struct {
	mu     sync.Mutex
	loaded bool
	path   string
	stores map[string]*kvStore
}

func (k *kvStores) load(dir string) {
	if k.loaded {
		return
	}
	k.loaded, k.stores = true, map[string]*kvStore{}
	if dir == "" {
		return
	}
	k.path = filepath.Join(dir, "agent-notes.json")
	if raw, err := os.ReadFile(k.path); err == nil {
		json.Unmarshal(raw, &k.stores)
	}
}

func (k *kvStores) save() {
	if k.path == "" {
		return
	}
	now := time.Now()
	for id, st := range k.stores {
		if now.Sub(st.Written) > kvLifetime {
			delete(k.stores, id)
		}
	}
	raw, _ := json.Marshal(k.stores)
	tmp := k.path + ".tmp"
	if os.WriteFile(tmp, raw, 0o600) == nil {
		os.Rename(tmp, k.path)
	}
}

func (st *kvStore) size() int {
	n := 0
	for key, value := range st.Values {
		n += len(key) + len(value)
	}
	return n
}

func (s *Server) handleKVCreate(w http.ResponseWriter, r *http.Request) {
	info(r).private = true
	k := &s.kv
	k.mu.Lock()
	defer k.mu.Unlock()
	k.load(s.cfg.StateDir)
	owner, mine := ownerOf(r), 0
	for _, st := range k.stores {
		if st.Owner == owner {
			mine++
		}
	}
	if mine >= ownedPerCaller {
		s.fail(w, r, http.StatusTooManyRequests, "too_many", "You have 5 note stores already; reuse one.")
		return
	}
	if len(k.stores) >= kvMaxStores {
		s.fail(w, r, http.StatusServiceUnavailable, "busy", "The server holds as many note stores as it can.")
		return
	}
	now := time.Now().UTC()
	st := &kvStore{ID: randomToken(10), Secret: randomToken(16), Owner: owner, Created: now, Written: now, Values: map[string]string{}}
	k.stores[st.ID] = st
	k.save()
	s.ok(w, r, map[string]any{"id": st.ID, "secret": st.Secret, "address": s.origin(r) + "/v1/kv/" + st.ID},
		meta{Note: "A private store of notes, up to 1 MB, kept 30 days after its last write. Send the secret as X-Secret; it is shown once."}, "private, no-store")
}

var kvDemo = &kvStore{ID: demoID, Secret: demoID, Created: time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC), Written: demoForever,
	Values: map[string]string{"greeting": "வணக்கம்", "plan": `{"step": 1, "todo": ["find pincode", "check IFSC"]}`}}

func (s *Server) kvStore(w http.ResponseWriter, r *http.Request) *kvStore {
	info(r).private = true
	s.kv.load(s.cfg.StateDir)
	if r.PathValue("id") == demoID && secretOf(r) == demoID {
		return kvDemo
	}
	st, ok := s.kv.stores[r.PathValue("id")]
	if !ok || !sameSecret(st.Secret, secretOf(r)) || time.Since(st.Written) > kvLifetime {
		s.notFound(w, r, "No note store with that id and secret (send the secret as X-Secret).")
		return nil
	}
	return st
}

func validKVKey(key string) bool {
	return key != "" && len(key) <= 200 && utf8.ValidString(key) && !strings.ContainsAny(key, "/\x00")
}

func (s *Server) handleKVList(w http.ResponseWriter, r *http.Request) {
	s.kv.mu.Lock()
	defer s.kv.mu.Unlock()
	st := s.kvStore(w, r)
	if st == nil {
		return
	}
	prefix := r.URL.Query().Get("prefix")
	keys := []string{}
	for key := range st.Values {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	total := len(keys)
	s.ok(w, r, map[string]any{"keys": keys, "bytes": st.size(), "written": st.Written}, meta{Total: &total}, "private, no-store")
}

func (s *Server) handleKVGet(w http.ResponseWriter, r *http.Request) {
	s.kv.mu.Lock()
	defer s.kv.mu.Unlock()
	st := s.kvStore(w, r)
	if st == nil {
		return
	}
	value, ok := st.Values[r.PathValue("key")]
	if !ok {
		s.notFound(w, r, "No note named "+r.PathValue("key")+".")
		return
	}
	s.ok(w, r, map[string]any{"key": r.PathValue("key"), "value": value}, meta{}, "private, no-store")
}

func (s *Server) handleKVSet(w http.ResponseWriter, r *http.Request) {
	raw, err := readAllLimited(w, r, kvMaxValue)
	if err != nil {
		s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "A note is at most 16 KB.")
		return
	}
	s.kv.mu.Lock()
	defer s.kv.mu.Unlock()
	st := s.kvStore(w, r)
	if st == nil {
		return
	}
	key := r.PathValue("key")
	if st == kvDemo {
		s.ok(w, r, map[string]any{"key": key, "saved": false}, meta{Note: "The demo store is read-only. Make your own with POST /v1/kv."}, "private, no-store")
		return
	}
	if !validKVKey(key) || !utf8.Valid(raw) {
		s.badRequest(w, r, "A key is up to 200 characters without /, and the note is UTF-8 text (JSON is fine).")
		return
	}
	old := st.Values[key]
	if st.size()-len(old)+len(key)+len(raw) > kvMaxBytes {
		s.fail(w, r, http.StatusRequestEntityTooLarge, "too_large", "The store would pass 1 MB. Delete some notes first.")
		return
	}
	st.Values[key] = string(raw)
	st.Written = time.Now().UTC()
	s.kv.save()
	s.ok(w, r, map[string]any{"key": key, "bytes": len(raw), "storeBytes": st.size()}, meta{}, "private, no-store")
}

func (s *Server) handleKVDelete(w http.ResponseWriter, r *http.Request) {
	s.kv.mu.Lock()
	defer s.kv.mu.Unlock()
	st := s.kvStore(w, r)
	if st == nil {
		return
	}
	if st == kvDemo {
		s.ok(w, r, map[string]any{"key": r.PathValue("key"), "deleted": false}, meta{Note: "The demo store is read-only."}, "private, no-store")
		return
	}
	_, existed := st.Values[r.PathValue("key")]
	delete(st.Values, r.PathValue("key"))
	st.Written = time.Now().UTC()
	s.kv.save()
	s.ok(w, r, map[string]any{"key": r.PathValue("key"), "deleted": existed}, meta{}, "private, no-store")
}
