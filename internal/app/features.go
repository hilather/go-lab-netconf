package app

import "github.com/hilather/go-lab-netconf/internal/capabilities"

func featureCatalog() []Feature {
	src := capabilities.Features()
	out := make([]Feature, len(src))
	for i, f := range src {
		out[i] = Feature{ID: f.ID, Apply: f.Apply, Path: f.Path}
	}
	return out
}
