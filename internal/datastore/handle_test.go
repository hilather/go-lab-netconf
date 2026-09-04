package datastore

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
	"github.com/hilather/go-lab-netconf/internal/notif"
	"github.com/hilather/go-lab-netconf/internal/yangtree"
)

func TestLockDenied(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	if err := h.Lock(context.Background(), Candidate, "sess-a"); err != nil {
		t.Fatal(err)
	}
	id, held := h.Locked(Candidate)
	if !held || id != "sess-a" {
		t.Fatalf("Locked = %q, %v", id, held)
	}
	err := h.Lock(context.Background(), Candidate, "sess-b")
	requireCode(t, err, domainerr.CodeLockDenied)
	id, held = h.Locked(Candidate)
	if !held || id != "sess-a" {
		t.Fatalf("lock holder changed: %q, %v", id, held)
	}
	if err := h.Lock(context.Background(), Candidate, "sess-a"); err != nil {
		t.Fatal(err)
	}
	err = h.Unlock(context.Background(), Candidate, "sess-b")
	requireCode(t, err, domainerr.CodeLockDenied)
	if err := h.Unlock(context.Background(), Candidate, "sess-a"); err != nil {
		t.Fatal(err)
	}
	if _, held := h.Locked(Candidate); held {
		t.Fatal("still locked")
	}
}

func TestLockHolderCanEditOthersCannot(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	if err := h.Lock(context.Background(), Candidate, "sess-a"); err != nil {
		t.Fatal(err)
	}
	err := h.Edit(context.Background(), Candidate, EditOp{Op: yangtree.OpMerge, Path: hostnamePath, Value: "no-session"})
	requireCode(t, err, domainerr.CodeLockDenied)
	if hostname(t, h, Candidate) != "lab-rtr-a" {
		t.Fatalf("unlocked edit mutated candidate = %q", hostname(t, h, Candidate))
	}
	ctxA := WithSessionID(context.Background(), "sess-a")
	if err := h.Edit(ctxA, Candidate, EditOp{Op: yangtree.OpMerge, Path: hostnamePath, Value: "from-holder"}); err != nil {
		t.Fatal(err)
	}
	if hostname(t, h, Candidate) != "from-holder" {
		t.Fatalf("lock holder edit = %q", hostname(t, h, Candidate))
	}
	ctxB := WithSessionID(context.Background(), "sess-b")
	err = h.Edit(ctxB, Candidate, EditOp{Op: yangtree.OpMerge, Path: hostnamePath, Value: "from-other"})
	requireCode(t, err, domainerr.CodeLockDenied)
	if hostname(t, h, Candidate) != "from-holder" {
		t.Fatalf("other session mutated locked candidate = %q", hostname(t, h, Candidate))
	}
}

func TestTwoProfileInstancesLockCandidateIndependently(t *testing.T) {
	a := newHandle(t, "router-a", "lab-rtr-a", nil)
	b := newHandle(t, "router-b", "lab-rtr-b", nil)
	if err := a.Lock(context.Background(), Candidate, "sess-a"); err != nil {
		t.Fatal(err)
	}
	if err := b.Lock(context.Background(), Candidate, "sess-b"); err != nil {
		t.Fatal(err)
	}
	idA, heldA := a.Locked(Candidate)
	idB, heldB := b.Locked(Candidate)
	if !heldA || idA != "sess-a" {
		t.Fatalf("a locked = %q, %v", idA, heldA)
	}
	if !heldB || idB != "sess-b" {
		t.Fatalf("b locked = %q, %v", idB, heldB)
	}
}

