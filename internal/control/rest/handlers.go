package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hilather/go-lab-netconf/internal/app"
	"github.com/hilather/go-lab-netconf/internal/buildinfo"
	"github.com/hilather/go-lab-netconf/internal/capabilities"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/notif"
	"github.com/hilather/go-lab-netconf/internal/observability"
)

func (s *Server) dispatch(w http.ResponseWriter, r *http.Request, instance string, rt compiledRoute, params map[string]string) {
	ctx := r.Context()
	if err := ctx.Err(); err != nil {
		s.writeProblem(w, r, instance, domainerr.WaitTimeout("request canceled"))
		return
	}
	switch rt.cap.ID {
	case capabilities.HealthLive:
		s.handleHealthLive(w, r)
	case capabilities.HealthReady:
		s.handleHealthReady(w, r, ctx)
	case capabilities.SessionCreate:
		s.handleSessionCreate(w, r, instance, app.ActorFrom(ctx))
	case capabilities.SessionGet:
		s.handleSessionGet(w, r, instance, app.ActorFrom(ctx))
	case capabilities.SessionDelete:
		s.handleSessionDelete(w, r, instance, app.ActorFrom(ctx))
	case capabilities.VersionGet:
		s.writeJSON(w, http.StatusOK, fromVersion(buildinfo.Current()))
	case capabilities.CapabilitiesGet:
		s.writeJSON(w, http.StatusOK, fromCapabilities())
	case capabilities.StatusGet:
		s.handleStatus(w, r, instance, ctx)
	case capabilities.SchemaGet:
		s.handleSchema(w, r, instance, ctx)
	case capabilities.FeaturesList:
		s.handleFeatures(w, r, instance, ctx)
	case capabilities.StateGet:
		s.handleGetState(w, r, instance, ctx)
	case capabilities.StateValidate:
		s.handleValidate(w, r, instance, ctx)
	case capabilities.ChangesPlan:
		s.handlePlan(w, r, instance, ctx)
	case capabilities.ChangesApply:
		s.handleApply(w, r, instance, ctx)
	case capabilities.StateExport:
		s.handleExport(w, r, instance, ctx)
	case capabilities.StateReset:
		s.handleReset(w, r, instance, ctx)
	case capabilities.ProfilesList:
		s.handleProfilesList(w, r, instance, ctx)
	case capabilities.ProfilesGet:
		s.handleProfilesGet(w, r, instance, ctx, params["name"])
	case capabilities.UsersList:
		s.handleUsersList(w, r, instance, ctx)
	case capabilities.DatastoreGet:
		s.handleDatastoreGet(w, r, instance, ctx, params["profile"], params["store"])
	case capabilities.DatastoreSet:
		s.handleDatastoreSet(w, r, instance, ctx, params["profile"], params["store"])
	case capabilities.DatastoreCommit:
		s.handleCommit(w, r, instance, ctx, params["profile"])
	case capabilities.DatastoreDiscard:
		s.handleDiscard(w, r, instance, ctx, params["profile"])
	case capabilities.SessionsList:
		s.handleSessionsList(w, r, instance, ctx)
	case capabilities.SessionKill:
		s.handleSessionKill(w, r, instance, ctx, params["id"])
	case capabilities.NotificationsList:
		s.handleNotificationsList(w, r, instance, ctx)
	case capabilities.NotificationsGet:
		s.handleNotificationGet(w, r, instance, ctx, params["id"])
	case capabilities.NotificationsWait:
		s.handleNotificationsWait(w, r, instance, ctx)
	case capabilities.NotificationsClear:
		s.handleNotificationsClear(w, r, instance, ctx)
	case capabilities.PreviewGet:
		s.handlePreview(w, r, instance, ctx)
	case capabilities.AuditQuery:
		s.handleAudit(w, r, instance, ctx)
	case capabilities.MetricsGet:
		s.handleMetrics(w, r)
	default:
		s.writeProblem(w, r, instance, domainerr.NotFound("not found"))
	}
}

func (s *Server) handleHealthLive(w http.ResponseWriter, r *http.Request) {
	if !s.isLive() {
		s.writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "down"})
		return
	}
	s.writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	_ = r
}

