package mcp

import (
	"encoding/json"
	"time"

	"github.com/hilather/go-lab-netconf/internal/app"
	"github.com/hilather/go-lab-netconf/internal/buildinfo"
	"github.com/hilather/go-lab-netconf/internal/capabilities"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
)

type emptyIn struct{}

type idIn struct {
	ID string `json:"id"`
}

type nameIn struct {
	Name string `json:"name"`
}

type profileIn struct {
	Profile string `json:"profile"`
}

type profileStoreIn struct {
	Profile string `json:"profile"`
	Store   string `json:"store"`
}

type datastoreSetIn struct {
	Profile string         `json:"profile"`
	Store   string         `json:"store"`
	Overlay map[string]any `json:"overlay"`
}

type exportIn struct {
	Format string `json:"format,omitempty"`
}

type changeIn struct {
	ExpectedRevision string        `json:"expectedRevision,omitempty"`
	IdempotencyKey   string        `json:"idempotencyKey,omitempty"`
	Operations       []app.ApplyOp `json:"operations,omitempty"`
}

type validateIn struct {
	Document any `json:"document,omitempty"`
}

type resetIn struct {
	Reason string `json:"reason,omitempty"`
}

type waitIn struct {
	Profile string `json:"profile,omitempty"`
	Timeout string `json:"timeout,omitempty"`
}

type notifListIn struct {
	Profile string `json:"profile,omitempty"`
}

type previewIn struct {
	User string `json:"user"`
	Path string `json:"path,omitempty"`
}

type auditQueryIn struct {
	Limit int `json:"limit,omitempty"`
}

func (in validateIn) bytes() []byte {
	return anyBytes(in.Document)
}

func (in datastoreSetIn) overlay() (json.RawMessage, error) {
	if in.Overlay == nil {
		return nil, domainerr.ValidationFailed("overlay is required",
			domainerr.FieldViolation{Path: "overlay", Code: "required", Message: "overlay is required"})
	}
	b, err := json.Marshal(in.Overlay)
	if err != nil {
		return nil, domainerr.ValidationFailed("overlay must be a JSON object",
			domainerr.FieldViolation{Path: "overlay", Code: "invalid_value", Message: err.Error()})
	}
	return json.RawMessage(b), nil
}

func anyBytes(v any) []byte {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case string:
		return []byte(t)
	case []byte:
		return t
	case json.RawMessage:
		return t
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return nil
		}
		return b
	}
}

func requireName(name string) error {
	if name == "" {
		return domainerr.ValidationFailed("name is required",
			domainerr.FieldViolation{Path: "name", Code: "required", Message: "name is required"})
	}
	return nil
}

func requireID(id string) error {
	if id == "" {
		return domainerr.ValidationFailed("id is required",
			domainerr.FieldViolation{Path: "id", Code: "required", Message: "id is required"})
	}
	return nil
}

func requireProfile(profile string) error {
	if profile == "" {
		return domainerr.ValidationFailed("profile is required",
			domainerr.FieldViolation{Path: "profile", Code: "required", Message: "profile is required"})
	}
	return nil
}

func fromVersion(info buildinfo.Info) map[string]any {
	return map[string]any{
		"version":   info.Version,
		"commit":    info.Commit,
		"buildTime": info.BuildTime,
		"protocols": map[string]any{
			"configAPI": info.Protocols.ConfigAPI,
			"rest":      info.Protocols.REST,
			"mcp":       info.Protocols.MCP,
		},
	}
}

func fromCapabilities() map[string]any {
	src := capabilities.DiscoveryList()
	items := make([]map[string]any, 0, len(src))
	for _, d := range src {
		items = append(items, map[string]any{
			"name": d.Name, "version": d.Version, "description": d.Description,
			"mutating": d.Mutating, "idempotent": d.Idempotent,
		})
	}
	return map[string]any{"capabilities": items}
}

func fromStatus(st app.Status, ready bool) map[string]any {
	listeners := make([]map[string]any, 0, len(st.Listeners))
	for _, l := range st.Listeners {
		listeners = append(listeners, map[string]any{"name": l.Name, "address": l.Address})
	}
	return map[string]any{
		"ready":     ready,
		"revision":  string(st.Revision),
		"listeners": listeners,
	}
}

