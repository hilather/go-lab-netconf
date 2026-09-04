package notif

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNopWaitTimeout(t *testing.T) {
	var n Nop
	got, err := n.Wait(context.Background(), WaitQuery{Profile: "router-a"})
	if !errors.Is(err, ErrWaitTimeout) {
		t.Fatalf("Wait error = %v, want wait_timeout", err)
	}
	if got.ID != "" {
		t.Fatalf("Wait notification = %+v, want empty", got)
	}
}

func TestNopWaitHonorsCancel(t *testing.T) {
	var n Nop
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := n.Wait(ctx, WaitQuery{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait error = %v, want context.Canceled", err)
	}
}

func TestNopGetNotFound(t *testing.T) {
	var n Nop
	_, err := n.Get(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get error = %v, want not_found", err)
	}
}

func TestNopListClearWipe(t *testing.T) {
	var n Nop
	list, err := n.List(context.Background(), Query{})
	if err != nil || list != nil {
		t.Fatalf("List = %v, %v", list, err)
	}
	if err := n.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
	n.OnCommit("router-a", []Change{{Path: "ietf-system:system/hostname", Operation: "replace"}})
	n.Wipe()
}

func TestNopWaitDeadline(t *testing.T) {
	var n Nop
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	_, err := n.Wait(ctx, WaitQuery{})
	if !errors.Is(err, ErrWaitTimeout) {
		t.Fatalf("Wait error = %v, want wait_timeout", err)
	}
}
