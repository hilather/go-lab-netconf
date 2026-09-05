package compiler

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/config"
)

const hostnamePath = "ietf-system:system/hostname"

func TestCompileFullYAML(t *testing.T) {
	st, err := config.LoadFile(filepath.Join(moduleRoot(t), "testdata", "config", "valid", "full.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := Compile(st, CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Revision == "" || snap.BootstrapRevision != snap.Revision {
		t.Fatalf("revision %q bootstrap %q", snap.Revision, snap.BootstrapRevision)
	}
	if snap.NetconfAddress != ":830" || snap.RestconfAddress != ":8303" {
		t.Fatalf("listeners %q %q", snap.NetconfAddress, snap.RestconfAddress)
	}
	p, ok := snap.ProfileNamed("router-a")
	if !ok {
		t.Fatal("missing profile")
	}
	got, ok := p.Running.Lookup(hostnamePath)
	if !ok || got != "lab-rtr-a" {
		t.Fatalf("hostname = %v, %v", got, ok)
	}
	if len(snap.Users) != 1 || snap.Users[0].Name != "alice" || snap.Users[0].Profile != "router-a" {
		t.Fatalf("users %+v", snap.Users)
	}
	if snap.Users[0].PasswordFile == "" {
		t.Fatal("password path must be kept")
	}
}

func TestCompileSplitHorizon(t *testing.T) {
	st, err := config.LoadFile(filepath.Join(moduleRoot(t), "testdata", "config", "valid", "split-horizon.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := Compile(st, CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := snap.ProfileNamed("router-a")
	b, _ := snap.ProfileNamed("router-b")
	ha, _ := a.Running.Lookup(hostnamePath)
	hb, _ := b.Running.Lookup(hostnamePath)
	if ha != "lab-rtr-a" || hb != "lab-rtr-b" {
		t.Fatalf("hostnames %v %v", ha, hb)
	}
	if snap.SharedProfileStore {
		t.Fatal("sharedProfileDatastore default false")
	}
}

func TestCompileAdmissionOmittedVsEmpty(t *testing.T) {
	omitted, err := config.LoadFile(filepath.Join(moduleRoot(t), "testdata", "config", "valid", "defaults.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := Compile(omitted, CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.AllowDenyAll || len(snap.Allow) != 2 {
		t.Fatalf("omitted allow %+v denyAll=%v", snap.Allow, snap.AllowDenyAll)
	}
	if !snap.Allowed(netip.MustParseAddr("127.0.0.1")) {
		t.Fatal("loopback must be allowed when CIDRs omitted")
	}

	empty, err := config.LoadFile(filepath.Join(moduleRoot(t), "testdata", "config", "valid", "admission-empty.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	snap, err = Compile(empty, CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if !snap.AllowDenyAll {
		t.Fatal("present empty CIDRs must be deny-all")
	}
	if snap.Allowed(netip.MustParseAddr("127.0.0.1")) {
		t.Fatal("deny-all must reject loopback")
	}
}

func TestCompileDoesNotNeedSecretBytes(t *testing.T) {
	st, err := config.LoadFile(filepath.Join(moduleRoot(t), "testdata", "config", "valid", "full.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(st.Spec.Auth.Tokens[0].SecretFile); err == nil {
		t.Fatal("fixture secret must not exist on disk")
	}
	if _, err := Compile(st, CompileOpts{}); err != nil {
		t.Fatal(err)
	}
}

func moduleRoot(t *testing.T) string {
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
			t.Fatal("go.mod")
		}
		dir = parent
	}
}
