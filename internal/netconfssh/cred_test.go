package netconfssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestPasswordChangeClosesSSHAndUnchangedUserStays(t *testing.T) {
	dir := t.TempDir()
	aliceFile := filepath.Join(dir, "alice")
	bobFile := filepath.Join(dir, "bob")
	if err := os.WriteFile(aliceFile, []byte("alice-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bobFile, []byte("bob-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	users := []User{
		{Name: "alice", PasswordFile: aliceFile},
		{Name: "bob", PasswordFile: bobFile},
	}
	s, addr := startSSH(t, Config{Users: users})
	alice := mustSSH(t, addr, "alice", "alice-secret", nil)
	bob := mustSSH(t, addr, "bob", "bob-secret", nil)

	if err := os.WriteFile(aliceFile, []byte("alice-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceUsers(users); err != nil {
		t.Fatal(err)
	}
	if _, err := alice.NewSession(); err != nil {
		t.Fatalf("trailing newline difference closed alice: %v", err)
	}

	if err := os.WriteFile(aliceFile, []byte("alice-new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceUsers(users); err != nil {
		t.Fatal(err)
	}
	expectConnClosed(t, alice)
	if _, err := bob.NewSession(); err != nil {
		t.Fatalf("bob closed when only alice's password changed: %v", err)
	}
	if _, err := sshDial(t, addr, "alice", "alice-secret", nil); err == nil {
		t.Fatal("old password still authenticates")
	}
	if _, err := sshDial(t, addr, "alice", "alice-new", nil); err != nil {
		t.Fatalf("new password: %v", err)
	}
}

func TestAuthorizedKeyEqualityKeepsOrCloses(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "alice.keys")
	bobFile := filepath.Join(dir, "bob")
	aliceSigner, aliceLine := testKey(t)
	_, otherLine := testKey(t)
	bobSigner := []byte("bob-secret\n")
	if err := os.WriteFile(bobFile, bobSigner, 0o600); err != nil {
		t.Fatal(err)
	}
	original := aliceLine + " alice\n\n" + otherLine + "\n"
	if err := os.WriteFile(keyFile, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	users := []User{
		{Name: "alice", AuthorizedKeysFile: keyFile},
		{Name: "bob", PasswordFile: bobFile},
	}
	s, addr := startSSH(t, Config{Users: users})
	alice := mustSSH(t, addr, "alice", "", aliceSigner)
	bob := mustSSH(t, addr, "bob", "bob-secret", nil)

	reordered := "# still alice\n" + otherLine + " bob\n" + aliceLine + " alice-renamed\n"
	if err := os.WriteFile(keyFile, []byte(reordered), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceUsers(users); err != nil {
		t.Fatal(err)
	}
	kept, err := alice.NewSession()
	if err != nil {
		t.Fatalf("key reorder, whitespace, or comment closed alice: %v", err)
	}
	_ = kept.Close()

	restricted := "restrict " + aliceLine + "\n"
	if err := os.WriteFile(keyFile, []byte(restricted), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceUsers(users); err != nil {
		t.Fatal(err)
	}
	expectConnClosed(t, alice)
	if _, err := bob.NewSession(); err != nil {
		t.Fatalf("bob closed when only alice's key options changed: %v", err)
	}
	again, err := sshDial(t, addr, "alice", "", aliceSigner)
	if err != nil {
		t.Fatalf("key options are compared for rotation but not enforced at handshake: %v", err)
	}
	_ = again.Close()
}

func TestRemovedUserClosesSSH(t *testing.T) {
	dir := t.TempDir()
	aliceFile := filepath.Join(dir, "alice")
	bobFile := filepath.Join(dir, "bob")
	if err := os.WriteFile(aliceFile, []byte("alice-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bobFile, []byte("bob-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, addr := startSSH(t, Config{Users: []User{
		{Name: "alice", PasswordFile: aliceFile},
		{Name: "bob", PasswordFile: bobFile},
	}})
	alice := mustSSH(t, addr, "alice", "alice-secret", nil)
	bob := mustSSH(t, addr, "bob", "bob-secret", nil)
	if err := s.ReplaceUsers([]User{{Name: "bob", PasswordFile: bobFile}}); err != nil {
		t.Fatal(err)
	}
	expectConnClosed(t, alice)
	if _, err := bob.NewSession(); err != nil {
		t.Fatalf("remaining user closed: %v", err)
	}
}

func TestPreAuthConnAuthenticatesAgainstNewMap(t *testing.T) {
	dir := t.TempDir()
	pass := filepath.Join(dir, "alice")
	if err := os.WriteFile(pass, []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	users := []User{{Name: "alice", PasswordFile: pass}}
	s, addr := startSSH(t, Config{Users: users})
	tcp, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tcp.Close() })
	waitTracked(t, s, 1)
	if err := os.WriteFile(pass, []byte("after\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceUsers(users); err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ClientConfig{
		User:            "alice",
		Auth:            []ssh.AuthMethod{ssh.Password("after")},
		HostKeyCallback: hostKeyCallback(t),
		Timeout:         3 * time.Second,
	}
	cc, chans, reqs, err := ssh.NewClientConn(tcp, addr, cfg)
	if err != nil {
		t.Fatalf("pre-auth connection was closed or rejected the new password: %v", err)
	}
	_ = ssh.NewClient(cc, chans, reqs).Close()
	if _, err := sshDial(t, addr, "alice", "before", nil); err == nil {
		t.Fatal("old password still authenticates")
	}
}

func TestDuplicateKeyRemovalCloses(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "alice.keys")
	signer, line := testKey(t)
	if err := os.WriteFile(keyFile, []byte(line+"\n"+line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	users := []User{{Name: "alice", AuthorizedKeysFile: keyFile}}
	s, addr := startSSH(t, Config{Users: users})
	alice := mustSSH(t, addr, "alice", "", signer)
	if err := os.WriteFile(keyFile, []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceUsers(users); err != nil {
		t.Fatal(err)
	}
	expectConnClosed(t, alice)
}

func TestGenerationCarryOver(t *testing.T) {
	dir := t.TempDir()
	pass := filepath.Join(dir, "p")
	keys := filepath.Join(dir, "k")
	if err := os.WriteFile(pass, []byte("pw\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, lineA := testKey(t)
	_, lineB := testKey(t)
	if err := os.WriteFile(keys, []byte(lineA+"\n"+lineB+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	users := []User{{Name: "alice", PasswordFile: pass, AuthorizedKeysFile: keys}}
	s, _ := startSSH(t, Config{Users: users})
	g := s.userMap()["alice"].gen
	if g == 0 {
		t.Fatal("generation must start at 1 or higher")
	}
	perms := s.userMap()["alice"].permissions()
	if perms == nil || perms.Extensions[credGenExt] == "" || strings.Contains(perms.Extensions[credGenExt], "pw") {
		t.Fatalf("permissions = %+v", perms)
	}
	if err := os.WriteFile(pass, []byte("pw\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keys, []byte("\n"+lineB+"\n"+lineA+" renamed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceUsers(users); err != nil {
		t.Fatal(err)
	}
	if got := s.userMap()["alice"].gen; got != g {
		t.Fatalf("unchanged credentials gen = %d, want %d", got, g)
	}
	if err := os.WriteFile(keys, []byte("from=\"192.0.2.1\" "+lineA+"\n"+lineB+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceUsers(users); err != nil {
		t.Fatal(err)
	}
	if got := s.userMap()["alice"].gen; got == g || got == 0 {
		t.Fatalf("option change gen = %d, previous %d", got, g)
	}
}

func TestBindAuthClosesStaleGeneration(t *testing.T) {
	dir := t.TempDir()
	pass := filepath.Join(dir, "p")
	if err := os.WriteFile(pass, []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	users := []User{{Name: "alice", PasswordFile: pass}}
	s, _ := startSSH(t, Config{Users: users})
	gen := s.userMap()["alice"].gen

	client, server := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	s.track(server, true)
	if s.bindAuth(server, "alice", strconv.FormatUint(gen, 10)) {
		t.Fatal("current generation was rejected")
	}

	if err := os.WriteFile(pass, []byte("other\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceUsers(users); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1)
	errCh := make(chan error, 1)
	go func() {
		_, err := client.Read(buf)
		errCh <- err
	}()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("rotation left a connection from the previous generation open")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the previous generation connection to close")
	}

	client2, server2 := net.Pipe()
	t.Cleanup(func() {
		_ = client2.Close()
		_ = server2.Close()
	})
	s.track(server2, true)
	if !s.bindAuth(server2, "alice", strconv.FormatUint(gen, 10)) {
		t.Fatal("registration of a stale generation must close")
	}
	if !s.bindAuth(server2, "alice", "") || !s.bindAuth(server2, "alice", "0") || !s.bindAuth(server2, "alice", "nope") {
		t.Fatal("missing, zero, or unparsable generation must close")
	}
	if s.bindAuth(server2, "alice", s.userMap()["alice"].permissions().Extensions[credGenExt]) {
		t.Fatal("current generation after rotation must bind")
	}
	if !s.bindAuth(client2, "alice", "1") {
		t.Fatal("untracked connection must close")
	}
}

func mustSSH(t *testing.T, addr, user, password string, signer ssh.Signer) *ssh.Client {
	t.Helper()
	c, err := sshDial(t, addr, user, password, signer)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func expectConnClosed(t *testing.T, cli *ssh.Client) {
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

func waitTracked(t *testing.T, s *Server, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		got := len(s.conns)
		s.mu.Unlock()
		if got >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("tracked connections < %d", n)
}

func testKey(t *testing.T) (ssh.Signer, string) {
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
