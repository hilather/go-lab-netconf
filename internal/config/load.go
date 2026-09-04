package config

import (
	"os"
	"path/filepath"

	"github.com/hilather/go-lab-netconf/internal/model"
)

// Load decodes, normalizes, and validates a YAML or JSON document.
func Load(data []byte) (*model.State, error) {
	return load(data, "")
}

// LoadFile reads path and calls Load. Relative secret paths resolve against
// the file's directory.
func LoadFile(path string) (*model.State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return load(b, filepath.Dir(path))
}

func load(data []byte, baseDir string) (*model.State, error) {
	st, err := Decode(data)
	if err != nil {
		return nil, err
	}
	n, err := Normalize(st)
	if err != nil {
		return nil, err
	}
	if err := validate(n, baseDir); err != nil {
		return nil, err
	}
	return n, nil
}
