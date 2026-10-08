package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcp-stdio must drop the startup administrator after a reset demotion.
// The second admin tool call is denied.
func TestMCPStdioDemotionDropsAdmin(t *testing.T) {
	cfg, _, _, _ := writeServeFixture(t, false)
	token := filepath.Join(filepath.Dir(cfg), "token")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)

	srv, svc, err := newStdioServer(ctx, cfg, token)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)

	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	t.Cleanup(func() {
		_ = clientWriter.Close()
		_ = serverWriter.Close()
		_ = serverReader.Close()
		_ = clientReader.Close()
	})
	go func() {
		_ = srv.Run(ctx, serverReader, serverWriter)
	}()

	client := sdk.NewClient(&sdk.Implementation{Name: "labnetconf-test", Version: "dev"}, nil)
	session, err := client.Connect(ctx, &sdk.IOTransport{Reader: clientReader, Writer: clientWriter}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	first, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name:      "netconf_state_export",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if first.IsError {
		raw, _ := json.Marshal(first)
		t.Fatalf("startup admin export denied: %s", raw)
	}

	body, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	demoted := strings.Replace(string(body), "role: administrator", "role: reader", 1)
	if demoted == string(body) {
		t.Fatal("fixture has no administrator role to demote")
	}
	if err := os.WriteFile(cfg, []byte(demoted), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := svc.Reset(ctx); err != nil {
		t.Fatal(err)
	}

	second, err := session.CallTool(ctx, &sdk.CallToolParams{
		Name:      "netconf_state_reset",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("reset call: %v", err)
	}
	raw, _ := json.Marshal(second)
	if !second.IsError || !strings.Contains(string(raw), "forbidden") {
		t.Fatalf("demoted stdio netconf_state_reset isError=%v body=%s, want forbidden", second.IsError, raw)
	}
}
