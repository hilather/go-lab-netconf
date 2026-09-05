package app

import (
	"context"
	"encoding/json"

	"github.com/hilather/go-lab-netconf/internal/buildinfo"
	"github.com/hilather/go-lab-netconf/internal/notif"
)

// Service is the HTTP-less capability surface. REST and MCP call these
// methods rather than implementing mutation or query logic.
type Service interface {
	Version(ctx context.Context) (Version, error)
	Capabilities(ctx context.Context) ([]Capability, error)
	Status(ctx context.Context) (Status, error)
	Schema(ctx context.Context) (json.RawMessage, error)
	Features(ctx context.Context) (Features, error)
	State(ctx context.Context) (StateView, error)
	ValidateState(ctx context.Context, body []byte) error
	ExportState(ctx context.Context) ([]byte, error)
	Reset(ctx context.Context) error
	Plan(ctx context.Context, ops []ApplyOp, expectedRev string) (Plan, error)
	Apply(ctx context.Context, ops []ApplyOp, expectedRev, idempotencyKey string) (ApplyResult, error)
	ListProfiles(ctx context.Context) ([]ProfileSummary, error)
	GetProfile(ctx context.Context, name string) (ProfileView, error)
	ListUsers(ctx context.Context) ([]UserView, error)
	GetDatastore(ctx context.Context, profile, store string) (json.RawMessage, error)
	SetDatastore(ctx context.Context, profile, store string, overlay json.RawMessage) error
	Commit(ctx context.Context, profile string) error
	Discard(ctx context.Context, profile string) error
	ListSessions(ctx context.Context) ([]Session, error)
	KillSession(ctx context.Context, id string) error
	ListNotifications(ctx context.Context, q NotifQuery) ([]Notification, error)
	GetNotification(ctx context.Context, id string) (Notification, error)
	WaitNotification(ctx context.Context, q WaitQuery) (Notification, error)
	ClearNotifications(ctx context.Context) error
	PreviewGet(ctx context.Context, user, path string) (json.RawMessage, error)
	QueryAudit(ctx context.Context, q AuditQuery) ([]AuditEvent, error)
}

// Version is process build metadata.
type Version = buildinfo.Info

// Notification is one RFC 5277 config-change record.
type Notification = notif.Notification

// NotifQuery selects stored notifications.
type NotifQuery = notif.Query

// WaitQuery selects a notification to wait for.
type WaitQuery = notif.WaitQuery
