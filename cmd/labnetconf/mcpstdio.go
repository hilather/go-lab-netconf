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
	"github.com/hilather/go-lab-netconf/internal/auth"
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
	s, _, err := newStdioServer(ctx, *path, *tokenFile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labnetconf mcp-stdio: %v\n", err)
		return 1
	}
	if err := s.Run(ctx, stdin, stdout); err != nil && ctx.Err() == nil {
		_, _ = fmt.Fprintf(stderr, "labnetconf mcp-stdio: %v\n", err)
		return 1
	}
	return 0
}

// newStdioServer boots the same server mcp-stdio serves. The startup
// bearer is pinned; later reset and apply re-check that secret.
func newStdioServer(ctx context.Context, configPath, tokenFile string) (*mcp.Server, *app.App, error) {
	svc, err := app.Boot(ctx, app.Options{BootstrapPath: configPath})
	if err != nil {
		return nil, nil, fmt.Errorf("load %s: %w", configPath, err)
	}
	allowLegacy := false
	var verifier *auth.Verifier
	var fixed *app.Actor
	var secret string
	if snap := svc.Active(); snap != nil && snap.Canonical != nil {
		allowLegacy = snap.Canonical.Spec.Management.MCP.AllowLegacyClients
		v, vErr := auth.FromSpec(snap.Canonical.Spec.Auth)
		if vErr != nil {
			return nil, nil, fmt.Errorf("auth: %w", vErr)
		}
		if err := v.RequireListen(); err != nil {
			return nil, nil, err
		}
		verifier = v
		raw, rErr := os.ReadFile(tokenFile)
		if rErr != nil {
			return nil, nil, fmt.Errorf("token-file: %w", rErr)
		}
		secret = firstSecretLine(raw)
		p, aErr := verifier.AuthenticateBearer(secret)
		if aErr != nil {
			return nil, nil, fmt.Errorf("token-file: %w", aErr)
		}
		a := app.Actor{ID: p.ID, Class: p.Class, Role: p.Role, Scopes: p.Scopes, Transport: "mcp"}
		fixed = &a
	}
	s, err := mcp.New(mcp.Config{
		Service:            svc,
		AllowLegacyClients: allowLegacy,
		Auth:               verifier,
		FixedActor:         fixed,
		StdioSecret:        secret,
	})
	if err != nil {
		return nil, nil, err
	}
	return s, svc, nil
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
