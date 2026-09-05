package restconf

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-netconf/internal/datastore"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
)

func TestGetHostname(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	ts := startServer(t, newServer(t, h))
	status, host := getHostnameJSON(t, ts, aliceUser, alicePass)
	if status != http.StatusOK || host != "lab-rtr-a" {
		t.Fatalf("GET hostname status=%d hostname=%q", status, host)
	}
}

func TestPatchWhenCleanUpdatesRunningAndCandidate(t *testing.T) {
	sink := &recordingSink{}
	inner := newHandle(t, "router-a", "lab-rtr-a", sink)
	spy := &spyHandle{Handle: inner}
	s := newServer(t, spy)
	ts := startServer(t, s)
	g0 := inner.Generation()

	res := doJSON(t, ts, http.MethodPatch, "/restconf/data/ietf-system:system/hostname",
		aliceUser, alicePass, `{"ietf-system:hostname":"restconf"}`)
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("PATCH status = %d code=%s", res.StatusCode, problemCode(t, res))
	}
	res.Body.Close()

	if leafHostname(t, inner, datastore.Running) != "restconf" {
		t.Fatalf("running = %q, want restconf", leafHostname(t, inner, datastore.Running))
	}
	if leafHostname(t, inner, datastore.Candidate) != "restconf" {
		t.Fatalf("candidate = %q, want restconf", leafHostname(t, inner, datastore.Candidate))
	}
	if inner.Dirty() {
		t.Fatal("dirty after clean PATCH")
	}
	if inner.Generation() != g0 {
		t.Fatalf("generation %d -> %d (WriteRunningIfCandidateClean must not Commit)", g0, inner.Generation())
	}
	if sink.n() != 0 {
		t.Fatalf("OnCommit called %d times (must not Commit)", sink.n())
	}
	writes, edits, commits := spy.counts()
	if writes != 1 || edits != 0 || commits != 0 {
		t.Fatalf("writes=%d edits=%d commits=%d; writes must use WriteRunningIfCandidateClean only", writes, edits, commits)
	}

	status, host := getHostnameJSON(t, ts, aliceUser, alicePass)
	if status != http.StatusOK || host != "restconf" {
		t.Fatalf("GET after PATCH status=%d hostname=%q", status, host)
	}
}

func TestPatchWhenDirty409CandidateDirty(t *testing.T) {
	sink := &recordingSink{}
	inner := newHandle(t, "router-a", "lab-rtr-a", sink)
	spy := &spyHandle{Handle: inner}
	ts := startServer(t, newServer(t, spy))
	g0 := inner.Generation()
	mergeCandidate(t, inner, "netconf-pending")

	res := doJSON(t, ts, http.MethodPatch, "/restconf/data/ietf-system:system/hostname",
		aliceUser, alicePass, `{"ietf-system:hostname":"blocked"}`)
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", res.StatusCode)
	}
	if got := problemCode(t, res); got != string(domainerr.CodeCandidateDirty) {
		t.Fatalf("code = %q, want candidate_dirty", got)
	}
	if leafHostname(t, inner, datastore.Running) != "lab-rtr-a" {
		t.Fatalf("running mutated while dirty = %q", leafHostname(t, inner, datastore.Running))
	}
	if leafHostname(t, inner, datastore.Candidate) != "netconf-pending" {
		t.Fatalf("candidate mutated while dirty = %q", leafHostname(t, inner, datastore.Candidate))
	}
	if inner.Generation() != g0 {
		t.Fatalf("generation changed on dirty PATCH: %d", inner.Generation())
	}
	if sink.n() != 0 {
		t.Fatalf("OnCommit on dirty PATCH: %d", sink.n())
	}
	writes, edits, commits := spy.counts()
	if writes != 1 {
		t.Fatalf("WriteRunningIfCandidateClean calls = %d, want 1", writes)
	}
	if edits != 0 || commits != 0 {
		t.Fatalf("edits=%d commits=%d on dirty PATCH", edits, commits)
	}
	status, host := getHostnameJSON(t, ts, aliceUser, alicePass)
	if status != http.StatusOK || host != "lab-rtr-a" {
		t.Fatalf("GET running after dirty PATCH status=%d hostname=%q", status, host)
	}
}

