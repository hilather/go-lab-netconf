package mcp

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestNoRESTRestconfWebOrSSHImports(t *testing.T) {
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	forbidden := []string{
		"github.com/hilather/go-lab-netconf/internal/control/rest",
		"github.com/hilather/go-lab-netconf/internal/restconf",
		"github.com/hilather/go-lab-netconf/internal/web",
		"github.com/hilather/go-lab-netconf/internal/netconfssh",
		"golang.org/x/crypto/ssh",
		"gopkg.in/yaml.v3",
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
		}
	}
}
