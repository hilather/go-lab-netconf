package rest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/web"
)

func TestSPAFallbackWhenEnabled(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.UI = web.NewHandler(nil)
	s.cfg.UIEnabled = func() bool { return true }

	got := doReq(t, s, http.MethodGet, "/", "")
	if got.Code != http.StatusOK {
		t.Fatalf("GET / code=%d body=%s", got.Code, got.Body.String())
	}
	if !strings.Contains(got.Body.String(), "LabNETCONF") {
		t.Fatalf("GET / body=%s", got.Body.String())
	}
	if ct := got.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("GET / content-type=%q", ct)
	}

	fallback := doReq(t, s, http.MethodGet, "/datastores", "")
	if fallback.Code != http.StatusOK || !strings.Contains(fallback.Body.String(), "LabNETCONF") {
		t.Fatalf("SPA fallback code=%d body=%s", fallback.Code, fallback.Body.String())
	}
}

func TestSPADisabledIs404(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.UI = web.NewHandler(nil)
	s.cfg.UIEnabled = func() bool { return false }

	got := doReq(t, s, http.MethodGet, "/", "")
	if got.Code != http.StatusNotFound {
		t.Fatalf("GET / code=%d body=%s", got.Code, got.Body.String())
	}
	if ct := got.Header().Get("Content-Type"); !strings.Contains(ct, "problem+json") {
		t.Fatalf("content-type=%q", ct)
	}
	body := got.Body.String()
	if !strings.Contains(body, `"code":"not_found"`) {
		t.Fatalf("body=%s", body)
	}
	if strings.Contains(strings.ToLower(body), "<!doctype") {
		t.Fatalf("disabled UI served HTML: %s", body)
	}
}

func TestMCPMountIsNotSPA(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.UI = web.NewHandler(nil)
	s.cfg.UIEnabled = func() bool { return true }
	s.cfg.MCP = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"mcp":true}`))
	})

	got := doReq(t, s, http.MethodPost, "/mcp", `{}`)
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"mcp":true`) {
		t.Fatalf("/mcp code=%d body=%s", got.Code, got.Body.String())
	}
}

func TestSPADoesNotCaptureAPI(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.UI = web.NewHandler(nil)
	s.cfg.UIEnabled = func() bool { return true }

	got := doReq(t, s, http.MethodGet, "/v1/does-not-exist", "")
	if got.Code != http.StatusNotFound {
		t.Fatalf("code=%d", got.Code)
	}
	if ct := got.Header().Get("Content-Type"); !strings.Contains(ct, "problem+json") {
		t.Fatalf("content-type=%q", ct)
	}
	if strings.Contains(got.Body.String(), "<!doctype") {
		t.Fatal("API miss served HTML")
	}

	rc := doReq(t, s, http.MethodGet, "/restconf/data", "")
	if rc.Code != http.StatusNotFound {
		t.Fatalf("/restconf code=%d", rc.Code)
	}
	if ct := rc.Header().Get("Content-Type"); !strings.Contains(ct, "problem+json") {
		t.Fatalf("/restconf content-type=%q", ct)
	}
}

func doReq(t *testing.T, s *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}
