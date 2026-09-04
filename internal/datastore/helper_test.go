package datastore

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
	"github.com/hilather/go-lab-netconf/internal/yangtree"
)

const hostnamePath = "ietf-system:system/hostname"

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

func mustNode(t *testing.T, hostname string) Node {
	t.Helper()
	n, err := yangtree.Compile(hostnameInstance(hostname), hostnameSchema())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func newHandle(t *testing.T, profile, hostname string, sink CommitSink) Handle {
	t.Helper()
	n := mustNode(t, hostname)
	return New(profile, n, Node{}, Node{}, sink)
}

func hostname(t *testing.T, h Handle, store Name) string {
	t.Helper()
	n, err := h.Get(context.Background(), store, Subtree{Path: hostnamePath})
	if err != nil {
		t.Fatal(err)
	}
	v, ok := n.Lookup(hostnamePath)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func mergeHostname(t *testing.T, h Handle, value string) {
	t.Helper()
	err := h.Edit(context.Background(), Candidate, EditOp{Op: yangtree.OpMerge, Path: hostnamePath, Value: value})
	if err != nil {
		t.Fatal(err)
	}
}

func requireCode(t *testing.T, err error, code domainerr.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s", code)
	}
	de, ok := domainerr.As(err)
	if !ok || de.Code != code {
		t.Fatalf("got %v, want %s", err, code)
	}
}
