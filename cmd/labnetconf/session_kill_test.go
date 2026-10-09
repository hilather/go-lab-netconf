package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-netconf/internal/ncrpc"
	"github.com/hilather/go-lab-netconf/internal/nctest"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/crypto/ssh"
)

func TestAdminKillClosesNETCONFAndKeepsSSH(t *testing.T) {
	proc, reader := startKillServe(t)
	cli := dialServeClient(t, proc.nc, "alice", "alice-lab-password", nil)
	t.Cleanup(func() { _ = cli.Close() })
	a := openNetconf(t, cli)
	hello, err := a.Handshake(nil)
	if err != nil {
		t.Fatal(err)
	}
	if hello.SessionID == "" {
		t.Fatal("server hello missing session-id")
	}

	items := mustListSessions(t, proc, proc.token)
	got, ok := findSession(items, hello.SessionID)
	if !ok {
		t.Fatalf("GET /v1/sessions = %+v, want id %s", items, hello.SessionID)
	}
	if got.User != "alice" || got.Profile != "router-a" {
		t.Fatalf("session row = %+v, want alice router-a", got)
	}

	mcpSess := connectMCP(t, "http://"+proc.mgmt+"/mcp", proc.token)
	mcpItems := mustMCPSessions(t, mcpSess)
	if _, ok := findSession(mcpItems, hello.SessionID); !ok {
		t.Fatalf("mcp netconf_sessions_list = %+v, want id %s", mcpItems, hello.SessionID)
	}

	lockRep, err := a.RPC(ncrpc.RPC{MessageID: "lk", Name: ncrpc.OpLock, Target: ncrpc.StoreCandidate})
	if err != nil || !lockRep.OK {
		t.Fatalf("lock before kill: %v %+v", err, lockRep.Errors)
	}

	status, body := postKill(t, proc, reader, hello.SessionID)
	if status != http.StatusForbidden {
		t.Fatalf("reader kill = %d %s, want 403", status, body)
	}
	status, body = postKill(t, proc, proc.token, "missing")
	if status != http.StatusNotFound || !strings.Contains(body, "not_found") {
		t.Fatalf("unknown kill = %d %s, want 404 not_found", status, body)
	}
	mcpMissing := mcpKill(t, mcpSess, "missing")
	raw, _ := json.Marshal(mcpMissing.StructuredContent)
	if !mcpMissing.IsError || !strings.Contains(string(raw), "not_found") {
		t.Fatalf("mcp kill missing = error:%v %s", mcpMissing.IsError, raw)
	}

	status, body = postKill(t, proc, proc.token, hello.SessionID)
	if status != http.StatusOK {
		t.Fatalf("admin kill = %d %s stderr=%s", status, body, proc.stderr.String())
	}
	expectNetconfEOF(t, a)

	b := openNetconf(t, cli)
	helloB, err := b.Handshake(nil)
	if err != nil {
		t.Fatalf("new NETCONF channel on the same SSH connection: %v", err)
	}
	if helloB.SessionID == "" || helloB.SessionID == hello.SessionID {
		t.Fatalf("new session id = %q, killed id = %q", helloB.SessionID, hello.SessionID)
	}
	waitKilledAndLocked(t, proc, hello.SessionID, b)

	mcpItems = mustMCPSessions(t, mcpSess)
	if _, ok := findSession(mcpItems, hello.SessionID); ok {
		t.Fatalf("mcp still lists killed session %+v", mcpItems)
	}
	if row, ok := findSession(mcpItems, helloB.SessionID); !ok || row.User != "alice" || row.Profile != "router-a" {
		t.Fatalf("mcp list after kill = %+v, want id %s alice router-a", mcpItems, helloB.SessionID)
	}

	killed := mcpKill(t, mcpSess, helloB.SessionID)
	if killed.IsError {
		raw, _ := json.Marshal(killed.StructuredContent)
		t.Fatalf("mcp netconf_session_kill: %s", raw)
	}
	expectNetconfEOF(t, b)
	c := openNetconf(t, cli)
	if _, err := c.Handshake(nil); err != nil {
		t.Fatalf("SSH connection died after session kill: %v", err)
	}
	waitSessionAbsent(t, proc, helloB.SessionID)
}

