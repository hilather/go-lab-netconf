package ncserver

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/hilather/go-lab-netconf/internal/datastore"
	"github.com/hilather/go-lab-netconf/internal/model"
	"github.com/hilather/go-lab-netconf/internal/ncrpc"
	"github.com/hilather/go-lab-netconf/internal/nctest"
	"github.com/hilather/go-lab-netconf/internal/notif"
	"github.com/hilather/go-lab-netconf/internal/yangtree"
)

const hostnamePath = "ietf-system:system/hostname"
const ietfSystemNS = "urn:ietf:params:xml:ns:yang:ietf-system"

func hostnameSchema() []model.SchemaLeaf {
	return []model.SchemaLeaf{
		{Path: "ietf-system:system", Type: "container"},
		{Path: hostnamePath, Type: "string", Access: "write"},
	}
}

func hostnameInstance(name string) map[string]any {
	return map[string]any{
		"ietf-system": map[string]any{
			"system": map[string]any{
				"hostname": name,
			},
		},
	}
}

func mustHandle(t *testing.T, profile, hostname string, sink datastore.CommitSink) datastore.Handle {
	t.Helper()
	n, err := yangtree.Compile(hostnameInstance(hostname), hostnameSchema())
	if err != nil {
		t.Fatal(err)
	}
	return datastore.New(profile, n, datastore.Node{}, datastore.Node{}, sink)
}

func startSession(t *testing.T, user User) (*nctest.Client, *Server, ncrpc.Hello) {
	t.Helper()
	if user.Namespaces == nil {
		user.Namespaces = map[string]string{"ietf-system": ietfSystemNS}
	}
	if user.Handle == nil {
		user.Handle = mustHandle(t, user.Profile, "lab-rtr-a", nil)
	}
	if user.Name == "" {
		user.Name = "alice"
	}
	if user.Profile == "" {
		user.Profile = "router-a"
	}
	srv := New(Config{Users: []User{user}, Sink: notif.Nop{}})
	a, b := net.Pipe()
	t.Cleanup(func() {
		_ = a.Close()
		_ = b.Close()
	})
	errc := make(chan error, 1)
	go func() {
		errc <- srv.Serve(context.Background(), user.Name, "127.0.0.1:1", a)
	}()
	t.Cleanup(func() {
		select {
		case <-errc:
		default:
		}
	})
	c := nctest.New(b)
	hello, err := c.Handshake(nil)
	if err != nil {
		t.Fatal(err)
	}
	if hello.SessionID == "" {
		t.Fatal("server hello missing session-id")
	}
	t.Cleanup(func() { _ = c.Close() })
	return c, srv, hello
}

func mustRPC(t *testing.T, c *nctest.Client, r ncrpc.RPC) ncrpc.Reply {
	t.Helper()
	rep, err := c.RPC(r)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func waitSessions(t *testing.T, srv *Server, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(srv.Sessions()) >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("sessions = %d, want >= %d", len(srv.Sessions()), n)
}
