package rest

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/capabilities"
)

func TestRenderOpenAPIIncludesCatalogPaths(t *testing.T) {
	raw, err := RenderOpenAPI()
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	paths, _ := doc["paths"].(map[string]any)
	if len(paths) == 0 {
		t.Fatal("no paths")
	}
	for _, c := range capabilities.All() {
		for _, b := range c.REST {
			item, ok := paths[b.Path].(map[string]any)
			if !ok {
				t.Errorf("missing path %s", b.Path)
				continue
			}
			if _, ok := item[strings.ToLower(b.Method)]; !ok {
				t.Errorf("missing %s %s", b.Method, b.Path)
			}
		}
	}
	if _, ok := paths["/restconf"]; ok {
		t.Fatal("openapi must not document /restconf")
	}
}

func TestRenderManifestRoundTrip(t *testing.T) {
	raw, err := capabilities.RenderManifest()
	if err != nil {
		t.Fatal(err)
	}
	var doc capabilities.Manifest
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Capabilities) != capabilities.TableRowCount {
		t.Fatalf("rows %d", len(doc.Capabilities))
	}
}
