package mcp

import (
	"context"
	"io"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

// RunStdio serves the same registry over process stdin/stdout. Logs go to
// stderr (never stdout). This is a developer adapter; --token-file is required.
func (s *Server) RunStdio(ctx context.Context) error {
	return s.run(ctx, &sdk.StdioTransport{})
}

// Run serves the same registry over in/out. A nil in uses process stdio.
func (s *Server) Run(ctx context.Context, in io.Reader, out io.Writer) error {
	if in == nil {
		return s.RunStdio(ctx)
	}
	r, ok := in.(io.ReadCloser)
	if !ok {
		r = io.NopCloser(in)
	}
	w, ok := out.(io.WriteCloser)
	if !ok {
		if out == nil {
			return s.RunStdio(ctx)
		}
		w = nopWriteCloser{out}
	}
	return s.run(ctx, &sdk.IOTransport{Reader: r, Writer: w})
}

func (s *Server) run(ctx context.Context, t sdk.Transport) error {
	return s.sdk.Run(ctx, t)
}
