package nctest

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/ncrpc"
	"github.com/hilather/go-lab-netconf/internal/ncserver"
	"github.com/hilather/go-lab-netconf/internal/netconfssh"
)

func TestHelloGetConfigRunning1830(t *testing.T) {
	addr := startStack(t, "127.0.0.1:1830", []ncserver.User{{
		Name:       "alice",
		Profile:    "router-a",
		Access:     "read-write",
		Handle:     hostnameHandle(t, "router-a", "lab-rtr-a"),
		Namespaces: map[string]string{"ietf-system": "urn:ietf:params:xml:ns:yang:ietf-system"},
	}}, nil)
	if !strings.HasSuffix(addr, ":1830") {
		t.Fatalf("listen %s, want :1830", addr)
	}
	c := openNETCONF(t, addr, "alice", "alice-lab-password")
	hello, err := c.Handshake(nil)
	if err != nil {
		t.Fatal(err)
	}
	if hello.SessionID == "" {
		t.Fatal("missing session-id")
	}
	joined := strings.Join(hello.Capabilities, "\n")
	if !strings.Contains(joined, "urn:ietf:params:netconf:base:1.1") {
		t.Fatalf("caps = %v", hello.Capabilities)
	}
	if strings.Contains(joined, "writable-running") {
		t.Fatalf("advertised writable-running: %v", hello.Capabilities)
	}
	rep, err := c.RPC(ncrpc.RPC{MessageID: "102", Name: ncrpc.OpGetConfig, Source: "running"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Errors) != 0 {
		t.Fatalf("errors = %+v", rep.Errors)
	}
	if !bytes.Contains(rep.Data, []byte("lab-rtr-a")) {
		t.Fatalf("running = %s", rep.Data)
	}
}

func TestBadPasswordFails(t *testing.T) {
	addr := startStack(t, "127.0.0.1:0", []ncserver.User{{
		Name:    "alice",
		Profile: "router-a",
		Handle:  hostnameHandle(t, "router-a", "lab-rtr-a"),
	}}, nil)
	cfg := sshClientConfig(t, "alice", "wrong-password")
	_, err := dialSSH(t, addr, cfg)
	if err == nil {
		t.Fatal("bad password authenticated")
	}
}

func TestUnknownUserSSHAuthFail(t *testing.T) {
	addr := startStack(t, "127.0.0.1:0", []ncserver.User{{
		Name:    "alice",
		Profile: "router-a",
		Handle:  hostnameHandle(t, "router-a", "lab-rtr-a"),
	}}, nil)
	cfg := sshClientConfig(t, "nonesuch", "alice-lab-password")
	_, err := dialSSH(t, addr, cfg)
	if err == nil {
		t.Fatal("unknown user authenticated")
	}
}

func TestWrongSubsystemRejected(t *testing.T) {
	addr := startStack(t, "127.0.0.1:0", []ncserver.User{{
		Name:    "alice",
		Profile: "router-a",
		Handle:  hostnameHandle(t, "router-a", "lab-rtr-a"),
	}}, nil)
	cli, err := dialSSH(t, addr, sshClientConfig(t, "alice", "alice-lab-password"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cli.Close() }()
	sess, err := cli.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()
	if err := sess.RequestSubsystem("sftp"); err == nil {
		t.Fatal("subsystem sftp was accepted")
	}
}

func TestBothCredentialsFromFixtureShape(t *testing.T) {
	users := []ncserver.User{{
		Name:       "alice",
		Profile:    "router-a",
		Handle:     hostnameHandle(t, "router-a", "lab-rtr-a"),
		Namespaces: map[string]string{"ietf-system": "urn:ietf:params:xml:ns:yang:ietf-system"},
	}}
	sshUsers := []netconfssh.User{{
		Name:               "alice",
		PasswordFile:       keyPath(t, "alice.password"),
		AuthorizedKeysFile: keyPath(t, "alice.pub"),
	}}
	addr := startStack(t, "127.0.0.1:0", users, sshUsers)
	c := openNETCONF(t, addr, "alice", "alice-lab-password")
	if _, err := c.Handshake(nil); err != nil {
		t.Fatalf("password factor: %v", err)
	}
	pem, err := osRead(t, keyPath(t, "alice"))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := parseSigner(t, pem)
	if err != nil {
		t.Fatal(err)
	}
	cli, err := dialSSH(t, addr, sshClientConfigKey(t, "alice", signer))
	if err != nil {
		t.Fatalf("key factor: %v", err)
	}
	_ = cli.Close()
}

func TestManagementOffStillAnswers(t *testing.T) {
	addr := startStack(t, "127.0.0.1:0", []ncserver.User{{
		Name:       "alice",
		Profile:    "router-a",
		Handle:     hostnameHandle(t, "router-a", "lab-rtr-a"),
		Namespaces: map[string]string{"ietf-system": "urn:ietf:params:xml:ns:yang:ietf-system"},
	}}, nil)
	c := openNETCONF(t, addr, "alice", "alice-lab-password")
	if _, err := c.Handshake(nil); err != nil {
		t.Fatal(err)
	}
	rep, err := c.RPC(ncrpc.RPC{MessageID: "1", Name: ncrpc.OpGetConfig, Source: "running"})
	if err != nil || !bytes.Contains(rep.Data, []byte("lab-rtr-a")) {
		t.Fatalf("get-config = %+v err=%v", rep, err)
	}
}
