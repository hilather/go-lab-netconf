package compiler

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/hilather/go-lab-netconf/internal/config"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
	"github.com/hilather/go-lab-netconf/internal/snapshot"
	"github.com/hilather/go-lab-netconf/internal/yangtree"
)

// CompileOpts controls revision metadata and the compile clock.
type CompileOpts struct {
	Now               time.Time
	BootstrapRevision model.Revision
	Generation        model.Generation
}

// Compile normalizes and validates st, compiles profile trees and users, and
// returns an immutable Snapshot. Secret bytes are never read.
func Compile(st *model.State, opts CompileOpts) (*snapshot.Snapshot, error) {
	n, err := config.Normalize(st)
	if err != nil {
		return nil, err
	}
	if err := config.Validate(n); err != nil {
		return nil, err
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	profiles, err := compileProfiles(n.Spec.Profiles)
	if err != nil {
		return nil, err
	}
	users := compileUsers(n.Spec.Users)
	allow, denyAll, err := compileAllow(n.Spec.Admission.AllowClientCidrs)
	if err != nil {
		return nil, err
	}

	rev, err := config.Revision(n)
	if err != nil {
		return nil, err
	}
	bootRev := opts.BootstrapRevision
	if bootRev == "" {
		bootRev = rev
	}

	return &snapshot.Snapshot{
		Canonical:          n,
		Revision:           rev,
		BootstrapRevision:  bootRev,
		Generation:         opts.Generation,
		CompiledAt:         now,
		NetconfAddress:     n.Spec.Listeners.Netconf.Address,
		RestconfAddress:    n.Spec.Listeners.Restconf.Address,
		ManagementAddress:  n.Spec.Listeners.Management.Address,
		RESTPath:           n.Spec.Listeners.Management.RESTPath,
		MCPPath:            n.Spec.Listeners.Management.MCPPath,
		HostKeyFile:        n.Spec.Listeners.Netconf.HostKeyFile,
		SharedProfileStore: n.Spec.Netconf.SharedProfileDatastore,
		Versions:           append([]string(nil), n.Spec.Netconf.Versions...),
		Allow:              allow,
		AllowDenyAll:       denyAll,
		Profiles:           profiles,
		Users:              users,
	}, nil
}

func compileProfiles(in []model.ProfileSpec) ([]snapshot.Profile, error) {
	out := make([]snapshot.Profile, 0, len(in))
	for _, p := range in {
		running, err := yangtree.Compile(p.Instance, p.Schema)
		if err != nil {
			return nil, err
		}
		if err := running.Validate(); err != nil {
			return nil, err
		}
		startup := yangtree.Node{}
		if p.Startup != nil {
			startup, err = yangtree.Compile(p.Startup, p.Schema)
			if err != nil {
				return nil, err
			}
			if err := startup.Validate(); err != nil {
				return nil, err
			}
		}
		out = append(out, snapshot.Profile{
			Name:    p.Name,
			Modules: append([]model.ModuleSpec(nil), p.Modules...),
			Schema:  append([]model.SchemaLeaf(nil), p.Schema...),
			Running: running,
			Startup: startup,
		})
	}
	return out, nil
}

func compileUsers(in []model.UserSpec) []snapshot.User {
	out := make([]snapshot.User, 0, len(in))
	for _, u := range in {
		out = append(out, snapshot.User{
			Name:               u.Name,
			Profile:            u.Profile,
			Access:             u.Access,
			PasswordFile:       u.PasswordFile,
			AuthorizedKeysFile: u.AuthorizedKeysFile,
		})
	}
	return out
}

func compileAllow(cidrs []string) ([]netip.Prefix, bool, error) {
	if cidrs != nil && len(cidrs) == 0 {
		return []netip.Prefix{}, true, nil
	}
	var out []netip.Prefix
	for i, c := range cidrs {
		p, err := netip.ParsePrefix(c)
		if err != nil {
			return nil, false, domainerr.ValidationFailed("invalid allowClientCidrs",
				domainerr.FieldViolation{Path: fmt.Sprintf("spec.admission.allowClientCidrs[%d]", i), Code: "invalid_value", Message: err.Error()})
		}
		out = append(out, p)
	}
	return out, false, nil
}
