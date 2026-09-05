package rest

import (
	"encoding/json"
	"net/http"

	"github.com/hilather/go-lab-netconf/internal/auth"
	"github.com/hilather/go-lab-netconf/internal/capabilities"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
)

func (s *Server) writeProblem(w http.ResponseWriter, r *http.Request, instance string, err error) {
	p := capabilities.ProblemFrom(err, instance)
	if p.Status == http.StatusUnauthorized {
		for _, v := range auth.WWWAuthenticate() {
			w.Header().Add("WWW-Authenticate", v)
		}
	}
	body, merr := json.Marshal(p)
	if merr != nil {
		http.Error(w, `{"type":"urn:labnetconf:error:internal","title":"Internal error","status":500}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", capabilities.ProblemContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(p.Status)
	_, _ = w.Write(body)
	_ = r
}

func asDomain(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := domainerr.As(err); ok {
		return err
	}
	return domainerr.ValidationFailed(err.Error())
}
