package datastore

import (
	"context"
	"sync"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/notif"
	"github.com/hilather/go-lab-netconf/internal/yangtree"
)

// Name is a datastore identity.
type Name string

const (
	Running   Name = "running"
	Candidate Name = "candidate"
	Startup   Name = "startup"
)

// Node is a path-addressable configuration tree.
type Node = yangtree.Node

// Subtree selects a path for Get. An empty Path is the whole store.
type Subtree struct {
	Path string
}

// EditOp is one merge, replace, or delete against a store.
type EditOp struct {
	Op    yangtree.Operation
	Path  string
	Value any
}

// CommitSink receives config-change events after a successful Commit.
// notif.Sink satisfies this interface. sink may be nil on New.
type CommitSink interface {
	OnCommit(profile string, changes []notif.Change)
}

var (
	_ CommitSink = (notif.Sink)(nil)
	_ CommitSink = notif.Nop{}
)

// Handle is one profile-instance's running/candidate/startup triple.
type Handle interface {
	Get(ctx context.Context, store Name, filter Subtree) (Node, error)
	// Edit applies op to candidate (NETCONF edit-config).
	Edit(ctx context.Context, store Name, op EditOp) error
	// WriteRunningIfCandidateClean applies op to running and copy-forwards
	// onto candidate when candidate is clean and unlocked; otherwise
	// candidate_dirty.
	WriteRunningIfCandidateClean(ctx context.Context, op EditOp) error
	Copy(ctx context.Context, src, dst Name) error
	Delete(ctx context.Context, store Name) error
	// Commit publishes candidate to running, increments storeGeneration,
	// then calls the constructor sink's OnCommit if sink != nil.
	Commit(ctx context.Context) error
	Discard(ctx context.Context) error
	Lock(ctx context.Context, store Name, sessionID string) error
	Unlock(ctx context.Context, store Name, sessionID string) error
	Dirty() bool
	Locked(store Name) (sessionID string, held bool)
	Generation() uint64
}

type handle struct {
	mu        sync.Mutex
	profile   string
	running   Node
	candidate Node
	startup   Node
	sink      CommitSink
	gen       uint64
	locks     map[Name]string
}

type sessionKey struct{}

// WithSessionID annotates ctx with the NETCONF session performing the call.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, sessionKey{}, sessionID)
}

func sessionIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	s, _ := ctx.Value(sessionKey{}).(string)
	return s
}

// New builds a per-profile-instance Handle. Each tree is cloned.
// A zero candidate or startup is filled from running. sink may be nil.
func New(profile string, running, candidate, startup Node, sink CommitSink) Handle {
	r := running.Clone()
	c := candidate
	if c.IsZero() {
		c = r.Clone()
	} else {
		c = c.Clone()
	}
	s := startup
	if s.IsZero() {
		s = r.Clone()
	} else {
		s = s.Clone()
	}
	return &handle{
		profile:   profile,
		running:   r,
		candidate: c,
		startup:   s,
		sink:      sink,
		locks:     map[Name]string{},
	}
}

func (h *handle) Get(ctx context.Context, store Name, filter Subtree) (Node, error) {
	if err := ctx.Err(); err != nil {
		return Node{}, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	n, err := h.store(store)
	if err != nil {
		return Node{}, err
	}
	p, err := yangtree.ParsePath(filter.Path)
	if err != nil {
		return Node{}, err
	}
	return n.Subtree(p), nil
}

func (h *handle) Edit(ctx context.Context, store Name, op EditOp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if store != Candidate {
		return domainerr.NotWritable("edit-config target must be candidate")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.denyIfLocked(store, sessionIDFrom(ctx)); err != nil {
		return err
	}
	return h.candidate.Apply(op.Op, op.Path, op.Value)
}

func (h *handle) WriteRunningIfCandidateClean(ctx context.Context, op EditOp) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.dirty() {
		return domainerr.CandidateDirty("candidate is dirty")
	}
	if _, held := h.locks[Candidate]; held {
		return domainerr.CandidateDirty("candidate is locked")
	}
	r := h.running.Clone()
	c := h.candidate.Clone()
	if err := r.Apply(op.Op, op.Path, op.Value); err != nil {
		return err
	}
	if err := c.Apply(op.Op, op.Path, op.Value); err != nil {
		return err
	}
	h.running = r
	h.candidate = c
	return nil
}

func (h *handle) Copy(ctx context.Context, src, dst Name) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	from, err := h.store(src)
	if err != nil {
		return err
	}
	if _, err := h.store(dst); err != nil {
		return err
	}
	if src == dst {
		return nil
	}
	if err := h.denyIfLocked(dst, sessionIDFrom(ctx)); err != nil {
		return err
	}
	cloned := from.Clone()
	switch dst {
	case Running:
		h.running = cloned
		h.gen++
	case Candidate:
		h.candidate = cloned
	case Startup:
		h.startup = cloned
		h.gen++
	}
	return nil
}

