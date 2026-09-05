package restconf

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/hilather/go-lab-netconf/internal/datastore"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
	"github.com/hilather/go-lab-netconf/internal/observability"
)

const (
	// DefaultAddr is the RESTCONF listen address when Config.Addr is empty.
	DefaultAddr = ":8303"

	// MediaYangJSON is the only RESTCONF data media type in 1.0.
	MediaYangJSON = "application/yang-data+json"
	mediaProblem  = "application/problem+json"
	mediaXRD      = "application/xrd+xml"

	yangLibraryDateDefault = "2016-06-21"
	maxBodyBytes           = 1 << 20
)

const hostMetaXRD = `<?xml version="1.0" encoding="UTF-8"?>
<XRD xmlns="http://docs.oasis-open.org/ns/xri/xrd-1.0">
  <Link rel="restconf" href="/restconf"/>
</XRD>
`

// User is one RESTCONF principal bound to a profile.
type User struct {
	Name     string
	Password []byte
	Profile  string
	Access   string
}

// Config is a dedicated RESTCONF HTTP listener.
type Config struct {
	Addr             string
	Users            []User
	Handles          map[string]datastore.Handle
	AllowClientCidrs []string // nil = loopback; empty = deny-all
	YangLibraryDate  string
	// HandleFor, if set, supplies the live datastore per authenticated user
	// so reset-rebuilt handles are used instead of the constructor map.
	HandleFor func(username, profile string) (datastore.Handle, bool)
	Metrics   *observability.Registry
}

// Server is an HTTP handler for RFC 8040 JSON RESTCONF.
type Server struct {
	addr      string
	users     []User
	handles   map[string]datastore.Handle
	handleFor func(username, profile string) (datastore.Handle, bool)
	cidrs     []*net.IPNet
	denyAll   bool
	yangDate  string
	metrics   *observability.Registry
}

var _ http.Handler = (*Server)(nil)

// New builds a RESTCONF handler. It does not bind a port.
func New(cfg Config) (*Server, error) {
	cidrs, denyAll, err := parseAdmission(cfg.AllowClientCidrs)
	if err != nil {
		return nil, err
	}
	handles := cfg.Handles
	if handles == nil {
		handles = map[string]datastore.Handle{}
	}
	copied := make(map[string]datastore.Handle, len(handles))
	for k, v := range handles {
		copied[k] = v
	}
	users := append([]User(nil), cfg.Users...)
	for i := range users {
		users[i].Password = append([]byte(nil), users[i].Password...)
	}
	date := strings.TrimSpace(cfg.YangLibraryDate)
	if date == "" {
		date = yangLibraryDateDefault
	}
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		addr = DefaultAddr
	}
	return &Server{
		addr:      addr,
		users:     users,
		handles:   copied,
		handleFor: cfg.HandleFor,
		cidrs:     cidrs,
		denyAll:   denyAll,
		yangDate:  date,
		metrics:   cfg.Metrics,
	}, nil
}

// ListenAndServe binds Config.Addr (default :8303) and serves until ctx is done.
func (s *Server) ListenAndServe(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	return s.Serve(ctx, ln)
}

// Serve serves RESTCONF on an already-bound listener until ctx is done.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	if ctx == nil {
		ctx = context.Background()
	}
	handler := http.Handler(s)
	if s.metrics != nil {
		handler = Instrument(s, s.metrics)
	}
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()
	select {
	case <-ctx.Done():
		shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shCtx)
		err := <-errCh
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case err := <-errCh:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// ServeHTTP dispatches RESTCONF resources on this listener only.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.admit(r.RemoteAddr) {
		writeError(w, domainerr.Forbidden("client address is not admitted"))
		return
	}
	path := r.URL.Path
	switch {
	case path == "/.well-known/host-meta":
		s.serveHostMeta(w, r)
		return
	case path == "/restconf/yang-library-version":
		s.serveYangLibraryVersion(w, r)
		return
	case path == "/restconf/operations":
		s.serveOperations(w, r)
		return
	case path == "/restconf/data" || strings.HasPrefix(path, "/restconf/data/"):
		s.serveData(w, r)
		return
	default:
		writeError(w, domainerr.NotFound("not a RESTCONF resource"))
	}
}

func (s *Server) serveHostMeta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", mediaXRD)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(hostMetaXRD))
}

func (s *Server) serveYangLibraryVersion(w http.ResponseWriter, r *http.Request) {
	if _, err := s.authenticate(r); err != nil {
		writeError(w, err)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := requireJSONAccept(r); err != nil {
		writeError(w, err)
		return
	}
	writeYangJSON(w, http.StatusOK, map[string]any{
		"ietf-restconf:yang-library-version": s.yangDate,
	})
}

func (s *Server) serveOperations(w http.ResponseWriter, r *http.Request) {
	if _, err := s.authenticate(r); err != nil {
		writeError(w, err)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := requireJSONAccept(r); err != nil {
		writeError(w, err)
		return
	}
	writeYangJSON(w, http.StatusOK, map[string]any{
		"ietf-restconf:operations": map[string]any{},
	})
}

func (s *Server) handleForUser(u *User) (datastore.Handle, bool) {
	if u == nil {
		return nil, false
	}
	if s.handleFor != nil {
		if h, ok := s.handleFor(u.Name, u.Profile); ok && h != nil {
			return h, true
		}
	}
	if h, ok := s.handles[u.Name]; ok && h != nil {
		return h, true
	}
	h, ok := s.handles[u.Profile]
	return h, ok && h != nil
}

func (s *Server) serveData(w http.ResponseWriter, r *http.Request) {
	u, err := s.authenticate(r)
	if err != nil {
		writeError(w, err)
		return
	}
	h, ok := s.handleForUser(u)
	if !ok || h == nil {
		writeError(w, domainerr.NotFound("no datastore for profile"))
		return
	}
	path, err := dataPath(r.URL.EscapedPath())
	if err != nil {
		writeError(w, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.getData(w, r, h, path)
	case http.MethodPut, http.MethodPost, http.MethodPatch, http.MethodDelete:
		if u.Access != model.UserAccessReadWrite {
			writeError(w, domainerr.Forbidden("read-only user"))
			return
		}
		s.writeData(w, r, h, path)
	default:
		w.Header().Set("Allow", "GET, PUT, POST, PATCH, DELETE")
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
