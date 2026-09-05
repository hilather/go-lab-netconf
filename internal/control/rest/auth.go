package rest

import (
	"net/http"

	"github.com/hilather/go-lab-netconf/internal/capabilities"
)

func (s *Server) authenticate(_ *http.Request, skip bool) error {
	_ = skip
	return nil
}

func (s *Server) authorize(_ *http.Request, _ capabilities.Capability) error {
	return nil
}
