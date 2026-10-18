package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRequestBin(t *testing.T) {
	created := request(t, http.MethodPost, "/v1/hooks", "").data(t)
	id, token := created["id"].(string), created["token"].(string)
	if id == "" || token == "" || created["url"] != testOrigin+"/v1/hook/"+id {
		t.Fatalf("unexpected bin: %v", created)
	}

	// Any method and any path below the bin is recorded; the sender's address is not.
	sent := request(t, http.MethodPost, "/v1/hook/"+id+"/payments?attempt=1", `{"amount":49900}`,
		"Content-Type", "application/json", "X-Signature", "abc", "CF-Connecting-IP", "49.204.0.1", "X-Forwarded-For", "49.204.0.1")
	if sent.status != http.StatusOK || sent.data(t)["received"] != true {
		t.Fatalf("send = %d %s", sent.status, sent.body)
	}
	if put := request(t, http.MethodPut, "/v1/hook/"+id, "\xff\xfe binary"); put.status != http.StatusOK {
		t.Fatalf("PUT = %d %s", put.status, put.body)
	}
	if del := request(t, http.MethodDelete, "/v1/hook/"+id+"/x", ""); del.status != http.StatusOK {
		t.Fatalf("DELETE = %d %s", del.status, del.body)
	}
	// Elsewhere the other methods are still refused.
	if other := request(t, http.MethodDelete, "/v1/pincode/600001", ""); other.status != http.StatusMethodNotAllowed {
		t.Errorf("DELETE on a lookup = %d", other.status)
	}

	if got := get(t, "/v1/hooks/"+id); got.status != http.StatusUnauthorized {
		t.Errorf("read without the token = %d", got.status)
	}
	if got := request(t, http.MethodGet, "/v1/hooks/"+id, "", "Authorization", "Bearer wrong"); got.status != http.StatusUnauthorized {
		t.Errorf("read with a wrong token = %d", got.status)
	}
	contents := request(t, http.MethodGet, "/v1/hooks/"+id, "", "Authorization", "Bearer "+token).data(t)
	requests := contents["requests"].([]any)
	if len(requests) != 3 {
		t.Fatalf("got %d requests: %v", len(requests), contents)
	}
	newest, first := requests[0].(map[string]any), requests[2].(map[string]any)
	if newest["method"] != "DELETE" || newest["path"] != "/x" {
		t.Errorf("newest = %v", newest)
	}
	if first["method"] != "POST" || first["path"] != "/payments" || first["query"] != "attempt=1" || first["body"] != `{"amount":49900}` || first["bodyEncoding"] != "text" {
		t.Errorf("first = %v", first)
	}
	headers := first["headers"].(map[string]any)
	if headers["X-Signature"] == nil || headers["Content-Type"] == nil {
		t.Errorf("headers lost: %v", headers)
	}
	for _, name := range []string{"Cf-Connecting-Ip", "X-Forwarded-For"} {
		if headers[name] != nil {
			t.Errorf("%s was stored", name)
		}
	}
	if binary := requests[1].(map[string]any); binary["bodyEncoding"] != "base64" {
		t.Errorf("binary body = %v", binary)
	}
	since := request(t, http.MethodGet, "/v1/hooks/"+id+"?since=2&token="+token, "").data(t)
	if n := len(since["requests"].([]any)); n != 1 {
		t.Errorf("since=2 gave %d requests", n)
	}

	if got := request(t, http.MethodPost, "/v1/hooks/"+id+"/delete", "", "Authorization", "Bearer "+token); got.status != http.StatusOK {
		t.Fatalf("delete = %d %s", got.status, got.body)
	}
	expectStatus(t, "/v1/hooks/"+id+"?token="+token, http.StatusNotFound)
	if got := request(t, http.MethodPost, "/v1/hook/"+id, "x"); got.status != http.StatusNotFound {
		t.Errorf("send to a deleted bin = %d", got.status)
	}
	expectStatus(t, "/v1/hook/nosuchbin", http.StatusNotFound)
}

