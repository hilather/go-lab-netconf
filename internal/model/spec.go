package model

const (
	MgmtAuthBearer = "bearer"

	RoleAdministrator = "administrator"
	RoleReader        = "reader"

	UserAccessRead      = "read"
	UserAccessReadWrite = "read-write"

	SchemaAccessRead  = "read"
	SchemaAccessWrite = "write"

	ValueFromProcessUptime = "processUptime"

	NetconfVersion10 = "1.0"
	NetconfVersion11 = "1.1"
)

// CompactSchemaTypes is the closed 1.0 path-type set. There is no YANG compiler.
var CompactSchemaTypes = []string{
	"string", "boolean", "int32", "int64", "uint32", "uint64",
	"decimal64", "identityref", "enumeration", "leaf-list", "list",
	"container", "empty",
}

// KnownRole reports whether role is a v1alpha1 management token role.
func KnownRole(role string) bool {
	switch role {
	case RoleAdministrator, RoleReader:
		return true
	default:
		return false
	}
}

// KnownSchemaType reports whether t is a compact schema type.
func KnownSchemaType(t string) bool {
	for _, k := range CompactSchemaTypes {
		if t == k {
			return true
		}
	}
	return false
}

// ListenersSpec configures NETCONF, RESTCONF, and management listeners.
// callHome and netconfTls exist so enabled:true can be rejected in 1.0.
type ListenersSpec struct {
	Netconf    NetconfListenerSpec  `json:"netconf"`
	Restconf   RestconfListenerSpec `json:"restconf"`
	Management MgmtListenerSpec     `json:"management"`
	CallHome   CallHomeSpec         `json:"callHome"`
	NetconfTLS NetconfTLSSpec       `json:"netconfTls"`
}

// NetconfListenerSpec is the SSH NETCONF data-plane listener.
type NetconfListenerSpec struct {
	Enabled     bool   `json:"enabled"`
	Address     string `json:"address"`
	HostKeyFile string `json:"hostKeyFile"`
}

// RestconfListenerSpec is the HTTP RESTCONF data-plane listener.
type RestconfListenerSpec struct {
	Enabled bool    `json:"enabled"`
	Address string  `json:"address"`
	TLS     TLSSpec `json:"tls"`
}

// TLSSpec is listeners.restconf.tls. enabled true is a 1.0 validate error.
type TLSSpec struct {
	Enabled bool `json:"enabled"`
}

// CallHomeSpec is listeners.callHome. enabled true is a 1.0 validate error.
type CallHomeSpec struct {
	Enabled bool `json:"enabled"`
}

// NetconfTLSSpec is listeners.netconfTls. enabled true is a 1.0 validate error.
type NetconfTLSSpec struct {
	Enabled bool `json:"enabled"`
}

// MgmtListenerSpec is the control-plane HTTP listener.
type MgmtListenerSpec struct {
	Address  string `json:"address"`
	RESTPath string `json:"restPath"`
	MCPPath  string `json:"mcpPath"`
}

// AuthSpec is spec.auth (not spec.management.auth).
type AuthSpec struct {
	Mode   string      `json:"mode"`
	Tokens []TokenSpec `json:"tokens"`
}

// TokenSpec is one static bearer principal. Secrets are file refs only.
type TokenSpec struct {
	ID         string   `json:"id"`
	Role       string   `json:"role"`
	SecretFile string   `json:"secretFile"`
	Scopes     []string `json:"scopes,omitempty"`
}

// UISpec is spec.ui.enabled.
type UISpec struct {
	Enabled bool `json:"enabled"`
}

// NetconfSpec is NETCONF protocol posture.
type NetconfSpec struct {
	Versions               []string          `json:"versions"`
	WritableRunning        bool              `json:"writableRunning"`
	Notifications          NotificationsSpec `json:"notifications"`
	SharedProfileDatastore bool              `json:"sharedProfileDatastore"`
}

// NotificationsSpec is RFC 5277 in-process config-change notifications.
type NotificationsSpec struct {
	Enabled bool `json:"enabled"`
}

// RestconfSpec is RESTCONF encoding posture. 1.0 is JSON only.
type RestconfSpec struct {
	JSON bool `json:"json"`
	XML  bool `json:"xml"`
}

// AdmissionSpec is data-plane client CIDRs for NETCONF and RESTCONF.
// Nil AllowClientCidrs means omitted (Normalize fills loopback).
// A non-nil empty slice is deny-all.
type AdmissionSpec struct {
	AllowClientCidrs []string `json:"allowClientCidrs"`
}

// ManagementSpec is origins and MCP knobs — not ui, not auth.
// spec.management.auth is unknown.
type ManagementSpec struct {
	AllowedOrigins []string `json:"allowedOrigins"`
	MCP            MCPSpec  `json:"mcp"`
}

// MCPSpec is management MCP adapter knobs.
type MCPSpec struct {
	AllowLegacyClients bool `json:"allowLegacyClients"`
}

// ProfileSpec is one named device profile (schema + instance tree).
type ProfileSpec struct {
	Name     string         `json:"name"`
	Modules  []ModuleSpec   `json:"modules,omitempty"`
	Schema   []SchemaLeaf   `json:"schema,omitempty"`
	Instance map[string]any `json:"instance"`
	Startup  map[string]any `json:"startup,omitempty"`
}

// ModuleSpec is a yang-library identifier. moduleFile/sourceFile are stored
// for later get-schema; 1.0 does not compile them.
type ModuleSpec struct {
	Name       string `json:"name"`
	Revision   string `json:"revision,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	ModuleFile string `json:"moduleFile,omitempty"`
	SourceFile string `json:"sourceFile,omitempty"`
}

// SchemaLeaf is one compact path → type entry.
type SchemaLeaf struct {
	Path      string `json:"path"`
	Type      string `json:"type"`
	Access    string `json:"access,omitempty"`
	Range     string `json:"range,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
	ValueFrom string `json:"valueFrom,omitempty"`
}

// UserSpec binds an SSH/RESTCONF user to exactly one profile.
// passwordFile and/or authorizedKeysFile; at least one path is required.
type UserSpec struct {
	Name               string `json:"name"`
	PasswordFile       string `json:"passwordFile,omitempty"`
	AuthorizedKeysFile string `json:"authorizedKeysFile,omitempty"`
	Profile            string `json:"profile"`
	Access             string `json:"access"`
}
