package ncrpc

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func sessionGolden(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "testdata", "sessions", name))
	if err != nil {
		t.Fatal(err)
	}
	return bytes.TrimRight(b, "\r\n")
}

func mustEncodeRPC(t *testing.T, r RPC) []byte {
	t.Helper()
	b, err := EncodeRPC(r)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustDecodeRPC(t *testing.T, raw []byte) RPC {
	t.Helper()
	msg, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.RPC == nil {
		t.Fatalf("decoded %#v, want rpc", msg)
	}
	return *msg.RPC
}

func equalRPC(t *testing.T, got, want RPC) {
	t.Helper()
	if got.MessageID != want.MessageID {
		t.Fatalf("message-id = %q, want %q", got.MessageID, want.MessageID)
	}
	if got.Name != want.Name {
		t.Fatalf("name = %q, want %q", got.Name, want.Name)
	}
	if got.Source != want.Source {
		t.Fatalf("source = %q, want %q", got.Source, want.Source)
	}
	if got.Target != want.Target {
		t.Fatalf("target = %q, want %q", got.Target, want.Target)
	}
	if got.DefaultOp != want.DefaultOp {
		t.Fatalf("default-operation = %q, want %q", got.DefaultOp, want.DefaultOp)
	}
	if got.SessionID != want.SessionID {
		t.Fatalf("session-id = %q, want %q", got.SessionID, want.SessionID)
	}
	if got.Stream != want.Stream {
		t.Fatalf("stream = %q, want %q", got.Stream, want.Stream)
	}
	if !bytes.Equal(got.Config, want.Config) {
		t.Fatalf("config = %q, want %q", got.Config, want.Config)
	}
	switch {
	case got.Filter == nil && want.Filter == nil:
	case got.Filter == nil || want.Filter == nil:
		t.Fatalf("filter = %#v, want %#v", got.Filter, want.Filter)
	default:
		if got.Filter.Type != want.Filter.Type || got.Filter.Select != want.Filter.Select || !bytes.Equal(got.Filter.Inner, want.Filter.Inner) {
			t.Fatalf("filter = %#v, want %#v", got.Filter, want.Filter)
		}
	}
}