func TestPasswordChangeDropsSSHKeepsOtherUser(t *testing.T) {
	proc := startPairServe(t)
	aliceCli := dialServeClient(t, proc.nc, "alice", "alice-lab-password", nil)
	t.Cleanup(func() { _ = aliceCli.Close() })
	aliceNC := openNetconf(t, aliceCli)
	hello, err := aliceNC.Handshake(nil)
	if err != nil {
		t.Fatal(err)
	}
	if hello.SessionID == "" {
		t.Fatal("server hello missing session-id")
	}
	bobCli := dialServeClient(t, proc.nc, "bob", "bob-lab-password", nil)
	t.Cleanup(func() { _ = bobCli.Close() })
	bobNC := openNetconf(t, bobCli)
	if _, err := bobNC.Handshake(nil); err != nil {
		t.Fatal(err)
	}
	lockRep, err := aliceNC.RPC(ncrpc.RPC{MessageID: "lk", Name: ncrpc.OpLock, Target: ncrpc.StoreCandidate})
	if err != nil || !lockRep.OK {
		t.Fatalf("lock before rotation: %v %+v", err, lockRep.Errors)
	}

	alice := aliceFromState(t, proc)
	const nextPass = "alice-rotated-password"
	if err := os.WriteFile(alice.passwordFile, []byte(nextPass+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rev := getRuntimeRevision(t, proc)
	postApply(t, proc, rev, "rotate-alice-password", map[string]any{
		"op": "upsertUser",
		"user": map[string]any{
			"name":         "alice",
			"passwordFile": alice.passwordFile,
			"profile":      alice.profile,
			"access":       "read-write",
		},
	})

	expectSSHClosed(t, aliceCli)
	expectNetconfEOF(t, aliceNC)
	expectNetconfWorks(t, bobNC, "g")
	waitKilledAndLocked(t, proc, hello.SessionID, bobNC)
	bobExtra, err := bobCli.NewSession()
	if err != nil {
		t.Fatalf("bob SSH connection closed when only alice's password changed: %v", err)
	}
	_ = bobExtra.Close()

	if c, err := dialServeAuth(t, proc.nc, "alice", "alice-lab-password", nil); err == nil {
		_ = c.Close()
		t.Fatal("old alice password still authenticates")
	}
	fresh := dialServeClient(t, proc.nc, "alice", nextPass, nil)
	t.Cleanup(func() { _ = fresh.Close() })
	freshNC := openNetconf(t, fresh)
	if _, err := freshNC.Handshake(nil); err != nil {
		t.Fatalf("reconnect with the new password: %v", err)
	}
}

func TestAuthorizedKeyChangeDropsSSHKeepsOtherUser(t *testing.T) {
	aliceSigner := labSigner(t, "alice")
	otherSigner, otherLine := newKeyLine(t)
	pub := strings.TrimSpace(string(mustRead(t, filepath.Join(repoRoot(t), "testdata", "keys", "alice.pub"))))
	proc, keyFile := startKeyPairServe(t, pub+"\n")

	aliceCli := dialServeClient(t, proc.nc, "alice", "", aliceSigner)
	t.Cleanup(func() { _ = aliceCli.Close() })
	aliceNC := openNetconf(t, aliceCli)
	hello, err := aliceNC.Handshake(nil)
	if err != nil {
		t.Fatal(err)
	}
	if hello.SessionID == "" {
		t.Fatal("server hello missing session-id")
	}
	bobCli := dialServeClient(t, proc.nc, "bob", "bob-lab-password", nil)
	t.Cleanup(func() { _ = bobCli.Close() })
	bobNC := openNetconf(t, bobCli)
	if _, err := bobNC.Handshake(nil); err != nil {
		t.Fatal(err)
	}
	lockRep, err := aliceNC.RPC(ncrpc.RPC{MessageID: "lk", Name: ncrpc.OpLock, Target: ncrpc.StoreCandidate})
	if err != nil || !lockRep.OK {
		t.Fatalf("lock before rotation: %v %+v", err, lockRep.Errors)
	}

	if err := os.WriteFile(keyFile, []byte(otherLine+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rev := getRuntimeRevision(t, proc)
	postApply(t, proc, rev, "replace-alice-key", map[string]any{
		"op": "upsertUser",
		"user": map[string]any{
			"name":               "alice",
			"authorizedKeysFile": keyFile,
			"profile":            "router-a",
			"access":             "read-write",
		},
	})

	expectSSHClosed(t, aliceCli)
	expectNetconfEOF(t, aliceNC)
	expectNetconfWorks(t, bobNC, "g")
	waitKilledAndLocked(t, proc, hello.SessionID, bobNC)
	bobExtra, err := bobCli.NewSession()
	if err != nil {
		t.Fatalf("bob SSH connection closed when only alice's key changed: %v", err)
	}
	_ = bobExtra.Close()
	if c, err := dialServeAuth(t, proc.nc, "alice", "", aliceSigner); err == nil {
		_ = c.Close()
		t.Fatal("replaced alice key still authenticates")
	}
	fresh := dialServeClient(t, proc.nc, "alice", "", otherSigner)
	t.Cleanup(func() { _ = fresh.Close() })
	freshNC := openNetconf(t, fresh)
	if _, err := freshNC.Handshake(nil); err != nil {
		t.Fatalf("reconnect with the replacement key: %v", err)
	}
}

func TestAuthorizedKeyOrderKeepsSSH(t *testing.T) {
	aliceSigner := labSigner(t, "alice")
	_, otherLine := newKeyLine(t)
	pub := strings.TrimSpace(string(mustRead(t, filepath.Join(repoRoot(t), "testdata", "keys", "alice.pub"))))
	original := pub + "\n\n" + otherLine + "\n"
	proc, keyFile := startKeyPairServe(t, original)

	aliceCli := dialServeClient(t, proc.nc, "alice", "", aliceSigner)
	t.Cleanup(func() { _ = aliceCli.Close() })
	aliceNC := openNetconf(t, aliceCli)
	if _, err := aliceNC.Handshake(nil); err != nil {
		t.Fatal(err)
	}
	bobCli := dialServeClient(t, proc.nc, "bob", "bob-lab-password", nil)
	t.Cleanup(func() { _ = bobCli.Close() })
	bobNC := openNetconf(t, bobCli)
	if _, err := bobNC.Handshake(nil); err != nil {
		t.Fatal(err)
	}

	reordered := otherLine + "\n" + pub + "\n"
	if err := os.WriteFile(keyFile, []byte(reordered), 0o600); err != nil {
		t.Fatal(err)
	}
	rev := getRuntimeRevision(t, proc)
	postApply(t, proc, rev, "reorder-alice-keys", map[string]any{
		"op": "upsertUser",
		"user": map[string]any{
			"name":               "alice",
			"authorizedKeysFile": keyFile,
			"profile":            "router-a",
			"access":             "read-write",
		},
	})

	expectNetconfWorks(t, aliceNC, "g")
	extra, err := aliceCli.NewSession()
	if err != nil {
		t.Fatalf("key reorder closed alice's SSH connection: %v", err)
	}
	_ = extra.Close()
	expectNetconfWorks(t, bobNC, "g2")
}

func TestResetClosesSSHWhenPasswordFileChanges(t *testing.T) {
	proc := startServe(t)
	cli := dialServeClient(t, proc.nc, "alice", "alice-lab-password", nil)
	t.Cleanup(func() { _ = cli.Close() })
	nc := openNetconf(t, cli)
	if _, err := nc.Handshake(nil); err != nil {
		t.Fatal(err)
	}
	alice := aliceFromState(t, proc)
	const nextPass = "alice-reset-password"
	if err := os.WriteFile(alice.passwordFile, []byte(nextPass+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	status, body := postJSON(t, proc, proc.token, http.MethodPost, "/v1/state:reset", `{"reason":"cred"}`)
	if status != http.StatusOK {
		t.Fatalf("reset = %d %s stderr=%s", status, body, proc.stderr.String())
	}
	expectSSHClosed(t, cli)
	if c, err := dialServeAuth(t, proc.nc, "alice", "alice-lab-password", nil); err == nil {
		_ = c.Close()
		t.Fatal("old password authenticates after reset reloaded the password file")
	}
	fresh, err := dialServeAuth(t, proc.nc, "alice", nextPass, nil)
	if err != nil {
		t.Fatalf("new password after reset: %v", err)
	}
	_ = fresh.Close()
}

func startKillServe(t *testing.T) (serveProc, string) {
	t.Helper()
	cfg, nc, rc, mgmt := writeServeFixture(t, true)
	dir := filepath.Dir(cfg)
	readerFile := filepath.Join(dir, "reader.token")
	const readerSecret = "abcdef0123456789abcdef0123456789"
	if err := os.WriteFile(readerFile, []byte(readerSecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	needle := "      - id: admin\n"
	insert := "      - id: reader\n        role: reader\n        secretFile: " + fmt.Sprintf("%q", readerFile) + "\n" + needle
	body := strings.Replace(string(raw), needle, insert, 1)
	if body == string(raw) {
		t.Fatal("admin token block not found")
	}
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return startServeCfg(t, cfg, nc, rc, mgmt), readerSecret
}

func startPairServe(t *testing.T) serveProc {
	t.Helper()
	cfg, nc, rc, mgmt := writeServeFixture(t, true)
	bobFile := filepath.Join(filepath.Dir(cfg), "bob.password")
	if err := os.WriteFile(bobFile, []byte("bob-lab-password\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	body += fmt.Sprintf("    - name: bob\n      passwordFile: %q\n      profile: router-a\n      access: read-write\n", bobFile)
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return startServeCfg(t, cfg, nc, rc, mgmt)
}

func startKeyPairServe(t *testing.T, authorized string) (serveProc, string) {
	t.Helper()
	cfg, nc, rc, mgmt := writeServeFixture(t, true)
	dir := filepath.Dir(cfg)
	keyFile := filepath.Join(dir, "alice.authorized_keys")
	if err := os.WriteFile(keyFile, []byte(authorized), 0o600); err != nil {
		t.Fatal(err)
	}
	bobFile := filepath.Join(dir, "bob.password")
	if err := os.WriteFile(bobFile, []byte("bob-lab-password\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	oldPass := filepath.Join(dir, "alice.password")
	body := strings.Replace(string(raw),
		"passwordFile: "+fmt.Sprintf("%q", oldPass),
		"authorizedKeysFile: "+fmt.Sprintf("%q", keyFile),
		1)
	if body == string(raw) {
		t.Fatal("alice passwordFile line not found")
	}
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	body += fmt.Sprintf("    - name: bob\n      passwordFile: %q\n      profile: router-a\n      access: read-write\n", bobFile)
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return startServeCfg(t, cfg, nc, rc, mgmt), keyFile
}

type sessionItem struct {
	ID      string `json:"id"`
	User    string `json:"user"`
	Profile string `json:"profile"`
}

func mustListSessions(t *testing.T, proc serveProc, token string) []sessionItem {
	t.Helper()
	status, body := getAuth(t, proc, token, "/v1/sessions")
	if status != http.StatusOK {
		t.Fatalf("GET /v1/sessions = %d %s", status, body)
	}
	var out struct {
		Items []sessionItem `json:"items"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatal(err)
	}
	return out.Items
}

func findSession(items []sessionItem, id string) (sessionItem, bool) {
	for _, it := range items {
		if it.ID == id {
			return it, true
		}
	}
	return sessionItem{}, false
}

func waitSessionAbsent(t *testing.T, proc serveProc, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last []sessionItem
	for {
		last = mustListSessions(t, proc, proc.token)
		if _, ok := findSession(last, id); !ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("session %s still listed: %+v", id, last)
		}
		time.Sleep(15 * time.Millisecond)
	}
}

func waitKilledAndLocked(t *testing.T, proc serveProc, killed string, next *nctest.Client) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	absent := false
	locked := false
	var last string
	for time.Now().Before(deadline) {
		if !absent {
			items := mustListSessions(t, proc, proc.token)
			_, still := findSession(items, killed)
			absent = !still
		}
		if !locked {
			rep, err := next.RPC(ncrpc.RPC{MessageID: "lk2", Name: ncrpc.OpLock, Target: ncrpc.StoreCandidate})
			if err == nil && rep.OK {
				locked = true
			} else if err != nil {
				last = err.Error()
			} else if len(rep.Errors) > 0 {
				last = rep.Errors[0].Tag
			}
		}
		if absent && locked {
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatalf("after kill absent=%v locked=%v last=%s", absent, locked, last)
}

func postKill(t *testing.T, proc serveProc, token, id string) (int, string) {
	t.Helper()
	return postJSON(t, proc, token, http.MethodPost, "/v1/sessions/"+id+":kill", `{}`)
}

func getAuth(t *testing.T, proc serveProc, token, path string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://"+proc.mgmt+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(raw)
}

func postJSON(t *testing.T, proc serveProc, token, method, path, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, "http://"+proc.mgmt+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(raw)
}

func connectMCP(t *testing.T, endpoint, token string) *sdk.ClientSession {
	t.Helper()
	client := sdk.NewClient(&sdk.Implementation{Name: "labnetconf-sessions", Version: "dev"}, nil)
	session, err := client.Connect(t.Context(), &sdk.StreamableClientTransport{
		Endpoint:             endpoint,
		DisableStandaloneSSE: true,
		HTTPClient:           &http.Client{Transport: bearerRT{token: token}},
	}, nil)
	if err != nil {
		t.Fatalf("mcp connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func mustMCPSessions(t *testing.T, session *sdk.ClientSession) []sessionItem {
	t.Helper()
	res, err := session.CallTool(t.Context(), &sdk.CallToolParams{
		Name:      "netconf_sessions_list",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("mcp list: %v", err)
	}
	if res.IsError {
		raw, _ := json.Marshal(res.StructuredContent)
		t.Fatalf("mcp list error %s", raw)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Items []sessionItem `json:"items"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("mcp sessions %s: %v", raw, err)
	}
	return out.Items
}

func mcpKill(t *testing.T, session *sdk.ClientSession, id string) *sdk.CallToolResult {
	t.Helper()
	res, err := session.CallTool(t.Context(), &sdk.CallToolParams{
		Name:      "netconf_session_kill",
		Arguments: map[string]any{"id": id},
	})
	if err != nil {
		t.Fatalf("mcp kill %s: %v", id, err)
	}
	return res
}

func dialServeClient(t *testing.T, addr, user, password string, signer ssh.Signer) *ssh.Client {
	t.Helper()
	c, err := dialServeAuth(t, addr, user, password, signer)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func dialServeAuth(t *testing.T, addr, user, password string, signer ssh.Signer) (*ssh.Client, error) {
	t.Helper()
	pem, err := os.ReadFile(filepath.Join(repoRoot(t), "testdata", "keys", "labnetconf-hostkey"))
	if err != nil {
		t.Fatal(err)
	}
	host, err := ssh.ParsePrivateKey(pem)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: ssh.FixedHostKey(host.PublicKey()),
		Timeout:         5 * time.Second,
	}
	if password != "" {
		cfg.Auth = append(cfg.Auth, ssh.Password(password))
	}
	if signer != nil {
		cfg.Auth = append(cfg.Auth, ssh.PublicKeys(signer))
	}
	tcp, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	cc, chans, reqs, err := ssh.NewClientConn(tcp, addr, cfg)
	if err != nil {
		_ = tcp.Close()
		return nil, err
	}
	return ssh.NewClient(cc, chans, reqs), nil
}

func openNetconf(t *testing.T, cli *ssh.Client) *nctest.Client {
	t.Helper()
	sess, err := cli.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		_ = sess.Close()
		t.Fatal(err)
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		_ = sess.Close()
		t.Fatal(err)
	}
	if err := sess.RequestSubsystem("netconf"); err != nil {
		_ = sess.Close()
		t.Fatal(err)
	}
	rw := &netconfRW{in: stdin, out: stdout, sess: sess}
	t.Cleanup(func() { _ = rw.Close() })
	return nctest.New(rw)
}

type netconfRW struct {
	in   io.WriteCloser
	out  io.Reader
	sess *ssh.Session
}

func (n *netconfRW) Read(p []byte) (int, error)  { return n.out.Read(p) }
func (n *netconfRW) Write(p []byte) (int, error) { return n.in.Write(p) }
func (n *netconfRW) Close() error                { return n.sess.Close() }

func expectNetconfWorks(t *testing.T, c *nctest.Client, id string) {
	t.Helper()
	rep, err := c.RPC(ncrpc.RPC{MessageID: id, Name: ncrpc.OpGet})
	if err != nil {
		t.Fatalf("get %s: %v", id, err)
	}
	if len(rep.Errors) > 0 || len(rep.Data) == 0 {
		t.Fatalf("get %s ok=%v errors=%+v data=%d", id, rep.OK, rep.Errors, len(rep.Data))
	}
}

func expectNetconfEOF(t *testing.T, c *nctest.Client) {
	t.Helper()
	_, err := c.RPC(ncrpc.RPC{MessageID: "eof", Name: ncrpc.OpGet})
	if err == nil {
		t.Fatal("NETCONF session still answered an RPC")
	}
	if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("NETCONF session error = %v, want EOF", err)
	}
}

func expectSSHClosed(t *testing.T, cli *ssh.Client) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		sess, err := cli.NewSession()
		if err != nil {
			return
		}
		_ = sess.Close()
		if time.Now().After(deadline) {
			t.Fatal("SSH connection still accepts sessions")
		}
		time.Sleep(15 * time.Millisecond)
	}
}

func labSigner(t *testing.T, name string) ssh.Signer {
	t.Helper()
	pem := mustRead(t, filepath.Join(repoRoot(t), "testdata", "keys", name))
	s, err := ssh.ParsePrivateKey(pem)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func newKeyLine(t *testing.T) (ssh.Signer, string) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	return signer, line
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
