package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hilather/go-lab-netconf/internal/app"
	"github.com/hilather/go-lab-netconf/internal/datastore"
	"github.com/hilather/go-lab-netconf/internal/model"
	"github.com/hilather/go-lab-netconf/internal/ncrpc"
	"github.com/hilather/go-lab-netconf/internal/ncserver"
	"github.com/hilather/go-lab-netconf/internal/nctest"
	"github.com/hilather/go-lab-netconf/internal/netconfssh"
	"github.com/hilather/go-lab-netconf/internal/restconf"
	"golang.org/x/crypto/ssh"
)

func TestApplyUserDemotionDeniesNextNETCONFWrite(t *testing.T) {
	proc := startServe(t)
	old := openServeNETCONF(t, proc.nc)
	if _, err := old.Handshake(nil); err != nil {
		t.Fatal(err)
	}

	rev := getRuntimeRevision(t, proc)
	alice := aliceFromState(t, proc)
	postApply(t, proc, rev, "demote-alice", map[string]any{
		"op": "upsertUser",
		"user": map[string]any{
			"name":         "alice",
			"passwordFile": alice.passwordFile,
			"profile":      alice.profile,
			"access":       "read",
		},
	})

	next := openServeNETCONF(t, proc.nc)
	if _, err := next.Handshake(nil); err != nil {
		t.Fatal(err)
	}
	rep, err := editHostname(next, "1")
	if err != nil {
		t.Fatalf("new session edit-config: %v", err)
	}
	tag := ""
	if len(rep.Errors) > 0 {
		tag = rep.Errors[0].Tag
	}
	if len(rep.Errors) == 0 || tag != "access-denied" {
		t.Fatalf("demoted alice edit-config ok=%v tag=%q errors=%+v; want access-denied", rep.OK, tag, rep.Errors)
	}

	if _, err := editHostname(old, "2"); err == nil {
		t.Fatal("session opened before demotion completed edit-config; want I/O error from a closed session")
	}

	if code := restconfWrite(t, proc, "alice-lab-password"); code != http.StatusForbidden {
		t.Fatalf("RESTCONF write after demotion status = %d, want 403", code)
	}
}

func TestApplyTightAdmissionClosesRESTCONF(t *testing.T) {
	proc := startServe(t)
	if code := hostMeta(t, proc); code != http.StatusOK {
		t.Fatalf("host-meta before tighten = %d, want 200", code)
	}
	rev := getRuntimeRevision(t, proc)
	rev = postApply(t, proc, rev, "tighten", map[string]any{
		"op": "replaceAdmission",
		"admission": map[string]any{
			"allowClientCidrs": []string{"192.0.2.0/24"},
		},
	})
	if code := hostMeta(t, proc); code != http.StatusForbidden {
		t.Fatalf("RESTCONF after admission tighten status = %d, want 403", code)
	}
	if err := dialServePassword(t, proc.nc, "alice-lab-password"); err == nil {
		t.Fatal("SSH from loopback succeeded after admission excluded it")
	}

	postApply(t, proc, rev, "deny-all", map[string]any{
		"op": "replaceAdmission",
		"admission": map[string]any{
			"allowClientCidrs": []string{},
		},
	})
	if code := hostMeta(t, proc); code != http.StatusForbidden {
		t.Fatalf("RESTCONF after empty admission status = %d, want 403", code)
	}
}

