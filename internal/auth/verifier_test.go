package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/capabilities"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestStaticBearer(t *testing.T) {
	v := Static(testSecret, "admin", model.RoleAdministrator)
	if err := v.RequireListen(); err != nil {
		t.Fatal(err)
	}
	p, err := v.Authenticate(Request{Authorization: "Bearer " + testSecret})
	if err != nil || p.ID != "admin" {
		t.Fatalf("%+v %v", p, err)
	}
	if !p.HasScope(capabilities.ScopeNetconfAdmin) || !p.HasScope(capabilities.ScopeNetconfRead) {
		t.Fatal("admin scopes")
	}
	_, err = v.Authenticate(Request{Authorization: "Basic dXNlcjpwYXNz"})
	if err == nil {
		t.Fatal("Basic must 401")
	}
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeUnauthorized {
		t.Fatalf("%v", err)
	}
	if !strings.Contains(WWWAuthenticate()[0], "Bearer") {
		t.Fatal(WWWAuthenticate())
	}
	if strings.Contains(strings.ToLower(WWWAuthenticate()[0]), "basic") {
		t.Fatal("no Basic challenge")
	}
}

func TestReaderScopes(t *testing.T) {
	v := Static(testSecret, "reader", model.RoleReader)
	p, err := v.Authenticate(Request{Authorization: "Bearer " + testSecret})
	if err != nil {
		t.Fatal(err)
	}
	if !p.HasScope(capabilities.ScopeNetconfRead) {
		t.Fatal("reader read")
	}
	if p.HasScope(capabilities.ScopeNetconfWrite) || p.HasScope(capabilities.ScopeNetconfAdmin) {
		t.Fatal("reader must not write")
	}
	if err := Authorize(p, []string{capabilities.ScopeNetconfAdmin}); err == nil {
		t.Fatal("admin scope")
	}
}

func TestFromSpecFileRefAndMinBytes(t *testing.T) {
	dir := t.TempDir()
	short := filepath.Join(dir, "short")
	if err := os.WriteFile(short, []byte("tooshort\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := FromSpec(model.AuthSpec{
		Mode: model.MgmtAuthBearer,
		Tokens: []model.TokenSpec{{
			ID: "admin", Role: model.RoleAdministrator, SecretFile: short,
		}},
	})
	if err == nil {
		t.Fatal("short token")
	}
	okPath := filepath.Join(dir, "ok")
	if err := os.WriteFile(okPath, []byte(testSecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	v, err := FromSpec(model.AuthSpec{
		Mode: model.MgmtAuthBearer,
		Tokens: []model.TokenSpec{{
			ID: "admin", Role: model.RoleAdministrator, SecretFile: okPath,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.TokenCount() != 1 {
		t.Fatal(v.TokenCount())
	}
}

func TestZeroTokensRefuseListen(t *testing.T) {
	v, err := FromSpec(model.AuthSpec{Mode: model.MgmtAuthBearer})
	if err != nil {
		t.Fatal(err)
	}
	if err := v.RequireListen(); err == nil {
		t.Fatal("zero tokens must fail closed")
	}
}

func TestMissingAuthUnauthorized(t *testing.T) {
	v := Static(testSecret, "admin", model.RoleAdministrator)
	_, err := v.Authenticate(Request{})
	if err == nil {
		t.Fatal("missing")
	}
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeUnauthorized {
		t.Fatalf("%v", err)
	}
}
