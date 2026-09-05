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
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-netconf/internal/ncrpc"
	"github.com/hilather/go-lab-netconf/internal/nctest"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/crypto/ssh"
)

func TestLiveCreateSubscriptionWaitReturnsRecord(t *testing.T) {
	cfg, ncAddr, rcAddr, mgmtAddr := writeServeFixture(t, true)
	token := readToken(t, cfg)
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

	c := openServeNETCONF(t, ncAddr)
	if _, err := c.Handshake(nil); err != nil {
		t.Fatal(err)
	}
	sub := mustServeRPC(t, c, ncrpc.RPC{MessageID: "1", Name: ncrpc.OpCreateSubscription, Stream: "NETCONF"})
	if !sub.OK {
		t.Fatalf("create-subscription = %+v", sub)
	}
	cfgXML := []byte(`<system xmlns="urn:ietf:params:xml:ns:yang:ietf-system"><hostname>ga-wait</hostname></system>`)
	if !mustServeRPC(t, c, ncrpc.RPC{MessageID: "2", Name: ncrpc.OpEditConfig, Target: "candidate", DefaultOp: "merge", Config: cfgXML}).OK {
		t.Fatal("edit-config")
	}
	if !mustServeRPC(t, c, ncrpc.RPC{MessageID: "3", Name: ncrpc.OpCommit}).OK {
		t.Fatal("commit")
	}

	req, err := http.NewRequest(http.MethodPost, "http://"+mgmtAddr+"/v1/notifications:wait", strings.NewReader(`{"profile":"router-a","timeout":"3s"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("wait %d %s stdout=%q stderr=%q", resp.StatusCode, body, stdout.String(), stderr.String())
	}
	if strings.Contains(string(body), "wait_timeout") {
		t.Fatalf("wait timed out: %s", body)
	}
	var rec map[string]any
	if err := json.Unmarshal(body, &rec); err != nil {
		t.Fatal(err)
	}
	if rec["id"] == nil || rec["id"] == "" {
		t.Fatalf("wait record %s", body)
	}
	if rec["profile"] != "router-a" {
		t.Fatalf("profile %s", body)
	}

	mcpRec := mcpWait(t, "http://"+mgmtAddr+"/mcp", token)
	if mcpRec["id"] == nil || mcpRec["id"] == "" {
		t.Fatalf("mcp wait %v", mcpRec)
	}
}

func TestServeUIWhenEnabled(t *testing.T) {
	cfg, ncAddr, rcAddr, mgmtAddr := writeServeFixture(t, true)
	enableUI(t, cfg)
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
	resp, err := http.Get("http://" + mgmtAddr + "/")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / %d %s stderr=%q", resp.StatusCode, raw, stderr.String())
	}
	if !bytes.Contains(raw, []byte("LabNETCONF")) {
		t.Fatalf("GET / body %s", raw)
	}
}

func enableUI(t *testing.T, cfg string) {
	t.Helper()
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := strings.Replace(string(b), "enabled: false", "enabled: true", 1)
	if err := os.WriteFile(cfg, []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readToken(t *testing.T, cfg string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(filepath.Dir(cfg), "token"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

func openServeNETCONF(t *testing.T, addr string) *nctest.Client {
	t.Helper()
	root := repoRoot(t)
	pem, err := os.ReadFile(filepath.Join(root, "testdata", "keys", "labnetconf-hostkey"))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.ParsePrivateKey(pem)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ClientConfig{
		User:            "alice",
		Auth:            []ssh.AuthMethod{ssh.Password("alice-lab-password")},
		HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()),
		Timeout:         5 * time.Second,
	}
	tcp, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	cc, chans, reqs, err := ssh.NewClientConn(tcp, addr, cfg)
	if err != nil {
		_ = tcp.Close()
		t.Fatal(err)
	}
	cli := ssh.NewClient(cc, chans, reqs)
	sess, err := cli.NewSession()
	if err != nil {
		_ = cli.Close()
		t.Fatal(err)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		_ = cli.Close()
		t.Fatal(err)
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		_ = cli.Close()
		t.Fatal(err)
	}
	if err := sess.RequestSubsystem("netconf"); err != nil {
		_ = cli.Close()
		t.Fatal(err)
	}
	rw := &serveSSHRW{in: stdin, out: stdout, sess: sess, cli: cli}
	t.Cleanup(func() { _ = rw.Close() })
	return nctest.New(rw)
}

type serveSSHRW struct {
	in   io.WriteCloser
	out  io.Reader
	sess *ssh.Session
	cli  *ssh.Client
}

func (s *serveSSHRW) Read(p []byte) (int, error)  { return s.out.Read(p) }
func (s *serveSSHRW) Write(p []byte) (int, error) { return s.in.Write(p) }
func (s *serveSSHRW) Close() error {
	_ = s.sess.Close()
	return s.cli.Close()
}

func mustServeRPC(t *testing.T, c *nctest.Client, r ncrpc.RPC) ncrpc.Reply {
	t.Helper()
	rep, err := c.RPC(r)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func mcpWait(t *testing.T, endpoint, token string) map[string]any {
	t.Helper()
	client := sdk.NewClient(&sdk.Implementation{Name: "labnetconf-ga", Version: "dev"}, nil)
	session, err := client.Connect(t.Context(), &sdk.StreamableClientTransport{
		Endpoint:             endpoint,
		DisableStandaloneSSE: true,
		HTTPClient:           &http.Client{Transport: bearerRT{token: token}},
	}, nil)
	if err != nil {
		t.Fatalf("mcp connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	res, err := session.CallTool(t.Context(), &sdk.CallToolParams{
		Name:      "netconf_notifications_wait",
		Arguments: map[string]any{"profile": "router-a", "timeout": "3s"},
	})
	if err != nil {
		t.Fatalf("mcp wait: %v", err)
	}
	if res.IsError {
		t.Fatalf("mcp wait error %+v", res)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("mcp structured %s: %v", raw, err)
	}
	if strings.Contains(string(raw), "wait_timeout") {
		t.Fatalf("mcp wait timed out: %s", raw)
	}
	return out
}

type bearerRT struct{ token string }

func (b bearerRT) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(clone)
}
