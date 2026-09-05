package notif

import (
	"context"
	"errors"
	"testing"
	"time"
)

const hostnamePath = "ietf-system:system/hostname"

func TestCommitThenWaitReturnsRecord(t *testing.T) {
	r := New(8)
	var sink Sink = r
	var waiter Waiter = r
	want := []Change{{Path: hostnamePath, Operation: "replace"}}
	sink.OnCommit("router-a", want)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := waiter.Wait(ctx, WaitQuery{Profile: "router-a"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == "" || got.Stream != StreamNETCONF || got.Profile != "router-a" {
		t.Fatalf("record = %+v", got)
	}
	if len(got.Changes) != 1 || got.Changes[0] != want[0] {
		t.Fatalf("changes = %+v, want %+v", got.Changes, want)
	}
}

func TestWaitExisting(t *testing.T) {
	r := New(8)
	want := []Change{{Path: hostnamePath, Operation: "replace"}}
	r.OnCommit("router-a", want)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := r.Wait(ctx, WaitQuery{Profile: "router-a"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == "" || got.Stream != StreamNETCONF || got.Profile != "router-a" {
		t.Fatalf("record = %+v", got)
	}
	if len(got.Changes) != 1 || got.Changes[0] != want[0] {
		t.Fatalf("changes = %+v, want %+v", got.Changes, want)
	}
	list, err := r.List(context.Background(), Query{Profile: "router-a"})
	if err != nil || len(list) != 1 || list[0].ID != got.ID {
		t.Fatalf("Wait consumed the record: list=%+v err=%v", list, err)
	}
}

func TestWaitExistingNewest(t *testing.T) {
	r := New(8)
	r.OnCommit("router-a", []Change{{Path: hostnamePath, Operation: "replace"}})
	r.OnCommit("router-a", []Change{{Path: hostnamePath, Operation: "merge"}})
	list, err := r.List(context.Background(), Query{})
	if err != nil || len(list) != 2 {
		t.Fatalf("List = %+v, %v", list, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := r.Wait(ctx, WaitQuery{Profile: "router-a"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != list[1].ID {
		t.Fatalf("Wait existing = %q, want newest %q", got.ID, list[1].ID)
	}
}

func TestWaitInserted(t *testing.T) {
	r := New(8)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	type result struct {
		n   Notification
		err error
	}
	ch := make(chan result, 1)
	go func() {
		n, err := r.Wait(ctx, WaitQuery{Profile: "router-a"})
		ch <- result{n, err}
	}()
	waitParked(t, r, 1)
	want := []Change{{Path: hostnamePath, Operation: "replace"}}
	r.OnCommit("router-a", want)
	got := <-ch
	if got.err != nil {
		t.Fatal(got.err)
	}
	if got.n.ID == "" || got.n.Profile != "router-a" || got.n.Stream != StreamNETCONF {
		t.Fatalf("inserted record = %+v", got.n)
	}
	if len(got.n.Changes) != 1 || got.n.Changes[0] != want[0] {
		t.Fatalf("changes = %+v, want %+v", got.n.Changes, want)
	}
	listed, err := r.Get(context.Background(), got.n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if listed.ID != got.n.ID {
		t.Fatalf("Get after Wait = %q, want %q", listed.ID, got.n.ID)
	}
}

func TestWaitTimeout(t *testing.T) {
	r := New(8)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := r.Wait(ctx, WaitQuery{Profile: "router-a"})
	if !errors.Is(err, ErrWaitTimeout) {
		t.Fatalf("Wait error = %v, want wait_timeout", err)
	}
	if time.Since(start) < 20*time.Millisecond {
		t.Fatalf("Wait returned before the deadline")
	}
	r.mu.Lock()
	n := len(r.waiters)
	r.mu.Unlock()
	if n != 0 {
		t.Fatalf("waiter leaked: %d", n)
	}
}

func TestWaitWipe(t *testing.T) {
	r := New(8)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ch := make(chan error, 1)
	go func() {
		_, err := r.Wait(ctx, WaitQuery{})
		ch <- err
	}()
	waitParked(t, r, 1)
	r.Wipe()
	err := <-ch
	if !errors.Is(err, ErrStoreWiped) {
		t.Fatalf("Wait error = %v, want store_wiped", err)
	}
	list, err := r.List(context.Background(), Query{})
	if err != nil || list != nil {
		t.Fatalf("List after Wipe = %v, %v", list, err)
	}
}

func TestWaitClearWipesWaiters(t *testing.T) {
	r := New(8)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ch := make(chan error, 1)
	go func() {
		_, err := r.Wait(ctx, WaitQuery{Profile: "router-a"})
		ch <- err
	}()
	waitParked(t, r, 1)
	if err := r.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
	err := <-ch
	if !errors.Is(err, ErrStoreWiped) {
		t.Fatalf("Wait error = %v, want store_wiped", err)
	}
}

func TestWaitAfterWipeParks(t *testing.T) {
	r := New(8)
	r.OnCommit("router-a", []Change{{Path: hostnamePath, Operation: "replace"}})
	r.Wipe()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	_, err := r.Wait(ctx, WaitQuery{})
	if !errors.Is(err, ErrWaitTimeout) {
		t.Fatalf("Wait after Wipe = %v, want wait_timeout (must park, not store_wiped)", err)
	}
}

func TestWaitProfileFilter(t *testing.T) {
	r := New(8)
	r.OnCommit("router-a", []Change{{Path: hostnamePath, Operation: "replace"}})
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	_, err := r.Wait(ctx, WaitQuery{Profile: "router-b"})
	if !errors.Is(err, ErrWaitTimeout) {
		t.Fatalf("Wait other profile = %v, want wait_timeout", err)
	}
}

func TestWaitWakesMatchingProfileOnly(t *testing.T) {
	r := New(8)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	aCh := make(chan error, 1)
	bCh := make(chan Notification, 1)
	go func() {
		_, err := r.Wait(ctx, WaitQuery{Profile: "router-a"})
		aCh <- err
	}()
	go func() {
		n, err := r.Wait(ctx, WaitQuery{Profile: "router-b"})
		if err != nil {
			bCh <- Notification{}
			return
		}
		bCh <- n
	}()
	waitParked(t, r, 2)
	r.OnCommit("router-b", []Change{{Path: hostnamePath, Operation: "replace"}})
	got := <-bCh
	if got.Profile != "router-b" || got.ID == "" {
		t.Fatalf("router-b Wait = %+v", got)
	}
	select {
	case err := <-aCh:
		t.Fatalf("router-a waiter returned early: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
}

func TestWaitHonorsCancel(t *testing.T) {
	r := New(8)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Wait(ctx, WaitQuery{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait error = %v, want context.Canceled", err)
	}
}

func TestRingEvictsOldest(t *testing.T) {
	r := New(2)
	r.OnCommit("a", []Change{{Path: hostnamePath, Operation: "replace"}})
	r.OnCommit("b", []Change{{Path: hostnamePath, Operation: "replace"}})
	first, err := r.List(context.Background(), Query{})
	if err != nil || len(first) != 2 {
		t.Fatalf("List = %+v, %v", first, err)
	}
	oldID := first[0].ID
	r.OnCommit("c", []Change{{Path: hostnamePath, Operation: "replace"}})
	_, err = r.Get(context.Background(), oldID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get evicted = %v, want not_found", err)
	}
	list, err := r.List(context.Background(), Query{})
	if err != nil || len(list) != 2 {
		t.Fatalf("List after evict = %+v, %v", list, err)
	}
	if list[0].Profile != "b" || list[1].Profile != "c" {
		t.Fatalf("order = %+v", list)
	}
}

func TestGetAndListFilter(t *testing.T) {
	r := New(8)
	r.OnCommit("router-a", []Change{{Path: hostnamePath, Operation: "replace"}})
	r.OnCommit("router-b", []Change{{Path: hostnamePath, Operation: "merge"}})
	list, err := r.List(context.Background(), Query{Profile: "router-a"})
	if err != nil || len(list) != 1 || list[0].Profile != "router-a" {
		t.Fatalf("filtered List = %+v, %v", list, err)
	}
	got, err := r.Get(context.Background(), list[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != list[0].ID {
		t.Fatalf("Get = %+v", got)
	}
	_, err = r.Get(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get missing = %v, want not_found", err)
	}
}

func TestULIDsUniqueAndStreamNETCONF(t *testing.T) {
	r := New(64)
	seen := map[string]bool{}
	for i := 0; i < 32; i++ {
		r.OnCommit("router-a", []Change{{Path: hostnamePath, Operation: "replace"}})
	}
	list, err := r.List(context.Background(), Query{})
	if err != nil || len(list) != 32 {
		t.Fatalf("List = %d, %v", len(list), err)
	}
	for _, n := range list {
		if n.Stream != StreamNETCONF {
			t.Fatalf("stream = %q", n.Stream)
		}
		if len(n.ID) != 26 {
			t.Fatalf("id %q length %d, want 26", n.ID, len(n.ID))
		}
		if seen[n.ID] {
			t.Fatalf("duplicate id %q", n.ID)
		}
		seen[n.ID] = true
	}
}

func TestReturnedChangesAreCopies(t *testing.T) {
	r := New(8)
	r.OnCommit("router-a", []Change{{Path: hostnamePath, Operation: "replace"}})
	list, err := r.List(context.Background(), Query{})
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %+v, %v", list, err)
	}
	list[0].Changes[0].Path = "mutated"
	got, err := r.Get(context.Background(), list[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Changes[0].Path != hostnamePath {
		t.Fatalf("store mutated via List copy: %q", got.Changes[0].Path)
	}
}

func TestNewClampsCapacity(t *testing.T) {
	if New(0).cap != defaultCapacity {
		t.Fatalf("default cap = %d", New(0).cap)
	}
	if New(-1).cap != defaultCapacity {
		t.Fatalf("negative cap = %d", New(-1).cap)
	}
	if New(10000).cap != maxCapacity {
		t.Fatalf("clamped cap = %d", New(10000).cap)
	}
}

func waitParked(t *testing.T, r *Ring, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		got := len(r.waiters)
		r.mu.Unlock()
		if got >= n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("Wait did not park %d waiter(s)", n)
}
