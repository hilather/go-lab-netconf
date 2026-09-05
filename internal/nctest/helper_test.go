package nctest

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hilather/go-lab-netconf/internal/datastore"
	"github.com/hilather/go-lab-netconf/internal/model"
	"github.com/hilather/go-lab-netconf/internal/ncserver"
	"github.com/hilather/go-lab-netconf/internal/netconfssh"
	"github.com/hilather/go-lab-netconf/internal/notif"
	"github.com/hilather/go-lab-netconf/internal/yangtree"
	"golang.org/x/crypto/ssh"
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

func keyPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "testdata", "keys", name)
}

type sshRW struct {
	in   io.WriteCloser
	out  io.Reader
	sess *ssh.Session
	cli  *ssh.Client
}

func (s *sshRW) Read(p []byte) (int, error)  { return s.out.Read(p) }
func (s *sshRW) Write(p []byte) (int, error) { return s.in.Write(p) }
func (s *sshRW) Close() error {
	_ = s.sess.Close()
	return s.cli.Close()
}

func hostKeyCallback(t *testing.T) ssh.HostKeyCallback {
	t.Helper()
	pem, err := os.ReadFile(keyPath(t, "labnetconf-hostkey"))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.ParsePrivateKey(pem)
	if err != nil {
		t.Fatal(err)
	}
	return ssh.FixedHostKey(signer.PublicKey())
}

func openNETCONF(t *testing.T, addr, user, password string) *Client {
	t.Helper()
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: hostKeyCallback(t),
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
	rw := &sshRW{in: stdin, out: stdout, sess: sess, cli: cli}
	t.Cleanup(func() { _ = rw.Close() })
	return New(rw)
}

func hostnameHandle(t *testing.T, profile, hostname string) datastore.Handle {
	t.Helper()
	n, err := yangtree.Compile(
		map[string]any{"ietf-system": map[string]any{"system": map[string]any{"hostname": hostname}}},
		[]model.SchemaLeaf{
			{Path: "ietf-system:system", Type: "container"},
			{Path: "ietf-system:system/hostname", Type: "string", Access: "write"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return datastore.New(profile, n, datastore.Node{}, datastore.Node{}, notif.Nop{})
}

func startStack(t *testing.T, addr string, users []ncserver.User, sshUsers []netconfssh.User) string {
	t.Helper()
	ncs := ncserver.New(ncserver.Config{Users: users, Sink: notif.Nop{}})
	if len(sshUsers) == 0 {
		sshUsers = []netconfssh.User{{
			Name:               "alice",
			PasswordFile:       keyPath(t, "alice.password"),
			AuthorizedKeysFile: keyPath(t, "alice.pub"),
		}}
	}
	s, err := netconfssh.New(netconfssh.Config{
		Address:     addr,
		HostKeyFile: keyPath(t, "labnetconf-hostkey"),
		Users:       sshUsers,
		Handler:     ncs.Serve,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = s.Close()
	})
	go func() { _ = s.Serve(ctx) }()
	deadline := time.Now().Add(2 * time.Second)
	for s.Addr() == nil && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if s.Addr() == nil {
		t.Fatal("no listen address")
	}
	return s.Addr().String()
}

func sshClientConfig(t *testing.T, user, password string) *ssh.ClientConfig {
	t.Helper()
	return &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: hostKeyCallback(t),
		Timeout:         5 * time.Second,
	}
}

func sshClientConfigKey(t *testing.T, user string, signer ssh.Signer) *ssh.ClientConfig {
	t.Helper()
	return &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKeyCallback(t),
		Timeout:         5 * time.Second,
	}
}

func dialSSH(t *testing.T, addr string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
	t.Helper()
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

func osRead(t *testing.T, path string) ([]byte, error) {
	t.Helper()
	return os.ReadFile(path)
}

func parseSigner(t *testing.T, pem []byte) (ssh.Signer, error) {
	t.Helper()
	return ssh.ParsePrivateKey(pem)
}
