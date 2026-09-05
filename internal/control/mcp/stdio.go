package mcp

import (
	"context"
	"errors"
	"io"

	"github.com/hilather/go-lab-netconf/internal/app"
)

// Config constructs a stdio MCP adapter. MCP-001 fills tools/resources.
type Config struct {
	Service app.Service
}

// Server is the MCP adapter over app.Service.
type Server struct {
	svc app.Service
}

// New builds a Server. Service is required.
func New(cfg Config) (*Server, error) {
	if cfg.Service == nil {
		return nil, errors.New("mcp: Service is required")
	}
	return &Server{svc: cfg.Service}, nil
}

// RunStdio serves MCP over stdin/stdout until in hits EOF or ctx is done.
// MCP-001 replaces the drain with the SDK Streamable transport.
func (s *Server) RunStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	_ = s
	_ = out
	if ctx == nil {
		ctx = context.Background()
	}
	if in == nil {
		return nil
	}
	errCh := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, in)
		errCh <- err
	}()
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		return nil
	}
}
