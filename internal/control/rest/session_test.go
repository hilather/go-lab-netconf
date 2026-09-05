package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/auth"
)

func TestSessionCreateGetDelete(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/session", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create %d %s", w.Code, w.Body.String())
	}
	var created sessionCreateJSON
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.CSRF == "" || created.ExpiresAt == "" {
		t.Fatalf("create body %+v", created)
	}
	var cookie string
	for _, c := range w.Result().Cookies() {
		if c.Name == auth.CookieName {
			cookie = c.Value
		}
	}
	if cookie == "" {
		t.Fatal("missing session cookie")
	}
	if !w.Result().Cookies()[0].HttpOnly {
		t.Fatal("cookie must be HttpOnly")
	}

	get := httptest.NewRequest(http.MethodGet, "/v1/session", nil)
	get.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, get)
	if w.Code != http.StatusOK {
		t.Fatalf("get %d %s", w.Code, w.Body.String())
	}
	var view sessionViewJSON
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.CSRF != created.CSRF {
		t.Fatalf("csrf recover %q want %q", view.CSRF, created.CSRF)
	}
	if view.Role != "administrator" || len(view.Scopes) == 0 {
		t.Fatalf("view %+v", view)
	}

	del := httptest.NewRequest(http.MethodDelete, "/v1/session", nil)
	del.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	del.Header.Set(auth.CSRFHeader, created.CSRF)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, del)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete %d %s", w.Code, w.Body.String())
	}

	after := httptest.NewRequest(http.MethodGet, "/v1/state", nil)
	after.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, after)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("cookie after delete %d %s", w.Code, w.Body.String())
	}
}

func TestSessionCreateRequiresBearer(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/session", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("WWW-Authenticate"), "Bearer") {
		t.Fatal(w.Header().Get("WWW-Authenticate"))
	}
}
