package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
)

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

func testdata(t *testing.T, elem ...string) string {
	t.Helper()
	parts := append([]string{repoRoot(t), "testdata", "config"}, elem...)
	return filepath.Join(parts...)
}

func requireDomainCode(t *testing.T, err error, want string) *domainerr.Error {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	de, ok := domainerr.As(err)
	if !ok {
		t.Fatalf("error is %T %v, want *domainerr.Error", err, err)
	}
	if hasCode(de, want) {
		return de
	}
	t.Fatalf("want code %q, got %s violations=%+v err=%v", want, de.Code, de.FieldViolations, err)
	return de
}

func hasCode(de *domainerr.Error, want string) bool {
	if string(de.Code) == want {
		return true
	}
	for _, v := range de.FieldViolations {
		if v.Code == want {
			return true
		}
	}
	return false
}
