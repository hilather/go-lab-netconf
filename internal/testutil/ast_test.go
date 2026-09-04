package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := RepoRoot(wd)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDialASTFence(t *testing.T) {
	if err := CheckDialFence(repoRoot(t)); err != nil {
		t.Fatal(err)
	}
}

func TestForbiddenModuleAST(t *testing.T) {
	if err := CheckForbiddenModules(repoRoot(t)); err != nil {
		t.Fatal(err)
	}
}

func TestDialFencePackagesExist(t *testing.T) {
	root := repoRoot(t)
	if len(DialFencePackages) != 8 {
		t.Fatalf("Dial fence length %d, want 8 packages", len(DialFencePackages))
	}
	want := []string{
		"internal/netconfssh", "internal/ncserver", "internal/ncframing", "internal/ncrpc",
		"internal/datastore", "internal/restconf", "internal/app", "internal/notif",
	}
	got := strings.Join(DialFencePackages, ",")
	for _, p := range want {
		if !strings.Contains(got, p) {
			t.Fatalf("Dial fence missing %s: %v", p, DialFencePackages)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil {
			t.Fatalf("missing package dir %s: %v", p, err)
		}
	}
}

func TestForbiddenListIncludesYangsonAndExec(t *testing.T) {
	joined := strings.Join(ForbiddenModuleSubstrings, ",")
	if !strings.Contains(joined, "yangson") {
		t.Fatalf("forbidden modules missing yangson: %v", ForbiddenModuleSubstrings)
	}
	execs := strings.Join(ForbiddenExecBasenames, ",")
	for _, b := range []string{"netopeer2-server", "sysrepod", "confd"} {
		if !strings.Contains(execs, b) {
			t.Fatalf("forbidden exec missing %s: %v", b, ForbiddenExecBasenames)
		}
	}
}

func TestCheckDialFenceReportsMissingPackage(t *testing.T) {
	dir := t.TempDir()
	if err := CheckDialFence(dir); err == nil {
		t.Fatal("expected missing package")
	}
}

func TestCheckForbiddenModulesRejectsImport(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/x\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.go"), []byte("package x\nimport \"github.com/sysrepo/libsysrepo-go\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := CheckForbiddenModules(dir)
	if err == nil || !strings.Contains(err.Error(), "sysrepo") {
		t.Fatalf("error = %v", err)
	}
}

func TestCheckForbiddenModulesRejectsExec(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/x\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "package x\nimport \"os/exec\"\nfunc f() { exec.Command(\"netopeer2-server\") }\n"
	if err := os.WriteFile(filepath.Join(dir, "bad.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	err := CheckForbiddenModules(dir)
	if err == nil || !strings.Contains(err.Error(), "netopeer2-server") {
		t.Fatalf("error = %v", err)
	}
}

func TestCheckForbiddenModulesRejectsYangsonGoMod(t *testing.T) {
	dir := t.TempDir()
	body := "module example.com/x\n\ngo 1.26\n\nrequire github.com/CESNET/yangson v0.0.0\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	err := CheckForbiddenModules(dir)
	if err == nil || !strings.Contains(err.Error(), "yangson") {
		t.Fatalf("error = %v", err)
	}
}

func TestCheckDialFenceRejectsDial(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range DialFencePackages {
		pkgDir := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(pkgDir, 0o755); err != nil {
			t.Fatal(err)
		}
		pkg := filepath.Base(rel)
		src := "package " + pkg + "\n"
		if rel == "internal/app" {
			src = "package app\nimport \"net\"\nfunc f() { net.Dial(\"tcp\", \"127.0.0.1:1\") }\n"
		}
		if err := os.WriteFile(filepath.Join(pkgDir, "doc.go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	err := CheckDialFence(dir)
	if err == nil || !strings.Contains(err.Error(), "dial") {
		t.Fatalf("error = %v", err)
	}
}
