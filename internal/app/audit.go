package app

import (
	"context"
	"time"

	"github.com/hilather/go-lab-netconf/internal/audit"
)

// Audit returns the process ring so adapters can share it as a Sink.
func (s *App) Audit() *audit.Fanout {
	if s == nil {
		return nil
	}
	return s.audit
}

func (s *App) recordAudit(ctx context.Context, ev audit.Event) string {
	if s == nil || s.audit == nil {
		return ""
	}
	if ev.Time.IsZero() {
		ev.Time = time.Now().UTC()
	}
	if ev.Result == "" {
		ev.Result = audit.ResultOK
	}
	if ev.ActorID == "" && ev.ActorClass == "" && ev.Transport == "" {
		actor := ActorFrom(ctx)
		ev.ActorID = actor.ID
		ev.ActorClass = actor.Class
		ev.Transport = actor.Transport
	}
	return s.audit.Record(ctx, ev).ID
}

func (s *App) QueryAudit(ctx context.Context, q AuditQuery) ([]AuditEvent, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	if s.audit == nil {
		return nil, nil
	}
	return s.audit.List(q.Limit), nil
}

func toAuditDiff(in []DiffEntry) []audit.RedactedEntry {
	if len(in) == 0 {
		return nil
	}
	out := make([]audit.RedactedEntry, len(in))
	for i, d := range in {
		out[i] = audit.RedactedEntry{Path: d.Path, Op: d.Op, Before: d.Before, After: d.After}
	}
	return out
}