func fromStateView(v app.StateView) (any, error) {
	canon, err := json.Marshal(v.Canonical)
	if err != nil {
		return nil, domainerr.ValidationFailed("marshal state: " + err.Error())
	}
	return map[string]any{
		"bootstrapRevision": string(v.BootstrapRevision),
		"runtimeRevision":   string(v.RuntimeRevision),
		"generation":        uint64(v.Generation),
		"drifted":           v.Drifted,
		"loadedAt":          rfc3339(v.LoadedAt),
		"canonical":         json.RawMessage(canon),
	}, nil
}

func fromPlan(p app.Plan) map[string]any {
	diff := p.Diff
	if diff == nil {
		diff = []app.DiffEntry{}
	}
	return map[string]any{
		"previousRevision":  string(p.PreviousRevision),
		"candidateRevision": string(p.CandidateRevision),
		"drifted":           p.Drifted,
		"diff":              diff,
		"operations":        p.Operations,
	}
}

func fromApply(r app.ApplyResult) map[string]any {
	out := fromPlan(r.Plan)
	out["applied"] = r.Applied
	out["generation"] = uint64(r.Generation)
	out["runtimeRevision"] = string(r.RuntimeRevision)
	return out
}

func fromFeatures(list app.Features) map[string]any {
	items := list.Items
	if items == nil {
		items = []app.Feature{}
	}
	return map[string]any{"items": items}
}

func fromProfiles(list []app.ProfileSummary) map[string]any {
	if list == nil {
		list = []app.ProfileSummary{}
	}
	items := make([]map[string]any, 0, len(list))
	for _, p := range list {
		items = append(items, map[string]any{"name": p.Name})
	}
	return map[string]any{"items": items}
}

func fromProfile(p app.ProfileView) map[string]any {
	return map[string]any{
		"name":     p.Name,
		"modules":  marshalRaw(p.Modules),
		"schema":   marshalRaw(p.Schema),
		"instance": marshalRaw(p.Instance),
	}
}

func fromUsers(list []app.UserView) map[string]any {
	items := make([]map[string]any, 0, len(list))
	for _, u := range list {
		item := map[string]any{
			"name":    u.Name,
			"profile": u.Profile,
			"access":  u.Access,
		}
		if u.PasswordFile != "" {
			item["passwordFile"] = u.PasswordFile
		}
		if u.AuthorizedKeysFile != "" {
			item["authorizedKeysFile"] = u.AuthorizedKeysFile
		}
		items = append(items, item)
	}
	return map[string]any{"items": items}
}

func fromSessions(list []app.Session) map[string]any {
	items := make([]map[string]any, 0, len(list))
	for _, sess := range list {
		items = append(items, map[string]any{
			"id": sess.ID, "user": sess.User, "profile": sess.Profile,
		})
	}
	return map[string]any{"items": items}
}

func fromNotification(n app.Notification) map[string]any {
	ch := make([]map[string]any, 0, len(n.Changes))
	for _, c := range n.Changes {
		ch = append(ch, map[string]any{"path": c.Path, "operation": c.Operation})
	}
	return map[string]any{"id": n.ID, "profile": n.Profile, "changes": ch}
}

func fromNotifications(list []app.Notification) map[string]any {
	items := make([]map[string]any, 0, len(list))
	for _, n := range list {
		items = append(items, fromNotification(n))
	}
	return map[string]any{"items": items}
}

func fromAudit(list []app.AuditEvent) map[string]any {
	events := make([]map[string]any, 0, len(list))
	for _, e := range list {
		events = append(events, map[string]any{
			"id":         e.ID,
			"time":       rfc3339(e.Time),
			"actorId":    e.ActorID,
			"capability": e.Capability,
			"result":     e.Result,
			"errorCode":  e.ErrorCode,
		})
	}
	return map[string]any{"events": events}
}

func fromExport(format string, body []byte) map[string]any {
	return map[string]any{
		"format": format,
		"body":   string(body),
	}
}

func okResult() map[string]any {
	return map[string]any{"ok": true}
}

func marshalRaw(v any) json.RawMessage {
	if v == nil {
		return json.RawMessage("null")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}

func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func treeJSON(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return map[string]any{}, nil
	}
	return out, nil
}
