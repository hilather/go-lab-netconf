package app

import (
	"context"
	"os"

	"github.com/hilather/go-lab-netconf/internal/audit"
	"github.com/hilather/go-lab-netconf/internal/compiler"
	"github.com/hilather/go-lab-netconf/internal/config"
	"github.com/hilather/go-lab-netconf/internal/datastore"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
	"github.com/hilather/go-lab-netconf/internal/snapshot"
	"github.com/hilather/go-lab-netconf/internal/yangtree"
)

// Reset rereads the bootstrap mount, restores running/candidate/startup,
// drops locks, and wipes the notification log. It never writes the file.
func (s *App) Reset(ctx context.Context) error {
	if err := s.requireCtx(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	hooks, err := s.resetLocked(ctx)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	for _, fn := range hooks {
		fn()
	}
	return nil
}

func (s *App) resetLocked(ctx context.Context) ([]func(), error) {
	prev := s.snaps.Load()
	gen := model.Generation(0)
	if prev != nil {
		gen = prev.Generation + 1
	}
	next, err := s.loadBootstrapCandidate(gen)
	if err != nil {
		return nil, err
	}
	s.snaps.Swap(next)
	s.snaps.SetBootstrap(next)
	s.rebuildHandles(next)
	s.idemp.clear()
	if s.waiter != nil {
		s.waiter.Wipe()
	}
	s.recordAudit(ctx, audit.Event{
		Capability: "state.reset",
		Previous:   revisionOf(prev),
		Revision:   next.Revision,
		Result:     audit.ResultOK,
	})
	return append([]func(){}, s.resetHooks...), nil
}

func (s *App) loadBootstrapCandidate(gen model.Generation) (*snapshot.Snapshot, error) {
	if s.bootstrapPath != "" {
		if _, err := os.Stat(s.bootstrapPath); err != nil {
			if os.IsNotExist(err) {
				return nil, domainerr.ValidationFailed("bootstrap file unavailable",
					domainerr.FieldViolation{Path: "bootstrapPath", Code: "required", Message: "bootstrap file is missing; active snapshot unchanged"})
			}
			return nil, domainerr.ValidationFailed("stat bootstrap: " + err.Error())
		}
		st, err := config.LoadFile(s.bootstrapPath)
		if err != nil {
			return nil, asDomain(err)
		}
		snap, err := compiler.Compile(st, compiler.CompileOpts{Generation: gen})
		if err != nil {
			return nil, asDomain(err)
		}
		return snap, nil
	}
	boot := s.snaps.Bootstrap()
	if boot == nil || boot.Canonical == nil {
		return nil, domainerr.ValidationFailed("no bootstrap snapshot",
			domainerr.FieldViolation{Path: "bootstrap", Code: "required", Message: "no bootstrap path or snapshot to reset to"})
	}
	copied, err := cloneState(boot.Canonical)
	if err != nil {
		return nil, err
	}
	snap, err := compiler.Compile(copied, compiler.CompileOpts{Generation: gen})
	if err != nil {
		return nil, asDomain(err)
	}
	return snap, nil
}

func (s *App) rebuildHandles(snap *snapshot.Snapshot) {
	s.profileHandles = map[string]datastore.Handle{}
	s.userHandles = map[string]datastore.Handle{}
	s.syncHandles(nil, snap)
}

// syncHandles keeps live datastore triples whose compiled profile identity
// (instance, startup, schema) and user→profile binding did not change.
func (s *App) syncHandles(prev, next *snapshot.Snapshot) {
	if next == nil {
		s.profileHandles = map[string]datastore.Handle{}
		s.userHandles = map[string]datastore.Handle{}
		return
	}
	if prev != nil && prev.SharedProfileStore != next.SharedProfileStore {
		prev = nil
	}
	sink := s.sink
	profiles := make(map[string]datastore.Handle, len(next.Profiles))
	rebuilt := map[string]bool{}
	for _, p := range next.Profiles {
		if prev != nil && profileCompileEqual(prev, p) {
			if h, ok := s.profileHandles[p.Name]; ok {
				profiles[p.Name] = h
				continue
			}
		}
		profiles[p.Name] = datastore.New(p.Name, p.Running, yangtree.Node{}, p.Startup, sink)
		rebuilt[p.Name] = true
	}

	users := make(map[string]datastore.Handle, len(next.Users))
	shared := next.SharedProfileStore
	for _, u := range next.Users {
		p, ok := next.ProfileNamed(u.Profile)
		if !ok {
			continue
		}
		if shared {
			users[u.Name] = profiles[u.Profile]
			continue
		}
		prevUser, hadPrev := snapshot.User{}, false
		if prev != nil {
			prevUser, hadPrev = prev.UserNamed(u.Name)
		}
		if hadPrev && prevUser.Profile == u.Profile && !rebuilt[u.Profile] {
			if h, ok := s.userHandles[u.Name]; ok {
				users[u.Name] = h
				continue
			}
		}
		users[u.Name] = datastore.New(p.Name, p.Running, yangtree.Node{}, p.Startup, sink)
	}
	s.profileHandles = profiles
	s.userHandles = users
}

func profileCompileEqual(prev *snapshot.Snapshot, p snapshot.Profile) bool {
	old, ok := prev.ProfileNamed(p.Name)
	if !ok {
		return false
	}
	if !schemaEqual(old.Schema, p.Schema) {
		return false
	}
	if !old.Running.Equal(p.Running) {
		return false
	}
	if old.Startup.IsZero() != p.Startup.IsZero() {
		return false
	}
	if !old.Startup.IsZero() && !old.Startup.Equal(p.Startup) {
		return false
	}
	return true
}

func schemaEqual(a, b []model.SchemaLeaf) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
