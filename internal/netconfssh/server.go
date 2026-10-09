package netconfssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"strconv"
	"sync"
	"sync/atomic"

	"golang.org/x/crypto/ssh"
)

// Handler serves one authenticated netconf subsystem channel.
// username is the SSH login; ch is the subsystem stream (not an ssh type).
type Handler func(ctx context.Context, username, remoteAddr string, ch io.ReadWriteCloser) error

// User is one spec.users entry. At least one of PasswordFile or
// AuthorizedKeysFile must be a readable path.
type User struct {
	Name               string
	PasswordFile       string
	AuthorizedKeysFile string
}

// Config is the SSH data-plane listener. AllowCIDRs is nil for the
// loopback default and a non-nil empty slice for deny-all.
type Config struct {
	Address     string
	HostKeyFile string
	Users       []User
	AllowCIDRs  []netip.Prefix
	Handler     Handler
}

// Server is a TCP SSH listener that accepts only subsystem netconf.
type Server struct {
	cfg     Config
	sshCfg  *ssh.ServerConfig
	users   atomic.Pointer[map[string]*loadedUser]
	allow   atomic.Pointer[[]netip.Prefix]
	nextGen atomic.Uint64
	ln      net.Listener

	mu    sync.Mutex
	conns map[net.Conn]*trackedConn
}

type loadedUser struct {
	name     string
	password []byte
	keys     []authKey
	gen      uint64
}

type authKey struct {
	key     ssh.PublicKey
	options []string
}

// trackedConn is one accepted TCP connection. user is empty until
// authentication finishes; rotation ignores those entries.
type trackedConn struct {
	user string
	gen  uint64
}

const subsystemNetconf = "netconf"

// New reads host-key and user secret files and binds Address.
func New(cfg Config) (*Server, error) {
	if cfg.Handler == nil {
		return nil, fmt.Errorf("netconfssh: handler is required")
	}
	if cfg.HostKeyFile == "" {
		return nil, fmt.Errorf("netconfssh: hostKeyFile is required")
	}
	if cfg.Address == "" {
		cfg.Address = ":830"
	}
	users, err := loadUsers(cfg.Users)
	if err != nil {
		return nil, err
	}
	hostPEM, err := os.ReadFile(cfg.HostKeyFile)
	if err != nil {
		return nil, fmt.Errorf("netconfssh: host key: %w", err)
	}
	if len(hostPEM) == 0 {
		return nil, fmt.Errorf("netconfssh: host key file is empty")
	}
	signer, err := ssh.ParsePrivateKey(hostPEM)
	if err != nil {
		return nil, fmt.Errorf("netconfssh: host key: %w", err)
	}

	s := &Server{
		cfg:    cfg,
		conns:  map[net.Conn]*trackedConn{},
		sshCfg: &ssh.ServerConfig{ServerVersion: "SSH-2.0-labnetconf"},
	}
	s.publishUsers(users)
	s.SetAllow(cfg.AllowCIDRs)
	s.sshCfg.AddHostKey(signer)
	s.sshCfg.PasswordCallback = s.passwordAuth
	s.sshCfg.PublicKeyCallback = s.publicKeyAuth
	s.sshCfg.NoClientAuth = false

	ln, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return nil, err
	}
	s.ln = ln
	return s, nil
}

// Addr is the bound listen address.
func (s *Server) Addr() net.Addr {
	if s == nil || s.ln == nil {
		return nil
	}
	return s.ln.Addr()
}

// Bound reports whether the SSH listener is accepting.
func (s *Server) Bound() bool {
	return s != nil && s.ln != nil
}

// Close stops Accept and closes tracked connections.
func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	var err error
	if s.ln != nil {
		err = s.ln.Close()
	}
	s.CloseConns()
	return err
}

// SetAllow publishes CIDRs. Nil is loopback. A non-nil empty list is
// deny-all. Connections that fail the new list are closed after the
// publish, without holding the connection mutex.
func (s *Server) SetAllow(cidrs []netip.Prefix) {
	if s == nil {
		return
	}
	next := copyPrefixes(cidrs)
	s.allow.Store(&next)
	s.mu.Lock()
	drop := make([]net.Conn, 0)
	for c := range s.conns {
		if !s.admitted(c.RemoteAddr()) {
			drop = append(drop, c)
		}
	}
	s.mu.Unlock()
	for _, c := range drop {
		_ = c.Close()
	}
}

// ReplaceUsers reloads credential files and swaps the live map. On error
// the previous map stays and no connection is closed. A user whose
// password bytes or authorized-key multiset changed gets a new
// generation, and SSH connections authenticated under an older
// generation are closed after the map is published. Connections that
// have not finished authentication stay up and authenticate against
// the new map.
func (s *Server) ReplaceUsers(users []User) error {
	if s == nil {
		return fmt.Errorf("netconfssh: nil server")
	}
	next, err := loadUsers(users)
	if err != nil {
		return err
	}
	for _, c := range s.publishUsers(next) {
		_ = c.Close()
	}
	return nil
}

// ClearUsers publishes an empty credential map without reading files.
func (s *Server) ClearUsers() {
	if s == nil {
		return
	}
	empty := map[string]*loadedUser{}
	s.storeUsers(empty)
}

