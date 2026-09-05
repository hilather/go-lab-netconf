package restconf

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestNoManagementAuthImport(t *testing.T) {
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
			if strings.Contains(path, "internal/auth") || strings.Contains(path, "internal/control") {
				t.Errorf("RESTCONF must not import management auth (%s imports %s)", name, path)
			}
		}
	}
}
