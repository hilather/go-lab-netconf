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
	"github.com/hilather/go-lab-netconf/internal/capabilities"
	"github.com/hilather/go-lab-netconf/internal/config"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/observability"
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
	Live              func() bool
	Ready             func() bool
	MaxBodyBytes      int64
	RequestTimeout    time.Duration
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	UI                http.Handler
	UIEnabled         func() bool
	Metrics           *observability.Registry
	Logger            *observability.Logger
}

// Server is the stdlib net/http management listener.
type Server struct {
	cfg     Config
	svc     app.Service
	routes  []compiledRoute
	handler http.Handler
	maxBody int64
	timeout time.Duration

	metrics *observability.Registry
	logger  *observability.Logger

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
	s := &Server{
		cfg:     cfg,
		svc:     cfg.Service,
		routes:  compileRoutes(capabilities.All()),
		maxBody: maxBody,
		timeout: timeout,
		metrics: cfg.Metrics,
		logger:  cfg.Logger,
		addr:    cfg.Addr,
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
	start := time.Now()
	sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
	w = sw
	reqID := requestID(r)
	w.Header().Set(headerRequestID, reqID)
	r.Header.Set(headerRequestID, reqID)
	instance := requestURNPrefix + reqID
	route := "unknown"
	defer func() {
		s.observeHTTP(route, sw.status(), start, reqID)
	}()

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

	rt, params, pathOK, methodOK := matchRoute(s.routes, r.Method, r.URL.Path)
	if pathOK {
		route = rt.path
		if !methodOK {
			w.Header().Set(headerAllow, allowedMethods(s.routes, r.URL.Path))
			s.writeProblem(w, r, instance, domainerr.ValidationFailed("method not allowed",
				domainerr.FieldViolation{Path: "", Code: "invalid_value", Message: "method not allowed"}))
			return
		}
		if err := s.authenticate(r, isHealthCap(rt.cap)); err != nil {
			s.writeProblem(w, r, instance, err)
			return
		}
		if err := s.authorize(r, rt.cap); err != nil {
			s.writeProblem(w, r, instance, err)
			return
		}
		s.dispatch(w, r, instance, rt, params)
		return
	}

	if s.tryUI(w, r, instance) {
		return
	}
	s.writeProblem(w, r, instance, domainerr.NotFound("not found"))
}

func (s *Server) observeHTTP(route string, status int, start time.Time, reqID string) {
	observability.ObserveHTTP(s.metrics, status, route)
	if s.logger != nil {
		s.logger.Log(observability.Record{
			Event:      observability.EventHTTPRequest,
			Component:  "rest",
			RequestID:  reqID,
			Result:     observability.HTTPCode(status),
			DurationMS: float64(time.Since(start).Milliseconds()),
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(status int) {
	w.code = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) status() int {
	if w.code == 0 {
		return http.StatusOK
	}
	return w.code
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
