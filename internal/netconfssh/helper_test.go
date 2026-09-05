package netconfssh

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func startSSH(t *testing.T, cfg Config) (*Server, string) {
	t.Helper()
	if cfg.Address == "" {
		cfg.Address = "127.0.0.1:0"
	}
	if cfg.HostKeyFile == "" {
		cfg.HostKeyFile = keyPath(t, "labnetconf-hostkey")
	}
	if cfg.Handler == nil {
		cfg.Handler = func(context.Context, string, string, io.ReadWriteCloser) error {
			return nil
		}
	}
	if len(cfg.Users) == 0 {
		cfg.Users = []User{{
			Name:               "alice",
			PasswordFile:       keyPath(t, "alice.password"),
			AuthorizedKeysFile: keyPath(t, "alice.pub"),
		}}
	}
	s, err := New(cfg)
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
		t.Fatal("listener has no address")
	}
	return s, s.Addr().String()
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

func sshDial(t *testing.T, addr, user, password string, signer ssh.Signer) (*ssh.Client, error) {
	t.Helper()
	cfg := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: hostKeyCallback(t),
		Timeout:         3 * time.Second,
	}
	if password != "" {
		cfg.Auth = append(cfg.Auth, ssh.Password(password))
	}
	if signer != nil {
		cfg.Auth = append(cfg.Auth, ssh.PublicKeys(signer))
	}
	tcp, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return nil, err
	}
	c, chans, reqs, err := ssh.NewClientConn(tcp, addr, cfg)
	if err != nil {
		_ = tcp.Close()
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

func aliceSigner(t *testing.T) ssh.Signer {
	t.Helper()
	pem, err := os.ReadFile(keyPath(t, "alice"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := ssh.ParsePrivateKey(pem)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func requestSubsystem(t *testing.T, client *ssh.Client, name string) error {
	t.Helper()
	sess, err := client.NewSession()
	if err != nil {
		return err
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess.RequestSubsystem(name)
}

func mustTCP(t *testing.T, addr string) *net.TCPAddr {
	t.Helper()
	a, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