func TestPatchWhenLocked409CandidateDirty(t *testing.T) {
	inner := newHandle(t, "router-a", "lab-rtr-a", nil)
	spy := &spyHandle{Handle: inner}
	ts := startServer(t, newServer(t, spy))
	if err := inner.Lock(context.Background(), datastore.Candidate, "sess-a"); err != nil {
		t.Fatal(err)
	}

	res := doJSON(t, ts, http.MethodPatch, "/restconf/data/ietf-system:system/hostname",
		aliceUser, alicePass, `{"ietf-system:hostname":"locked"}`)
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", res.StatusCode)
	}
	if got := problemCode(t, res); got != string(domainerr.CodeCandidateDirty) {
		t.Fatalf("code = %q, want candidate_dirty (locked candidate is not lock_denied)", got)
	}
	if leafHostname(t, inner, datastore.Running) != "lab-rtr-a" {
		t.Fatalf("running mutated while candidate locked = %q", leafHostname(t, inner, datastore.Running))
	}
	if leafHostname(t, inner, datastore.Candidate) != "lab-rtr-a" {
		t.Fatalf("candidate mutated while locked = %q", leafHostname(t, inner, datastore.Candidate))
	}
	writes, edits, commits := spy.counts()
	if writes != 1 || edits != 0 || commits != 0 {
		t.Fatalf("writes=%d edits=%d commits=%d", writes, edits, commits)
	}
}

func TestPatchWhenRunningLocked409LockDenied(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	ts := startServer(t, newServer(t, h))
	if err := h.Lock(context.Background(), datastore.Running, "sess-a"); err != nil {
		t.Fatal(err)
	}
	res := doJSON(t, ts, http.MethodPatch, "/restconf/data/ietf-system:system/hostname",
		aliceUser, alicePass, `{"ietf-system:hostname":"bypass"}`)
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", res.StatusCode)
	}
	if got := problemCode(t, res); got != string(domainerr.CodeLockDenied) {
		t.Fatalf("code = %q, want lock_denied", got)
	}
	if leafHostname(t, h, datastore.Running) != "lab-rtr-a" {
		t.Fatalf("running mutated = %q", leafHostname(t, h, datastore.Running))
	}
}

func TestHostMetaYangLibraryOperations(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	ts := startServer(t, newServer(t, h))

	res, err := ts.Client().Get(ts.URL + "/.well-known/host-meta")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("host-meta status = %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, mediaXRD) {
		t.Fatalf("host-meta content-type = %q", ct)
	}
	if string(body) != hostMetaXRD {
		t.Fatalf("host-meta XRD = %s", body)
	}

	res = doJSON(t, ts, http.MethodGet, "/restconf/yang-library-version", aliceUser, alicePass, "")
	m := readJSON(t, res)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("yang-library-version status = %d", res.StatusCode)
	}
	if m["ietf-restconf:yang-library-version"] != yangLibraryDateDefault {
		t.Fatalf("yang-library-version = %#v", m)
	}

	res = doJSON(t, ts, http.MethodGet, "/restconf/operations", aliceUser, alicePass, "")
	m = readJSON(t, res)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("operations status = %d", res.StatusCode)
	}
	ops, _ := m["ietf-restconf:operations"].(map[string]any)
	if ops == nil {
		t.Fatalf("operations = %#v", m)
	}
}

func TestNotMountedOnManagement(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	ts := startServer(t, newServer(t, h))
	res := doJSON(t, ts, http.MethodGet, "/v1/state", aliceUser, alicePass, "")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("/v1 on RESTCONF listener status = %d, want 404", res.StatusCode)
	}
	if got := problemCode(t, res); got != string(domainerr.CodeNotFound) {
		t.Fatalf("code = %q", got)
	}
}

func TestDedicatedListenerBindsOwnPort(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	s := newServer(t, h)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- s.Serve(ctx, ln) }()

	url := "http://" + ln.Addr().String() + "/restconf/data/ietf-system:system/hostname"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth(aliceUser, alicePass)
	req.Header.Set("Accept", MediaYangJSON)
	client := &http.Client{Timeout: 3 * time.Second}
	var res *http.Response
	for i := 0; i < 50; i++ {
		res, err = client.Do(req)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
		req, _ = http.NewRequest(http.MethodGet, url, nil)
		req.SetBasicAuth(aliceUser, alicePass)
		req.Header.Set("Accept", MediaYangJSON)
	}
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("dedicated listener GET status = %d", res.StatusCode)
	}
	m := readJSON(t, res)
	if m["ietf-system:hostname"] != "lab-rtr-a" {
		t.Fatalf("body = %#v", m)
	}
	cancel()
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return")
	}
}

