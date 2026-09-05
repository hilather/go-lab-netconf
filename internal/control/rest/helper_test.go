package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/app"
	"github.com/hilather/go-lab-netconf/internal/auth"
	"github.com/hilather/go-lab-netconf/internal/model"
)

const testToken = "0123456789abcdef0123456789abcdef"

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func bootTestApp(t *testing.T) *app.App {
	t.Helper()
	svc, _ := bootAuthedApp(t, testToken)
	return svc
}

func bootNamedApp(t *testing.T, name string) *app.App {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "testdata", "config", "valid", name))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "labnetconf.yaml")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := app.Boot(context.Background(), app.Options{BootstrapPath: path})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func bootAuthedApp(t *testing.T, token string) (*app.App, string) {
	t.Helper()
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "admin.token")
	if err := os.WriteFile(tokenPath, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "testdata", "config", "valid", "defaults.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	body := injectAuthYAML(string(src), tokenPath, nil)
	path := filepath.Join(dir, "labnetconf.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := app.Boot(context.Background(), app.Options{BootstrapPath: path})
	if err != nil {
		t.Fatal(err)
	}
	return svc, path
}

func injectAuthYAML(src, secretFile string, allowedOrigins []string) string {
	var b strings.Builder
	b.WriteString("  auth:\n    mode: bearer\n    tokens:\n")
	if secretFile != "" {
		b.WriteString("      - id: admin\n        role: administrator\n        secretFile: ")
		b.WriteString(secretFile)
		b.WriteByte('\n')
	}
	if len(allowedOrigins) > 0 {
		b.WriteString("  management:\n    allowedOrigins:\n")
		for _, o := range allowedOrigins {
			b.WriteString("      - ")
			b.WriteString(o)
			b.WriteByte('\n')
		}
	}
	const needle = "spec:\n"
	i := strings.Index(src, needle)
	if i < 0 {
		return src + "\n" + b.String()
	}
	return src[:i+len(needle)] + b.String() + src[i+len(needle):]
}

func newTestServer(t *testing.T) (*Server, *app.App) {
	t.Helper()
	svc := bootTestApp(t)
	return newServerFor(t, svc)
}

func newServerFor(t *testing.T, svc *app.App) (*Server, *app.App) {
	t.Helper()
	s, err := New(Config{
		Service: svc,
		Auth:    auth.Static(testToken, "admin", model.RoleAdministrator),
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, svc
}

func doJSON(t *testing.T, s *Server, method, path, body string) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w.Result()
}

func decodeMap(t *testing.T, r *http.Response) map[string]any {
	t.Helper()
	defer func() { _ = r.Body.Close() }()
	var m map[string]any
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}
	return m
}

func readBody(t *testing.T, r *http.Response) string {
	t.Helper()
	defer func() { _ = r.Body.Close() }()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func hostnameOverlay(host string) string {
	raw, _ := json.Marshal(map[string]any{
		"ietf-system": map[string]any{
			"system": map[string]any{"hostname": host},
		},
	})
	return string(raw)
}

func treeHostname(t *testing.T, raw string) string {
	t.Helper()
	var tree map[string]any
	if err := json.Unmarshal([]byte(raw), &tree); err != nil {
		t.Fatal(err)
	}
	mod, _ := tree["ietf-system"].(map[string]any)
	sys, _ := mod["system"].(map[string]any)
	s, _ := sys["hostname"].(string)
	return s
}
