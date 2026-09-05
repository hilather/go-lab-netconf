package app

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/hilather/go-lab-netconf/internal/audit"
	"github.com/hilather/go-lab-netconf/internal/datastore"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/notif"
	"github.com/hilather/go-lab-netconf/internal/yangtree"
)

func (s *App) GetDatastore(ctx context.Context, profile, store string) (json.RawMessage, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	h, err := s.handleForProfile(profile)
	if err != nil {
		return nil, err
	}
	name, err := storeName(store)
	if err != nil {
		return nil, err
	}
	n, err := h.Get(ctx, name, datastore.Subtree{})
	if err != nil {
		return nil, err
	}
	return marshalTree(n.Map())
}

func (s *App) SetDatastore(ctx context.Context, profile, store string, overlay json.RawMessage) error {
	if err := s.requireCtx(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	h, err := s.handleForProfile(profile)
	if err != nil {
		return err
	}
	name, err := storeName(store)
	if err != nil {
		return err
	}
	tree, err := decodeOverlay(overlay)
	if err != nil {
		return err
	}
	op := datastore.EditOp{Op: yangtree.OpMerge, Path: "", Value: tree}
	switch name {
	case datastore.Candidate:
		err = h.Edit(ctx, datastore.Candidate, op)
	case datastore.Running:
		err = h.WriteRunningIfCandidateClean(ctx, op)
	case datastore.Startup:
		err = setStartup(ctx, h, tree)
	default:
		return domainerr.ValidationFailed("unknown datastore")
	}
	if err != nil {
		return err
	}
	s.recordAudit(ctx, audit.Event{Capability: "datastore.set", Result: audit.ResultOK})
	return nil
}

func (s *App) Commit(ctx context.Context, profile string) error {
	if err := s.requireCtx(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	h, err := s.handleForProfile(profile)
	if err != nil {
		return err
	}
	if err := h.Commit(ctx); err != nil {
		return err
	}
	s.recordAudit(ctx, audit.Event{Capability: "datastore.commit", Result: audit.ResultOK})
	return nil
}

func (s *App) Discard(ctx context.Context, profile string) error {
	if err := s.requireCtx(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	h, err := s.handleForProfile(profile)
	if err != nil {
		return err
	}
	if err := h.Discard(ctx); err != nil {
		return err
	}
	s.recordAudit(ctx, audit.Event{Capability: "datastore.discard", Result: audit.ResultOK})
	return nil
}

func (s *App) PreviewGet(ctx context.Context, user, path string) (json.RawMessage, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if user == "" {
		return nil, domainerr.ValidationFailed("user is required",
			domainerr.FieldViolation{Path: "user", Code: "required", Message: "user is required"})
	}
	h, ok := s.userHandles[user]
	if !ok {
		return nil, domainerr.NotFound("user " + user + " not found")
	}
	n, err := h.Get(ctx, datastore.Running, datastore.Subtree{Path: path})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return marshalTree(n.Map())
	}
	v, ok := n.Lookup(path)
	if !ok {
		return json.RawMessage("{}"), nil
	}
	return marshalTree(v)
}

func (s *App) ListNotifications(ctx context.Context, q NotifQuery) ([]Notification, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	w := s.waiter
	s.mu.Unlock()
	if w == nil {
		return nil, nil
	}
	return w.List(ctx, q)
}

func (s *App) GetNotification(ctx context.Context, id string) (Notification, error) {
	if err := s.requireCtx(ctx); err != nil {
		return Notification{}, err
	}
	s.mu.Lock()
	w := s.waiter
	s.mu.Unlock()
	n, err := w.Get(ctx, id)
	if err != nil {
		return Notification{}, mapNotifErr(err)
	}
	return n, nil
}

func (s *App) WaitNotification(ctx context.Context, q WaitQuery) (Notification, error) {
	if err := s.requireCtx(ctx); err != nil {
		return Notification{}, err
	}
	s.mu.Lock()
	w := s.waiter
	s.mu.Unlock()
	n, err := w.Wait(ctx, q)
	if err != nil {
		return Notification{}, mapNotifErr(err)
	}
	return n, nil
}

func (s *App) ClearNotifications(ctx context.Context) error {
	if err := s.requireCtx(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	w := s.waiter
	s.mu.Unlock()
	if err := w.Clear(ctx); err != nil {
		return err
	}
	s.recordAudit(ctx, audit.Event{Capability: "notifications.clear", Result: audit.ResultOK})
	return nil
}

func (s *App) handleForProfile(name string) (datastore.Handle, error) {
	if name == "" {
		return nil, domainerr.ValidationFailed("profile is required",
			domainerr.FieldViolation{Path: "profile", Code: "required", Message: "profile is required"})
	}
	h, ok := s.profileHandles[name]
	if !ok {
		return nil, domainerr.NotFound("profile " + name + " not found")
	}
	return h, nil
}

func storeName(store string) (datastore.Name, error) {
	switch datastore.Name(store) {
	case datastore.Running, datastore.Candidate, datastore.Startup:
		return datastore.Name(store), nil
	default:
		return "", domainerr.ValidationFailed("unknown datastore",
			domainerr.FieldViolation{Path: "store", Code: "invalid_value", Message: "store must be running, candidate, or startup"})
	}
}

func decodeOverlay(overlay json.RawMessage) (map[string]any, error) {
	if len(overlay) == 0 {
		return nil, domainerr.ValidationFailed("overlay is required",
			domainerr.FieldViolation{Path: "overlay", Code: "required", Message: "overlay is required"})
	}
	var tree map[string]any
	if err := json.Unmarshal(overlay, &tree); err != nil {
		return nil, domainerr.ValidationFailed("overlay must be a JSON object",
			domainerr.FieldViolation{Path: "overlay", Code: "invalid_value", Message: err.Error()})
	}
	if tree == nil {
		tree = map[string]any{}
	}
	return tree, nil
}

func setStartup(ctx context.Context, h datastore.Handle, overlay map[string]any) error {
	cur, err := h.Get(ctx, datastore.Startup, datastore.Subtree{})
	if err != nil {
		return err
	}
	tmp := cur.Clone()
	if err := tmp.Apply(yangtree.OpMerge, "", overlay); err != nil {
		return err
	}
	prev, err := h.Get(ctx, datastore.Candidate, datastore.Subtree{})
	if err != nil {
		return err
	}
	if err := h.Edit(ctx, datastore.Candidate, datastore.EditOp{Op: yangtree.OpReplace, Path: "", Value: tmp.Map()}); err != nil {
		return err
	}
	if err := h.Copy(ctx, datastore.Candidate, datastore.Startup); err != nil {
		_ = h.Edit(ctx, datastore.Candidate, datastore.EditOp{Op: yangtree.OpReplace, Path: "", Value: prev.Map()})
		return err
	}
	return h.Edit(ctx, datastore.Candidate, datastore.EditOp{Op: yangtree.OpReplace, Path: "", Value: prev.Map()})
}

func marshalTree(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, domainerr.ValidationFailed("marshal tree: " + err.Error())
	}
	return b, nil
}

func mapNotifErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, notif.ErrWaitTimeout) {
		return domainerr.WaitTimeout("wait_timeout")
	}
	if errors.Is(err, notif.ErrNotFound) {
		return domainerr.NotFound("notification not found")
	}
	return asDomain(err)
}
