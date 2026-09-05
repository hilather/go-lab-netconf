package app

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/datastore"
)

func TestResetRestoresHostnameAfterCommit(t *testing.T) {
	const bootstrapHost = "lab-rtr-a"
	const committedHost = "after-commit"

	svc, _ := mustBoot(t)
	ctx := context.Background()

	if got := hostname(t, svc, "router-a", datastore.Running); got != bootstrapHost {
		t.Fatalf("running hostname before edit = %q, want %q", got, bootstrapHost)
	}
	if got := hostname(t, svc, "router-a", datastore.Candidate); got != bootstrapHost {
		t.Fatalf("candidate hostname before edit = %q, want %q", got, bootstrapHost)
	}
	if got := hostname(t, svc, "router-a", datastore.Startup); got != bootstrapHost {
		t.Fatalf("startup hostname before edit = %q, want %q", got, bootstrapHost)
	}

	setHostname(t, svc, "router-a", "candidate", committedHost)
	if got := hostname(t, svc, "router-a", datastore.Running); got != bootstrapHost {
		t.Fatalf("running hostname changed before commit = %q, want %q", got, bootstrapHost)
	}
	if got := hostname(t, svc, "router-a", datastore.Candidate); got != committedHost {
		t.Fatalf("candidate hostname = %q, want %q", got, committedHost)
	}
	if err := svc.Commit(ctx, "router-a"); err != nil {
		t.Fatal(err)
	}
	if got := hostname(t, svc, "router-a", datastore.Running); got != committedHost {
		t.Fatalf("running hostname after commit = %q, want %q", got, committedHost)
	}
	if got := hostname(t, svc, "router-a", datastore.Candidate); got != committedHost {
		t.Fatalf("candidate hostname after commit = %q, want %q", got, committedHost)
	}

	h, ok := svc.Datastore("router-a")
	if !ok {
		t.Fatal("missing profile handle")
	}
	if err := h.Lock(ctx, datastore.Candidate, "sess-a"); err != nil {
		t.Fatal(err)
	}

	if err := svc.Reset(ctx); err != nil {
		t.Fatal(err)
	}

	if got := hostname(t, svc, "router-a", datastore.Running); got != bootstrapHost {
		t.Fatalf("running hostname after reset = %q, want %q", got, bootstrapHost)
	}
	if got := hostname(t, svc, "router-a", datastore.Candidate); got != bootstrapHost {
		t.Fatalf("candidate hostname after reset = %q, want %q", got, bootstrapHost)
	}
	if got := hostname(t, svc, "router-a", datastore.Startup); got != bootstrapHost {
		t.Fatalf("startup hostname after reset = %q, want %q", got, bootstrapHost)
	}

	h2, ok := svc.Datastore("router-a")
	if !ok {
		t.Fatal("missing profile handle after reset")
	}
	if _, held := h2.Locked(datastore.Candidate); held {
		t.Fatal("reset must drop locks")
	}
}
