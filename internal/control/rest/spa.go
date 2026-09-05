package rest

import (
	"net/http"
	"path"
	"strings"
)

// tryMCP serves POST /mcp after native routing misses. rest must not
// import internal/control/mcp; cmd wires Config.MCP.
func (s *Server) tryMCP(w http.ResponseWriter, r *http.Request) bool {
	if s.cfg.MCP == nil {
		return false
	}
	p := path.Clean("/" + r.URL.Path)
	if p != "/mcp" && !strings.HasPrefix(p, "/mcp/") {
		return false
	}
	s.cfg.MCP.ServeHTTP(w, r)
	return true
}

// tryUI serves the operator SPA after native routing misses. rest must not
// import internal/web; cmd wires Config.UI.
func (s *Server) tryUI(w http.ResponseWriter, r *http.Request, instance string) bool {
	if s.cfg.UI == nil || reservedManagementPath(r.URL.Path) {
		return false
	}
	if s.cfg.UIEnabled != nil && !s.cfg.UIEnabled() {
		return false
	}
	s.cfg.UI.ServeHTTP(w, r)
	return true
}

func reservedManagementPath(p string) bool {
	p = path.Clean("/" + p)
	switch {
	case p == "/v1" || strings.HasPrefix(p, "/v1/"):
		return true
	case p == "/mcp" || strings.HasPrefix(p, "/mcp/"):
		return true
	case p == "/restconf" || strings.HasPrefix(p, "/restconf/"):
		return true
	default:
		return false
	}
}