func TestUnknownPathEmptyOnGetErrorOnEdit(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	n, err := h.Get(context.Background(), Running, Subtree{Path: "ietf-system:system/nope"})
	if err != nil {
		t.Fatal(err)
	}
	if !n.IsEmpty() {
		t.Fatalf("unknown get = %#v", n.Map())
	}
	err = h.Edit(context.Background(), Candidate, EditOp{Op: yangtree.OpMerge, Path: "ietf-system:system/nope", Value: "x"})
	requireCode(t, err, domainerr.CodeValidationFailed)
	if hostname(t, h, Candidate) != "lab-rtr-a" {
		t.Fatalf("candidate mutated on unknown edit = %q", hostname(t, h, Candidate))
	}
}

func TestStoreGenerationIncrementsOnCommitAndCopyConfig(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	g0 := h.Generation()
	mergeHostname(t, h, "pending")
	if h.Generation() != g0 {
		t.Fatalf("candidate-only edit incremented generation: %d -> %d", g0, h.Generation())
	}
	if err := h.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if h.Generation() != g0+1 {
		t.Fatalf("commit generation = %d, want %d", h.Generation(), g0+1)
	}
	if err := h.Copy(context.Background(), Running, Startup); err != nil {
		t.Fatal(err)
	}
	if h.Generation() != g0+2 {
		t.Fatalf("copy running→startup generation = %d, want %d", h.Generation(), g0+2)
	}
	mergeHostname(t, h, "again")
	if err := h.Copy(context.Background(), Candidate, Running); err != nil {
		t.Fatal(err)
	}
	if h.Generation() != g0+3 {
		t.Fatalf("copy candidate→running generation = %d, want %d", h.Generation(), g0+3)
	}
	if err := h.Copy(context.Background(), Running, Candidate); err != nil {
		t.Fatal(err)
	}
	if h.Generation() != g0+3 {
		t.Fatalf("copy running→candidate incremented generation: %d", h.Generation())
	}
}

func TestFailedValidateLeavesCandidateIntact(t *testing.T) {
	schema := []model.SchemaLeaf{
		{Path: hostnamePath, Type: "int32", Access: "write"},
	}
	n, err := yangtree.Compile(hostnameInstance("lab-rtr-a"), schema)
	if err != nil {
		t.Fatal(err)
	}
	h := New("router-a", n, n, n, nil)
	g0 := h.Generation()
	err = h.Commit(context.Background())
	requireCode(t, err, domainerr.CodeValidationFailed)
	if hostname(t, h, Candidate) != "lab-rtr-a" {
		t.Fatalf("candidate after failed validate = %q", hostname(t, h, Candidate))
	}
	if hostname(t, h, Running) != "lab-rtr-a" {
		t.Fatalf("running after failed validate = %q", hostname(t, h, Running))
	}
	if h.Generation() != g0 {
		t.Fatalf("generation changed on failed validate: %d", h.Generation())
	}
}

