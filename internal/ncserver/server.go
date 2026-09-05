package ncserver

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/hilather/go-lab-netconf/internal/datastore"
	"github.com/hilather/go-lab-netconf/internal/model"
	"github.com/hilather/go-lab-netconf/internal/notif"
)

// User binds an authenticated SSH/RESTCONF name to one profile-instance.
type User struct {
	Name       string
	Profile    string
	Access     string
	Handle     datastore.Handle
	Namespaces map[string]string
}

// Config is the session engine. Sink is the same object passed to
// datastore.New; it may be nil.
type Config struct {
	Users []User
	Sink  notif.Sink
}

// SessionInfo is one row of the live session table.
type SessionInfo struct {
	ID         string
	Username   string
	Profile    string
	RemoteAddr string
}

// Server owns the session table and dispatches RPCs into datastores.
type Server struct {
	users  map[string]User
	sink   notif.Sink
	nextID atomic.Uint64

	mu       sync.Mutex
	sessions map[string]*session
}

// New copies cfg.Users into an index. Duplicate names keep the last entry.
func New(cfg Config) *Server {
	users := make(map[string]User, len(cfg.Users))
	for _, u := range cfg.Users {
		if u.Access == "" {
			u.Access = model.UserAccessReadWrite
		}
		users[u.Name] = u
	}
	return &Server{
		users:    users,
		sink:     cfg.Sink,
		sessions: map[string]*session{},
	}
}

// Serve runs one NETCONF session on rw until close-session, kill, or error.
func (s *Server) Serve(ctx context.Context, username, remoteAddr string, rw io.ReadWriteCloser) error {
	if s == nil {
		return fmt.Errorf("ncserver: nil server")
	}
	user, ok := s.users[username]
	if !ok {
		_ = rw.Close()
		return fmt.Errorf("ncserver: unknown user")
	}
	if user.Handle == nil {
		_ = rw.Close()
		return fmt.Errorf("ncserver: user %q has no datastore", username)
	}
	ctx, cancel := context.WithCancel(ctx)
	id := fmt.Sprintf("%d", s.nextID.Add(1))
	sess := &session{
		id:         id,
		user:       user,
		remoteAddr: remoteAddr,
		rw:         rw,
		cancel:     cancel,
		server:     s,
	}
	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()
	defer func() {
		cancel()
		s.drop(id)
		sess.dropLocks()
		_ = rw.Close()
	}()
	go func() {
		<-ctx.Done()
		_ = rw.Close()
	}()
	return sess.run(ctx)
}

// Sessions returns a snapshot of the table.
func (s *Server) Sessions() []SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SessionInfo, 0, len(s.sessions))
	for _, sess := range s.sessions {
		out = append(out, SessionInfo{
			ID:         sess.id,
			Username:   sess.user.Name,
			Profile:    sess.user.Profile,
			RemoteAddr: sess.remoteAddr,
		})
	}
	return out
}

// Kill cancels a session. The session drops its datastore locks on exit.
func (s *Server) Kill(id string) error {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("ncserver: no such session")
	}
	sess.cancel()
	_ = sess.rw.Close()
	return nil
}

func (s *Server) drop(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

func (s *session) dropLocks() {
	if s.user.Handle == nil {
		return
	}
	ctx := context.Background()
	for _, store := range []datastore.Name{datastore.Running, datastore.Candidate, datastore.Startup} {
		_ = s.user.Handle.Unlock(ctx, store, s.id)
	}
}
