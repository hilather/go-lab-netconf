package config

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
)

// Validate checks a (preferably normalized) state. It does not mutate st.
// Secret paths must be non-empty; bytes on disk are not required.
func Validate(st *model.State) error {
	return validate(st, "")
}

func validate(st *model.State, baseDir string) error {
	if st == nil {
		return domainerr.ValidationFailed("nil state",
			domainerr.FieldViolation{Path: "", Code: violationRequired, Message: "state is nil"})
	}
	var vs []domainerr.FieldViolation
	validateDocument(st, &vs)
	validateListeners(&st.Spec.Listeners, &vs)
	validateAuth(&st.Spec.Auth, baseDir, &vs)
	validateNetconf(&st.Spec.Netconf, &vs)
	validateAdmission(&st.Spec.Admission, &vs)
	validateProfiles(st.Spec.Profiles, &vs)
	validateUsers(st.Spec.Users, st.Spec.Profiles, &vs)
	validateDataPlaneRequirements(&st.Spec, &vs)

	if err := rejectUnsupportedFlags(&st.Spec, vs); err != nil {
		return err
	}
	if len(vs) > 0 {
		return domainerr.ValidationFailed("bootstrap document is invalid", vs...)
	}
	return nil
}

func rejectUnsupportedFlags(sp *model.Spec, vs []domainerr.FieldViolation) error {
	if sp.Listeners.Restconf.TLS.Enabled {
		v := domainerr.FieldViolation{
			Path:    "spec.listeners.restconf.tls.enabled",
			Code:    violationTLSUnsupported,
			Message: "tls.enabled must be false in 1.0",
		}
		return domainerr.TLSUnsupported("RESTCONF TLS is not supported in 1.0", append(vs, v)...)
	}
	if sp.Listeners.NetconfTLS.Enabled {
		v := domainerr.FieldViolation{
			Path:    "spec.listeners.netconfTls.enabled",
			Code:    violationTLSUnsupported,
			Message: "netconfTls.enabled must be false in 1.0",
		}
		return domainerr.TLSUnsupported("NETCONF TLS is not supported in 1.0", append(vs, v)...)
	}
	if sp.Listeners.CallHome.Enabled {
		v := domainerr.FieldViolation{
			Path:    "spec.listeners.callHome.enabled",
			Code:    violationCallHomeUnsupported,
			Message: "callHome.enabled must be false in 1.0",
		}
		return domainerr.CallHomeUnsupported("call-home is not supported in 1.0", append(vs, v)...)
	}
	return nil
}

func validateDocument(st *model.State, vs *[]domainerr.FieldViolation) {
	if st.APIVersion != model.APIVersionV1Alpha1 {
		code := violationUnsupportedVersion
		msg := fmt.Sprintf("apiVersion must be %q", model.APIVersionV1Alpha1)
		if strings.TrimSpace(st.APIVersion) == "" {
			code = violationRequired
			msg = "apiVersion is required"
		}
		*vs = append(*vs, domainerr.FieldViolation{Path: "apiVersion", Code: code, Message: msg})
	}
	if st.Kind != model.KindLabNETCONF {
		code := violationInvalidValue
		msg := fmt.Sprintf("kind must be %q", model.KindLabNETCONF)
		if strings.TrimSpace(st.Kind) == "" {
			code = violationRequired
			msg = "kind is required"
		}
		*vs = append(*vs, domainerr.FieldViolation{Path: "kind", Code: code, Message: msg})
	}
	if strings.TrimSpace(st.Metadata.Name) == "" {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "metadata.name",
			Code:    violationRequired,
			Message: "metadata.name is required",
		})
	}
}

func validateListeners(l *model.ListenersSpec, vs *[]domainerr.FieldViolation) {
	validateTCPAddr("spec.listeners.netconf.address", l.Netconf.Address, vs)
	validateTCPAddr("spec.listeners.restconf.address", l.Restconf.Address, vs)
	validateTCPAddr("spec.listeners.management.address", l.Management.Address, vs)
	if l.Management.RESTPath != "" && !strings.HasPrefix(l.Management.RESTPath, "/") {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.listeners.management.restPath",
			Code:    violationInvalidValue,
			Message: "restPath must start with /",
		})
	}
	if l.Management.MCPPath != "" && !strings.HasPrefix(l.Management.MCPPath, "/") {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.listeners.management.mcpPath",
			Code:    violationInvalidValue,
			Message: "mcpPath must start with /",
		})
	}
	if l.Netconf.Enabled && strings.TrimSpace(l.Netconf.HostKeyFile) == "" {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.listeners.netconf.hostKeyFile",
			Code:    violationRequired,
			Message: "hostKeyFile is required when NETCONF is enabled (path non-empty; bytes required to serve)",
		})
	}
}

