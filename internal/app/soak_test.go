package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hilather/go-lab-netconf/internal/datastore"
	"github.com/hilather/go-lab-netconf/internal/notif"
	"github.com/hilather/go-lab-netconf/internal/yangtree"
)

func TestSoakCommitDiscardWait(t *testing.T) {
	ring := notif.New(64)
	path := copyFixture(t, "defaults.yaml")
	svc, err := Boot(context.Background(), Options{BootstrapPath: path, Sink: ring})
	if err != nil {
		t.Fatal(err)
	}
	h, ok := svc.Datastore("router-a")
	if !ok {
		t.Fatal("missing datastore")
	}
	const n = 16
	var wg sync.WaitGroup
	errCh := make(chan error, n*2)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, err := ring.Wait(ctx, notif.WaitQuery{Profile: "router-a"})
			if err != nil && err != notif.ErrWaitTimeout && err != context.DeadlineExceeded {
				errCh <- err
			}
		}()
	}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			if err := h.Edit(ctx, datastore.Candidate, datastore.EditOp{
				Op:    yangtree.OpMerge,
				Path:  "ietf-system:system/hostname",
				Value: "soak",
			}); err != nil {
				errCh <- err
				return
			}
			if err := h.Commit(ctx); err != nil {
				errCh <- err
				return
			}
			if err := h.Discard(ctx); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}
