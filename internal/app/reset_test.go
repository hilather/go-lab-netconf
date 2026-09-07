package app

import (
	"context"
	"os"
	"sync/atomic"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/datastore"
	"github.com/hilather/go-lab-netconf/internal/model"
	"github.com/hilather/go-lab-netconf/internal/notif"
)

type recordWaiter struct {
	notif.Nop
	wipes atomic.Int32
}

func (r *recordWaiter) Wipe() {
	r.wipes.Add(1)
	r.Nop.Wipe()
}

func TestResetNeverWritesBootstrap(t *testing.T) {
	path := copyFixture(t, "defaults.yaml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	w := &recordWaiter{}
	svc, err := Boot(context.Background(), Options{BootstrapPath: path, Sink: w, Waiter: w})
	if err != nil {
		t.Fatal(err)
	}
	setHostname(t, svc, "router-a", "candidate", "scratch")
	if err := svc.Commit(context.Background(), "router-a"); err != nil {
		t.Fatal(err)
	}
	if hostname(t, svc, "router-a", datastore.Running) != "scratch" {
		t.Fatalf("commit did not publish hostname: %q", hostname(t, svc, "router-a", datastore.Running))
	}
	if err := svc.Reset(context.Background()); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("reset must never write bootstrap")
	}
	if w.wipes.Load() != 1 {
		t.Fatalf("Wipe calls = %d, want 1", w.wipes.Load())
	}
	if hostname(t, svc, "router-a", datastore.Running) != "lab-rtr-a" {
		t.Fatalf("running hostname after reset = %q, want lab-rtr-a", hostname(t, svc, "router-a", datastore.Running))
	}
}

func TestCopyOnCompileIsolatesUsers(t *testing.T) {
	// Isolation only when sharedProfileDatastore is explicitly false.
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
	path := copyFixtureWithContent(t, yaml)
	svc, err := Boot(context.Background(), Options{BootstrapPath: path})
	if err != nil {
		t.Fatal(err)
	}
	snap := svc.Active()
	ctx := context.Background()
	_, err = svc.Apply(ctx, []ApplyOp{{
		Op: OpUpsertUser,
		User: &model.UserSpec{
			Name:         "bob",
			PasswordFile: "/run/secrets/netconf-bob",
			Profile:      "router-a",
			Access:       model.UserAccessReadWrite,
		},
	}}, string(snap.Revision), "k-bob")
	if err != nil {
		t.Fatal(err)
	}
	alice, ok := svc.UserDatastore("alice")
	if !ok {
		t.Fatal("alice handle")
	}
	bob, ok := svc.UserDatastore("bob")
	if !ok {
		t.Fatal("bob handle")
	}
	if alice == bob {
		t.Fatal("copy-on-compile must not share handles when shared=false")
	}
	mergeHostnameOnHandle(t, alice, "from-alice")
	n, err := bob.Get(ctx, datastore.Candidate, datastore.Subtree{Path: hostnamePath})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := n.Lookup(hostnamePath)
	if !ok || got != "lab-rtr-a" {
		t.Fatalf("bob candidate leaked alice edit: %v", got)
	}
}

func copyFixtureWithContent(t *testing.T, content string) string {
	t.Helper()
	path := t.TempDir() + "/labnetconf.yaml"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
