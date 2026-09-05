package app

import (
	"context"
	"os"

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
	defer s.mu.Unlock()

	prev := s.snaps.Load()
	gen := model.Generation(0)
	if prev != nil {
		gen = prev.Generation + 1
	}
	next, err := s.loadBootstrapCandidate(gen)
	if err != nil {
		return err
	}
	s.snaps.Swap(next)
	s.snaps.SetBootstrap(next)
	s.rebuildHandles(next)
	s.idemp.clear()
	if s.waiter != nil {
		s.waiter.Wipe()
	}
	return nil
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
	profiles := map[string]datastore.Handle{}
	users := map[string]datastore.Handle{}
	if snap == nil {
		s.profileHandles = profiles
		s.userHandles = users
		return
	}
	sink := s.sink
	shared := snap.SharedProfileStore
	for _, p := range snap.Profiles {
		profiles[p.Name] = datastore.New(p.Name, p.Running, yangtree.Node{}, p.Startup, sink)
	}
	for _, u := range snap.Users {
		p, ok := snap.ProfileNamed(u.Profile)
		if !ok {
			continue
		}
		if shared {
			users[u.Name] = profiles[u.Profile]
			continue
		}
		users[u.Name] = datastore.New(p.Name, p.Running, yangtree.Node{}, p.Startup, sink)
	}
	s.profileHandles = profiles
	s.userHandles = users
}
