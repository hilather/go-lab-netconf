// Package notif is the in-process config-change notification port.
package notif

import (
	"context"
	"errors"
)

// ErrWaitTimeout is returned when Wait expires or when Nop.Wait is called.
var ErrWaitTimeout = errors.New("wait_timeout")

// ErrNotFound is returned when Get looks up an unknown notification id.
var ErrNotFound = errors.New("not_found")

// Change is a config-change leaf recorded on commit.
type Change struct {
	Path      string
	Operation string
}

// Notification is one RFC 5277 config-change record.
type Notification struct {
	ID      string
	Profile string
	Changes []Change
}

// Query selects stored notifications.
type Query struct {
	Profile string
}

// WaitQuery selects a notification to wait for.
type WaitQuery struct {
	Profile string
}

// Sink is the production object cmd injects into datastore, SSH, and APP.
type Sink interface {
	OnCommit(profile string, changes []Change)
	Wipe()
}

// Waiter is the tester/control-plane port over the same object as Sink.
type Waiter interface {
	List(ctx context.Context, q Query) ([]Notification, error)
	Get(ctx context.Context, id string) (Notification, error)
	Wait(ctx context.Context, q WaitQuery) (Notification, error)
	Clear(ctx context.Context) error
	Wipe()
}

// Nop is a no-op Sink and Waiter. Wait returns wait_timeout.
type Nop struct{}

var (
	_ Sink   = Nop{}
	_ Waiter = Nop{}
)

// OnCommit discards the commit event.
func (Nop) OnCommit(string, []Change) {}

// Wipe is a no-op.
func (Nop) Wipe() {}

// List returns an empty set.
func (Nop) List(context.Context, Query) ([]Notification, error) {
	return nil, nil
}

// Get reports not_found; Nop stores nothing.
func (Nop) Get(context.Context, string) (Notification, error) {
	return Notification{}, ErrNotFound
}

// Wait returns wait_timeout without blocking.
func (Nop) Wait(ctx context.Context, _ WaitQuery) (Notification, error) {
	if err := ctx.Err(); err != nil {
		return Notification{}, err
	}
	return Notification{}, ErrWaitTimeout
}

// Clear is a no-op.
func (Nop) Clear(context.Context) error { return nil }
