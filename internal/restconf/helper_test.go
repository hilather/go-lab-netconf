package restconf

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/datastore"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
	"github.com/hilather/go-lab-netconf/internal/notif"
	"github.com/hilather/go-lab-netconf/internal/yangtree"
)

const (
	hostnamePath = "ietf-system:system/hostname"
	aliceUser    = "alice"
	alicePass    = "alice-secret"
	adminToken   = "0123456789abcdef0123456789abcdef"
)

type recordingSink struct {
	mu    sync.Mutex
	calls int
}

func (s *recordingSink) OnCommit(string, []notif.Change) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
}

func (s *recordingSink) n() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

type spyHandle struct {
	datastore.Handle
	mu      sync.Mutex
	writes  int
	edits   int
	commits int
}

func (s *spyHandle) WriteRunningIfCandidateClean(ctx context.Context, op datastore.EditOp) error {
	s.mu.Lock()
	s.writes++
	s.mu.Unlock()
	return s.Handle.WriteRunningIfCandidateClean(ctx, op)
}

func (s *spyHandle) Edit(ctx context.Context, store datastore.Name, op datastore.EditOp) error {
	s.mu.Lock()
	s.edits++
	s.mu.Unlock()
	return s.Handle.Edit(ctx, store, op)
}

func (s *spyHandle) Commit(ctx context.Context) error {
	s.mu.Lock()
	s.commits++
	s.mu.Unlock()
	return s.Handle.Commit(ctx)
}

func (s *spyHandle) counts() (writes, edits, commits int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writes, s.edits, s.commits
}

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

func mustNode(t *testing.T, hostname string) datastore.Node {
	t.Helper()
	n, err := yangtree.Compile(hostnameInstance(hostname), hostnameSchema())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func newHandle(t *testing.T, profile, hostname string, sink datastore.CommitSink) datastore.Handle {
	t.Helper()
	n := mustNode(t, hostname)
	return datastore.New(profile, n, datastore.Node{}, datastore.Node{}, sink)
}

func leafHostname(t *testing.T, h datastore.Handle, store datastore.Name) string {
	t.Helper()
	n, err := h.Get(context.Background(), store, datastore.Subtree{Path: hostnamePath})
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

func alice() User {
	return User{
		Name:     aliceUser,
		Password: []byte(alicePass),
		Profile:  "router-a",
		Access:   model.UserAccessReadWrite,
	}
}

func newServer(t *testing.T, h datastore.Handle, users ...User) *Server {
	t.Helper()
	if len(users) == 0 {
		users = []User{alice()}
	}
	handles := map[string]datastore.Handle{}
	for _, u := range users {
		if _, ok := handles[u.Profile]; !ok {
			handles[u.Profile] = h
		}
	}
	if h != nil && len(handles) == 0 {
		handles["router-a"] = h
	}
	s, err := New(Config{
		Users:   users,
		Handles: handles,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func startServer(t *testing.T, s *Server) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	return ts
}

func doJSON(t *testing.T, ts *httptest.Server, method, path, user, pass, body string) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = bytes.NewBufferString(body)
	}
	req, err := http.NewRequest(method, ts.URL+path, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	req.Header.Set("Accept", MediaYangJSON)
	if body != "" {
		req.Header.Set("Content-Type", MediaYangJSON)
	}
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func readJSON(t *testing.T, res *http.Response) map[string]any {
	t.Helper()
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json %s: %v body=%s", res.Status, err, b)
	}
	return m
}

func problemCode(t *testing.T, res *http.Response) string {
	t.Helper()
	m := readJSON(t, res)
	if m == nil {
		return ""
	}
	c, _ := m["code"].(string)
	return c
}

func getHostnameJSON(t *testing.T, ts *httptest.Server, user, pass string) (int, string) {
	t.Helper()
	res := doJSON(t, ts, http.MethodGet, "/restconf/data/ietf-system:system/hostname", user, pass, "")
	status := res.StatusCode
	m := readJSON(t, res)
	if m == nil {
		return status, ""
	}
	s, _ := m["ietf-system:hostname"].(string)
	return status, s
}

func mergeCandidate(t *testing.T, h datastore.Handle, value string) {
	t.Helper()
	err := h.Edit(context.Background(), datastore.Candidate, datastore.EditOp{
		Op:    yangtree.OpMerge,
		Path:  hostnamePath,
		Value: value,
	})
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
