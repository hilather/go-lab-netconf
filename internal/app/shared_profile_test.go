package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/datastore"
)

func TestSharedProfileDatastoreDefaultTrueCouplesManagementAndUser(t *testing.T) {
	svc, snap := mustBoot(t)
	if !snap.SharedProfileStore {
		t.Fatalf("SharedProfileStore = false, want true (default)")
	}
	ph, ok := svc.Datastore("router-a")
	if !ok {
		t.Fatal("profile handle missing")
	}
	uh, ok := svc.UserDatastore("alice")
	if !ok {
		t.Fatal("user handle missing")
	}
	if ph != uh {
		t.Fatal("default true must couple profile and user handles (same pointer)")
	}
	if ph.Dirty() || uh.Dirty() {
		t.Fatal("clean at boot")
	}
	setHostname(t, svc, "router-a", "candidate", "via-mgmt")
	if !ph.Dirty() || !uh.Dirty() {
		t.Fatal("candidate dirty must be visible on both handles when shared=true")
	}
	if hostname(t, svc, "router-a", datastore.Candidate) != "via-mgmt" {
		t.Fatalf("candidate hostname = %q, want via-mgmt", hostname(t, svc, "router-a", datastore.Candidate))
	}
	// User handle sees the same candidate; verify via direct handle Get.
	n, err := uh.Get(context.Background(), datastore.Candidate, datastore.Subtree{})
	if err != nil {
		t.Fatal(err)
	}
	v, ok := n.Lookup(hostnamePath)
	if !ok || v != "via-mgmt" {
		t.Fatalf("user candidate = %v %v", v, ok)
	}

	var tree map[string]any
	raw, err := svc.GetDatastore(context.Background(), "router-a", "candidate")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
}

func TestSharedProfileDatastoreExplicitFalseIsolates(t *testing.T) {
	// Bootstrap with explicit sharedProfileDatastore: false to ensure opt-out still isolates.
	yaml := `apiVersion: labnetconf.dev/v1alpha1
kind: LabNETCONF
metadata:
  name: lab-device
spec:
  listeners:
    netconf:
      hostKeyFile: /run/secrets/labnetconf-hostkey
  netconf:
    sharedProfileDatastore: false
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
	path := filepath.Join(t.TempDir(), "labnetconf.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := Boot(context.Background(), Options{BootstrapPath: path})
	if err != nil {
		t.Fatal(err)
	}
	snap := svc.Active()
	if snap.SharedProfileStore {
		t.Fatal("explicit false must not be shared")
	}
	ph, _ := svc.Datastore("router-a")
	uh, _ := svc.UserDatastore("alice")
	if ph == uh {
		t.Fatal("explicit false must isolate profile and user handles")
	}
	setHostname(t, svc, "router-a", "candidate", "mgmt-edit")
	if !ph.Dirty() {
		t.Fatal("profile candidate should be dirty")
	}
	if uh.Dirty() {
		t.Fatal("user candidate must stay clean when shared=false")
	}
}
