package testutil

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// DialFencePackages must not call net.Dial / DialContext / DialTimeout in production files.
var DialFencePackages = []string{
	"internal/netconfssh",
	"internal/ncserver",
	"internal/ncframing",
	"internal/ncrpc",
	"internal/datastore",
	"internal/restconf",
	"internal/app",
	"internal/notif",
}

// ForbiddenModuleSubstrings must not appear in import paths or go.mod.
var ForbiddenModuleSubstrings = []string{
	"sysrepo",
	"netopeer2",
	"confd",
	"openyuma",
	"nso",
	"freeconf",
	"yangson",
}

// ForbiddenExecBasenames must not be exec.Command / CommandContext arguments.
var ForbiddenExecBasenames = []string{
	"netopeer2-server",
	"sysrepod",
	"confd",
}

var dialSelectors = map[string]bool{
	"Dial":        true,
	"DialContext": true,
	"DialTimeout": true,
}

var execCommandNames = map[string]bool{
	"Command":        true,
	"CommandContext": true,
}

// RepoRoot walks up from dir until go.mod is found.
func RepoRoot(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", dir)
		}
		dir = parent
	}
}

// CheckDialFence reports Dial/DialContext/DialTimeout in production files of DialFencePackages.
func CheckDialFence(root string) error {
	var hits []string
	for _, rel := range DialFencePackages {
		dir := filepath.Join(root, filepath.FromSlash(rel))
		if _, err := os.Stat(dir); err != nil {
			return fmt.Errorf("dial fence package missing: %s", rel)
		}
		found, err := scanDir(dir, func(path string, f *ast.File) []string {
			var out []string
			ast.Inspect(f, func(n ast.Node) bool {
				name, ok := dialCallName(n)
				if ok {
					out = append(out, fmt.Sprintf("%s: %s", path, name))
				}
				return true
			})
			return out
		})
		if err != nil {
			return err
		}
		hits = append(hits, found...)
	}
	if len(hits) > 0 {
		return fmt.Errorf("dial in data-plane packages:\n  %s", strings.Join(hits, "\n  "))
	}
	return nil
}

// CheckForbiddenModules reports forbidden import paths, go.mod require rows, and exec basenames.
func CheckForbiddenModules(root string) error {
	var hits []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if skipDir(base) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			ipath := strings.Trim(imp.Path.Value, `"`)
			if matchForbiddenModule(ipath) {
				hits = append(hits, fmt.Sprintf("%s imports %s", rel, ipath))
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			name, ok := execBasename(n)
			if ok {
				hits = append(hits, fmt.Sprintf("%s exec %s", rel, name))
			}
			return true
		})
		return nil
	}); err != nil {
		return err
	}
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(mod), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "module ") || strings.HasPrefix(line, "go ") {
			continue
		}
		for _, bad := range ForbiddenModuleSubstrings {
			if strings.Contains(line, bad) {
				hits = append(hits, fmt.Sprintf("go.mod: %s", line))
				break
			}
		}
	}
	if len(hits) > 0 {
		return fmt.Errorf("forbidden modules or exec basenames:\n  %s", strings.Join(hits, "\n  "))
	}
	return nil
}

func skipDir(base string) bool {
	switch base {
	case ".git", "testdata", "vendor", "node_modules", "dist", "bin",
		"go-lab-netconf-design-pack":
		return true
	default:
		return false
	}
}

func matchForbiddenModule(ipath string) bool {
	lower := strings.ToLower(ipath)
	for _, bad := range ForbiddenModuleSubstrings {
		if strings.Contains(lower, bad) {
			return true
		}
	}
	return false
}

func dialCallName(n ast.Node) (string, bool) {
	switch x := n.(type) {
	case *ast.SelectorExpr:
		if x.Sel != nil && dialSelectors[x.Sel.Name] {
			return x.Sel.Name, true
		}
	case *ast.CallExpr:
		id, ok := x.Fun.(*ast.Ident)
		if ok && dialSelectors[id.Name] {
			return id.Name, true
		}
	}
	return "", false
}

func execBasename(n ast.Node) (string, bool) {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil || !execCommandNames[sel.Sel.Name] {
		return "", false
	}
	argIdx := 0
	if sel.Sel.Name == "CommandContext" && len(call.Args) > 1 {
		argIdx = 1
	}
	if argIdx >= len(call.Args) {
		return "", false
	}
	lit, ok := call.Args[argIdx].(*ast.BasicLit)
	if !ok {
		return "", false
	}
	val := strings.Trim(lit.Value, `"`)
	base := filepath.Base(val)
	for _, bad := range ForbiddenExecBasenames {
		if base == bad {
			return bad, true
		}
	}
	return "", false
}

func scanDir(dir string, fn func(path string, f *ast.File) []string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var out []string
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		out = append(out, fn(path, f)...)
	}
	return out, nil
}
