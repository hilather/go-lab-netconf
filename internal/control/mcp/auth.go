package mcp

import (
	"net/http"
	"strings"

	"github.com/hilather/go-lab-netconf/internal/app"
	"github.com/hilather/go-lab-netconf/internal/auth"
	"github.com/hilather/go-lab-netconf/internal/capabilities"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
)

func actorOf(p auth.Principal) app.Actor {
	return app.Actor{
		ID:        p.ID,
		Class:     p.Class,
		Role:      p.Role,
		Scopes:    append([]string(nil), p.Scopes...),
		Transport: "mcp",
	}
}

func (s *Server) authenticate(r *http.Request) (app.Actor, error) {
	if s.cfg.Auth == nil {
		return app.Actor{}, domainerr.Unauthorized("authentication required")
	}
	h := strings.TrimSpace(r.Header.Get(headerAuthorization))
	if h != "" && strings.HasPrefix(strings.ToLower(h), "basic ") {
		return app.Actor{}, domainerr.Unauthorized("MCP accepts bearer tokens only")
	}
	p, err := s.cfg.Auth.Authenticate(auth.Request{
		Authorization: h,
		RemoteAddr:    r.RemoteAddr,
	})
	if err != nil {
		return app.Actor{}, err
	}
	return actorOf(p), nil
}

func (s *Server) authorizeResource(actor app.Actor, uri string) error {
	if s.cfg.Auth == nil {
		return domainerr.Unauthorized("authentication required")
	}
	cap, ok := lookupResourceCap(uri)
	if !ok {
		return domainerr.NotFound("not found")
	}
	return auth.AuthorizeScopes(actor.Scopes, cap.RequiredScopes)
}

func (s *Server) authorizeTool(actor app.Actor, name string) error {
	if s.cfg.Auth == nil {
		return domainerr.Unauthorized("authentication required")
	}
	caps := capabilities.LookupTool(name)
	if len(caps) == 0 {
		return nil
	}
	return auth.AuthorizeScopes(actor.Scopes, caps[0].RequiredScopes)
}

func lookupResourceCap(uri string) (capabilities.Capability, bool) {
	if c, ok := capabilities.LookupResource(uri); ok {
		return c, true
	}
	for _, tmpl := range capabilities.Resources() {
		if resourceTemplateMatch(tmpl, uri) {
			return capabilities.LookupResource(tmpl)
		}
	}
	return capabilities.Capability{}, false
}

func resourceTemplateMatch(tmpl, uri string) bool {
	tParts := strings.Split(tmpl, "/")
	uParts := strings.Split(uri, "/")
	if len(tParts) != len(uParts) {
		return false
	}
	for i := range tParts {
		t, u := tParts[i], uParts[i]
		if strings.HasPrefix(t, "{") && strings.HasSuffix(t, "}") {
			if u == "" {
				return false
			}
			continue
		}
		if t != u {
			return false
		}
	}
	return true
}