func validateAuth(a *model.AuthSpec, baseDir string, vs *[]domainerr.FieldViolation) {
	if a.Mode != "" && a.Mode != model.MgmtAuthBearer {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.auth.mode",
			Code:    violationInvalidValue,
			Message: "mode must be bearer",
		})
	}
	ids := map[string]int{}
	for i, tok := range a.Tokens {
		p := fmt.Sprintf("spec.auth.tokens[%d]", i)
		if strings.TrimSpace(tok.ID) == "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: p + ".id", Code: violationEmptyID, Message: "token id is required"})
		} else if prev, ok := ids[tok.ID]; ok {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    p + ".id",
				Code:    violationDuplicateID,
				Message: fmt.Sprintf("duplicate token id %q (also tokens[%d])", tok.ID, prev),
			})
		} else {
			ids[tok.ID] = i
		}
		if tok.Role != "" && !model.KnownRole(tok.Role) {
			*vs = append(*vs, domainerr.FieldViolation{Path: p + ".role", Code: violationInvalidValue, Message: "role must be administrator or reader"})
		}
		checkSecretPath(p+".secretFile", tok.SecretFile, baseDir, MinTokenBytes, vs)
	}
}

func validateNetconf(n *model.NetconfSpec, vs *[]domainerr.FieldViolation) {
	if n.WritableRunning {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.netconf.writableRunning",
			Code:    violationInvalidValue,
			Message: "writableRunning must be false",
		})
	}
	seen := map[string]bool{}
	for i, v := range n.Versions {
		if v != model.NetconfVersion10 && v != model.NetconfVersion11 {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    fmt.Sprintf("spec.netconf.versions[%d]", i),
				Code:    violationInvalidValue,
				Message: `versions must be a subset of {"1.0","1.1"}`,
			})
		}
		if seen[v] {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    fmt.Sprintf("spec.netconf.versions[%d]", i),
				Code:    violationDuplicateID,
				Message: "duplicate version",
			})
		}
		seen[v] = true
	}
}

func validateAdmission(a *model.AdmissionSpec, vs *[]domainerr.FieldViolation) {
	for i, c := range a.AllowClientCidrs {
		if _, err := netip.ParsePrefix(c); err != nil {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    fmt.Sprintf("spec.admission.allowClientCidrs[%d]", i),
				Code:    violationInvalidValue,
				Message: fmt.Sprintf("invalid CIDR %q", c),
			})
		}
	}
}

func validateProfiles(profiles []model.ProfileSpec, vs *[]domainerr.FieldViolation) {
	names := map[string]int{}
	for i, p := range profiles {
		path := fmt.Sprintf("spec.profiles[%d]", i)
		if strings.TrimSpace(p.Name) == "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".name", Code: violationEmptyID, Message: "profile name is required"})
		} else if prev, ok := names[p.Name]; ok {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    path + ".name",
				Code:    violationDuplicateID,
				Message: fmt.Sprintf("duplicate profile name %q (also profiles[%d])", p.Name, prev),
			})
		} else {
			names[p.Name] = i
		}
		for j, m := range p.Modules {
			if strings.TrimSpace(m.Name) == "" {
				*vs = append(*vs, domainerr.FieldViolation{
					Path:    fmt.Sprintf("%s.modules[%d].name", path, j),
					Code:    violationRequired,
					Message: "module name is required",
				})
			}
		}
		for j, s := range p.Schema {
			sp := fmt.Sprintf("%s.schema[%d]", path, j)
			if strings.TrimSpace(s.Path) == "" {
				*vs = append(*vs, domainerr.FieldViolation{Path: sp + ".path", Code: violationRequired, Message: "schema path is required"})
			}
			if strings.TrimSpace(s.Type) == "" {
				*vs = append(*vs, domainerr.FieldViolation{Path: sp + ".type", Code: violationRequired, Message: "schema type is required"})
			} else if !model.KnownSchemaType(s.Type) {
				*vs = append(*vs, domainerr.FieldViolation{Path: sp + ".type", Code: violationInvalidValue, Message: "unsupported compact schema type"})
			}
			if s.Access != "" && s.Access != model.SchemaAccessRead && s.Access != model.SchemaAccessWrite {
				*vs = append(*vs, domainerr.FieldViolation{Path: sp + ".access", Code: violationInvalidValue, Message: "access must be read or write"})
			}
			if s.ValueFrom != "" && s.ValueFrom != model.ValueFromProcessUptime {
				*vs = append(*vs, domainerr.FieldViolation{Path: sp + ".valueFrom", Code: violationInvalidValue, Message: "valueFrom must be processUptime"})
			}
		}
	}
}