func TestRequestBinLimits(t *testing.T) {
	server := newTestServer(Config{Version: "t", StateDir: t.TempDir(), MaxBodyBytes: 1 << 20})
	defer server.Close()
	send := func(method, path, body string, headers ...string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		for i := 0; i+1 < len(headers); i += 2 {
			req.Header.Set(headers[i], headers[i+1])
		}
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, req)
		return recorder
	}
	created := send(http.MethodPost, "/v1/hooks", "")
	var bin struct {
		Data struct{ ID, Token string }
	}
	decodeJSON(created.Body.String(), &bin)
	id := bin.Data.ID

	// A large body is cut at 64 KB and its full size reported.
	if got := send(http.MethodPost, "/v1/hook/"+id, strings.Repeat("a", hookBodyLimit+500)); got.Code != http.StatusOK {
		t.Fatalf("large body = %d", got.Code)
	}
	// The bin keeps its last hookKeep requests.
	for i := range hookKeep + 5 {
		if i == hookPerMinute-1 {
			server.extras.hooks.sweep(time.Now()) // a new minute
		}
		send(http.MethodGet, "/v1/hook/"+id+"/"+strconv.Itoa(i), "")
	}
	read := send(http.MethodGet, "/v1/hooks/"+id+"?limit=100", "", "Authorization", "Bearer "+bin.Data.Token)
	var contents struct {
		Data struct {
			Requests []hookRequest
		}
	}
	decodeJSON(read.Body.String(), &contents)
	if n := len(contents.Data.Requests); n != hookKeep {
		t.Errorf("kept %d requests, want %d", n, hookKeep)
	}
	if newest := contents.Data.Requests[0]; newest.Path != "/"+strconv.Itoa(hookKeep+4) {
		t.Errorf("newest = %v", newest.Path)
	}

	// More than hookPerMinute in a minute is refused.
	server.extras.hooks.sweep(time.Now())
	refused := 0
	for range hookPerMinute + 3 {
		if send(http.MethodPost, "/v1/hook/"+id, "x").Code == http.StatusTooManyRequests {
			refused++
		}
	}
	if refused != 3 {
		t.Errorf("refused %d, want 3", refused)
	}

	// The stored total follows what is kept, and drops when the bin is deleted.
	if stored := server.extras.hooks.bytes.Load(); stored <= 0 || stored > hookKeep*hookBodyLimit {
		t.Errorf("stored bytes = %d", stored)
	}
	send(http.MethodPost, "/v1/hooks/"+id+"/delete", "", "Authorization", "Bearer "+bin.Data.Token)
	if stored := server.extras.hooks.bytes.Load(); stored != 0 {
		t.Errorf("stored bytes after delete = %d", stored)
	}

	// Bins survive a restart, and expire.
	again := send(http.MethodPost, "/v1/hooks", "")
	decodeJSON(again.Body.String(), &bin)
	server.extras.hooks.sweep(time.Now().Add(hookLifetime + time.Minute))
	if got := send(http.MethodPost, "/v1/hook/"+bin.Data.ID, "x"); got.Code != http.StatusNotFound {
		t.Errorf("expired bin = %d", got.Code)
	}
}

func TestRequestBinStaysOutOfTheLog(t *testing.T) {
	server := newTestServer(Config{Version: "t", LogRequests: true})
	defer server.Close()
	var logged bytes.Buffer
	server.logOut = &logged
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/hooks", nil))
	var bin struct{ Data struct{ ID, Token string } }
	decodeJSON(recorder.Body.String(), &bin)
	for _, path := range []string{"/v1/hook/" + bin.Data.ID + "/SECRETPATH?SECRETQUERY=1", "/v1/hooks/" + bin.Data.ID + "?token=" + bin.Data.Token} {
		server.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPatch, path, strings.NewReader("SECRETBODY")))
		server.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}
	for _, secret := range []string{"SECRETPATH", "SECRETQUERY", "SECRETBODY", bin.Data.Token, bin.Data.ID} {
		if strings.Contains(logged.String(), secret) {
			t.Errorf("%s reached the log:\n%s", secret, logged.String())
		}
	}
}