func (s *Server) handleHealthReady(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	if !s.isReady(ctx) {
		s.writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "not ready"})
		return
	}
	s.writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	_ = r
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context) {
	st, err := s.svc.Status(ctx)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	listeners := make([]listenerJSON, 0, len(st.Listeners))
	for _, l := range st.Listeners {
		listeners = append(listeners, listenerJSON{Name: l.Name, Address: l.Address})
	}
	s.writeJSON(w, http.StatusOK, statusResponse{
		Ready:     s.isReady(ctx),
		Revision:  string(st.Revision),
		Listeners: listeners,
	})
	_ = r
}

func (s *Server) handleSchema(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context) {
	b, err := s.svc.Schema(ctx)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	if len(b) == 0 {
		b = []byte("{}")
	}
	s.writeBytes(w, http.StatusOK, "application/schema+json", b)
	_ = r
}

func (s *Server) handleFeatures(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context) {
	list, err := s.svc.Features(ctx)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	items := list.Items
	if items == nil {
		items = []app.Feature{}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"items": items})
	_ = r
}

func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context) {
	v, err := s.svc.State(ctx)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	canon, err := json.Marshal(v.Canonical)
	if err != nil {
		s.writeProblem(w, r, instance, domainerr.ValidationFailed("marshal state: "+err.Error()))
		return
	}
	if v.RuntimeRevision != "" {
		w.Header().Set(headerRevision, string(v.RuntimeRevision))
	}
	s.writeJSON(w, http.StatusOK, stateViewJSON{
		BootstrapRevision: string(v.BootstrapRevision),
		RuntimeRevision:   string(v.RuntimeRevision),
		Generation:        uint64(v.Generation),
		Drifted:           v.Drifted,
		LoadedAt:          rfc3339(v.LoadedAt),
		Canonical:         canon,
	})
	_ = r
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context) {
	body, ok := s.readRawBody(w, r, instance, true)
	if !ok {
		return
	}
	if err := s.svc.ValidateState(ctx, body); err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context) {
	var in changeRequest
	if !s.decodeJSON(w, r, instance, &in) {
		return
	}
	plan, err := s.svc.Plan(ctx, in.Operations, expectedRevision(r, in.ExpectedRevision))
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromPlan(plan))
}

func (s *Server) handleApply(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context) {
	var in changeRequest
	if !s.decodeJSON(w, r, instance, &in) {
		return
	}
	res, err := s.svc.Apply(ctx, in.Operations, expectedRevision(r, in.ExpectedRevision), idempotencyKey(r, in.IdempotencyKey))
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	if res.RuntimeRevision != "" {
		w.Header().Set(headerRevision, string(res.RuntimeRevision))
	}
	s.writeJSON(w, http.StatusOK, fromApply(res))
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context) {
	format := strings.ToLower(r.URL.Query().Get("format"))
	switch format {
	case "", "yaml", "yml":
	case "json":
		st, err := s.svc.State(ctx)
		if err != nil {
			s.writeProblem(w, r, instance, asDomain(err))
			return
		}
		s.writeJSON(w, http.StatusOK, st.Canonical)
		return
	default:
		s.writeProblem(w, r, instance, domainerr.ValidationFailed("unknown export format",
			domainerr.FieldViolation{Path: "format", Code: "invalid_value", Message: "format must be yaml or json"}))
		return
	}
	body, err := s.svc.ExportState(ctx)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeBytes(w, http.StatusOK, "application/yaml", body)
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context) {
	var in struct {
		Reason string `json:"reason"`
	}
	if !s.decodeJSONOptional(w, r, instance, &in) {
		return
	}
	if err := s.svc.Reset(ctx); err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleProfilesList(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context) {
	list, err := s.svc.ListProfiles(ctx)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	if list == nil {
		list = []app.ProfileSummary{}
	}
	items := make([]profileJSON, 0, len(list))
	for _, p := range list {
		items = append(items, profileJSON{Name: p.Name})
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"items": items})
	_ = r
}

func (s *Server) handleProfilesGet(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, name string) {
	p, err := s.svc.GetProfile(ctx, name)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, profileJSON{
		Name:     p.Name,
		Modules:  marshalRaw(p.Modules),
		Schema:   marshalRaw(p.Schema),
		Instance: marshalRaw(p.Instance),
	})
}