func validateUsers(users []model.UserSpec, profiles []model.ProfileSpec, vs *[]domainerr.FieldViolation) {
	profileNames := map[string]bool{}
	for _, p := range profiles {
		if p.Name != "" {
			profileNames[p.Name] = true
		}
	}
	names := map[string]int{}
	for i, u := range users {
		path := fmt.Sprintf("spec.users[%d]", i)
		if strings.TrimSpace(u.Name) == "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".name", Code: violationEmptyID, Message: "user name is required"})
		} else if prev, ok := names[u.Name]; ok {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    path + ".name",
				Code:    violationDuplicateID,
				Message: fmt.Sprintf("duplicate user name %q (also users[%d])", u.Name, prev),
			})
		} else {
			names[u.Name] = i
		}
		if strings.TrimSpace(u.Profile) == "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".profile", Code: violationRequired, Message: "profile is required"})
		} else if !profileNames[u.Profile] {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    path + ".profile",
				Code:    violationInvalidValue,
				Message: fmt.Sprintf("profile %q is not defined", u.Profile),
			})
		}
		if u.Access != model.UserAccessRead && u.Access != model.UserAccessReadWrite {
			code := violationInvalidValue
			msg := "access must be read or read-write"
			if strings.TrimSpace(u.Access) == "" {
				code = violationRequired
				msg = "access is required"
			}
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".access", Code: code, Message: msg})
		}
		pw := strings.TrimSpace(u.PasswordFile)
		keys := strings.TrimSpace(u.AuthorizedKeysFile)
		if pw == "" && keys == "" {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    path,
				Code:    violationRequired,
				Message: "passwordFile and/or authorizedKeysFile is required",
			})
		}
	}
}

func validateDataPlaneRequirements(sp *model.Spec, vs *[]domainerr.FieldViolation) {
	if !sp.Listeners.Netconf.Enabled && !sp.Listeners.Restconf.Enabled {
		return
	}
	if len(sp.Profiles) == 0 {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.profiles",
			Code:    violationRequired,
			Message: "at least one profile is required when a data plane is enabled",
		})
	}
	if len(sp.Users) == 0 {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.users",
			Code:    violationRequired,
			Message: "at least one user is required when a data plane is enabled",
		})
	}
}

func validateTCPAddr(path, addr string, vs *[]domainerr.FieldViolation) {
	if strings.TrimSpace(addr) == "" {
		*vs = append(*vs, domainerr.FieldViolation{Path: path, Code: violationRequired, Message: "address is required"})
		return
	}
	if _, err := net.ResolveTCPAddr("tcp", addr); err != nil {
		*vs = append(*vs, domainerr.FieldViolation{Path: path, Code: violationInvalidValue, Message: "invalid TCP host:port"})
	}
}

func checkSecretPath(field, path, baseDir string, minBytes int, vs *[]domainerr.FieldViolation) {
	if strings.TrimSpace(path) == "" {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    field,
			Code:    violationRequired,
			Message: "secretFile is required (file ref, never inline)",
		})
		return
	}
	if minBytes <= 0 {
		return
	}
	resolved := path
	if !filepath.IsAbs(path) && baseDir != "" {
		resolved = filepath.Join(baseDir, path)
	}
	b, err := os.ReadFile(resolved)
	if err != nil {
		// Missing bytes are allowed at validate; serve requires them.
		return
	}
	if len(b) < minBytes {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    field,
			Code:    violationInvalidValue,
			Message: fmt.Sprintf("token secret must be at least %d bytes", minBytes),
		})
	}
}
