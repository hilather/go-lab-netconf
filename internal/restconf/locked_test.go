package restconf

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/datastore"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
)

func TestUnknownUser401(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	s := newServer(t, h)
	ts := startServer(t, s)

	res := doJSON(t, ts, http.MethodGet, "/restconf/data/ietf-system:system/hostname", "nosuch", "wrong", "")
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unknown user status = %d, want 401", res.StatusCode)
	}
	if got := problemCode(t, res); got != string(domainerr.CodeUnauthorized) {
		t.Fatalf("code = %q, want unauthorized", got)
	}

	status, host := getHostnameJSON(t, ts, aliceUser, alicePass)
	if status != http.StatusOK || host != "lab-rtr-a" {
		t.Fatalf("known user GET status=%d hostname=%q (401 must not be blanket)", status, host)
	}
}

func TestCommitVisibleOnRESTCONFGet(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	s := newServer(t, h)
	ts := startServer(t, s)

	status, host := getHostnameJSON(t, ts, aliceUser, alicePass)
	if status != http.StatusOK || host != "lab-rtr-a" {
		t.Fatalf("GET running before commit status=%d hostname=%q", status, host)
	}

	mergeCandidate(t, h, "after-commit")
	if leafHostname(t, h, datastore.Running) != "lab-rtr-a" {
		t.Fatalf("running mutated by candidate edit = %q", leafHostname(t, h, datastore.Running))
	}
	status, host = getHostnameJSON(t, ts, aliceUser, alicePass)
	if status != http.StatusOK || host != "lab-rtr-a" {
		t.Fatalf("GET must read running, not candidate; status=%d hostname=%q", status, host)
	}

	if err := h.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if leafHostname(t, h, datastore.Running) != "after-commit" {
		t.Fatalf("handle running after commit = %q", leafHostname(t, h, datastore.Running))
	}
	status, host = getHostnameJSON(t, ts, aliceUser, alicePass)
	if status != http.StatusOK || host != "after-commit" {
		t.Fatalf("GET after commit status=%d hostname=%q, want after-commit", status, host)
	}
}

func TestManagementTokenDoesNotUnlock(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	s := newServer(t, h)
	ts := startServer(t, s)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/restconf/data/ietf-system:system/hostname", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Accept", MediaYangJSON)
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bearer status = %d, want 401", res.StatusCode)
	}
	if got := problemCode(t, res); got != string(domainerr.CodeUnauthorized) {
		t.Fatalf("code = %q", got)
	}
	if wa := res.Header.Get("WWW-Authenticate"); wa == "" {
		t.Fatal("missing WWW-Authenticate")
	}

	status, host := getHostnameJSON(t, ts, aliceUser, alicePass)
	if status != http.StatusOK || host != "lab-rtr-a" {
		t.Fatalf("Basic still works status=%d hostname=%q", status, host)
	}
}

func TestMissingAuth401(t *testing.T) {
	h := newHandle(t, "router-a", "lab-rtr-a", nil)
	ts := startServer(t, newServer(t, h))
	res := doJSON(t, ts, http.MethodGet, "/restconf/data/ietf-system:system/hostname", "", "", "")
	if res.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		t.Fatalf("status = %d body=%s", res.StatusCode, body)
	}
	if got := problemCode(t, res); got != string(domainerr.CodeUnauthorized) {
		t.Fatalf("code = %q", got)
	}
}
