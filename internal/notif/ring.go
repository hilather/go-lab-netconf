package notif

import (
	"context"
	"errors"
	"sync"

	"github.com/oklog/ulid/v2"
)

const (
	defaultCapacity = 256
	maxCapacity     = 4096
)

// Ring is a bounded in-process config-change log. It is memory-only:
// Wipe empties it; a process restart does the same. It never Dials.
type Ring struct {
	mu      sync.Mutex
	cap     int
	items   []Notification
	waiters []waiter
}

type waiter struct {
	q  WaitQuery
	ch chan waitResult
}

type waitResult struct {
	n   Notification
	err error
}

var (
	_ Sink   = (*Ring)(nil)
	_ Waiter = (*Ring)(nil)
)

// New returns a ring of capacity n. n <= 0 uses 256; n is clamped to 4096.
func New(n int) *Ring {
	if n <= 0 {
		n = defaultCapacity
	}
	if n > maxCapacity {
		n = maxCapacity
	}
	return &Ring{cap: n}
}

// OnCommit appends a stream NETCONF config-change record and wakes matching waiters.
func (r *Ring) OnCommit(profile string, changes []Change) {
	n := Notification{
		ID:      ulid.Make().String(),
		Stream:  StreamNETCONF,
		Profile: profile,
		Changes: cloneChanges(changes),
	}
	r.mu.Lock()
	if len(r.items) >= r.cap {
		copy(r.items, r.items[1:])
		r.items = r.items[:r.cap-1]
	}
	r.items = append(r.items, n)
	taken := r.takeWaitersLocked(n)
	r.mu.Unlock()
	for _, w := range taken {
		w.ch <- waitResult{n: cloneNotification(n)}
	}
}

// Wipe empties the ring and unblocks waiters with store_wiped.
func (r *Ring) Wipe() {
	r.broadcastWiped()
}

// Clear empties the ring. Parked waiters see store_wiped; a later Wait parks again.
func (r *Ring) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.broadcastWiped()
	return nil
}

// List returns matching records oldest-first.
func (r *Ring) List(ctx context.Context, q Query) ([]Notification, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Notification
	for _, n := range r.items {
		if match(WaitQuery(q), n) {
			out = append(out, cloneNotification(n))
		}
	}
	return out, nil
}

// Get returns one record by ULID.
func (r *Ring) Get(ctx context.Context, id string) (Notification, error) {
	if err := ctx.Err(); err != nil {
		return Notification{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, n := range r.items {
		if n.ID == id {
			return cloneNotification(n), nil
		}
	}
	return Notification{}, ErrNotFound
}

// Wait returns the newest matching record if one exists, otherwise parks
// until a matching insert, context deadline (wait_timeout), cancel, or wipe.
func (r *Ring) Wait(ctx context.Context, q WaitQuery) (Notification, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Notification{}, waitErr(err)
	}
	ch := make(chan waitResult, 1)
	r.mu.Lock()
	if n, ok := r.matchNewestLocked(q); ok {
		r.mu.Unlock()
		return n, nil
	}
	r.waiters = append(r.waiters, waiter{q: q, ch: ch})
	r.mu.Unlock()

	select {
	case res := <-ch:
		return res.n, res.err
	case <-ctx.Done():
		r.removeWaiter(ch)
		return Notification{}, waitErr(ctx.Err())
	}
}

func (r *Ring) broadcastWiped() {
	r.mu.Lock()
	r.items = r.items[:0]
	waiters := r.waiters
	r.waiters = nil
	r.mu.Unlock()
	for _, w := range waiters {
		w.ch <- waitResult{err: ErrStoreWiped}
	}
}

func (r *Ring) matchNewestLocked(q WaitQuery) (Notification, bool) {
	for i := len(r.items) - 1; i >= 0; i-- {
		if match(q, r.items[i]) {
			return cloneNotification(r.items[i]), true
		}
	}
	return Notification{}, false
}

func (r *Ring) takeWaitersLocked(n Notification) []waiter {
	if len(r.waiters) == 0 {
		return nil
	}
	var taken []waiter
	remaining := r.waiters[:0]
	for _, w := range r.waiters {
		if match(w.q, n) {
			taken = append(taken, w)
			continue
		}
		remaining = append(remaining, w)
	}
	for i := len(remaining); i < len(r.waiters); i++ {
		r.waiters[i] = waiter{}
	}
	r.waiters = remaining
	return taken
}

func (r *Ring) removeWaiter(ch chan waitResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	remaining := r.waiters[:0]
	for _, w := range r.waiters {
		if w.ch != ch {
			remaining = append(remaining, w)
		}
	}
	for i := len(remaining); i < len(r.waiters); i++ {
		r.waiters[i] = waiter{}
	}
	r.waiters = remaining
}

func match(q WaitQuery, n Notification) bool {
	return q.Profile == "" || q.Profile == n.Profile
}

func waitErr(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrWaitTimeout
	}
	return err
}

func cloneNotification(n Notification) Notification {
	n.Changes = cloneChanges(n.Changes)
	return n
}

func cloneChanges(in []Change) []Change {
	if in == nil {
		return nil
	}
	out := make([]Change, len(in))
	copy(out, in)
	return out
}
