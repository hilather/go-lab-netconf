package auth

import (
	"github.com/hilather/go-lab-netconf/internal/capabilities"
	"github.com/hilather/go-lab-netconf/internal/model"
)

// DefaultScopes returns the frozen role → scope set. An explicit token
// Scopes list wins over role expansion.
func DefaultScopes(role string) []string {
	switch role {
	case model.RoleReader:
		return []string{capabilities.ScopeNetconfRead}
	case model.RoleAdministrator:
		return allScopes()
	default:
		return nil
	}
}

func allScopes() []string {
	return []string{
		capabilities.ScopeNetconfRead,
		capabilities.ScopeNetconfWrite,
		capabilities.ScopeNetconfAdmin,
		capabilities.ScopeNetconfAuditRead,
	}
}

func expandScopes(role string, scopes []string) (string, []string) {
	out := append([]string(nil), scopes...)
	if len(out) > 0 {
		if role == "" {
			role = model.RoleAdministrator
		}
		return role, out
	}
	if role == "" {
		role = model.RoleAdministrator
	}
	return role, DefaultScopes(role)
}
