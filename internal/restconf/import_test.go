package restconf

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestNoControlWebSSHOrYAMLImports(t *testing.T) {
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	forbidden := []string{
		"gopkg.in/yaml.v3",
		"github.com/hilather/go-lab-netconf/internal/control",
		"github.com/hilather/go-lab-netconf/internal/web",
		"github.com/hilather/go-lab-netconf/internal/ncframing",
		"github.com/hilather/go-lab-netconf/internal/ncrpc",
		"github.com/hilather/go-lab-netconf/internal/netconfssh",
		"github.com/hilather/go-lab-netconf/internal/ncserver",
		"github.com/hilather/go-lab-netconf/internal/notif",
		"github.com/hilather/go-lab-netconf/internal/config",
		"golang.org/x/crypto/ssh",
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
