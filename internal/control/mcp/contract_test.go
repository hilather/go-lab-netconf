package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/auth"
	"github.com/hilather/go-lab-netconf/internal/model"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestContractReadsAndDatastore(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)

	ver := structuredMap(t, callTool(t, cs, "netconf_version_get", map[string]any{}))
	protocols, _ := ver["protocols"].(map[string]any)
	if protocols == nil || protocols["mcp"] != ProtocolVersion {
		t.Fatalf("version=%v", ver)
	}

	caps := structuredMap(t, callTool(t, cs, "netconf_capabilities_get", map[string]any{}))
	raw, _ := json.Marshal(caps["capabilities"])
	if !strings.Contains(string(raw), "netconf_datastore_get") {
		t.Fatalf("capabilities %s", raw)
	}

	got := structuredMap(t, callTool(t, cs, "netconf_datastore_get", map[string]any{
		"profile": "router-a", "store": "running",
	}))
	if treeHostname(t, got) != "lab-rtr-a" {
		t.Fatalf("running hostname %v", got)
	}

	set := structuredMap(t, callTool(t, cs, "netconf_datastore_set", map[string]any{
		"profile": "router-a", "store": "candidate", "overlay": hostnameOverlay("via-mcp"),
	}))
	if set["ok"] != true {
		t.Fatalf("set %v", set)
	}
	cand := structuredMap(t, callTool(t, cs, "netconf_datastore_get", map[string]any{
		"profile": "router-a", "store": "candidate",
	}))
	if treeHostname(t, cand) != "via-mcp" {
		t.Fatal("candidate after set")
	}
	run := structuredMap(t, callTool(t, cs, "netconf_datastore_get", map[string]any{
		"profile": "router-a", "store": "running",
	}))
	if treeHostname(t, run) != "lab-rtr-a" {
		t.Fatal("running must stay until commit")
	}
	_ = structuredMap(t, callTool(t, cs, "netconf_datastore_commit", map[string]any{
		"profile": "router-a",
	}))
	run = structuredMap(t, callTool(t, cs, "netconf_datastore_get", map[string]any{
		"profile": "router-a", "store": "running",
	}))
	if treeHostname(t, run) != "via-mcp" {
		t.Fatal("running after commit")
	}

	users := structuredMap(t, callTool(t, cs, "netconf_users_list", map[string]any{}))
	items, _ := users["items"].([]any)
	if len(items) == 0 {
		t.Fatal("users")
	}
	u, _ := items[0].(map[string]any)
	if _, ok := u["password"]; ok {
		t.Fatalf("users leaked password bytes: %v", u)
	}
}

func TestMutatingToolRecordsAuditActor(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	_ = structuredMap(t, callTool(t, cs, "netconf_datastore_set", map[string]any{
		"profile": "router-a", "store": "candidate", "overlay": hostnameOverlay("audit-mcp"),
	}))
	got := structuredMap(t, callTool(t, cs, "netconf_audit_query", map[string]any{}))
	events, _ := got["events"].([]any)
	if len(events) == 0 {
		t.Fatal("expected audit event")
	}
	ev, _ := events[0].(map[string]any)
	if ev["actorId"] != "admin" {
		t.Fatalf("actorId %v", ev["actorId"])
	}
	if ev["transport"] != "mcp" {
		t.Fatalf("transport %v", ev["transport"])
	}
	if ev["capability"] != "datastore.set" {
		t.Fatalf("capability %v", ev["capability"])
	}
}

func TestContractResourceState(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	res, err := cs.ReadResource(t.Context(), &sdk.ReadResourceParams{URI: "labnetconf://state"})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || len(res.Contents) == 0 {
		t.Fatal("empty resource")
	}
	if !strings.Contains(res.Contents[0].Text, "bootstrapRevision") {
		t.Fatalf("state %s", res.Contents[0].Text)
	}
}

func TestContractResourceDatastore(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	res, err := cs.ReadResource(t.Context(), &sdk.ReadResourceParams{URI: "labnetconf://datastores/router-a/running"})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || len(res.Contents) == 0 {
		t.Fatal("empty datastore resource")
	}
	if treeHostname(t, json.RawMessage(res.Contents[0].Text)) != "lab-rtr-a" {
		t.Fatalf("datastore resource %s", res.Contents[0].Text)
	}
}

func TestReaderForbiddenOnAdminTool(t *testing.T) {
	svc := bootTestApp(t)
	s, err := New(Config{
		Service:            svc,
		AllowLegacyClients: true,
		Auth:               auth.Static(testBearerToken, "reader", model.RoleReader),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	res := callTool(t, cs, "netconf_state_reset", map[string]any{})
	if !res.IsError {
		t.Fatal("reader must not reset")
	}
	raw, _ := json.Marshal(res.StructuredContent)
	if !strings.Contains(string(raw), "forbidden") {
		t.Fatalf("%s", raw)
	}
}

func TestNotificationsWaitTimeout(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	res := callTool(t, cs, "netconf_notifications_wait", map[string]any{
		"profile": "router-a", "timeout": "50ms",
	})
	if !res.IsError {
		t.Fatal("wait must time out")
	}
	raw, _ := json.Marshal(res.StructuredContent)
	if !strings.Contains(string(raw), "wait_timeout") {
		t.Fatalf("%s", raw)
	}
}

func TestPreviewGet(t *testing.T) {
	s, _ := newTestServer(t)
	ts := startHTTP(t, s)
	cs := connectClient(t, ts)
	got := callTool(t, cs, "netconf_preview_get", map[string]any{
		"user": "alice", "path": "ietf-system:system/hostname",
	})
	if got.IsError {
		t.Fatalf("preview %v", got)
	}
}

func TestManifestPin(t *testing.T) {
	body, err := RenderManifest()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), ProtocolVersion) {
		t.Fatalf("manifest missing protocol: %s", body)
	}
	if !strings.Contains(string(body), SDKModule) {
		t.Fatal("manifest missing SDK module")
	}
	if !strings.Contains(string(body), `"netconf_version_get"`) {
		t.Fatal("manifest missing tools")
	}
	if !strings.Contains(string(body), `"labnetconf://state"`) {
		t.Fatal("manifest missing resources")
	}
}
