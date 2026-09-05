package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthcheckAgainstReady(t *testing.T) {
	ready := true
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health/ready" {
			http.NotFound(w, r)
			return
		}
		if !ready {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"not ready"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(ts.Close)

	url := ts.URL + "/v1/health/ready"
	var stdout, stderr bytes.Buffer
	code := healthcheckCmd([]string{"--url", url}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("ready exit %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok") {
		t.Fatalf("stdout %q", stdout.String())
	}

	ready = false
	stdout.Reset()
	stderr.Reset()
	code = healthcheckCmd([]string{"--url", url}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("not ready exit %d", code)
	}
	if !strings.Contains(stderr.String(), "status 503") {
		t.Fatalf("stderr %q", stderr.String())
	}
}

func TestHealthcheckBadFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := healthcheckCmd([]string{"--bogus"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d", code)
	}
}

func TestHealthcheckViaRun(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ts.Close)
	var stdout, stderr bytes.Buffer
	code := run([]string{"labnetconf", "healthcheck", "--url", ts.URL + "/v1/health/ready"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q", code, stderr.String())
	}
}
