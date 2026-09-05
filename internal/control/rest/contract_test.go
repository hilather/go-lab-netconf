package rest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/capabilities"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
)

func TestHealthUnauthenticated(t *testing.T) {
	s, _ := newTestServer(t)
	for _, path := range []string{"/v1/health/live", "/v1/health/ready"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s %d %s", path, w.Code, w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Fatalf("%s ct %s", path, ct)
		}
		if !strings.Contains(w.Body.String(), `"status":"ok"`) {
			t.Fatalf("%s body %s", path, w.Body.String())
		}
	}
}

func TestRootIs404ProblemJSON(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET / %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "problem+json") {
		t.Fatalf("ct %s", ct)
	}
	if !strings.Contains(w.Body.String(), `"code":"not_found"`) {
		t.Fatalf("body %s", w.Body.String())
	}
}

func TestRestconfNotMounted(t *testing.T) {
	s, _ := newTestServer(t)
	for _, path := range []string{"/restconf", "/restconf/data", "/restconf/data/ietf-system:system"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s %d", path, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "problem+json") {
			t.Fatalf("%s ct %s", path, ct)
		}
	}
}

func TestDatastoreGetSetCommit(t *testing.T) {
	s, _ := newTestServer(t)
	got := doJSON(t, s, http.MethodGet, "/v1/datastores/router-a/running", "")
	if got.StatusCode != http.StatusOK {
		t.Fatalf("get running %d %s", got.StatusCode, readBody(t, got))
	}
	if treeHostname(t, readBody(t, got)) != "lab-rtr-a" {
		t.Fatal("bootstrap hostname")
	}

	set := doJSON(t, s, http.MethodPost, "/v1/datastores/router-a/candidate:set", hostnameOverlay("via-rest"))
	if set.StatusCode != http.StatusOK {
		t.Fatalf("set %d %s", set.StatusCode, readBody(t, set))
	}
	_ = set.Body.Close()

	cand := doJSON(t, s, http.MethodGet, "/v1/datastores/router-a/candidate", "")
	if treeHostname(t, readBody(t, cand)) != "via-rest" {
		t.Fatal("candidate after set")
	}
	run := doJSON(t, s, http.MethodGet, "/v1/datastores/router-a/running", "")
	if treeHostname(t, readBody(t, run)) != "lab-rtr-a" {
		t.Fatal("running must stay until commit")
	}

	commit := doJSON(t, s, http.MethodPost, "/v1/datastores/router-a:commit", "")
	if commit.StatusCode != http.StatusOK {
		t.Fatalf("commit %d %s", commit.StatusCode, readBody(t, commit))
	}
	_ = commit.Body.Close()

	run = doJSON(t, s, http.MethodGet, "/v1/datastores/router-a/running", "")
	if treeHostname(t, readBody(t, run)) != "via-rest" {
		t.Fatal("running after commit")
	}
}

func TestCandidateDirtyOnRunningSet(t *testing.T) {
	s, _ := newTestServer(t)
	set := doJSON(t, s, http.MethodPost, "/v1/datastores/router-a/candidate:set", hostnameOverlay("dirty"))
	if set.StatusCode != http.StatusOK {
		t.Fatalf("set %d %s", set.StatusCode, readBody(t, set))
	}
	_ = set.Body.Close()
	run := doJSON(t, s, http.MethodPost, "/v1/datastores/router-a/running:set", hostnameOverlay("clobber"))
	body := readBody(t, run)
	if run.StatusCode != http.StatusConflict {
		t.Fatalf("running set while dirty %d %s", run.StatusCode, body)
	}
	if !strings.Contains(body, `"code":"candidate_dirty"`) {
		t.Fatalf("body %s", body)
	}
}

func TestStateExportYAMLOmitsSecretBytes(t *testing.T) {
	svc := bootNamedApp(t, "full.yaml")
	s, _ := newServerFor(t, svc)
	resp := doJSON(t, s, http.MethodGet, "/v1/state:export", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("export %d %s", resp.StatusCode, readBody(t, resp))
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "yaml") {
		t.Fatalf("ct %s", ct)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "passwordFile:") {
		t.Fatalf("expected passwordFile path:\n%s", body)
	}
	if !strings.Contains(body, "/run/secrets/netconf-alice") {
		t.Fatalf("expected secret path, not bytes:\n%s", body)
	}
	if !strings.Contains(body, "secretFile:") {
		t.Fatalf("expected token secretFile path:\n%s", body)
	}
	for _, leak := range []string{"supersecret", "BEGIN ", "-----"} {
		if strings.Contains(body, leak) {
			t.Fatalf("secret bytes present %q:\n%s", leak, body)
		}
	}

	users := doJSON(t, s, http.MethodGet, "/v1/users", "")
	m := decodeMap(t, users)
	items, _ := m["items"].([]any)
	if len(items) == 0 {
		t.Fatal("users")
	}
	u, _ := items[0].(map[string]any)
	if _, ok := u["password"]; ok {
		t.Fatalf("users leaked password bytes: %v", u)
	}
	if u["passwordFile"] != "/run/secrets/netconf-alice" {
		t.Fatalf("users %v", u)
	}
}