// CloseConns closes tracked connections. The copy happens under the
// mutex; Close runs after it is released.
func (s *Server) CloseConns() {
	if s == nil {
		return
	}
	s.mu.Lock()
	conns := make([]net.Conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.conns = map[net.Conn]*trackedConn{}
	s.mu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
}

func (s *Server) storeUsers(users map[string]*loadedUser) {
	s.users.Store(&users)
}

// publishUsers assigns generations, stores next, and returns connections
// whose user and generation are no longer current. Callers close those
// connections after this returns. The generation counter starts at 1.
func (s *Server) publishUsers(next map[string]*loadedUser) []net.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.userMap()
	for _, nu := range next {
		if ou, ok := old[nu.name]; ok && ou.gen != 0 && credentialsEqual(ou, nu) {
			nu.gen = ou.gen
			continue
		}
		nu.gen = s.nextGen.Add(1)
	}
	s.users.Store(&next)
	var drop []net.Conn
	for c, info := range s.conns {
		if info == nil || info.user == "" {
			continue
		}
		u := next[info.user]
		if u == nil || u.gen != info.gen {
			drop = append(drop, c)
		}
	}
	return drop
}

// bindAuth records user and generation on a tracked connection. It
// returns true when the connection must be closed. A missing or
// unparsable generation, including 0, is closed and is not stored.
func (s *Server) bindAuth(conn net.Conn, user, genStr string) bool {
	gen, err := strconv.ParseUint(genStr, 10, 64)
	if err != nil || gen == 0 || user == "" {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	info := s.conns[conn]
	if info == nil {
		return true
	}
	info.user = user
	info.gen = gen
	live := s.userMap()
	u := live[user]
	return u == nil || u.gen != gen
}

func credGenOf(p *ssh.Permissions) string {
	if p == nil || p.Extensions == nil {
		return ""
	}
	return p.Extensions[credGenExt]
}

func (s *Server) userMap() map[string]*loadedUser {
	p := s.users.Load()
	if p == nil {
		return nil
	}
	return *p
}

func (s *Server) admitted(addr net.Addr) bool {
	var cidrs []netip.Prefix
	if p := s.allow.Load(); p != nil {
		cidrs = *p
	}
	return admit(addr, cidrs)
}

func copyPrefixes(in []netip.Prefix) []netip.Prefix {
	if in == nil {
		return nil
	}
	out := make([]netip.Prefix, len(in))
	copy(out, in)
	return out
}

// Serve accepts SSH connections until ctx is cancelled or the listener closes.
func (s *Server) Serve(ctx context.Context) error {
	if s == nil || s.ln == nil {
		return fmt.Errorf("netconfssh: server is not listening")
	}
	go func() {
		<-ctx.Done()
		_ = s.Close()
	}()
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go s.handle(ctx, conn)
	}
}

func (s *Server) handle(ctx context.Context, conn net.Conn) {
	defer func() { _ = conn.Close() }()
	if !s.admitted(conn.RemoteAddr()) {
		return
	}
	s.track(conn, true)
	defer s.track(conn, false)
	if !s.admitted(conn.RemoteAddr()) {
		return
	}

	sshConn, chans, reqs, err := ssh.NewServerConn(conn, s.sshCfg)
	if err != nil {
		return
	}
	defer func() { _ = sshConn.Close() }()
	if s.bindAuth(conn, sshConn.User(), credGenOf(sshConn.Permissions)) {
		// The mux blocks once its request or channel buffer fills.
		// Discard both before Close so a pipelined client cannot leak it.
		go ssh.DiscardRequests(reqs)
		go rejectIncoming(chans)
		return
	}
	go ssh.DiscardRequests(reqs)

	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			_ = newCh.Reject(ssh.UnknownChannelType, "only session channels")
			continue
		}
		ch, chReqs, err := newCh.Accept()
		if err != nil {
			continue
		}
		go s.session(ctx, sshConn.User(), conn.RemoteAddr().String(), ch, chReqs)
	}
}

func (s *Server) session(ctx context.Context, username, remote string, ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer func() { _ = ch.Close() }()
	for req := range reqs {
		switch req.Type {
		case "subsystem":
			name, err := parseSubsystem(req.Payload)
			if err != nil || name != subsystemNetconf {
				if req.WantReply {
					_ = req.Reply(false, nil)
				}
				continue
			}
			if req.WantReply {
				_ = req.Reply(true, nil)
			}
			go discardChannelRequests(reqs)
			if s.cfg.Handler != nil {
				_ = s.cfg.Handler(ctx, username, remote, ch)
			}
			return
		default:
			if req.WantReply {
				_ = req.Reply(false, nil)
			}
		}
	}
}

func (s *Server) track(c net.Conn, add bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if add {
		if _, ok := s.conns[c]; !ok {
			s.conns[c] = &trackedConn{}
		}
		return
	}
	delete(s.conns, c)
}

func parseSubsystem(payload []byte) (string, error) {
	var msg struct {
		Name string
	}
	if err := ssh.Unmarshal(payload, &msg); err != nil {
		return "", err
	}
	return msg.Name, nil
}

func discardChannelRequests(reqs <-chan *ssh.Request) {
	for req := range reqs {
		if req.WantReply {
			_ = req.Reply(false, nil)
		}
	}
}

func rejectIncoming(chans <-chan ssh.NewChannel) {
	for ch := range chans {
		_ = ch.Reject(ssh.ConnectionFailed, "closed")
	}
}
