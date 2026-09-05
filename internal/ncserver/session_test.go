package ncserver

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-netconf/internal/model"
	"github.com/hilather/go-lab-netconf/internal/ncrpc"
	"github.com/hilather/go-lab-netconf/internal/nctest"
	"github.com/hilather/go-lab-netconf/internal/notif"
	"github.com/hilather/go-lab-netconf/internal/observability"
)

func TestRPCAndSessionMetrics(t *testing.T) {
	reg := observability.NewRegistry()
	user := User{Name: "alice", Profile: "router-a"}
	user.Namespaces = map[string]string{"ietf-system": ietfSystemNS}
	user.Handle = mustHandle(t, user.Profile, "lab-rtr-a", nil)
	srv := New(Config{Users: []User{user}, Sink: notif.Nop{}, Metrics: reg})
	a, b := net.Pipe()
	t.Cleanup(func() {
		_ = a.Close()
		_ = b.Close()
	})
	errc := make(chan error, 1)
	go func() {
		errc <- srv.Serve(context.Background(), user.Name, "127.0.0.1:1", a)
	}()
	c := nctest.New(b)
	if _, err := c.Handshake(nil); err != nil {
		t.Fatal(err)
	}
	rep := mustRPC(t, c, ncrpc.RPC{MessageID: "1", Name: ncrpc.OpGetConfig, Source: "running"})
	if len(rep.Errors) != 0 {
		t.Fatalf("%+v", rep.Errors)
	}
	v, ok := reg.Get(observability.MetricRPCsTotal, map[string]string{"rpc": "get-config", "decision": "ok"})
	if !ok || v < 1 {
		t.Fatalf("rpc counter %v %v", v, ok)
	}
	sessions, _ := reg.Get(observability.MetricSessions, nil)
	if sessions != 1 {
		t.Fatalf("sessions %v", sessions)
	}
	_ = b.Close()
	select {
	case <-errc:
	case <-time.After(2 * time.Second):
		t.Fatal("session did not exit")
	}
	sessions, _ = reg.Get(observability.MetricSessions, nil)
	if sessions != 0 {
		t.Fatalf("sessions after close %v", sessions)
	}
}

func TestHelloGetConfigRunning(t *testing.T) {
	c, _, _ := startSession(t, User{Name: "alice", Profile: "router-a"})
	rep := mustRPC(t, c, ncrpc.RPC{MessageID: "102", Name: ncrpc.OpGetConfig, Source: "running"})
	if len(rep.Errors) != 0 {
		t.Fatalf("errors = %+v", rep.Errors)
	}
	if !bytes.Contains(rep.Data, []byte("lab-rtr-a")) {
		t.Fatalf("running data = %s", rep.Data)
	}
	if !bytes.Contains(rep.Data, []byte(ietfSystemNS)) {
		t.Fatalf("missing namespace: %s", rep.Data)
	}
	if bytes.Contains(rep.Data, []byte("writable-running")) {
		t.Fatalf("data advertised writable-running: %s", rep.Data)
	}
}

func TestHelloCapabilities(t *testing.T) {
	_, _, hello := startSession(t, User{Name: "alice", Profile: "router-a"})
	joined := strings.Join(hello.Capabilities, "\n")
	for _, c := range []string{
		"urn:ietf:params:netconf:base:1.0",
		"urn:ietf:params:netconf:base:1.1",
		"urn:ietf:params:netconf:capability:candidate:1.0",
		"urn:ietf:params:netconf:capability:startup:1.0",
		"urn:ietf:params:netconf:capability:validate:1.0",
		"urn:ietf:params:netconf:capability:notification:1.0",
	} {
		if !strings.Contains(joined, c) {
			t.Fatalf("hello missing %s: %v", c, hello.Capabilities)
		}
	}
	for _, bad := range []string{"writable-running", "xpath", "confirmed-commit"} {
		if strings.Contains(joined, bad) {
			t.Fatalf("hello contains %s: %v", bad, hello.Capabilities)
		}
	}
}

