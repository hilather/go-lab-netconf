package rest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hilather/go-lab-netconf/internal/app"
	"github.com/hilather/go-lab-netconf/internal/auth"
	"github.com/hilather/go-lab-netconf/internal/capabilities"
	"github.com/hilather/go-lab-netconf/internal/config"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
)

const (
	DefaultAddr              = config.DefaultMgmtAddress
	DefaultMaxBodyBytes      = 1 << 20
	DefaultRequestTimeout    = 30 * time.Second
	DefaultReadHeaderTimeout = 5 * time.Second
	DefaultReadTimeout       = 30 * time.Second
	headerRequestID          = "X-Request-ID"
	headerIdempotency        = "Idempotency-Key"
	headerIfMatch            = "If-Match"
	headerExpected           = "X-LabNETCONF-Expected-Revision"
	headerRevision           = "X-LabNETCONF-Revision"
	headerAllow              = "Allow"
	requestURNPrefix         = "urn:labnetconf:request:"
)

// Config constructs a management HTTP server.
type Config struct {
	Addr              string
	Service           app.Service
	AllowedOrigins    []string
	Live              func() bool
	Ready             func() bool
	MaxBodyBytes      int64
	RequestTimeout    time.Duration
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	Auth              *auth.Verifier
	Sessions          *auth.Store
	CookieSecure      bool
	UI                http.Handler
	UIEnabled         func() bool
}

// Server is the stdlib net/http management listener.
type Server struct {
	cfg     Config
	svc     app.Service
	routes  []compiledRoute
	handler http.Handler
	maxBody int64
	timeout time.Duration

	mu     sync.Mutex
	http   *http.Server
	ln     net.Listener
	closed atomic.Bool
	addr   string
}

// New builds a Server. Routes come from the frozen capability registry.
func New(cfg Config) (*Server, error) {
	if cfg.Service == nil {
		return nil, errors.New("rest: Service is required")
	}
	maxBody := cfg.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = DefaultMaxBodyBytes
	}
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = DefaultRequestTimeout
	}
	if cfg.Sessions == nil {
		cfg.Sessions = auth.NewStore(auth.DefaultSessionConfig())
	}
	if cfg.Auth != nil {
		sessions := cfg.Sessions
		cfg.Auth.OnIdentityChange(func() {
			if sessions != nil {
				sessions.Clear()
			}
		})
	}
	s := &Server{
		cfg:     cfg,
		svc:     cfg.Service,
		routes:  compileRoutes(capabilities.All()),
		maxBody: maxBody,
		timeout: timeout,
		addr:    cfg.Addr,
	}
	if appSvc, ok := s.svc.(*app.App); ok {
		appSvc.OnReset(s.reloadAuth)
		appSvc.OnApply(s.reloadAuth)
	}
	s.handler = http.HandlerFunc(s.serveHTTP)
	return s, nil
}

// Handler returns the management mux. Safe for httptest.NewServer / ServeHTTP.
func (s *Server) Handler() http.Handler {
	return s.handler
}

