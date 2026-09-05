package rest

import (
	"net/http"
	"strings"

	"github.com/hilather/go-lab-netconf/internal/app"
	"github.com/hilather/go-lab-netconf/internal/auth"
	"github.com/hilather/go-lab-netconf/internal/capabilities"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
)

func actorOf(p auth.Principal, transport string) app.Actor {
	return app.Actor{
		ID:        p.ID,
		Class:     p.Class,
		Role:      p.Role,
		Scopes:    append([]string(nil), p.Scopes...),
		Transport: transport,
	}
}

func (s *Server) authenticate(r *http.Request) (app.Actor, error) {
	if s.cfg.Auth == nil {
		return app.Actor{}, domainerr.Unauthorized("authentication required")
	}

	hdr := strings.TrimSpace(r.Header.Get("Authorization"))
	if hdr != "" {
		p, err := s.cfg.Auth.Authenticate(auth.Request{
			Authorization: hdr,
			RemoteAddr:    r.RemoteAddr,
		})
		if err != nil {
			return app.Actor{}, err
		}
		return actorOf(p, "rest"), nil
	}

	if c, err := r.Cookie(auth.CookieName); err == nil && c.Value != "" && s.cfg.Sessions != nil {
		sess, _, ok := s.cfg.Sessions.Lookup(c.Value)
		if ok {
			return actorOf(auth.PrincipalFromSession(sess), "rest"), nil
		}
	}

	return app.Actor{}, domainerr.Unauthorized("authentication required")
}

func (s *Server) authorize(r *http.Request, actor app.Actor, cap capabilities.Capability) error {
	if s.cfg.Auth == nil {
		return domainerr.Unauthorized("authentication required")
	}
	if err := auth.AuthorizeScopes(actor.Scopes, cap.RequiredScopes); err != nil {
		return err
	}
	if !auth.UnsafeMethod(r.Method) {
		return nil
	}
	if strings.TrimSpace(r.Header.Get("Authorization")) != "" {
		return nil
	}
	c, err := r.Cookie(auth.CookieName)
	if err != nil || c.Value == "" {
		return nil
	}
	if s.cfg.Sessions == nil || !s.cfg.Sessions.ValidCSRF(c.Value, r.Header.Get(auth.CSRFHeader)) {
		return domainerr.Forbidden("CSRF token is missing or invalid")
	}
	return nil
}

func (s *Server) cookieSecure(r *http.Request) bool {
	return auth.CookieSecure(r, s.cfg.CookieSecure)
}
