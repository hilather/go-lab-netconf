package netconfssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"sync"

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
	cfg    Config
	sshCfg *ssh.ServerConfig
	users  map[string]*loadedUser
	ln     net.Listener

	mu    sync.Mutex
	conns map[net.Conn]struct{}
}

type loadedUser struct {
	name     string
	password []byte
	keys     []ssh.PublicKey
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
		users:  users,
		conns:  map[net.Conn]struct{}{},
		sshCfg: &ssh.ServerConfig{ServerVersion: "SSH-2.0-labnetconf"},
	}
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

// Close stops Accept and closes tracked connections.
func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	var err error
	if s.ln != nil {
		err = s.ln.Close()
	}
	s.mu.Lock()
	conns := make([]net.Conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.conns = map[net.Conn]struct{}{}
	s.mu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
	return err
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
	defer conn.Close()
	if !admit(conn.RemoteAddr(), s.cfg.AllowCIDRs) {
		return
	}
	s.track(conn, true)
	defer s.track(conn, false)

	sshConn, chans, reqs, err := ssh.NewServerConn(conn, s.sshCfg)
	if err != nil {
		return
	}
	defer sshConn.Close()
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
	defer ch.Close()
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
		s.conns[c] = struct{}{}
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
