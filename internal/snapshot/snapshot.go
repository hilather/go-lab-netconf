package snapshot

import (
	"net/netip"
	"time"

	"github.com/hilather/go-lab-netconf/internal/model"
	"github.com/hilather/go-lab-netconf/internal/yangtree"
)

// Snapshot is immutable after Compile returns. Datastore handles are not
// stored here; they are process runtime rebuilt by app.
type Snapshot struct {
	Canonical         *model.State
	Revision          model.Revision
	BootstrapRevision model.Revision
	Generation        model.Generation
	CompiledAt        time.Time

	NetconfAddress     string
	RestconfAddress    string
	ManagementAddress  string
	RESTPath           string
	MCPPath            string
	HostKeyFile        string
	SharedProfileStore bool
	Versions           []string

	Allow        []netip.Prefix
	AllowDenyAll bool

	Profiles []Profile
	Users    []User
}

// Profile is one compiled device profile.
type Profile struct {
	Name    string
	Modules []model.ModuleSpec
	Schema  []model.SchemaLeaf
	Running yangtree.Node
	Startup yangtree.Node
}

// User binds a data-plane identity to a profile. Secret bytes are never loaded.
type User struct {
	Name               string
	Profile            string
	Access             string
	PasswordFile       string
	AuthorizedKeysFile string
}

// Drifted reports whether the live revision differs from bootstrap.
func (s *Snapshot) Drifted() bool {
	if s == nil {
		return false
	}
	return s.Revision != "" && s.BootstrapRevision != "" && s.Revision != s.BootstrapRevision
}

// Spec is the compiled canonical spec, or a zero spec.
func (s *Snapshot) Spec() model.Spec {
	if s == nil || s.Canonical == nil {
		return model.Spec{}
	}
	return s.Canonical.Spec
}

// ProfileNamed returns the compiled profile or false.
func (s *Snapshot) ProfileNamed(name string) (Profile, bool) {
	if s == nil {
		return Profile{}, false
	}
	for i := range s.Profiles {
		if s.Profiles[i].Name == name {
			return s.Profiles[i], true
		}
	}
	return Profile{}, false
}

// UserNamed returns the compiled user or false.
func (s *Snapshot) UserNamed(name string) (User, bool) {
	if s == nil {
		return User{}, false
	}
	for i := range s.Users {
		if s.Users[i].Name == name {
			return s.Users[i], true
		}
	}
	return User{}, false
}

// Allowed reports whether unmapped ip is in allowClientCidrs.
func (s *Snapshot) Allowed(ip netip.Addr) bool {
	if s == nil {
		return false
	}
	if s.AllowDenyAll {
		return false
	}
	if len(s.Allow) == 0 {
		return true
	}
	probe := ip.Unmap()
	for _, p := range s.Allow {
		if p.Contains(probe) || p.Contains(ip) {
			return true
		}
	}
	return false
}
