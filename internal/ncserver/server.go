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
	"github.com/hilather/go-lab-netconf/internal/observability"
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
	// HandleFor, if set, supplies the live datastore at session start so
	// reset-rebuilt handles are used instead of the constructor snapshot.
	HandleFor func(username string) (datastore.Handle, bool)
	Metrics   *observability.Registry
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
	users     atomic.Pointer[map[string]User]
	sink      notif.Sink
	handleFor func(username string) (datastore.Handle, bool)
	metrics   *observability.Registry
	nextID    atomic.Uint64

	mu       sync.Mutex
	sessions map[string]*session
}

// New copies cfg.Users into an index. Duplicate names keep the last entry.
func New(cfg Config) *Server {
	s := &Server{
		sink:      cfg.Sink,
		handleFor: cfg.HandleFor,
		metrics:   cfg.Metrics,
		sessions:  map[string]*session{},
	}
	s.storeUsers(cfg.Users)
	return s
}

// ReplaceUsers publishes users and closes sessions whose name, access,
// profile, or datastore handle no longer match. The handle compared is the
// one stored on the user, not a fresh profile lookup.
func (s *Server) ReplaceUsers(users []User) {
	if s == nil {
		return
	}
	next := indexUsers(users)
	s.users.Store(&next)
	s.mu.Lock()
	victims := make([]*session, 0)
	for id, sess := range s.sessions {
		live, ok := next[sess.user.Name]
		if ok && live.Access == sess.user.Access && live.Profile == sess.user.Profile && live.Handle == sess.user.Handle {
			continue
		}
		delete(s.sessions, id)
		victims = append(victims, sess)
	}
	if len(victims) > 0 {
		s.publishSessionsLocked()
	}
	s.mu.Unlock()
	for _, sess := range victims {
		sess.cancel()
		_ = sess.rw.Close()
	}
}

func indexUsers(in []User) map[string]User {
	users := make(map[string]User, len(in))
	for _, u := range in {
		if u.Access == "" {
			u.Access = model.UserAccessReadWrite
		}
		if u.Namespaces != nil {
			cp := make(map[string]string, len(u.Namespaces))
			for k, v := range u.Namespaces {
				cp[k] = v
			}
			u.Namespaces = cp
		}
		users[u.Name] = u
	}
	return users
}

func (s *Server) storeUsers(in []User) {
	users := indexUsers(in)
	s.users.Store(&users)
}

func (s *Server) userByName(name string) (User, bool) {
	p := s.users.Load()
	if p == nil {
		return User{}, false
	}
	u, ok := (*p)[name]
	return u, ok
}

// Serve runs one NETCONF session on rw until close-session, kill, or error.
func (s *Server) Serve(ctx context.Context, username, remoteAddr string, rw io.ReadWriteCloser) error {
	if s == nil {
		return fmt.Errorf("ncserver: nil server")
	}
	user, ok := s.userByName(username)
	if !ok {
		_ = rw.Close()
		return fmt.Errorf("ncserver: unknown user")
	}
	if s.handleFor != nil {
		if h, ok := s.handleFor(username); ok {
			user.Handle = h
		}
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
	s.publishSessionsLocked()
	live, liveOK := s.userByName(username)
	stale := !liveOK || live.Access != user.Access || live.Profile != user.Profile || live.Handle != user.Handle
	if stale {
		delete(s.sessions, id)
		s.publishSessionsLocked()
	}
	s.mu.Unlock()
	if stale {
		cancel()
		_ = rw.Close()
		return fmt.Errorf("ncserver: user access changed")
	}
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
	s.publishSessionsLocked()
	s.mu.Unlock()
}

func (s *Server) publishSessionsLocked() {
	observability.SetSessions(s.metrics, len(s.sessions))
}

func (s *Server) observeRPC(name string, ok bool) {
	observability.ObserveRPC(s.metrics, name, observability.RPCDecision(ok))
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
