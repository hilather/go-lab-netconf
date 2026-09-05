package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/datastore"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/snapshot"
	"github.com/hilather/go-lab-netconf/internal/yangtree"
)

const hostnamePath = "ietf-system:system/hostname"

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

func copyFixture(t *testing.T, name string) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "testdata", "config", "valid", name))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "labnetconf.yaml")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustBoot(t *testing.T) (*App, *snapshot.Snapshot) {
	t.Helper()
	path := copyFixture(t, "defaults.yaml")
	svc, err := Boot(context.Background(), Options{BootstrapPath: path})
	if err != nil {
		t.Fatal(err)
	}
	snap := svc.Active()
	if snap == nil {
		t.Fatal("no snapshot")
	}
	return svc, snap
}

func hostname(t *testing.T, svc *App, profile string, store datastore.Name) string {
	t.Helper()
	raw, err := svc.GetDatastore(context.Background(), profile, string(store))
	if err != nil {
		t.Fatal(err)
	}
	var tree map[string]any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
	mod, _ := tree["ietf-system"].(map[string]any)
	sys, _ := mod["system"].(map[string]any)
	s, _ := sys["hostname"].(string)
	return s
}

func setHostname(t *testing.T, svc *App, profile, store, value string) {
	t.Helper()
	overlay, err := json.Marshal(map[string]any{
		"ietf-system": map[string]any{
			"system": map[string]any{"hostname": value},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetDatastore(context.Background(), profile, store, overlay); err != nil {
		t.Fatal(err)
	}
}

func requireCode(t *testing.T, err error, code domainerr.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s", code)
	}
	de, ok := domainerr.As(err)
	if !ok || de.Code != code {
		t.Fatalf("err=%v want %s", err, code)
	}
}

func mergeHostnameOnHandle(t *testing.T, h datastore.Handle, value string) {
	t.Helper()
	err := h.Edit(context.Background(), datastore.Candidate, datastore.EditOp{
		Op:    yangtree.OpMerge,
		Path:  hostnamePath,
		Value: value,
	})
	if err != nil {
		t.Fatal(err)
	}
}