func (s *Server) handleUsersList(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context) {
	list, err := s.svc.ListUsers(ctx)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	items := make([]userJSON, 0, len(list))
	for _, u := range list {
		items = append(items, userJSON{
			Name:               u.Name,
			Profile:            u.Profile,
			Access:             u.Access,
			PasswordFile:       u.PasswordFile,
			AuthorizedKeysFile: u.AuthorizedKeysFile,
		})
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"items": items})
	_ = r
}

func (s *Server) handleDatastoreGet(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, profile, store string) {
	raw, err := s.svc.GetDatastore(ctx, profile, store)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeBytes(w, http.StatusOK, "application/json; charset=utf-8", raw)
}

func (s *Server) handleDatastoreSet(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, profile, store string) {
	if err := s.checkJSONContentType(r); err != nil {
		s.writeProblem(w, r, instance, err)
		return
	}
	body, ok := s.readRawBody(w, r, instance, true)
	if !ok {
		return
	}
	if err := s.svc.SetDatastore(ctx, profile, store, json.RawMessage(body)); err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleCommit(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, profile string) {
	var in struct{}
	if !s.decodeJSONOptional(w, r, instance, &in) {
		return
	}
	if err := s.svc.Commit(ctx, profile); err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDiscard(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, profile string) {
	var in struct{}
	if !s.decodeJSONOptional(w, r, instance, &in) {
		return
	}
	if err := s.svc.Discard(ctx, profile); err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleSessionsList(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context) {
	list, err := s.svc.ListSessions(ctx)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	items := make([]sessionJSON, 0, len(list))
	for _, sess := range list {
		items = append(items, sessionJSON{ID: sess.ID, User: sess.User, Profile: sess.Profile})
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"items": items})
	_ = r
}

func (s *Server) handleSessionKill(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, id string) {
	var in struct{}
	if !s.decodeJSONOptional(w, r, instance, &in) {
		return
	}
	if err := s.svc.KillSession(ctx, id); err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleNotificationsList(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context) {
	q := notif.Query{Profile: r.URL.Query().Get("profile")}
	list, err := s.svc.ListNotifications(ctx, q)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	items := make([]notificationJSON, 0, len(list))
	for _, n := range list {
		items = append(items, fromNotification(n))
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleNotificationGet(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, id string) {
	n, err := s.svc.GetNotification(ctx, id)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromNotification(n))
}

func (s *Server) handleNotificationsWait(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context) {
	var in waitRequest
	if !s.decodeJSONOptional(w, r, instance, &in) {
		return
	}
	waitCtx := ctx
	var cancel context.CancelFunc
	if in.Timeout != "" {
		d, err := time.ParseDuration(in.Timeout)
		if err != nil || d < 0 {
			s.writeProblem(w, r, instance, domainerr.ValidationFailed("invalid timeout",
				domainerr.FieldViolation{Path: "timeout", Code: "invalid_value", Message: "timeout must be a Go duration"}))
			return
		}
		waitCtx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}
	n, err := s.svc.WaitNotification(waitCtx, notif.WaitQuery{Profile: in.Profile})
	if err != nil {
		if waitCtx.Err() != nil {
			s.writeProblem(w, r, instance, domainerr.WaitTimeout("wait_timeout"))
			return
		}
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromNotification(n))
}

func (s *Server) handleNotificationsClear(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context) {
	var in struct{}
	if !s.decodeJSONOptional(w, r, instance, &in) {
		return
	}
	if err := s.svc.ClearNotifications(ctx); err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context) {
	user := strings.TrimSpace(r.URL.Query().Get("user"))
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	raw, err := s.svc.PreviewGet(ctx, user, path)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeBytes(w, http.StatusOK, "application/json; charset=utf-8", raw)
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			s.writeProblem(w, r, instance, domainerr.ValidationFailed("invalid limit",
				domainerr.FieldViolation{Path: "limit", Code: "invalid_value", Message: "limit must be a non-negative integer"}))
			return
		}
		limit = n
	}
	list, err := s.svc.QueryAudit(ctx, app.AuditQuery{Limit: limit})
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	events := make([]auditJSON, 0, len(list))
	for _, e := range list {
		events = append(events, auditJSON{
			ID:         e.ID,
			Time:       rfc3339(e.Time),
			ActorID:    e.ActorID,
			Capability: e.Capability,
			Result:     e.Result,
			ErrorCode:  e.ErrorCode,
		})
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"events": events})
	_ = r
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	observability.Handler(s.metrics).ServeHTTP(w, r)
}
