package app

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/capabilities"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
)

type fakeSessions struct {
	rows   []Session
	killed []string
}

func (f *fakeSessions) ListSessions() []Session {
	return append([]Session(nil), f.rows...)
}

func (f *fakeSessions) KillSession(id string) bool {
	for i, row := range f.rows {
		if row.ID != id {
			continue
		}
		f.killed = append(f.killed, id)
		f.rows = append(f.rows[:i], f.rows[i+1:]...)
		return true
	}
	return false
}

func TestSessionTableNilMatchesStub(t *testing.T) {
	svc, _ := mustBoot(t)
	list, err := svc.ListSessions(context.Background())
	if err != nil || list != nil {
		t.Fatalf("nil table list = %#v %v, want nil slice", list, err)
	}
	requireCode(t, svc.KillSession(context.Background(), "1"), domainerr.CodeNotFound)
	requireCode(t, svc.KillSession(context.Background(), ""), domainerr.CodeValidationFailed)
}

func TestSessionTableListSortsAndKillAudits(t *testing.T) {
	svc, _ := mustBoot(t)
	fake := &fakeSessions{rows: []Session{
		{ID: "10", User: "b", Profile: "p"},
		{ID: "x", User: "c", Profile: "p"},
		{ID: "2", User: "a", Profile: "p"},
	}}
	svc.SetSessionTable(fake)
	list, err := svc.ListSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].ID != "2" || list[1].ID != "10" || list[2].ID != "x" {
		t.Fatalf("sorted list = %+v", list)
	}

	empty := &fakeSessions{}
	svc.SetSessionTable(empty)
	list, err = svc.ListSessions(context.Background())
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("empty table list = %#v %v, want non-nil empty", list, err)
	}

	svc.SetSessionTable(fake)
	ctx := WithActor(context.Background(), Actor{ID: "tok-admin", Class: "token", Transport: "rest"})
	if err := svc.KillSession(ctx, "2"); err != nil {
		t.Fatal(err)
	}
	if len(fake.killed) != 1 || fake.killed[0] != "2" {
		t.Fatalf("killed = %+v", fake.killed)
	}
	events, err := svc.QueryAudit(ctx, AuditQuery{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	var hits int
	for _, ev := range events {
		if ev.Capability != string(capabilities.SessionKill) {
			continue
		}
		hits++
		if ev.Reason != "2" || ev.Result != "ok" || ev.ActorID != "tok-admin" || ev.ActorClass != "token" || ev.Transport != "rest" {
			t.Fatalf("audit = %+v", ev)
		}
	}
	if hits != 1 {
		t.Fatalf("session.kill events = %d", hits)
	}
	requireCode(t, svc.KillSession(ctx, "missing"), domainerr.CodeNotFound)
	events, err = svc.QueryAudit(ctx, AuditQuery{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	hits = 0
	for _, ev := range events {
		if ev.Capability == string(capabilities.SessionKill) {
			hits++
		}
	}
	if hits != 1 {
		t.Fatalf("failed kill recorded audit, session.kill events = %d", hits)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.ListSessions(canceled); err == nil {
		t.Fatal("canceled list")
	}
	if err := svc.KillSession(canceled, "10"); err == nil {
		t.Fatal("canceled kill")
	}
	if len(fake.killed) != 1 {
		t.Fatalf("canceled kill touched the table: %+v", fake.killed)
	}

	svc.SetSessionTable(nil)
	list, err = svc.ListSessions(context.Background())
	if err != nil || list != nil {
		t.Fatalf("cleared table list = %#v %v", list, err)
	}
	requireCode(t, svc.KillSession(context.Background(), "10"), domainerr.CodeNotFound)
}
