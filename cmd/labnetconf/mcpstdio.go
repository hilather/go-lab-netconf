package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hilather/go-lab-netconf/internal/app"
	"github.com/hilather/go-lab-netconf/internal/config"
	"github.com/hilather/go-lab-netconf/internal/control/mcp"
)

func mcpStdioCmd(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mcp-stdio", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", "", "path to bootstrap YAML or JSON")
	tokenFile := fs.String("token-file", "", "bearer token file (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *path == "" {
		_, _ = fmt.Fprintln(stderr, "labnetconf mcp-stdio: --config is required")
		return 2
	}
	if *tokenFile == "" {
		_, _ = fmt.Fprintln(stderr, "labnetconf mcp-stdio: --token-file is required")
		return 2
	}
	raw, err := os.ReadFile(*tokenFile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labnetconf mcp-stdio: token-file: %v\n", err)
		return 1
	}
	secret := firstSecretLine(raw)
	if len(secret) < config.MinTokenBytes {
		_, _ = fmt.Fprintf(stderr, "labnetconf mcp-stdio: token-file must be at least %d bytes\n", config.MinTokenBytes)
		return 1
	}
	svc, err := app.Boot(ctx, app.Options{BootstrapPath: *path, Sink: productionSink()})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labnetconf mcp-stdio: load %s: %v\n", *path, err)
		return 1
	}
	s, err := mcp.New(mcp.Config{Service: svc})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labnetconf mcp-stdio: %v\n", err)
		return 1
	}
	if err := s.RunStdio(ctx, stdin, stdout); err != nil && ctx.Err() == nil {
		_, _ = fmt.Fprintf(stderr, "labnetconf mcp-stdio: %v\n", err)
		return 1
	}
	return 0
}

func firstSecretLine(raw []byte) string {
	for _, line := range bytes.Split(raw, []byte("\n")) {
		s := strings.TrimSpace(string(line))
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		return s
	}
	return strings.TrimSpace(string(raw))
}