func TestEditConfigCandidateThenGetConfig(t *testing.T) {
	c, _, _ := startSession(t, User{Name: "alice", Profile: "router-a"})
	cfg := []byte(`<system xmlns="urn:ietf:params:xml:ns:yang:ietf-system"><hostname>after-edit</hostname></system>`)
	rep := mustRPC(t, c, ncrpc.RPC{
		MessageID: "103",
		Name:      ncrpc.OpEditConfig,
		Target:    "candidate",
		DefaultOp: "merge",
		Config:    cfg,
	})
	if !rep.OK {
		t.Fatalf("edit-config = %+v", rep)
	}
	running := mustRPC(t, c, ncrpc.RPC{MessageID: "104", Name: ncrpc.OpGetConfig, Source: "running"})
	if !bytes.Contains(running.Data, []byte("lab-rtr-a")) {
		t.Fatalf("running changed before commit: %s", running.Data)
	}
	cand := mustRPC(t, c, ncrpc.RPC{MessageID: "105", Name: ncrpc.OpGetConfig, Source: "candidate"})
	if !bytes.Contains(cand.Data, []byte("after-edit")) {
		t.Fatalf("candidate = %s", cand.Data)
	}
	commit := mustRPC(t, c, ncrpc.RPC{MessageID: "106", Name: ncrpc.OpCommit})
	if !commit.OK {
		t.Fatalf("commit = %+v", commit)
	}
	running = mustRPC(t, c, ncrpc.RPC{MessageID: "107", Name: ncrpc.OpGetConfig, Source: "running"})
	if !bytes.Contains(running.Data, []byte("after-edit")) {
		t.Fatalf("running after commit = %s", running.Data)
	}
}

func TestEditConfigRunningRejected(t *testing.T) {
	c, _, _ := startSession(t, User{Name: "alice", Profile: "router-a"})
	cfg := []byte(`<system xmlns="urn:ietf:params:xml:ns:yang:ietf-system"><hostname>nope</hostname></system>`)
	rep := mustRPC(t, c, ncrpc.RPC{
		MessageID: "1",
		Name:      ncrpc.OpEditConfig,
		Target:    "running",
		Config:    cfg,
	})
	if len(rep.Errors) == 0 || rep.Errors[0].Tag != "operation-not-supported" {
		t.Fatalf("edit running = %+v", rep)
	}
}

func TestXPathFilterRejected(t *testing.T) {
	c, _, _ := startSession(t, User{Name: "alice", Profile: "router-a"})
	rep := mustRPC(t, c, ncrpc.RPC{
		MessageID: "1",
		Name:      ncrpc.OpGet,
		Filter:    &ncrpc.Filter{Type: "xpath", Select: "/foo"},
	})
	if len(rep.Errors) == 0 || rep.Errors[0].Tag != "operation-not-supported" {
		t.Fatalf("xpath get = %+v", rep)
	}
}

func TestCreateSubscriptionSucceedsDeliversNothing(t *testing.T) {
	sink := notif.Nop{}
	h := mustHandle(t, "router-a", "lab-rtr-a", sink)
	c, _, _ := startSession(t, User{Name: "alice", Profile: "router-a", Handle: h})
	rep := mustRPC(t, c, ncrpc.RPC{MessageID: "113", Name: ncrpc.OpCreateSubscription, Stream: "NETCONF"})
	if !rep.OK {
		t.Fatalf("create-subscription = %+v", rep)
	}
	cfg := []byte(`<system xmlns="urn:ietf:params:xml:ns:yang:ietf-system"><hostname>notified</hostname></system>`)
	if !mustRPC(t, c, ncrpc.RPC{MessageID: "2", Name: ncrpc.OpEditConfig, Target: "candidate", Config: cfg}).OK {
		t.Fatal("edit")
	}
	if !mustRPC(t, c, ncrpc.RPC{MessageID: "3", Name: ncrpc.OpCommit}).OK {
		t.Fatal("commit")
	}
	got := mustRPC(t, c, ncrpc.RPC{MessageID: "4", Name: ncrpc.OpGetConfig, Source: "running"})
	if len(got.Errors) != 0 {
		t.Fatalf("got notification instead of rpc-reply: %+v", got)
	}
	if !bytes.Contains(got.Data, []byte("notified")) {
		t.Fatalf("running = %s", got.Data)
	}
}

