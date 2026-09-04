package datastore

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/yangtree"
)

func TestCommitMovesCandidateToRunning(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	if hostname(t, h, Running) != "lab-rtr-a" {
		t.Fatalf("running before edit = %q", hostname(t, h, Running))
	}
	mergeHostname(t, h, "after-commit")
	if hostname(t, h, Running) != "lab-rtr-a" {
		t.Fatalf("running changed before commit = %q", hostname(t, h, Running))
	}
	if hostname(t, h, Candidate) != "after-commit" {
		t.Fatalf("candidate = %q", hostname(t, h, Candidate))
	}
	if !h.Dirty() {
		t.Fatal("candidate should be dirty")
	}
	if err := h.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hostname(t, h, Running) != "after-commit" {
		t.Fatalf("running after commit = %q, want after-commit", hostname(t, h, Running))
	}
	if hostname(t, h, Candidate) != "after-commit" {
		t.Fatalf("candidate after commit = %q", hostname(t, h, Candidate))
	}
	if h.Dirty() {
		t.Fatal("dirty after commit")
	}
	mergeHostname(t, h, "next-candidate")
	if hostname(t, h, Running) != "after-commit" {
		t.Fatalf("running aliased candidate after commit = %q", hostname(t, h, Running))
	}
	if hostname(t, h, Candidate) != "next-candidate" {
		t.Fatalf("candidate after post-commit edit = %q", hostname(t, h, Candidate))
	}
}

func TestDiscardRestoresCandidateFromRunning(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	mergeHostname(t, h, "scratch")
	if hostname(t, h, Candidate) != "scratch" {
		t.Fatalf("candidate = %q", hostname(t, h, Candidate))
	}
	if hostname(t, h, Running) != "lab-rtr-a" {
		t.Fatalf("running = %q", hostname(t, h, Running))
	}
	if err := h.Discard(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hostname(t, h, Candidate) != "lab-rtr-a" {
		t.Fatalf("candidate after discard = %q, want lab-rtr-a", hostname(t, h, Candidate))
	}
	if hostname(t, h, Running) != "lab-rtr-a" {
		t.Fatalf("running after discard = %q", hostname(t, h, Running))
	}
	if h.Dirty() {
		t.Fatal("dirty after discard")
	}
	mergeHostname(t, h, "scratch-again")
	if hostname(t, h, Running) != "lab-rtr-a" {
		t.Fatalf("running aliased candidate after discard = %q", hostname(t, h, Running))
	}
	if hostname(t, h, Candidate) != "scratch-again" {
		t.Fatalf("candidate after post-discard edit = %q", hostname(t, h, Candidate))
	}
}

func TestCandidateIsolationTwoProfiles(t *testing.T) {
	a := newHandle(t, "router-a", "lab-rtr-a", nil)
	b := newHandle(t, "router-b", "lab-rtr-b", nil)
	mergeHostname(t, a, "from-a")
	if hostname(t, b, Candidate) != "lab-rtr-b" {
		t.Fatalf("b candidate leaked a's edit: %q", hostname(t, b, Candidate))
	}
	if hostname(t, b, Running) != "lab-rtr-b" {
		t.Fatalf("b running = %q", hostname(t, b, Running))
	}
	if hostname(t, a, Candidate) != "from-a" {
		t.Fatalf("a candidate = %q", hostname(t, a, Candidate))
	}
	if hostname(t, a, Running) != "lab-rtr-a" {
		t.Fatalf("a running changed without commit = %q", hostname(t, a, Running))
	}
}

func TestGetRunningUnchangedUntilCommit(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	mergeHostname(t, h, "pending")
	n, err := h.Get(context.Background(), Running, Subtree{})
	if err != nil {
		t.Fatal(err)
	}
	v, ok := n.Lookup(hostnamePath)
	if !ok || v != "lab-rtr-a" {
		t.Fatalf("GET running = %v, %v", v, ok)
	}
	if err := h.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	n, err = h.Get(context.Background(), Running, Subtree{})
	if err != nil {
		t.Fatal(err)
	}
	v, ok = n.Lookup(hostnamePath)
	if !ok || v != "pending" {
		t.Fatalf("GET running after commit = %v, %v", v, ok)
	}
}

func TestSharedProfileNameIsolatedWhenCopied(t *testing.T) {
	n := mustNode(t, "lab-rtr-a")
	h1 := New("router-a", n, n, n, nil)
	h2 := New("router-a", n, n, n, nil)
	if err := h1.Edit(context.Background(), Candidate, EditOp{Op: yangtree.OpMerge, Path: hostnamePath, Value: "user-1"}); err != nil {
		t.Fatal(err)
	}
	if hostname(t, h2, Candidate) != "lab-rtr-a" {
		t.Fatalf("shared profile name leaked candidate: %q", hostname(t, h2, Candidate))
	}
	if hostname(t, h1, Candidate) != "user-1" {
		t.Fatalf("h1 candidate = %q", hostname(t, h1, Candidate))
	}
	if hostname(t, h2, Running) != "lab-rtr-a" || hostname(t, h1, Running) != "lab-rtr-a" {
		t.Fatalf("running h1=%q h2=%q", hostname(t, h1, Running), hostname(t, h2, Running))
	}
}