func TestNotificationsWaitTimeoutAndWipe(t *testing.T) {
	s, _ := newTestServer(t)
	wait := doJSON(t, s, http.MethodPost, "/v1/notifications:wait", `{"profile":"router-a","timeout":"50ms"}`)
	body := readBody(t, wait)
	if wait.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("wait %d %s", wait.StatusCode, body)
	}
	if !strings.Contains(body, `"code":"wait_timeout"`) {
		t.Fatalf("wait body %s", body)
	}

	reset := doJSON(t, s, http.MethodPost, "/v1/state:reset", "")
	if reset.StatusCode != http.StatusOK {
		t.Fatalf("reset %d %s", reset.StatusCode, readBody(t, reset))
	}
	_ = reset.Body.Close()

	wait = doJSON(t, s, http.MethodPost, "/v1/notifications:wait", `{"timeout":"50ms"}`)
	body = readBody(t, wait)
	if wait.StatusCode != http.StatusGatewayTimeout || !strings.Contains(body, `"code":"wait_timeout"`) {
		t.Fatalf("wait after wipe %d %s", wait.StatusCode, body)
	}

	clear := doJSON(t, s, http.MethodPost, "/v1/notifications:clear", "")
	if clear.StatusCode != http.StatusOK {
		t.Fatalf("clear %d %s", clear.StatusCode, readBody(t, clear))
	}
	_ = clear.Body.Close()
}

func TestParityRoutesFromCatalog(t *testing.T) {
	s, _ := newTestServer(t)
	for _, c := range capabilities.All() {
		for _, b := range c.REST {
			path := instantiatePath(b.Path)
			method := b.Method
			var body string
			if method != http.MethodGet && method != http.MethodHead {
				switch c.ID {
				case capabilities.DatastoreSet:
					body = hostnameOverlay("route-check")
				case capabilities.ChangesPlan, capabilities.ChangesApply:
					continue
				case capabilities.StateValidate:
					body = "{}"
				default:
					body = "{}"
				}
			}
			resp := doJSON(t, s, method, path, body)
			if resp.StatusCode == http.StatusNotFound && c.ID != capabilities.SessionKill && c.ID != capabilities.NotificationsGet {
				t.Errorf("%s %s -> 404", method, path)
			}
			_ = resp.Body.Close()
		}
	}
}

func TestProblemJSONCode(t *testing.T) {
	p := capabilities.ProblemFrom(domainerr.CandidateDirty("x"), "urn:labnetconf:request:1")
	if p.Status != 409 || p.Code != domainerr.CodeCandidateDirty {
		t.Fatalf("%+v", p)
	}
	if p.Type != "urn:labnetconf:error:candidate-dirty" {
		t.Fatalf("type %s", p.Type)
	}
}

func TestUnknownV1IsProblemJSON(t *testing.T) {
	s, _ := newTestServer(t)
	resp := doJSON(t, s, http.MethodGet, "/v1/does-not-exist", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("%d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "problem+json") {
		t.Fatalf("ct %s", ct)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, `"code":"not_found"`) {
		t.Fatalf("%s", body)
	}
}

func TestCapabilitiesListsParityTools(t *testing.T) {
	s, _ := newTestServer(t)
	resp := doJSON(t, s, http.MethodGet, "/v1/capabilities", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%d", resp.StatusCode)
	}
	m := decodeMap(t, resp)
	raw, _ := json.Marshal(m["capabilities"])
	if !strings.Contains(string(raw), "netconf_datastore_get") {
		t.Fatalf("%s", raw)
	}
	if !strings.Contains(string(raw), "netconf_notifications_wait") {
		t.Fatalf("%s", raw)
	}
}

func instantiatePath(path string) string {
	repl := map[string]string{
		"{name}":    "router-a",
		"{profile}": "router-a",
		"{store}":   "running",
		"{id}":      "missing",
	}
	out := path
	for k, v := range repl {
		out = strings.ReplaceAll(out, k, v)
	}
	return out
}

func TestUIDisabledRootStaysProblemJSON(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.UI = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<!doctype html>")
	})
	s.cfg.UIEnabled = func() bool { return false }
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("%d", w.Code)
	}
	if strings.Contains(w.Body.String(), "<!doctype") {
		t.Fatal("disabled UI served HTML")
	}
}