func TestReadOnlyAccessDeniedOnEdit(t *testing.T) {
	c, _, _ := startSession(t, User{Name: "vendor", Profile: "router-a", Access: model.UserAccessRead})
	cfg := []byte(`<system xmlns="urn:ietf:params:xml:ns:yang:ietf-system"><hostname>nope</hostname></system>`)
	rep := mustRPC(t, c, ncrpc.RPC{
		MessageID: "1",
		Name:      ncrpc.OpEditConfig,
		Target:    "candidate",
		Config:    cfg,
	})
	if len(rep.Errors) == 0 || rep.Errors[0].Tag != "access-denied" {
		t.Fatalf("read-only edit = %+v", rep)
	}
	got := mustRPC(t, c, ncrpc.RPC{MessageID: "2", Name: ncrpc.OpGetConfig, Source: "running"})
	if !bytes.Contains(got.Data, []byte("lab-rtr-a")) {
		t.Fatalf("read-only get-config = %s", got.Data)
	}
}

func TestUnknownUserServeFails(t *testing.T) {
	srv := New(Config{Users: []User{{Name: "alice", Profile: "router-a", Handle: mustHandle(t, "router-a", "lab-rtr-a", nil)}}})
	a, b := net.Pipe()
	defer func() { _ = a.Close() }()
	defer func() { _ = b.Close() }()
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(context.Background(), "nonesuch", "127.0.0.1:1", a) }()
	err := <-errc
	if err == nil {
		t.Fatal("unknown user served")
	}
	if !strings.Contains(err.Error(), "unknown user") {
		t.Fatalf("error = %v", err)
	}
}

func TestSessionTable(t *testing.T) {
	c, srv, _ := startSession(t, User{Name: "alice", Profile: "router-a"})
	waitSessions(t, srv, 1)
	list := srv.Sessions()
	if len(list) != 1 || list[0].Username != "alice" || list[0].Profile != "router-a" {
		t.Fatalf("sessions = %+v", list)
	}
	_ = c
}

func TestUserProfileIsolation(t *testing.T) {
	ha := mustHandle(t, "router-a", "lab-rtr-a", nil)
	hb := mustHandle(t, "router-b", "lab-rtr-b", nil)
	srv := New(Config{Users: []User{
		{Name: "alice", Profile: "router-a", Handle: ha, Namespaces: map[string]string{"ietf-system": ietfSystemNS}},
		{Name: "vendor", Profile: "router-b", Handle: hb, Namespaces: map[string]string{"ietf-system": ietfSystemNS}},
	}})
	a1, b1 := net.Pipe()
	a2, b2 := net.Pipe()
	t.Cleanup(func() {
		_ = a1.Close()
		_ = b1.Close()
		_ = a2.Close()
		_ = b2.Close()
	})
	go func() { _ = srv.Serve(context.Background(), "alice", "127.0.0.1:1", a1) }()
	go func() { _ = srv.Serve(context.Background(), "vendor", "127.0.0.1:2", a2) }()
	ca := nctestClient(t, b1)
	cb := nctestClient(t, b2)
	ra := mustRPC(t, ca, ncrpc.RPC{MessageID: "1", Name: ncrpc.OpGetConfig, Source: "running"})
	rb := mustRPC(t, cb, ncrpc.RPC{MessageID: "1", Name: ncrpc.OpGetConfig, Source: "running"})
	if !bytes.Contains(ra.Data, []byte("lab-rtr-a")) {
		t.Fatalf("alice = %s", ra.Data)
	}
	if !bytes.Contains(rb.Data, []byte("lab-rtr-b")) {
		t.Fatalf("vendor = %s", rb.Data)
	}
	if bytes.Contains(ra.Data, []byte("lab-rtr-b")) {
		t.Fatalf("alice saw vendor tree: %s", ra.Data)
	}
}

func nctestClient(t *testing.T, rw net.Conn) *nctest.Client {
	t.Helper()
	c := nctest.New(rw)
	if _, err := c.Handshake(nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}
