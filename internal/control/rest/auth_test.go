package rest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/auth"
	"github.com/hilather/go-lab-netconf/internal/capabilities"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
)

func TestBearerRequired(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/state", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "problem+json") {
		t.Fatalf("ct %s", ct)
	}
	if wa := w.Header().Get("WWW-Authenticate"); !strings.Contains(wa, "Bearer") {
		t.Fatalf("www-authenticate %s", wa)
	}
	if strings.Contains(strings.ToLower(waHeader(w)), "basic") {
		t.Fatal("no Basic challenge")
	}
	if !strings.Contains(w.Body.String(), `"code":"unauthorized"`) {
		t.Fatalf("%s", w.Body.String())
	}
}

func waHeader(w *httptest.ResponseRecorder) string {
	return strings.Join(w.Header().Values("WWW-Authenticate"), ",")
}

func TestShortBearerRejected(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/state", nil)
	req.Header.Set("Authorization", "Bearer short")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
}

func TestNilVerifierDenies(t *testing.T) {
	svc := bootTestApp(t)
	s, err := New(Config{Service: svc})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/state", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("nil verifier must deny, got %d", w.Code)
	}
}

func TestNoBasicOnV1(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/state", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("%d", w.Code)
	}
	if !strings.Contains(w.Header().Get("WWW-Authenticate"), "Bearer") {
		t.Fatal(w.Header().Get("WWW-Authenticate"))
	}
}

func TestCSRFRequiredOnCookiePOST(t *testing.T) {
	s, _ := newTestServer(t)
	p := auth.Principal{
		ID:     "admin",
		Class:  auth.ClassToken,
		Role:   model.RoleAdministrator,
		Scopes: auth.DefaultScopes(model.RoleAdministrator),
	}
	cookie, csrf, _, err := s.cfg.Sessions.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/state:reset", strings.NewReader(`{"reason":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("csrf missing want 403 got %d %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/state:reset", strings.NewReader(`{"reason":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.CSRFHeader, "not-the-token")
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("bad csrf %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/state:reset", strings.NewReader(`{"reason":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.CSRFHeader, csrf)
	req.AddCookie(auth.NewSessionCookie(cookie, s.cookieSecure(req), s.cfg.Sessions.MaxAge()))
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("valid csrf %d %s", w.Code, w.Body.String())
	}
}

func TestReaderForbiddenOnAdmin(t *testing.T) {
	svc := bootTestApp(t)
	s, err := New(Config{
		Service: svc,
		Auth:    auth.Static(testToken, "reader", model.RoleReader),
	})
	if err != nil {
		t.Fatal(err)
	}
	got := doJSON(t, s, http.MethodGet, "/v1/state", "")
	if got.StatusCode != http.StatusOK {
		t.Fatalf("reader GET state %d %s", got.StatusCode, readBody(t, got))
	}
	_ = got.Body.Close()

	reset := doJSON(t, s, http.MethodPost, "/v1/state:reset", `{"reason":"x"}`)
	body := readBody(t, reset)
	if reset.StatusCode != http.StatusForbidden {
		t.Fatalf("reader reset %d %s", reset.StatusCode, body)
	}
	if !strings.Contains(body, `"code":"forbidden"`) {
		t.Fatalf("%s", body)
	}

	audit := doJSON(t, s, http.MethodGet, "/v1/audit", "")
	if audit.StatusCode != http.StatusForbidden {
		t.Fatalf("reader audit %d %s", audit.StatusCode, readBody(t, audit))
	}
	_ = audit.Body.Close()
}

func TestOriginExactMatch(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.AllowedOrigins = []string{"https://lab.example"}
	req := httptest.NewRequest(http.MethodGet, "/v1/state", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"code":"origin_not_allowed"`) {
		t.Fatalf("%s", w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/state", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Origin", "https://lab.example")
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("allowed origin %d %s", w.Code, w.Body.String())
	}
}

func TestUsersListRedacted(t *testing.T) {
	svc := bootNamedApp(t, "full.yaml")
	s, _ := newServerFor(t, svc)
	resp := doJSON(t, s, http.MethodGet, "/v1/users", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("users %d %s", resp.StatusCode, readBody(t, resp))
	}
	body := readBody(t, resp)
	if strings.Contains(body, `"password"`) || strings.Contains(body, "BEGIN ") {
		t.Fatalf("users leaked secret bytes:\n%s", body)
	}
	if !strings.Contains(body, `"passwordFile":"/run/secrets/netconf-alice"`) {
		t.Fatalf("expected passwordFile path:\n%s", body)
	}
}

func TestGETStateOmitsSecretBytes(t *testing.T) {
	svc := bootNamedApp(t, "full.yaml")
	s, _ := newServerFor(t, svc)
	resp := doJSON(t, s, http.MethodGet, "/v1/state", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("state %d %s", resp.StatusCode, readBody(t, resp))
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "secretFile") {
		t.Fatalf("expected secretFile path:\n%s", body)
	}
	for _, leak := range []string{"supersecret", "BEGIN ", "-----"} {
		if strings.Contains(body, leak) {
			t.Fatalf("secret bytes present %q:\n%s", leak, body)
		}
	}
}

func TestAuditRingOnMutation(t *testing.T) {
	s, _ := newTestServer(t)
	set := doJSON(t, s, http.MethodPost, "/v1/datastores/router-a/candidate:set", hostnameOverlay("audited"))
	if set.StatusCode != http.StatusOK {
		t.Fatalf("set %d %s", set.StatusCode, readBody(t, set))
	}
	_ = set.Body.Close()

	resp := doJSON(t, s, http.MethodGet, "/v1/audit", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("audit %d %s", resp.StatusCode, readBody(t, resp))
	}
	body := readBody(t, resp)
	if !strings.Contains(body, `"capability":"datastore.set"`) {
		t.Fatalf("%s", body)
	}
}

func TestHealthStaysUnauthenticated(t *testing.T) {
	s, _ := newTestServer(t)
	for _, path := range []string{"/v1/health/live", "/v1/health/ready"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s %d %s", path, w.Code, w.Body.String())
		}
	}
}

func TestUnauthorizedMapping(t *testing.T) {
	p := capabilities.ProblemFrom(domainerr.Unauthorized("authentication required"), "urn:labnetconf:request:1")
	if p.Status != http.StatusUnauthorized || p.Code != domainerr.CodeUnauthorized {
		t.Fatalf("%+v", p)
	}
}
