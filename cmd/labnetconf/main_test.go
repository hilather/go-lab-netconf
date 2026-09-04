package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/config"
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

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labnetconf", "version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "labnetconf") {
		t.Fatalf("version output %q missing labnetconf", out)
	}
	if !strings.Contains(out, "labnetconf.dev/v1alpha1") {
		t.Fatalf("version %q missing config API", out)
	}
	if !strings.Contains(out, "2026-07-28") {
		t.Fatalf("version %q missing MCP protocol", out)
	}
}

func TestUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labnetconf"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr %q missing usage", stderr.String())
	}
}

func TestHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labnetconf", "help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	out := stdout.String()
	for _, s := range []string{"version", "serve", "validate", "canonicalize", "healthcheck", "mcp-stdio"} {
		if !strings.Contains(out, s) {
			t.Fatalf("help missing %s: %q", s, out)
		}
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labnetconf", "query-remote"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

func TestUnimplementedCommands(t *testing.T) {
	for _, cmd := range []string{"serve", "healthcheck", "mcp-stdio"} {
		var stdout, stderr bytes.Buffer
		code := run([]string{"labnetconf", cmd}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("%s exit %d, want 1", cmd, code)
		}
		if !strings.Contains(stderr.String(), "not implemented") {
			t.Fatalf("%s stderr %q missing not implemented", cmd, stderr.String())
		}
	}
}

func TestValidateAndCanonicalize(t *testing.T) {
	path := filepath.Join(repoRoot(t), "testdata/config/valid/full.yaml")
	var stdout, stderr bytes.Buffer
	code := run([]string{"labnetconf", "validate", "--config", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("validate exit %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok revision=") {
		t.Fatalf("validate %q", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"labnetconf", "canonicalize", "--config", path, "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("canonicalize exit %d stderr=%q", code, stderr.String())
	}
	st, err := config.Load(stdout.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if st.Kind != "LabNETCONF" {
		t.Fatalf("kind %q", st.Kind)
	}
}

func TestValidateRequiresConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labnetconf", "validate"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d", code)
	}
}

func TestValidateRejectsInvalid(t *testing.T) {
	path := filepath.Join(repoRoot(t), "testdata/config/invalid/tls-enabled.yaml")
	var stdout, stderr bytes.Buffer
	code := run([]string{"labnetconf", "validate", "--config", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "tls_unsupported") {
		t.Fatalf("stderr %q missing tls_unsupported", stderr.String())
	}
}
