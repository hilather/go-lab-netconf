package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateNotes(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "ok.md")
	body := "# LabNETCONF 1.0.0-rc.1\n\n## Highlights\n\nx\n\n## Added\n\nx\n\n## Residual\n\nNo NETCONF over TLS in 1.0. No call-home.\n\n## Deployment and operations\n\nx\n\n## CI and release evidence\n\nx\n"
	if err := os.WriteFile(ok, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateNotes(ok); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(bad, []byte("# x\nTODO\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateNotes(bad); err == nil {
		t.Fatal("expected missing headings")
	}
	claim := filepath.Join(dir, "claim.md")
	claimed := "# LabNETCONF 1.0.0-rc.1\n\n## Highlights\n\ncall-home is supported\n\n## Added\n\nx\n\n## Residual\n\nx\n\n## Deployment and operations\n\nx\n\n## CI and release evidence\n\nx\n"
	if err := os.WriteFile(claim, []byte(claimed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateNotes(claim); err == nil {
		t.Fatal("expected TLS/call-home claim reject")
	}
}

func TestValidateRepoReleaseNotes(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		p := filepath.Join(root, "docs", "releases", "v1.0.0-rc.1.md")
		if _, err := os.Stat(p); err == nil {
			if err := validateNotes(p); err != nil {
				t.Fatal(err)
			}
			return
		}
		root = filepath.Dir(root)
	}
	t.Fatal("docs/releases/v1.0.0-rc.1.md not found")
}
