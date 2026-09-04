package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
)

var expectedInvalid = map[string]string{
	"unknown-field.yaml":         violationUnknownField,
	"unknown-kebab.yaml":         violationUnknownField,
	"reserved-callhome.yaml":     violationReservedKey,
	"reserved-manager.yaml":      violationReservedKey,
	"reserved-remote.yaml":       violationReservedKey,
	"tls-enabled.yaml":           violationTLSUnsupported,
	"callhome-enabled.yaml":      violationCallHomeUnsupported,
	"netconftls-enabled.yaml":    violationTLSUnsupported,
	"writable-running-true.yaml": violationInvalidValue,
	"management-auth.yaml":       violationUnknownField,
	"short-token.yaml":           violationInvalidValue,
	"missing-profile.yaml":       violationInvalidValue,
	"duplicate-user.yaml":        violationDuplicateID,
	"auth-mode-not-bearer.yaml":  violationInvalidValue,
	"user-no-credentials.yaml":   violationRequired,
}

func TestConfigCompat(t *testing.T) {
	t.Chdir(repoRoot(t))
	validDir := testdata(t, "valid")
	ents, err := os.ReadDir(validDir)
	if err != nil {
		t.Fatal(err)
	}
	var validCount int
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		validCount++
		name := e.Name()
		t.Run("valid/"+name, func(t *testing.T) {
			st, err := LoadFile(filepath.Join(validDir, name))
			if err != nil {
				t.Fatal(err)
			}
			if st.APIVersion != model.APIVersionV1Alpha1 || st.Kind != model.KindLabNETCONF {
				t.Fatalf("api=%q kind=%q", st.APIVersion, st.Kind)
			}
			rev, err := Revision(st)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(rev), model.RevisionPrefix) {
				t.Fatalf("revision %q", rev)
			}
			raw, err := CanonicalJSON(st)
			if err != nil {
				t.Fatal(err)
			}
			again, err := Load(raw)
			if err != nil {
				t.Fatal(err)
			}
			rev2, err := Revision(again)
			if err != nil {
				t.Fatal(err)
			}
			if rev != rev2 {
				t.Fatalf("round-trip revision %s != %s", rev, rev2)
			}
			switch name {
			case "defaults.yaml":
				if !equalCIDRs(st.Spec.Admission.AllowClientCidrs, DefaultLoopbackCIDRs) {
					t.Fatalf("omitted CIDRs = %v, want loopback %v", st.Spec.Admission.AllowClientCidrs, DefaultLoopbackCIDRs)
				}
			case "admission-empty.yaml":
				if st.Spec.Admission.AllowClientCidrs == nil || len(st.Spec.Admission.AllowClientCidrs) != 0 {
					t.Fatalf("empty list must stay deny-all, got %#v", st.Spec.Admission.AllowClientCidrs)
				}
			case "both-credentials.yaml":
				if len(st.Spec.Users) != 1 {
					t.Fatalf("users = %d", len(st.Spec.Users))
				}
				u := st.Spec.Users[0]
				if u.PasswordFile == "" || u.AuthorizedKeysFile == "" {
					t.Fatalf("both credential files required, got password=%q keys=%q", u.PasswordFile, u.AuthorizedKeysFile)
				}
			case "split-horizon.yaml":
				if len(st.Spec.Profiles) != 2 || len(st.Spec.Users) != 2 {
					t.Fatalf("profiles=%d users=%d", len(st.Spec.Profiles), len(st.Spec.Users))
				}
				if st.Spec.Users[0].Profile == st.Spec.Users[1].Profile {
					t.Fatal("split-horizon users must bind distinct profiles")
				}
			case "full.yaml":
				if st.Spec.Auth.Tokens[0].SecretFile != "/run/secrets/labnetconf-token" {
					t.Fatalf("secret path = %q", st.Spec.Auth.Tokens[0].SecretFile)
				}
			}
		})
	}
	if validCount < 4 {
		t.Fatalf("expected at least defaults, full, split-horizon, both-credentials, got %d", validCount)
	}

	invalidDir := testdata(t, "invalid")
	ients, err := os.ReadDir(invalidDir)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range ients {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		name := e.Name()
		want, ok := expectedInvalid[name]
		if !ok {
			t.Errorf("invalid/%s has no expectedInvalid entry", name)
			continue
		}
		seen[name] = true
		t.Run("invalid/"+name, func(t *testing.T) {
			_, err := LoadFile(filepath.Join(invalidDir, name))
			_ = requireDomainCode(t, err, want)
		})
	}
	for name := range expectedInvalid {
		if !seen[name] {
			t.Errorf("expectedInvalid %s missing on disk", name)
		}
	}
}

func equalCIDRs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestValidateNil(t *testing.T) {
	err := Validate(nil)
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeValidationFailed {
		t.Fatalf("err=%v", err)
	}
}

func TestRevisionOmitsSecretBytes(t *testing.T) {
	const payload = "labnetconf-secret-payload-do-not-hash-this-value"
	if len(payload) < MinTokenBytes {
		t.Fatalf("payload is %d bytes, want >= %d", len(payload), MinTokenBytes)
	}
	dir := t.TempDir()
	secretName := "admin.token"
	if err := os.WriteFile(filepath.Join(dir, secretName), []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	yaml := `apiVersion: labnetconf.dev/v1alpha1
kind: LabNETCONF
metadata:
  name: secret-bytes
spec:
  listeners:
    netconf:
      hostKeyFile: /run/secrets/labnetconf-hostkey
  auth:
    tokens:
      - id: admin
        role: administrator
        secretFile: ` + secretName + `
  profiles:
    - name: router-a
      schema:
        - path: "ietf-system:system/hostname"
          type: string
          access: write
      instance:
        ietf-system:
          system:
            hostname: "lab-rtr-a"
  users:
    - name: alice
      passwordFile: /run/secrets/netconf-alice
      profile: router-a
      access: read-write
`
	cfg := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfg, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := LoadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if st.Spec.Auth.Tokens[0].SecretFile != secretName {
		t.Fatalf("secretFile = %q", st.Spec.Auth.Tokens[0].SecretFile)
	}
	y, err := CanonicalYAML(st)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(y), secretName) {
		t.Fatalf("canonical YAML missing secret path %q: %s", secretName, y)
	}
	if strings.Contains(string(y), payload) {
		t.Fatal("canonical YAML leaked secret bytes")
	}
	rev, err := Revision(st)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rev), payload) {
		t.Fatal("revision leaked secret bytes")
	}
}
