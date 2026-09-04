package config

import (
	"os"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
)

func TestKnownFieldsRejectsUnknown(t *testing.T) {
	_, err := LoadFile(testdata(t, "invalid", "unknown-field.yaml"))
	de := requireDomainCode(t, err, violationUnknownField)
	if de.Code != domainerr.CodeUnknownField {
		t.Fatalf("top-level code=%s, want unknown_field", de.Code)
	}
}

func TestKnownFieldsRejectsKebab(t *testing.T) {
	_, err := LoadFile(testdata(t, "invalid", "unknown-kebab.yaml"))
	_ = requireDomainCode(t, err, violationUnknownField)
}

func TestReservedKeysReject(t *testing.T) {
	for _, name := range []string{"reserved-callhome.yaml", "reserved-manager.yaml", "reserved-remote.yaml"} {
		t.Run(name, func(t *testing.T) {
			_, err := LoadFile(testdata(t, "invalid", name))
			de := requireDomainCode(t, err, violationReservedKey)
			if de.Code != domainerr.CodeReservedKey {
				t.Fatalf("top-level code=%s, want reserved_key", de.Code)
			}
		})
	}
}

func TestTLSEnabledReject(t *testing.T) {
	_, err := LoadFile(testdata(t, "invalid", "tls-enabled.yaml"))
	de := requireDomainCode(t, err, violationTLSUnsupported)
	if de.Code != domainerr.CodeTLSUnsupported {
		t.Fatalf("top-level code=%s, want tls_unsupported", de.Code)
	}
}

func TestNetconfTLSEnabledReject(t *testing.T) {
	_, err := LoadFile(testdata(t, "invalid", "netconftls-enabled.yaml"))
	de := requireDomainCode(t, err, violationTLSUnsupported)
	if de.Code != domainerr.CodeTLSUnsupported {
		t.Fatalf("top-level code=%s, want tls_unsupported", de.Code)
	}
}

func TestCallHomeEnabledReject(t *testing.T) {
	_, err := LoadFile(testdata(t, "invalid", "callhome-enabled.yaml"))
	de := requireDomainCode(t, err, violationCallHomeUnsupported)
	if de.Code != domainerr.CodeCallHomeUnsupported {
		t.Fatalf("top-level code=%s, want callhome_unsupported", de.Code)
	}
}

func TestWritableRunningTrueReject(t *testing.T) {
	_, err := LoadFile(testdata(t, "invalid", "writable-running-true.yaml"))
	_ = requireDomainCode(t, err, violationInvalidValue)
}

func TestAdmissionOmittedVsEmpty(t *testing.T) {
	omitted, err := LoadFile(testdata(t, "valid", "defaults.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !equalCIDRs(omitted.Spec.Admission.AllowClientCidrs, DefaultLoopbackCIDRs) {
		t.Fatalf("omitted = %v", omitted.Spec.Admission.AllowClientCidrs)
	}
	empty, err := LoadFile(testdata(t, "valid", "admission-empty.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if empty.Spec.Admission.AllowClientCidrs == nil || len(empty.Spec.Admission.AllowClientCidrs) != 0 {
		t.Fatalf("empty = %#v", empty.Spec.Admission.AllowClientCidrs)
	}
}

func TestBothCredentialsUser(t *testing.T) {
	st, err := LoadFile(testdata(t, "valid", "both-credentials.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	u := st.Spec.Users[0]
	if u.PasswordFile == "" || u.AuthorizedKeysFile == "" {
		t.Fatalf("want both files, got %+v", u)
	}
}

func TestValidateDoesNotRequireSecretBytes(t *testing.T) {
	st, err := LoadFile(testdata(t, "valid", "full.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	path := st.Spec.Auth.Tokens[0].SecretFile
	if path != "/run/secrets/labnetconf-token" {
		t.Fatalf("secretFile = %q", path)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("validate must succeed without secret bytes on disk")
	}
}

func TestManagementAuthUnknown(t *testing.T) {
	_, err := LoadFile(testdata(t, "invalid", "management-auth.yaml"))
	_ = requireDomainCode(t, err, violationUnknownField)
}

func TestShortTokenRejectsWhenBytesPresent(t *testing.T) {
	_, err := LoadFile(testdata(t, "invalid", "short-token.yaml"))
	_ = requireDomainCode(t, err, violationInvalidValue)
}

func TestDefaultsMaterialize(t *testing.T) {
	st, err := LoadFile(testdata(t, "valid", "defaults.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Spec.Listeners.Netconf.Address != DefaultNetconfAddress {
		t.Fatalf("netconf address %q", st.Spec.Listeners.Netconf.Address)
	}
	if st.Spec.Listeners.Restconf.Address != DefaultRestconfAddress {
		t.Fatalf("restconf address %q", st.Spec.Listeners.Restconf.Address)
	}
	if st.Spec.Auth.Mode != model.MgmtAuthBearer {
		t.Fatalf("mode %q", st.Spec.Auth.Mode)
	}
	if st.Spec.Netconf.WritableRunning {
		t.Fatal("writableRunning default")
	}
	if !st.Spec.Listeners.Netconf.Enabled || !st.Spec.Listeners.Restconf.Enabled {
		t.Fatal("data planes default enabled")
	}
}
