package app

import (
	"context"
	"sync"

	"github.com/hilather/go-lab-netconf/internal/compiler"
	"github.com/hilather/go-lab-netconf/internal/config"
	"github.com/hilather/go-lab-netconf/internal/datastore"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/notif"
	"github.com/hilather/go-lab-netconf/internal/observability"
	"github.com/hilather/go-lab-netconf/internal/snapshot"
)

const defaultIdempotencyMax = 256

// Options constructs an App.
type Options struct {
	Snapshots      *snapshot.Store
	BootstrapPath  string
	Sink           notif.Sink
	Waiter         notif.Waiter
	IdempotencyMax int
	Metrics        *observability.Registry
	Logger         *observability.Logger
}

// App is the process-local Service implementation.
type App struct {
	mu            sync.Mutex
	snaps         *snapshot.Store
	bootstrapPath string
	sink          notif.Sink
	waiter        notif.Waiter
	idemp         *idempCache
	metrics       *observability.Registry
	logger        *observability.Logger

	profileHandles map[string]datastore.Handle
	userHandles    map[string]datastore.Handle
}

var _ Service = (*App)(nil)

// New returns an App. A nil Snapshots becomes an empty snapshot.Store.
// A nil Sink/Waiter becomes notif.Nop. cmd passes one object into both.
func New(opts Options) *App {
	if opts.Snapshots == nil {
		opts.Snapshots = snapshot.NewStore()
	}
	sink, waiter := opts.Sink, opts.Waiter
	if sink == nil && waiter == nil {
		n := notif.Nop{}
		sink, waiter = n, n
	} else if sink == nil {
		sink = notif.Nop{}
	} else if waiter == nil {
		if w, ok := sink.(notif.Waiter); ok {
			waiter = w
		} else {
			waiter = notif.Nop{}
		}
	}
	idempMax := opts.IdempotencyMax
	if idempMax <= 0 {
		idempMax = defaultIdempotencyMax
	}
	a := &App{
		snaps:          opts.Snapshots,
		bootstrapPath:  opts.BootstrapPath,
		sink:           sink,
		waiter:         waiter,
		idemp:          newIdempCache(idempMax),
		metrics:        opts.Metrics,
		logger:         opts.Logger,
		profileHandles: map[string]datastore.Handle{},
		userHandles:    map[string]datastore.Handle{},
	}
	if snap := a.snaps.Load(); snap != nil {
		a.rebuildHandles(snap)
	}
	if a.metrics != nil {
		a.metrics.OnScrape(func() { a.observeGauges() })
		a.observeGauges()
	}
	return a
}

// Boot loads bootstrap YAML, compiles a snapshot, and installs it.
func Boot(ctx context.Context, opts Options) (*App, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if opts.BootstrapPath == "" {
		return nil, domainerr.ValidationFailed("bootstrap path is required",
			domainerr.FieldViolation{Path: "bootstrapPath", Code: "required", Message: "bootstrap path is required"})
	}
	st, err := config.LoadFile(opts.BootstrapPath)
	if err != nil {
		return nil, asDomain(err)
	}
	snap, err := compiler.Compile(st, compiler.CompileOpts{})
	if err != nil {
		return nil, asDomain(err)
	}
	if opts.Snapshots == nil {
		opts.Snapshots = snapshot.NewStore()
	}
	opts.Snapshots.InstallBootstrap(snap)
	return New(opts), nil
}

// Snapshots is the live config pointer.
func (s *App) Snapshots() *snapshot.Store {
	if s == nil {
		return nil
	}
	return s.snaps
}

// Active is the live snapshot, or nil.
func (s *App) Active() *snapshot.Snapshot {
	if s == nil || s.snaps == nil {
		return nil
	}
	return s.snaps.Load()
}

// Datastore returns the control-plane handle for a profile name.
func (s *App) Datastore(profile string) (datastore.Handle, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.profileHandles[profile]
	return h, ok
}

// UserDatastore returns the per-user handle (copy-on-compile unless shared).
func (s *App) UserDatastore(user string) (datastore.Handle, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.userHandles[user]
	return h, ok
}

func (s *App) requireCtx(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func (s *App) active() (*snapshot.Snapshot, error) {
	if s == nil || s.snaps == nil {
		return nil, domainerr.ValidationFailed("no snapshot store")
	}
	snap := s.snaps.Load()
	if snap == nil {
		return nil, domainerr.ValidationFailed("no active snapshot")
	}
	return snap, nil
}

func asDomain(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := domainerr.As(err); ok {
		return err
	}
	return domainerr.ValidationFailed(err.Error())
}

func (s *App) observeApply(result string) {
	if s == nil {
		return
	}
	observability.ObserveApply(s.metrics, result)
}

func (s *App) observeGauges() {
	if s == nil || s.metrics == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[datastore.Handle]struct{}{}
	locks := 0
	for _, h := range s.profileHandles {
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		locks += datastore.LockCount(h)
	}
	for _, h := range s.userHandles {
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		locks += datastore.LockCount(h)
	}
	observability.SetLocks(s.metrics, locks)
	n := 0
	if s.waiter != nil {
		list, err := s.waiter.List(context.Background(), notif.Query{})
		if err == nil {
			n = len(list)
		}
	}
	observability.SetNotifications(s.metrics, n)
}