func TestApplyPasswordReloadAndUnreadablePassword(t *testing.T) {
	proc := startServe(t)
	newPass := "new-alice-password-value"
	newFile := filepath.Join(filepath.Dir(proc.cfg), "alice.password.new")
	if err := os.WriteFile(newFile, []byte(newPass+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	alice := aliceFromState(t, proc)
	rev := getRuntimeRevision(t, proc)
	rev = postApply(t, proc, rev, "rotate-password", map[string]any{
		"op": "upsertUser",
		"user": map[string]any{
			"name":         "alice",
			"passwordFile": newFile,
			"profile":      alice.profile,
			"access":       "read-write",
		},
	})
	if err := dialServePassword(t, proc.nc, "alice-lab-password"); err == nil {
		t.Fatal("old password still authenticates after passwordFile reload")
	}
	if err := dialServePassword(t, proc.nc, newPass); err != nil {
		t.Fatalf("new password rejected after reload: %v", err)
	}

	live := openServeNETCONFPassword(t, proc.nc, newPass)
	if _, err := live.Handshake(nil); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(filepath.Dir(proc.cfg), "alice.password.missing")
	postApply(t, proc, rev, "missing-password", map[string]any{
		"op": "upsertUser",
		"user": map[string]any{
			"name":         "alice",
			"passwordFile": missing,
			"profile":      alice.profile,
			"access":       "read-write",
		},
	})
	err := dialServePassword(t, proc.nc, newPass)
	if err == nil || !strings.Contains(err.Error(), "ssh: unable to authenticate") {
		t.Fatalf("loopback dial after unreadable password = %v, want ssh: unable to authenticate", err)
	}
	if _, err := editHostname(live, "1"); err == nil {
		t.Fatal("NETCONF session opened before the failed reload stayed open")
	}
}

func TestAdmissionOnlyApplyKeepsSession(t *testing.T) {
	cfg, nc, rc, mgmt := writeServeFixture(t, true)
	raw, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(string(raw), "spec:\n", "spec:\n  netconf:\n    sharedProfileDatastore: false\n", 1)
	if body == string(raw) {
		t.Fatal("fixture has no spec block")
	}
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	proc := startServeCfg(t, cfg, nc, rc, mgmt)
	c := openServeNETCONF(t, proc.nc)
	if _, err := c.Handshake(nil); err != nil {
		t.Fatal(err)
	}
	rev := getRuntimeRevision(t, proc)
	postApply(t, proc, rev, "admission-keep", map[string]any{
		"op": "replaceAdmission",
		"admission": map[string]any{
			"allowClientCidrs": []string{"127.0.0.0/8", "::1/128", "192.0.2.0/24"},
		},
	})
	rep, err := editHostname(c, "1")
	if err != nil {
		t.Fatalf("admission-only apply closed the session: %v", err)
	}
	if !rep.OK {
		t.Fatalf("admission-only apply denied edit-config: %+v", rep.Errors)
	}
}

func TestReloadDataPlaneRelativePasswordFile(t *testing.T) {
	cfg, nc, rc, mgmt := writeServeFixture(t, true)
	raw, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(filepath.Dir(cfg), "alice.password")
	body := strings.Replace(string(raw), abs, "alice.password", 1)
	if body == string(raw) {
		t.Fatal("fixture passwordFile was not absolute")
	}
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	proc := startServeCfg(t, cfg, nc, rc, mgmt)
	rev := getRuntimeRevision(t, proc)
	postApply(t, proc, rev, "relative-admission", map[string]any{
		"op": "replaceAdmission",
		"admission": map[string]any{
			"allowClientCidrs": []string{"127.0.0.0/8", "::1/128", "192.0.2.0/24"},
		},
	})
	if err := dialServePassword(t, proc.nc, "alice-lab-password"); err != nil {
		t.Fatalf("relative passwordFile did not authenticate over SSH after apply: %v", err)
	}
	req, err := http.NewRequest(http.MethodGet, "http://"+proc.rc+"/restconf/data/ietf-system:system/hostname", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("alice", "alice-lab-password")
	req.Header.Set("Accept", "application/yang-data+json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("RESTCONF after relative password reload status = %d body %s", res.StatusCode, raw)
	}
}

// TestDataPlaneReloadSerializes is a consistency guard. The first reload
// reads the tight-admission snapshot and blocks, still holding the mutex,
// until a second reload is blocked on that same lock. The newer snapshot
// must be what the listeners serve after both calls return.
func TestDataPlaneReloadSerializes(t *testing.T) {
	cfg, _, _, _ := writeServeFixture(t, false)
	baseDir, err := filepath.Abs(filepath.Dir(cfg))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc, err := app.Boot(ctx, app.Options{BootstrapPath: cfg})
	if err != nil {
		t.Fatal(err)
	}
	snap := svc.Active()
	ncUsers, sshUsers, rcUsers, err := dataPlaneUsers(svc, snap, baseDir)
	if err != nil {
		t.Fatal(err)
	}
	ncs := ncserver.New(ncserver.Config{
		Users: ncUsers,
		HandleFor: func(username string) (datastore.Handle, bool) {
			if h, ok := svc.UserDatastore(username); ok {
				return h, true
			}
			live := svc.Active()
			if live == nil {
				return nil, false
			}
			u, ok := live.UserNamed(username)
			if !ok {
				return nil, false
			}
			return svc.Datastore(u.Profile)
		},
	})
	sshSrv, err := netconfssh.New(netconfssh.Config{
		Address:     "127.0.0.1:0",
		HostKeyFile: snap.HostKeyFile,
		Users:       sshUsers,
		AllowCIDRs:  sshCIDRs(snap),
		Handler:     ncs.Serve,
	})
	if err != nil {
		t.Fatal(err)
	}
	rcSrv, err := restconf.New(restconf.Config{
		Users:            rcUsers,
		AllowClientCidrs: restconfCIDRs(snap),
		HandleFor: func(username, profile string) (datastore.Handle, bool) {
			if h, ok := svc.UserDatastore(username); ok {
				return h, true
			}
			return svc.Datastore(profile)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	rcLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var releaseOnce sync.Once
	releaseCh := make(chan struct{})
	release := func() { releaseOnce.Do(func() { close(releaseCh) }) }
	t.Cleanup(func() {
		reloadDataPlaneHook = nil
		release()
		cancel()
		_ = sshSrv.Close()
		_ = rcLn.Close()
	})
	go func() { _ = sshSrv.Serve(ctx) }()
	go func() { _ = rcSrv.Serve(ctx, rcLn) }()

	var mu sync.Mutex
	svc.OnApply(func() {
		reloadDataPlane(svc, baseDir, ncs, sshSrv, rcSrv, &mu)
	})
	entered := make(chan struct{})
	var calls atomic.Int32
	reloadDataPlaneHook = func() {
		if calls.Add(1) == 1 {
			close(entered)
			<-releaseCh
		}
	}

	rev0 := string(snap.Revision)
	errCh1 := make(chan error, 1)
	go func() {
		_, err := svc.Apply(ctx, []app.ApplyOp{{
			Op:        app.OpReplaceAdmission,
			Admission: &model.AdmissionSpec{AllowClientCidrs: []string{"192.0.2.0/24"}},
		}}, rev0, "ser-tighten")
		errCh1 <- err
	}()
	select {
	case <-entered:
	case err := <-errCh1:
		t.Fatalf("first apply returned before the reload hook: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("first reload did not reach the hook")
	}

	passB := filepath.Join(baseDir, "alice.password.b")
	if err := os.WriteFile(passB, []byte("password-b-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rev1 := string(svc.Active().Revision)
	errCh2 := make(chan error, 1)
	go func() {
		_, err := svc.Apply(ctx, []app.ApplyOp{
			{
				Op:        app.OpReplaceAdmission,
				Admission: &model.AdmissionSpec{AllowClientCidrs: []string{"127.0.0.0/8", "::1/128"}},
			},
			{
				Op: app.OpUpsertUser,
				User: &model.UserSpec{
					Name:         "alice",
					PasswordFile: passB,
					Profile:      "router-a",
					Access:       model.UserAccessRead,
				},
			},
		}, rev1, "ser-newer")
		errCh2 <- err
	}()
	deadline := time.Now().Add(10 * time.Second)
	for reloadDataPlaneStacks() < 2 {
		select {
		case err := <-errCh2:
			t.Fatalf("second apply finished before overlapping reload: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("second reload did not block inside reloadDataPlane")
		}
		time.Sleep(10 * time.Millisecond)
	}
	release()
	if err := <-errCh1; err != nil {
		t.Fatal(err)
	}
	if err := <-errCh2; err != nil {
		t.Fatal(err)
	}

	if code := hostMeta(t, serveProc{rc: rcLn.Addr().String()}); code != http.StatusOK {
		t.Fatalf("host-meta after overlapping reloads = %d, want 200", code)
	}
	ncAddr := sshSrv.Addr().String()
	if err := dialServePassword(t, ncAddr, "alice-lab-password"); err == nil {
		t.Fatal("old password authenticated after the newer snapshot")
	}
	if err := dialServePassword(t, ncAddr, "password-b-value"); err != nil {
		t.Fatalf("new password rejected after the newer snapshot: %v", err)
	}
	c := openServeNETCONFPassword(t, ncAddr, "password-b-value")
	if _, err := c.Handshake(nil); err != nil {
		t.Fatal(err)
	}
	rep, err := editHostname(c, "1")
	if err != nil {
		t.Fatalf("edit-config: %v", err)
	}
	tag := ""
	if len(rep.Errors) > 0 {
		tag = rep.Errors[0].Tag
	}
	if len(rep.Errors) == 0 || tag != "access-denied" {
		t.Fatalf("newer access edit-config ok=%v tag=%q errors=%+v; want access-denied", rep.OK, tag, rep.Errors)
	}
}

func reloadDataPlaneStacks() int {
	buf := make([]byte, 64*1024)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return strings.Count(string(buf[:n]), "reloadDataPlane(")
		}
		buf = make([]byte, len(buf)*2)
	}
}

type serveProc struct {
	cfg, nc, rc, mgmt, token string
	stdout, stderr           *syncBuf
}

func startServe(t *testing.T) serveProc {
	t.Helper()
	cfg, nc, rc, mgmt := writeServeFixture(t, true)
	return startServeCfg(t, cfg, nc, rc, mgmt)
}

func startServeCfg(t *testing.T, cfg, nc, rc, mgmt string) serveProc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	var stdout, stderr syncBuf
	errCh := make(chan int, 1)
	go func() {
		errCh <- serveCmd(ctx, []string{
			"--config", cfg,
			"--netconf-listen", nc,
			"--restconf-listen", rc,
			"--management-listen", mgmt,
		}, &stdout, &stderr)
	}()
	waitHTTP(t, errCh, &stdout, &stderr, "http://"+mgmt+"/v1/health/ready")
	return serveProc{cfg: cfg, nc: nc, rc: rc, mgmt: mgmt, token: readToken(t, cfg), stdout: &stdout, stderr: &stderr}
}

func getRuntimeRevision(t *testing.T, proc serveProc) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://"+proc.mgmt+"/v1/state", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+proc.token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/state %d %s stderr=%s", res.StatusCode, raw, proc.stderr.String())
	}
	var st struct {
		RuntimeRevision string `json:"runtimeRevision"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	if st.RuntimeRevision == "" {
		t.Fatalf("state missing runtimeRevision: %s", raw)
	}
	return st.RuntimeRevision
}

type aliceUser struct {
	passwordFile string
	profile      string
}

func aliceFromState(t *testing.T, proc serveProc) aliceUser {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://"+proc.mgmt+"/v1/state", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+proc.token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/state %d %s", res.StatusCode, raw)
	}
	var st struct {
		Canonical struct {
			Spec struct {
				Users []struct {
					Name         string `json:"name"`
					PasswordFile string `json:"passwordFile"`
					Profile      string `json:"profile"`
				} `json:"users"`
			} `json:"spec"`
		} `json:"canonical"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	for _, u := range st.Canonical.Spec.Users {
		if u.Name == "alice" {
			return aliceUser{passwordFile: u.PasswordFile, profile: u.Profile}
		}
	}
	t.Fatal("state missing alice")
	return aliceUser{}
}

func postApply(t *testing.T, proc serveProc, rev, key string, op map[string]any) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"expectedRevision": rev,
		"operations":       []any{op},
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+proc.mgmt+"/v1/changes:apply", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+proc.token)
	req.Header.Set("Idempotency-Key", key)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("apply %s status %d body %s stderr=%s", key, res.StatusCode, raw, proc.stderr.String())
	}
	var out struct {
		RuntimeRevision string `json:"runtimeRevision"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.RuntimeRevision == "" {
		t.Fatalf("apply %s missing runtimeRevision: %s", key, raw)
	}
	return out.RuntimeRevision
}

func editHostname(c *nctest.Client, id string) (ncrpc.Reply, error) {
	return c.RPC(ncrpc.RPC{
		MessageID: id,
		Name:      ncrpc.OpEditConfig,
		Target:    "candidate",
		DefaultOp: "merge",
		Config:    []byte(`<system xmlns="urn:ietf:params:xml:ns:yang:ietf-system"><hostname>pwned</hostname></system>`),
	})
}

func hostMeta(t *testing.T, proc serveProc) int {
	t.Helper()
	res, err := http.Get("http://" + proc.rc + "/.well-known/host-meta")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	_, _ = io.Copy(io.Discard, res.Body)
	return res.StatusCode
}

func restconfWrite(t *testing.T, proc serveProc, password string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, "http://"+proc.rc+"/restconf/data/ietf-system:system/hostname", strings.NewReader(`{"ietf-system:hostname":"nope"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("alice", password)
	req.Header.Set("Content-Type", "application/yang-data+json")
	req.Header.Set("Accept", "application/yang-data+json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	_, _ = io.Copy(io.Discard, res.Body)
	return res.StatusCode
}

func dialServePassword(t *testing.T, addr, password string) error {
	t.Helper()
	pem, err := os.ReadFile(filepath.Join(repoRoot(t), "testdata", "keys", "labnetconf-hostkey"))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.ParsePrivateKey(pem)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ClientConfig{
		User:            "alice",
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()),
		Timeout:         5 * time.Second,
	}
	tcp, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return err
	}
	cc, chans, reqs, err := ssh.NewClientConn(tcp, addr, cfg)
	if err != nil {
		_ = tcp.Close()
		return err
	}
	_ = ssh.NewClient(cc, chans, reqs).Close()
	return nil
}
