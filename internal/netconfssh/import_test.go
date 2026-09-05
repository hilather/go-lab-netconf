package netconfssh

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestNoControlWebOrDialImports(t *testing.T) {
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	forbidden := []string{
		"github.com/hilather/go-lab-netconf/internal/control",
		"github.com/hilather/go-lab-netconf/internal/web",
		"gopkg.in/yaml.v3",
		"github.com/modelcontextprotocol",
	}
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
			for _, bad := range forbidden {
				if path == bad || strings.HasPrefix(path, bad+"/") {
					t.Errorf("%s imports %s", name, path)
				}
			}
			if strings.Contains(path, "Dial") {
				t.Errorf("%s imports %s", name, path)
			}
		}
	}
}

func TestExportedAPIHasNoSSHTypes(t *testing.T) {
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
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			switch x := decl.(type) {
			case *ast.FuncDecl:
				if x.Name == nil || !x.Name.IsExported() {
					continue
				}
				if exprHasSSH(x.Type) {
					t.Errorf("%s: exported %s mentions ssh types", name, x.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range x.Specs {
					switch vs := spec.(type) {
					case *ast.TypeSpec:
						if vs.Name == nil || !vs.Name.IsExported() {
							continue
						}
						if exportedTypeHasSSH(vs.Type) {
							t.Errorf("%s: exported type %s mentions ssh types", name, vs.Name.Name)
						}
					case *ast.ValueSpec:
						for _, ident := range vs.Names {
							if ident.IsExported() && exprHasSSH(vs.Type) {
								t.Errorf("%s: exported %s mentions ssh types", name, ident.Name)
							}
						}
					}
				}
			}
		}
	}
}

func exportedTypeHasSSH(n ast.Expr) bool {
	st, ok := n.(*ast.StructType)
	if !ok {
		return exprHasSSH(n)
	}
	if st.Fields == nil {
		return false
	}
	for _, f := range st.Fields.List {
		exported := len(f.Names) == 0
		for _, name := range f.Names {
			if name.IsExported() {
				exported = true
			}
		}
		if exported && exprHasSSH(f.Type) {
			return true
		}
	}
	return false
}

func exprHasSSH(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if ok && id.Name == "ssh" {
			found = true
		}
		return true
	})
	return found
}
