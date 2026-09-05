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
	svc, err := app.Boot(ctx, app.Options{BootstrapPath: *path})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labnetconf mcp-stdio: load %s: %v\n", *path, err)
		return 1
	}
	allowLegacy := false
	var verifier *auth.Verifier
	var fixed *app.Actor
	if snap := svc.Active(); snap != nil && snap.Canonical != nil {
		allowLegacy = snap.Canonical.Spec.Management.MCP.AllowLegacyClients
		v, vErr := auth.FromSpec(snap.Canonical.Spec.Auth)
		if vErr != nil {
			_, _ = fmt.Fprintf(stderr, "labnetconf mcp-stdio: auth: %v\n", vErr)
			return 1
		}
		if err := v.RequireListen(); err != nil {
			_, _ = fmt.Fprintf(stderr, "labnetconf mcp-stdio: %v\n", err)
			return 1
		}
		verifier = v
		raw, rErr := os.ReadFile(*tokenFile)
		if rErr != nil {
			_, _ = fmt.Fprintf(stderr, "labnetconf mcp-stdio: token-file: %v\n", rErr)
			return 1
		}
		secret := firstSecretLine(raw)
		p, aErr := verifier.AuthenticateBearer(secret)
		if aErr != nil {
			_, _ = fmt.Fprintf(stderr, "labnetconf mcp-stdio: token-file: %v\n", aErr)
			return 1
		}
		a := app.Actor{ID: p.ID, Class: p.Class, Role: p.Role, Scopes: p.Scopes, Transport: "mcp"}
		fixed = &a
	}
	s, err := mcp.New(mcp.Config{
		Service:            svc,
		AllowLegacyClients: allowLegacy,
		Auth:               verifier,
		FixedActor:         fixed,
	})
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