func TestWriteRunningIfCandidateClean(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	op := EditOp{Op: yangtree.OpMerge, Path: hostnamePath, Value: "restconf"}
	if err := h.WriteRunningIfCandidateClean(context.Background(), op); err != nil {
		t.Fatal(err)
	}
	if hostname(t, h, Running) != "restconf" || hostname(t, h, Candidate) != "restconf" {
		t.Fatalf("running=%q candidate=%q", hostname(t, h, Running), hostname(t, h, Candidate))
	}
	if h.Dirty() {
		t.Fatal("dirty after clean RESTCONF write")
	}

	mergeHostname(t, h, "netconf-pending")
	err := h.WriteRunningIfCandidateClean(context.Background(), EditOp{Op: yangtree.OpMerge, Path: hostnamePath, Value: "blocked"})
	requireCode(t, err, domainerr.CodeCandidateDirty)
	if hostname(t, h, Running) != "restconf" {
		t.Fatalf("running mutated while dirty = %q", hostname(t, h, Running))
	}
	if hostname(t, h, Candidate) != "netconf-pending" {
		t.Fatalf("candidate mutated while dirty RESTCONF = %q", hostname(t, h, Candidate))
	}

	if err := h.Discard(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := h.Lock(context.Background(), Candidate, "sess-a"); err != nil {
		t.Fatal(err)
	}
	err = h.WriteRunningIfCandidateClean(context.Background(), EditOp{Op: yangtree.OpMerge, Path: hostnamePath, Value: "locked"})
	requireCode(t, err, domainerr.CodeCandidateDirty)
	if hostname(t, h, Running) != "restconf" {
		t.Fatalf("running mutated while locked = %q", hostname(t, h, Running))
	}
}

func TestEditRunningRejected(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	err := h.Edit(context.Background(), Running, EditOp{Op: yangtree.OpMerge, Path: hostnamePath, Value: "nope"})
	requireCode(t, err, domainerr.CodeNotWritable)
	if hostname(t, h, Running) != "lab-rtr-a" {
		t.Fatalf("running = %q", hostname(t, h, Running))
	}
}

func TestDeleteConfigStartupOnly(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	err := h.Delete(context.Background(), Running)
	requireCode(t, err, domainerr.CodeNotWritable)
	if err := h.Delete(context.Background(), Startup); err != nil {
		t.Fatal(err)
	}
	n, err := h.Get(context.Background(), Startup, Subtree{})
	if err != nil {
		t.Fatal(err)
	}
	if hostname := func() string {
		v, ok := n.Lookup(hostnamePath)
		if !ok {
			return ""
		}
		s, _ := v.(string)
		return s
	}(); hostname != "" {
		t.Fatalf("startup after delete-config = %q", hostname)
	}
	if hostname(t, h, Running) != "lab-rtr-a" {
		t.Fatalf("running after delete-config startup = %q", hostname(t, h, Running))
	}
}

func TestCommitCallsSinkAfterPublish(t *testing.T) {
	s := &recordingSink{}
	h := newHandle(t, "router-a", "lab-rtr-a", s)
	s.h = h
	mergeHostname(t, h, "published")
	if err := h.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.calls != 1 {
		t.Fatalf("OnCommit calls = %d", s.calls)
	}
	if s.profile != "router-a" {
		t.Fatalf("profile = %q", s.profile)
	}
	if s.seenRunning != "published" {
		t.Fatalf("OnCommit saw running %q, want published (publish must precede sink)", s.seenRunning)
	}
	if len(s.changes) == 0 {
		t.Fatal("expected changes")
	}
}

func TestNilSinkCommit(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	mergeHostname(t, h, "ok")
	if err := h.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestNotifSinkSatisfiesCommitSink(t *testing.T) {
	var sink CommitSink = notif.Nop{}
	h := newHandle(t, "router-a", "lab-rtr-a", sink)
	mergeHostname(t, h, "via-nop")
	if err := h.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hostname(t, h, Running) != "via-nop" {
		t.Fatalf("running = %q", hostname(t, h, Running))
	}
}

func TestCopyConfigAndDiscardRestoreStartup(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	mergeHostname(t, h, "cand")
	if err := h.Copy(context.Background(), Candidate, Startup); err != nil {
		t.Fatal(err)
	}
	if hostname(t, h, Startup) != "cand" {
		t.Fatalf("startup = %q", hostname(t, h, Startup))
	}
	if hostname(t, h, Running) != "lab-rtr-a" {
		t.Fatalf("running after copy to startup = %q", hostname(t, h, Running))
	}
}

type recordingSink struct {
	h           Handle
	calls       int
	profile     string
	changes     []notif.Change
	seenRunning string
}

func (s *recordingSink) OnCommit(profile string, changes []notif.Change) {
	s.calls++
	s.profile = profile
	s.changes = append([]notif.Change(nil), changes...)
	if s.h == nil {
		return
	}
	n, err := s.h.Get(context.Background(), Running, Subtree{Path: hostnamePath})
	if err != nil {
		return
	}
	if v, ok := n.Lookup(hostnamePath); ok {
		s.seenRunning, _ = v.(string)
	}
}
