package netconfssh

import (
	"context"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestPasswordAuthSucceeds(t *testing.T) {
	got := make(chan string, 1)
	_, addr := startSSH(t, Config{
		Handler: func(_ context.Context, user, _ string, ch io.ReadWriteCloser) error {
			got <- user
			_, _ = ch.Write([]byte("ready"))
			return nil
		},
	})
	c, err := sshDial(t, addr, "alice", "alice-lab-password", nil)
	if err != nil {
		t.Fatalf("password auth: %v", err)
	}
	defer c.Close()
	if err := requestSubsystem(t, c, "netconf"); err != nil {
		t.Fatalf("subsystem netconf: %v", err)
	}
	select {
	case u := <-got:
		if u != "alice" {
			t.Fatalf("handler user = %q", u)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("handler was not invoked")
	}
}

func TestPublicKeyAuthSucceeds(t *testing.T) {
	got := make(chan string, 1)
	_, addr := startSSH(t, Config{
		Handler: func(_ context.Context, user, _ string, ch io.ReadWriteCloser) error {
			got <- user
			return nil
		},
	})
	c, err := sshDial(t, addr, "alice", "", aliceSigner(t))
	if err != nil {
		t.Fatalf("publickey auth: %v", err)
	}
	defer c.Close()
	if err := requestSubsystem(t, c, "netconf"); err != nil {
		t.Fatalf("subsystem netconf: %v", err)
	}
	select {
	case u := <-got:
		if u != "alice" {
			t.Fatalf("handler user = %q", u)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("handler was not invoked")
	}
}

func TestBothCredentialsPasswordAndKey(t *testing.T) {
	_, addr := startSSH(t, Config{
		Users: []User{{
			Name:               "alice",
			PasswordFile:       keyPath(t, "alice.password"),
			AuthorizedKeysFile: keyPath(t, "alice.pub"),
		}},
	})
	c, err := sshDial(t, addr, "alice", "alice-lab-password", nil)
	if err != nil {
		t.Fatalf("password factor: %v", err)
	}
	_ = c.Close()
	c, err = sshDial(t, addr, "alice", "", aliceSigner(t))
	if err != nil {
		t.Fatalf("key factor: %v", err)
	}
	_ = c.Close()
}

func TestBadPasswordFails(t *testing.T) {
	called := false
	_, addr := startSSH(t, Config{
		Handler: func(context.Context, string, string, io.ReadWriteCloser) error {
			called = true
			return nil
		},
	})
	c, err := sshDial(t, addr, "alice", "wrong-password", nil)
	if err == nil {
		_ = c.Close()
		t.Fatal("bad password authenticated")
	}
	if called {
		t.Fatal("handler ran after bad password")
	}
}

func TestUnknownUserSSHAuthFail(t *testing.T) {
	called := false
	_, addr := startSSH(t, Config{
		Handler: func(context.Context, string, string, io.ReadWriteCloser) error {
			called = true
			return nil
		},
	})
	c, err := sshDial(t, addr, "nonesuch", "alice-lab-password", nil)
	if err == nil {
		_ = c.Close()
		t.Fatal("unknown user authenticated")
	}
	if called {
		t.Fatal("handler ran for unknown user")
	}
}

func TestWrongSubsystemRejected(t *testing.T) {
	called := false
	_, addr := startSSH(t, Config{
		Handler: func(context.Context, string, string, io.ReadWriteCloser) error {
			called = true
			return nil
		},
	})
	c, err := sshDial(t, addr, "alice", "alice-lab-password", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := requestSubsystem(t, c, "sftp"); err == nil {
		t.Fatal("subsystem sftp was accepted")
	}
	if called {
		t.Fatal("handler ran for subsystem sftp")
	}
	c2, err := sshDial(t, addr, "alice", "alice-lab-password", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	if err := requestSubsystem(t, c2, "netconf"); err != nil {
		t.Fatalf("subsystem netconf: %v", err)
	}
}

func TestAdmissionEmptyDenyAll(t *testing.T) {
	_, addr := startSSH(t, Config{
		AllowCIDRs: []netip.Prefix{},
	})
	tcp, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		t.Fatalf("tcp dial: %v", err)
	}
	defer tcp.Close()
	cfg := &ssh.ClientConfig{
		User:            "alice",
		Auth:            []ssh.AuthMethod{ssh.Password("alice-lab-password")},
		HostKeyCallback: hostKeyCallback(t),
		Timeout:         2 * time.Second,
	}
	_, _, _, err = ssh.NewClientConn(tcp, addr, cfg)
	if err == nil {
		t.Fatal("empty allowClientCidrs accepted loopback")
	}
}

func TestAdmissionOmittedLoopback(t *testing.T) {
	if !admit(mustTCP(t, "127.0.0.1:1"), nil) {
		t.Fatal("omitted CIDRs must allow 127.0.0.1")
	}
	if !admit(mustTCP(t, "[::1]:1"), nil) {
		t.Fatal("omitted CIDRs must allow ::1")
	}
	if admit(mustTCP(t, "8.8.8.8:1"), nil) {
		t.Fatal("omitted CIDRs must deny 8.8.8.8")
	}
	if admit(mustTCP(t, "127.0.0.1:1"), []netip.Prefix{}) {
		t.Fatal("empty CIDRs must deny 127.0.0.1")
	}
}

func TestParseCIDRsNilVsEmpty(t *testing.T) {
	got, err := ParseCIDRs(nil)
	if err != nil || got != nil {
		t.Fatalf("nil CIDRs = %v, %v", got, err)
	}
	got, err = ParseCIDRs([]string{})
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty CIDRs = %#v, %v", got, err)
	}
}
