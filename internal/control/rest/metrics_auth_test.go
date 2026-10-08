package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/auth"
	"github.com/hilather/go-lab-netconf/internal/model"
)

// Catalog metrics.get requires netconf.read. Docs say only health is
// unauthenticated. A scrape without a bearer or session must be rejected.
func TestMetricsRequireReadScope(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/metrics", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated metrics status = %d, want 401; body %s", w.Code, w.Body.String())
	}

	reader, err := New(Config{
		Service: s.svc,
		Auth:    auth.Static(testToken, "reader", model.RoleReader),
	})
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/v1/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	w = httptest.NewRecorder()
	reader.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("reader metrics status = %d, want 200; body %s", w.Code, w.Body.String())
	}
}
