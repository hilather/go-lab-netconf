package main

import (
	"testing"

	"github.com/hilather/go-lab-netconf/internal/capabilities"
	"github.com/hilather/go-lab-netconf/internal/control/rest"
	"github.com/hilather/go-lab-netconf/internal/observability"
)

func TestPlannedFiles(t *testing.T) {
	files, err := plannedFiles()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		capabilities.ManifestRelPath:     false,
		rest.OpenAPIRelPath:              false,
		capabilities.ErrorCatalogRelPath: false,
		observability.CatalogRelPath:     false,
	}
	for _, f := range files {
		if _, ok := want[f.rel]; !ok {
			t.Errorf("unexpected %s", f.rel)
			continue
		}
		if len(f.body) == 0 {
			t.Errorf("empty %s", f.rel)
		}
		want[f.rel] = true
	}
	for rel, seen := range want {
		if !seen {
			t.Errorf("missing %s", rel)
		}
	}
}