// ListenAndServe binds Addr and serves until Shutdown.
func (s *Server) ListenAndServe() error {
	addr := s.cfg.Addr
	if addr == "" {
		addr = DefaultAddr
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

// Serve serves on ln until Shutdown.
func (s *Server) Serve(ln net.Listener) error {
	s.mu.Lock()
	if s.closed.Load() {
		s.mu.Unlock()
		_ = ln.Close()
		return nil
	}
	if s.http != nil {
		s.mu.Unlock()
		_ = ln.Close()
		return errors.New("rest: server already started")
	}
	rh := s.cfg.ReadHeaderTimeout
	if rh <= 0 {
		rh = DefaultReadHeaderTimeout
	}
	rt := s.cfg.ReadTimeout
	if rt <= 0 {
		rt = DefaultReadTimeout
	}
	hs := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: rh,
		ReadTimeout:       rt,
		WriteTimeout:      s.cfg.WriteTimeout,
		MaxHeaderBytes:    1 << 16,
	}
	s.http = hs
	s.ln = ln
	s.addr = ln.Addr().String()
	alreadyClosed := s.closed.Load()
	s.mu.Unlock()
	if alreadyClosed {
		_ = ln.Close()
		return nil
	}
	err := hs.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Bound reports whether a listener is accepting.
func (s *Server) Bound() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ln != nil && s.http != nil && !s.closed.Load()
}

// Shutdown closes the listener and waits for in-flight requests.
func (s *Server) Shutdown(ctx context.Context) error {
	s.closed.Store(true)
	s.mu.Lock()
	hs := s.http
	ln := s.ln
	s.mu.Unlock()
	if hs != nil {
		return hs.Shutdown(ctx)
	}
	if ln != nil {
		return ln.Close()
	}
	return nil
}

// Addr returns the bound address after Serve, or the configured listen address.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln != nil {
		return s.ln.Addr().String()
	}
	if s.cfg.Addr != "" {
		return s.cfg.Addr
	}
	return DefaultAddr
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	reqID := requestID(r)
	w.Header().Set(headerRequestID, reqID)
	r.Header.Set(headerRequestID, reqID)
	instance := requestURNPrefix + reqID

	ctx := r.Context()
	var cancel context.CancelFunc
	if s.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	r = r.WithContext(ctx)

	defer func() {
		if rec := recover(); rec != nil {
			_ = rec
			s.writeProblem(w, r, instance, domainerr.ValidationFailed("internal error"))
		}
	}()

	if err := auth.CheckOrigin(r.Header.Get("Origin"), s.cfg.AllowedOrigins); err != nil {
		s.writeProblem(w, r, instance, err)
		return
	}
	if r.Method == http.MethodOptions {
		s.writeProblem(w, r, instance, domainerr.Forbidden("CORS is disabled"))
		return
	}

	rt, params, pathOK, methodOK := matchRoute(s.routes, r.Method, r.URL.Path)
	if pathOK {
		if !methodOK {
			w.Header().Set(headerAllow, allowedMethods(s.routes, r.URL.Path))
			s.writeProblem(w, r, instance, domainerr.ValidationFailed("method not allowed",
				domainerr.FieldViolation{Path: "", Code: "invalid_value", Message: "method not allowed"}))
			return
		}
		if isHealthCap(rt.cap) {
			s.dispatch(w, r, instance, rt, params)
			return
		}
		actor, err := s.authenticate(r)
		if err != nil {
			s.writeProblem(w, r, instance, err)
			return
		}
		if err := s.authorize(r, actor, rt.cap); err != nil {
			s.writeProblem(w, r, instance, err)
			return
		}
		r = r.WithContext(app.WithActor(r.Context(), actor))
		s.dispatch(w, r, instance, rt, params)
		return
	}

	if s.tryUI(w, r, instance) {
		return
	}
	s.writeProblem(w, r, instance, domainerr.NotFound("not found"))
}

func (s *Server) reloadAuth() {
	if s.cfg.Auth == nil {
		return
	}
	appSvc, ok := s.svc.(*app.App)
	if !ok {
		return
	}
	snap := appSvc.Active()
	if snap == nil || snap.Canonical == nil {
		return
	}
	s.cfg.AllowedOrigins = append([]string(nil), snap.Canonical.Spec.Management.AllowedOrigins...)
	next, err := auth.FromSpec(snap.Canonical.Spec.Auth)
	if err != nil {
		return
	}
	if err := next.RequireListen(); err != nil {
		return
	}
	changed := !s.cfg.Auth.Equivalent(next)
	s.cfg.Auth.Replace(next)
	if changed && s.cfg.Sessions != nil {
		s.cfg.Sessions.Clear()
	}
}

func isHealthCap(cap capabilities.Capability) bool {
	return cap.ID == capabilities.HealthLive || cap.ID == capabilities.HealthReady
}

func (s *Server) isLive() bool {
	if s.cfg.Live != nil {
		return s.cfg.Live()
	}
	return true
}

func (s *Server) isReady(ctx context.Context) bool {
	if s.cfg.Ready != nil {
		return s.cfg.Ready()
	}
	st, err := s.svc.Status(ctx)
	if err != nil {
		return false
	}
	return st.Ready
}

func requestID(r *http.Request) string {
	if id := r.Header.Get(headerRequestID); id != "" {
		return id
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "req-fallback"
	}
	return hex.EncodeToString(b[:])
}