func (h *handle) Delete(ctx context.Context, store Name) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if store != Startup {
		return domainerr.NotWritable("delete-config target must be startup")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.denyIfLocked(store, sessionIDFrom(ctx)); err != nil {
		return err
	}
	cleared := h.startup.Clone()
	if err := cleared.Apply(yangtree.OpReplace, "", map[string]any{}); err != nil {
		return err
	}
	h.startup = cleared
	return nil
}

func (h *handle) Commit(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	h.mu.Lock()
	if err := h.denyIfLocked(Candidate, sessionIDFrom(ctx)); err != nil {
		h.mu.Unlock()
		return err
	}
	if err := h.denyIfLocked(Running, sessionIDFrom(ctx)); err != nil {
		h.mu.Unlock()
		return err
	}
	if err := h.candidate.Validate(); err != nil {
		h.mu.Unlock()
		return err
	}
	changes := toNotif(h.running.Diff(h.candidate))
	h.running = h.candidate.Clone()
	h.gen++
	profile := h.profile
	sink := h.sink
	h.mu.Unlock()
	if sink != nil {
		sink.OnCommit(profile, changes)
	}
	return nil
}

func (h *handle) Discard(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.denyIfLocked(Candidate, sessionIDFrom(ctx)); err != nil {
		return err
	}
	h.candidate = h.running.Clone()
	return nil
}

func (h *handle) Lock(ctx context.Context, store Name, sessionID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if sessionID == "" {
		return domainerr.ValidationFailed("session id is required to lock")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, err := h.store(store); err != nil {
		return err
	}
	holder, held := h.locks[store]
	if held && holder != sessionID {
		return domainerr.LockDenied("datastore is locked by session " + holder)
	}
	h.locks[store] = sessionID
	return nil
}

func (h *handle) Unlock(ctx context.Context, store Name, sessionID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, err := h.store(store); err != nil {
		return err
	}
	holder, held := h.locks[store]
	if !held {
		return domainerr.LockDenied("datastore is not locked")
	}
	if holder != sessionID {
		return domainerr.LockDenied("datastore is locked by session " + holder)
	}
	delete(h.locks, store)
	return nil
}

func (h *handle) Dirty() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.dirty()
}

func (h *handle) dirty() bool {
	return !h.running.Equal(h.candidate)
}

func (h *handle) Locked(store Name) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	id, ok := h.locks[store]
	return id, ok
}

func (h *handle) Generation() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.gen
}

func (h *handle) store(name Name) (Node, error) {
	switch name {
	case Running:
		return h.running, nil
	case Candidate:
		return h.candidate, nil
	case Startup:
		return h.startup, nil
	default:
		return Node{}, domainerr.ValidationFailed("unknown datastore")
	}
}

func (h *handle) denyIfLocked(store Name, sessionID string) error {
	holder, held := h.locks[store]
	if !held {
		return nil
	}
	if sessionID != "" && holder == sessionID {
		return nil
	}
	return domainerr.LockDenied("datastore is locked by session " + holder)
}

func toNotif(in []yangtree.Change) []notif.Change {
	if len(in) == 0 {
		return nil
	}
	out := make([]notif.Change, len(in))
	for i, c := range in {
		out[i] = notif.Change{Path: c.Path, Operation: c.Operation}
	}
	return out
}