func TestAdmissionOmittedLoopbackEmptyDenyAll(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	omitted, err := New(Config{
		Users:   []User{alice()},
		Handles: map[string]datastore.Handle{"router-a": h},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !omitted.admit("127.0.0.1:9") || !omitted.admit("[::1]:9") {
		t.Fatal("omitted CIDRs must admit loopback")
	}
	if omitted.admit("10.1.2.3:9") {
		t.Fatal("omitted CIDRs must not admit 10.1.2.3")
	}

	empty, err := New(Config{
		Users:            []User{alice()},
		Handles:          map[string]datastore.Handle{"router-a": h},
		AllowClientCidrs: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if empty.admit("127.0.0.1:9") || empty.admit("[::1]:9") || empty.admit("10.1.2.3:9") {
		t.Fatal("empty CIDR list must deny all")
	}

	listed, err := New(Config{
		Users:            []User{alice()},
		Handles:          map[string]datastore.Handle{"router-a": h},
		AllowClientCidrs: []string{"10.99.42.0/24"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !listed.admit("10.99.42.7:9") {
		t.Fatal("listed CIDR must admit overlay")
	}
	if listed.admit("127.0.0.1:9") {
		t.Fatal("listed CIDR without loopback must deny 127.0.0.1")
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/restconf/data/ietf-system:system/hostname", nil)
	r.RemoteAddr = "10.1.2.3:1234"
	r.SetBasicAuth(aliceUser, alicePass)
	omitted.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-loopback omitted status = %d", w.Code)
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/restconf/data/ietf-system:system/hostname", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.SetBasicAuth(aliceUser, alicePass)
	empty.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("loopback empty-list status = %d", w.Code)
	}
}

func TestXMLRejected(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	ts := startServer(t, newServer(t, h))
	req, err := http.NewRequest(http.MethodPatch, ts.URL+"/restconf/data/ietf-system:system/hostname",
		bytes.NewBufferString(`<hostname>x</hostname>`))
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth(aliceUser, alicePass)
	req.Header.Set("Content-Type", "application/yang-data+xml")
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("XML PATCH status = %d, want 415", res.StatusCode)
	}
	if leafHostname(t, h, datastore.Running) != "lab-rtr-a" {
		t.Fatalf("running mutated by XML PATCH = %q", leafHostname(t, h, datastore.Running))
	}
}

func TestPutPostDelete(t *testing.T) {
	inner := newHandle(t, "router-a", "lab-rtr-a", nil)
	spy := &spyHandle{Handle: inner}
	ts := startServer(t, newServer(t, spy))

	res := doJSON(t, ts, http.MethodPut, "/restconf/data/ietf-system:system/hostname",
		aliceUser, alicePass, `{"ietf-system:hostname":"put-host"}`)
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT status = %d code=%s", res.StatusCode, problemCode(t, res))
	}
	res.Body.Close()
	if leafHostname(t, inner, datastore.Running) != "put-host" || leafHostname(t, inner, datastore.Candidate) != "put-host" {
		t.Fatalf("after PUT running=%q candidate=%q", leafHostname(t, inner, datastore.Running), leafHostname(t, inner, datastore.Candidate))
	}

	res = doJSON(t, ts, http.MethodPost, "/restconf/data/ietf-system:system",
		aliceUser, alicePass, `{"ietf-system:system":{"hostname":"post-host"}}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("POST status = %d code=%s", res.StatusCode, problemCode(t, res))
	}
	res.Body.Close()
	if leafHostname(t, inner, datastore.Running) != "post-host" {
		t.Fatalf("after POST running=%q", leafHostname(t, inner, datastore.Running))
	}

	res = doJSON(t, ts, http.MethodDelete, "/restconf/data/ietf-system:system/hostname",
		aliceUser, alicePass, "")
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status = %d code=%s", res.StatusCode, problemCode(t, res))
	}
	res.Body.Close()
	if leafHostname(t, inner, datastore.Running) != "" {
		t.Fatalf("after DELETE running=%q", leafHostname(t, inner, datastore.Running))
	}

	writes, edits, commits := spy.counts()
	if writes != 3 || edits != 0 || commits != 0 {
		t.Fatalf("writes=%d edits=%d commits=%d", writes, edits, commits)
	}
}

func TestUserIsolation(t *testing.T) {
	a := newHandle(t, "router-a", "lab-rtr-a", nil)
	b := newHandle(t, "router-b", "lab-rtr-b", nil)
	s, err := New(Config{
		Users: []User{
			alice(),
			{Name: "bob", Password: []byte("bob-secret"), Profile: "router-b", Access: model.UserAccessReadWrite},
		},
		Handles: map[string]datastore.Handle{"router-a": a, "router-b": b},
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := startServer(t, s)
	_, hostA := getHostnameJSON(t, ts, aliceUser, alicePass)
	_, hostB := getHostnameJSON(t, ts, "bob", "bob-secret")
	if hostA != "lab-rtr-a" || hostB != "lab-rtr-b" {
		t.Fatalf("alice=%q bob=%q", hostA, hostB)
	}
	res := doJSON(t, ts, http.MethodPatch, "/restconf/data/ietf-system:system/hostname",
		aliceUser, alicePass, `{"ietf-system:hostname":"from-alice"}`)
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("alice PATCH status = %d", res.StatusCode)
	}
	if leafHostname(t, b, datastore.Running) != "lab-rtr-b" {
		t.Fatalf("bob running leaked alice write = %q", leafHostname(t, b, datastore.Running))
	}
	_, hostB = getHostnameJSON(t, ts, "bob", "bob-secret")
	if hostB != "lab-rtr-b" {
		t.Fatalf("bob GET after alice PATCH = %q", hostB)
	}
}

func TestReadOnlyUserCannotWrite(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	s, err := New(Config{
		Users: []User{{
			Name: "reader", Password: []byte("r"), Profile: "router-a", Access: model.UserAccessRead,
		}},
		Handles: map[string]datastore.Handle{"router-a": h},
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := startServer(t, s)
	status, host := getHostnameJSON(t, ts, "reader", "r")
	if status != http.StatusOK || host != "lab-rtr-a" {
		t.Fatalf("reader GET status=%d hostname=%q", status, host)
	}
	res := doJSON(t, ts, http.MethodPatch, "/restconf/data/ietf-system:system/hostname",
		"reader", "r", `{"ietf-system:hostname":"nope"}`)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("reader PATCH status = %d", res.StatusCode)
	}
	readJSON(t, res)
	if leafHostname(t, h, datastore.Running) != "lab-rtr-a" {
		t.Fatalf("running = %q", leafHostname(t, h, datastore.Running))
	}
}

func TestUnknownPathGetEmptyEdit400(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	ts := startServer(t, newServer(t, h))
	res := doJSON(t, ts, http.MethodGet, "/restconf/data/ietf-system:system/nope", aliceUser, alicePass, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unknown GET status = %d", res.StatusCode)
	}
	m := readJSON(t, res)
	if len(m) != 0 {
		t.Fatalf("unknown GET body = %#v", m)
	}
	res = doJSON(t, ts, http.MethodPatch, "/restconf/data/ietf-system:system/nope",
		aliceUser, alicePass, `{"ietf-system:nope":"x"}`)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown PATCH status = %d", res.StatusCode)
	}
	readJSON(t, res)
	if leafHostname(t, h, datastore.Running) != "lab-rtr-a" {
		t.Fatalf("running mutated = %q", leafHostname(t, h, datastore.Running))
	}
}

func TestHostMetaGolden(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("go.mod not found")
		}
		root = parent
	}
	want, err := os.ReadFile(filepath.Join(root, "testdata", "sessions", "restconf", "host-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != hostMetaXRD {
		t.Fatalf("host-meta golden mismatch\n got %q\nwant %q", hostMetaXRD, want)
	}
}

func TestNewRejectsBadCIDR(t *testing.T) {
	_, err := New(Config{AllowClientCidrs: []string{"not-a-cidr"}})
	requireCode(t, err, domainerr.CodeValidationFailed)
}

func TestDataPathListKey(t *testing.T) {
	got, err := dataPath("/restconf/data/ietf-interfaces:interfaces/interface=eth0/enabled")
	if err != nil {
		t.Fatal(err)
	}
	want := "ietf-interfaces:interfaces/interface[name=eth0]/enabled"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRootGetAndPatch(t *testing.T) {
	inner := newHandle(t, "router-a", "lab-rtr-a", nil)
	spy := &spyHandle{Handle: inner}
	ts := startServer(t, newServer(t, spy))
	res := doJSON(t, ts, http.MethodGet, "/restconf/data", aliceUser, alicePass, "")
	m := readJSON(t, res)
	sys, _ := m["ietf-system:system"].(map[string]any)
	if sys == nil || sys["hostname"] != "lab-rtr-a" {
		t.Fatalf("root GET = %#v", m)
	}
	body, err := json.Marshal(map[string]any{
		"ietf-system:system": map[string]any{"hostname": "from-root"},
	})
	if err != nil {
		t.Fatal(err)
	}
	res = doJSON(t, ts, http.MethodPatch, "/restconf/data", aliceUser, alicePass, string(body))
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("root PATCH status = %d code=%s", res.StatusCode, problemCode(t, res))
	}
	res.Body.Close()
	if leafHostname(t, inner, datastore.Running) != "from-root" || leafHostname(t, inner, datastore.Candidate) != "from-root" {
		t.Fatalf("running=%q candidate=%q", leafHostname(t, inner, datastore.Running), leafHostname(t, inner, datastore.Candidate))
	}
	if _, edits, commits := spy.counts(); edits != 0 || commits != 0 {
		t.Fatalf("root PATCH used Edit/Commit")
	}
}
