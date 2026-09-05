package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hilather/go-lab-netconf/internal/notif"
)

type syncBuf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestServeBindsAndHealthReady(t *testing.T) {
	cfg, ncAddr, rcAddr, mgmtAddr := writeServeFixture(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	var stdout, stderr syncBuf
	errCh := make(chan int, 1)
	go func() {
		errCh <- serveCmd(ctx, []string{
			"--config", cfg,
			"--netconf-listen", ncAddr,
			"--restconf-listen", rcAddr,
			"--management-listen", mgmtAddr,
		}, &stdout, &stderr)
	}()

	waitHTTP(t, errCh, &stdout, &stderr, "http://"+mgmtAddr+"/v1/health/ready")

	resp, err := http.Get("http://" + mgmtAddr + "/v1/health/ready")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ready %d %s stdout=%q stderr=%q", resp.StatusCode, body, stdout.String(), stderr.String())
	}

	req, err := http.NewRequest(http.MethodGet, "http://"+rcAddr+"/restconf/data/ietf-system:system/hostname", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("alice", "alice-lab-password")
	req.Header.Set("Accept", "application/yang-data+json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("RESTCONF GET %d %s stderr=%q", res.StatusCode, raw, stderr.String())
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["ietf-system:hostname"] != "lab-rtr-a" {
		t.Fatalf("hostname JSON %s", raw)
	}

	conn, err := net.DialTimeout("tcp", ncAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("netconf tcp %v stdout=%q", err, stdout.String())
	}
	_ = conn.Close()

	root, err := http.Get("http://" + mgmtAddr + "/")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, root.Body)
	_ = root.Body.Close()
	if root.StatusCode != http.StatusNotFound {
		t.Fatalf("GET / %d, want 404", root.StatusCode)
	}
	if ct := root.Header.Get("Content-Type"); !strings.Contains(ct, "problem+json") {
		t.Fatalf("GET / content-type %q", ct)
	}

	var hcOut, hcErr strings.Builder
	if code := healthcheckCmd([]string{"--url", "http://" + mgmtAddr + "/v1/health/ready"}, &hcOut, &hcErr); code != 0 {
		t.Fatalf("healthcheck exit %d stderr=%q", code, hcErr.String())
	}
	if !strings.Contains(hcOut.String(), "ok") {
		t.Fatalf("healthcheck %q", hcOut.String())
	}

	cancel()
	select {
	case code := <-errCh:
		if code != 0 {
			t.Fatalf("serve exit %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not exit")
	}
}

func TestServeManagementOffStillAnswersRestconf(t *testing.T) {
	cfg, ncAddr, rcAddr, _ := writeServeFixture(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	var stdout, stderr syncBuf
	errCh := make(chan int, 1)
	go func() {
		errCh <- serveCmd(ctx, []string{
			"--config", cfg,
			"--netconf-listen", ncAddr,
			"--restconf-listen", rcAddr,
			"--management-listen", "off",
		}, &stdout, &stderr)
	}()

	deadline := time.Now().Add(5 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		select {
		case code := <-errCh:
			t.Fatalf("serve exited %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		default:
		}
		req, err := http.NewRequest(http.MethodGet, "http://"+rcAddr+"/restconf/data/ietf-system:system/hostname", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.SetBasicAuth("alice", "alice-lab-password")
		req.Header.Set("Accept", "application/yang-data+json")
		res, err := http.DefaultClient.Do(req)
		if err == nil {
			raw, _ := io.ReadAll(res.Body)
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK && strings.Contains(string(raw), "lab-rtr-a") {
				if !strings.Contains(stdout.String(), "management: not bound") {
					t.Fatalf("stdout %q missing management off", stdout.String())
				}
				return
			}
			last = fmt.Errorf("status %d body %s", res.StatusCode, raw)
		} else {
			last = err
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("RESTCONF never answered: %v stdout=%q stderr=%q", last, stdout.String(), stderr.String())
}

func TestProductionSinkIsRing(t *testing.T) {
	sink := productionSink()
	if _, ok := sink.(*notif.Ring); !ok {
		t.Fatalf("production sink is %T, want *notif.Ring", sink)
	}
}

func TestMCPStdioLoadsAndExits(t *testing.T) {
	cfg, _, _, _ := writeServeFixture(t, false)
	token := filepath.Join(filepath.Dir(cfg), "token")
	var stdout, stderr strings.Builder
	code := mcpStdioCmd(context.Background(), []string{"--config", cfg, "--token-file", token}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q", code, stderr.String())
	}
}

func writeServeFixture(t *testing.T, pickPorts bool) (cfgPath, ncAddr, rcAddr, mgmtAddr string) {
	t.Helper()
	dir := t.TempDir()
	root := repoRoot(t)
	copyFile(t, filepath.Join(root, "testdata", "keys", "labnetconf-hostkey"), filepath.Join(dir, "hostkey"))
	copyFile(t, filepath.Join(root, "testdata", "keys", "alice.password"), filepath.Join(dir, "alice.password"))
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte("0123456789abcdef0123456789abcdef\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ncAddr = "127.0.0.1:0"
	rcAddr = "127.0.0.1:0"
	mgmtAddr = "127.0.0.1:0"
	if pickPorts {
		ncAddr = freeTCP(t)
		rcAddr = freeTCP(t)
		mgmtAddr = freeTCP(t)
	}
	cfgPath = filepath.Join(dir, "labnetconf.yaml")
	body := fmt.Sprintf(`apiVersion: labnetconf.dev/v1alpha1
kind: LabNETCONF
metadata:
  name: serve-fixture
spec:
  listeners:
    netconf:
      enabled: true
      address: %q
      hostKeyFile: %q
    restconf:
      enabled: true
      address: %q
    management:
      address: %q
  auth:
    mode: bearer
    tokens:
      - id: admin
        role: administrator
        secretFile: %q
  ui:
    enabled: false
  admission:
    allowClientCidrs:
      - "127.0.0.0/8"
      - "::1/128"
  profiles:
    - name: router-a
      modules:
        - name: ietf-system
          revision: "2014-08-06"
          namespace: "urn:ietf:params:xml:ns:yang:ietf-system"
      schema:
        - path: "ietf-system:system/hostname"
          type: string
          access: write
      instance:
        ietf-system:
          system:
            hostname: "lab-rtr-a"
  users:
    - name: alice
      passwordFile: %q
      profile: router-a
      access: read-write
`, ncAddr, filepath.Join(dir, "hostkey"), rcAddr, mgmtAddr, filepath.Join(dir, "token"), filepath.Join(dir, "alice.password"))
	if err := os.WriteFile(cfgPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfgPath, ncAddr, rcAddr, mgmtAddr
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func freeTCP(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

type logBuf interface{ String() string }

func waitHTTP(t *testing.T, errCh <-chan int, stdout, stderr logBuf, url string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		select {
		case code := <-errCh:
			t.Fatalf("serve exited %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		default:
		}
		resp, err := http.Get(url)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
			last = fmt.Errorf("status %d", resp.StatusCode)
		} else {
			last = err
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("never became ready: %v stdout=%q stderr=%q", last, stdout.String(), stderr.String())
}
