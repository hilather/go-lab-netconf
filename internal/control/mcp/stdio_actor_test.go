package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/app"
	"github.com/hilather/go-lab-netconf/internal/auth"
)

// mcp-stdio pins the token's actor for the life of the process. Reset that
// demotes or replaces that token must stop admin tool calls.
func TestStdioActorLosesAdminAfterResetDemotion(t *testing.T) {
	s, svc, cfgPath, src := bootStdioAdmin(t)
	demoted := injectTokenYAML(src, tokenPathOf(t, cfgPath), "reader")
	if err := os.WriteFile(cfgPath, []byte(demoted), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.Reset(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertResetDenied(t, s)
}

func TestStdioActorClearsWhenSecretStopsAuthenticating(t *testing.T) {
	s, svc, cfgPath, _ := bootStdioAdmin(t)
	rotated := "fedcba9876543210fedcba9876543210"
	if err := os.WriteFile(tokenPathOf(t, cfgPath), []byte(rotated+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := svc.Reset(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertResetDenied(t, s)
}

func TestStdioActorClearsWhenTokenRemoved(t *testing.T) {
	s, svc, cfgPath, src := bootStdioAdmin(t)
	if err := os.WriteFile(cfgPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.Reset(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertResetDenied(t, s)
}

// A failed Reset does not run hooks, so the startup pin stays.
func TestStdioActorKeepsPinWhenResetFails(t *testing.T) {
	s, svc, cfgPath, _ := bootStdioAdmin(t)
	if err := os.Remove(cfgPath); err != nil {
		t.Fatal(err)
	}
	if err := svc.Reset(context.Background()); err == nil {
		t.Fatal("missing bootstrap must fail reset")
	}
	actor := s.actorFrom(context.Background())
	if err := s.authorizeTool(actor, "netconf_state_reset"); err != nil {
		t.Fatalf("failed reset cleared the stdio pin: %v (role=%s scopes=%v)", err, actor.Role, actor.Scopes)
	}
}

func TestNewRejectsFixedActorWithoutStdioSecret(t *testing.T) {
	svc := bootTestApp(t)
	fixed := app.Actor{ID: "admin", Role: "administrator", Scopes: []string{"netconf.admin"}, Transport: "mcp"}
	_, err := New(Config{
		Service:    svc,
		Auth:       auth.Static(testBearerToken, "admin", "administrator"),
		FixedActor: &fixed,
	})
	if err == nil {
		t.Fatal("New accepted FixedActor without StdioSecret")
	}
}

func TestNewRejectsStdioSecretWithoutAuth(t *testing.T) {
	svc := bootTestApp(t)
	_, err := New(Config{Service: svc, StdioSecret: testBearerToken})
	if err == nil {
		t.Fatal("New accepted StdioSecret without Auth")
	}
}

func bootStdioAdmin(t *testing.T) (s *Server, svc *app.App, cfgPath, src string) {
	t.Helper()
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "admin.token")
	secret := testBearerToken
	if err := os.WriteFile(tokenPath, []byte(secret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "testdata", "config", "valid", "defaults.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	src = string(raw)
	body := injectTokenYAML(src, tokenPath, "administrator")
	cfgPath = filepath.Join(dir, "labnetconf.yaml")
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err = app.Boot(context.Background(), app.Options{BootstrapPath: cfgPath})
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := auth.FromSpec(svc.Active().Canonical.Spec.Auth)
	if err != nil {
		t.Fatal(err)
	}
	p, err := verifier.AuthenticateBearer(secret)
	if err != nil {
		t.Fatal(err)
	}
	fixed := app.Actor{ID: p.ID, Class: p.Class, Role: p.Role, Scopes: append([]string(nil), p.Scopes...), Transport: "mcp"}
	s, err = New(Config{
		Service:            svc,
		Auth:               verifier,
		FixedActor:         &fixed,
		StdioSecret:        secret,
		AllowLegacyClients: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s, svc, cfgPath, src
}

func tokenPathOf(t *testing.T, cfgPath string) string {
	t.Helper()
	return filepath.Join(filepath.Dir(cfgPath), "admin.token")
}

func assertResetDenied(t *testing.T, s *Server) {
	t.Helper()
	actor := s.actorFrom(context.Background())
	if err := s.authorizeTool(actor, "netconf_state_reset"); err == nil {
		t.Fatalf("stdio actor still authorized for netconf_state_reset (role=%s scopes=%v)", actor.Role, actor.Scopes)
	}
}

func injectTokenYAML(src, secretFile, role string) string {
	var b strings.Builder
	b.WriteString("  auth:\n    mode: bearer\n    tokens:\n")
	b.WriteString("      - id: admin\n        role: ")
	b.WriteString(role)
	b.WriteString("\n        secretFile: ")
	b.WriteString(secretFile)
	b.WriteByte('\n')
	const needle = "spec:\n"
	i := strings.Index(src, needle)
	if i < 0 {
		return src + "\n" + b.String()
	}
	return src[:i+len(needle)] + b.String() + src[i+len(needle):]
}
