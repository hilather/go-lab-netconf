package notif

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSoakWaitCommitRace(t *testing.T) {
	r := New(64)
	const n = 32
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			got, err := r.Wait(ctx, WaitQuery{Profile: "router-a"})
			if err != nil {
				errCh <- err
				return
			}
			if got.ID == "" || got.Stream != StreamNETCONF {
				errCh <- err
			}
		}()
	}
	time.Sleep(20 * time.Millisecond)
	for i := 0; i < n; i++ {
		r.OnCommit("router-a", []Change{{Path: hostnamePath, Operation: "replace"}})
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	list, err := r.List(context.Background(), Query{Profile: "router-a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatal("ring empty after soak commits")
	}
}
