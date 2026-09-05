package mcp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoRESTImport(t *testing.T) {
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(path, "internal/control/rest") {
				t.Errorf("%s imports REST %s", name, path)
			}
			if path == "github.com/hilather/go-lab-netconf/internal/web" ||
				strings.HasPrefix(path, "github.com/hilather/go-lab-netconf/internal/web/") {
				t.Errorf("%s imports web %s", name, path)
			}
			if path == "github.com/hilather/go-lab-netconf/internal/restconf" {
				t.Errorf("%s imports restconf %s", name, path)
			}
		}
	}
}

func TestNoHTTPCallToREST(t *testing.T) {
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	clientSels := map[string]bool{"Get": true, "Post": true, "PostForm": true, "Head": true}
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel == nil || !clientSels[sel.Sel.Name] {
				return true
			}
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "http" {
				t.Errorf("%s calls http.%s (MCP must not HTTP-call REST)", name, sel.Sel.Name)
			}
			return true
		})
	}
}

func TestSDKOnlyOnAdapter(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "testdata", "vendor", "node_modules", "dist", "bin", "go-lab-netconf-design-pack":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		if strings.HasPrefix(rel, filepath.Join("internal", "control", "mcp")+string(os.PathSeparator)) {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			ipath := strings.Trim(imp.Path.Value, `"`)
			if ipath == "github.com/modelcontextprotocol/go-sdk" || strings.HasPrefix(ipath, "github.com/modelcontextprotocol/go-sdk/") {
				t.Errorf("%s imports official SDK; only internal/control/mcp may", rel)
			}
			if ipath == "github.com/google/jsonschema-go" || strings.HasPrefix(ipath, "github.com/google/jsonschema-go/") {
				t.Errorf("%s imports jsonschema-go directly", rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
