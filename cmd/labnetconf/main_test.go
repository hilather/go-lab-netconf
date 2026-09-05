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
	code := run([]string{"labnetconf", "version"}, nil, &stdout, &stderr)
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
	code := run([]string{"labnetconf"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr %q missing usage", stderr.String())
	}
}

func TestHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labnetconf", "help"}, nil, &stdout, &stderr)
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
	code := run([]string{"labnetconf", "query-remote"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

func TestServeRequiresConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labnetconf", "serve"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2 stderr=%q", code, stderr.String())
	}
}

func TestHealthcheckRequiresReachableURL(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labnetconf", "healthcheck", "--url", "http://127.0.0.1:1/v1/health/ready"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit %d, want 1 stderr=%q", code, stderr.String())
	}
}

func TestMCPStdioRequiresFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labnetconf", "mcp-stdio"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--config") {
		t.Fatalf("stderr %q missing --config", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"labnetconf", "mcp-stdio", "--config", "x.yaml"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("token-file exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--token-file") {
		t.Fatalf("stderr %q missing --token-file", stderr.String())
	}
}

func TestMCPStdioRequiresTokenFile(t *testing.T) {
	path := filepath.Join(repoRoot(t), "testdata/config/valid/defaults.yaml")
	var stdout, stderr bytes.Buffer
	code := run([]string{"labnetconf", "mcp-stdio", "--config", path}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--token-file is required") {
		t.Fatalf("stderr %q missing token-file required", stderr.String())
	}
}

func TestMCPStdioRequiresConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"labnetconf", "mcp-stdio", "--token-file", "x"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--config is required") {
		t.Fatalf("stderr %q missing config required", stderr.String())
	}
}

func TestValidateAndCanonicalize(t *testing.T) {
	path := filepath.Join(repoRoot(t), "testdata/config/valid/full.yaml")
	var stdout, stderr bytes.Buffer
	code := run([]string{"labnetconf", "validate", "--config", path}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("validate exit %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok revision=") {
		t.Fatalf("validate %q", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"labnetconf", "canonicalize", "--config", path, "--format", "json"}, nil, &stdout, &stderr)
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
	code := run([]string{"labnetconf", "validate"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d", code)
	}
}

func TestValidateRejectsInvalid(t *testing.T) {
	path := filepath.Join(repoRoot(t), "testdata/config/invalid/tls-enabled.yaml")
	var stdout, stderr bytes.Buffer
	code := run([]string{"labnetconf", "validate", "--config", path}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "tls_unsupported") {
		t.Fatalf("stderr %q missing tls_unsupported", stderr.String())
	}
}
