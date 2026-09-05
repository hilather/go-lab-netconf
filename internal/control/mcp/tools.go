package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/hilather/go-lab-netconf/internal/app"
	"github.com/hilather/go-lab-netconf/internal/buildinfo"
	"github.com/hilather/go-lab-netconf/internal/capabilities"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerTools() {
	addTool(s, "netconf_version_get", func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		_ = actor
		return fromVersion(buildinfo.Current()), nil
	})
	addTool(s, "netconf_capabilities_get", func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		_ = actor
		return fromCapabilities(), nil
	})
	addTool(s, "netconf_status_get", func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		_ = actor
		st, err := s.svc.Status(ctx)
		if err != nil {
			return nil, err
		}
		return fromStatus(st, st.Ready), nil
	})
	addTool(s, "netconf_schema_get", func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		_ = actor
		b, err := s.svc.Schema(ctx)
		if err != nil {
			return nil, err
		}
		return treeJSON(b)
	})
	addTool(s, "netconf_features_list", func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		_ = actor
		list, err := s.svc.Features(ctx)
		if err != nil {
			return nil, err
		}
		return fromFeatures(list), nil
	})
	addTool(s, "netconf_state_get", func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		_ = actor
		v, err := s.svc.State(ctx)
		if err != nil {
			return nil, err
		}
		return fromStateView(v)
	})
	addTool(s, "netconf_state_validate", func(ctx context.Context, actor app.Actor, in validateIn) (any, error) {
		_ = actor
		if err := s.svc.ValidateState(ctx, in.bytes()); err != nil {
			return nil, err
		}
		return okResult(), nil
	})
	addTool(s, "netconf_state_export", func(ctx context.Context, actor app.Actor, in exportIn) (any, error) {
		_ = actor
		format := strings.ToLower(in.Format)
		switch format {
		case "", "yaml", "yml":
			body, err := s.svc.ExportState(ctx)
			if err != nil {
				return nil, err
			}
			return fromExport("yaml", body), nil
		case "json":
			st, err := s.svc.State(ctx)
			if err != nil {
				return nil, err
			}
			raw, err := json.Marshal(st.Canonical)
			if err != nil {
				return nil, domainerr.ValidationFailed("marshal state: " + err.Error())
			}
			return fromExport("json", raw), nil
		default:
			return nil, domainerr.ValidationFailed("unknown export format",
				domainerr.FieldViolation{Path: "format", Code: "invalid_value", Message: "format must be yaml or json"})
		}
	})
	addTool(s, "netconf_state_reset", func(ctx context.Context, actor app.Actor, in resetIn) (any, error) {
		_ = actor
		_ = in
		if err := s.svc.Reset(ctx); err != nil {
			return nil, err
		}
		return okResult(), nil
	})
	addTool(s, "netconf_change_plan", func(ctx context.Context, actor app.Actor, in changeIn) (any, error) {
		_ = actor
		p, err := s.svc.Plan(ctx, in.Operations, in.ExpectedRevision)
		if err != nil {
			return nil, err
		}
		return fromPlan(p), nil
	})
	addTool(s, "netconf_change_apply", func(ctx context.Context, actor app.Actor, in changeIn) (any, error) {
		_ = actor
		r, err := s.svc.Apply(ctx, in.Operations, in.ExpectedRevision, in.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		return fromApply(r), nil
	})
	addTool(s, "netconf_profiles_list", func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		_ = actor
		list, err := s.svc.ListProfiles(ctx)
		if err != nil {
			return nil, err
		}
		return fromProfiles(list), nil
	})
	addTool(s, "netconf_profile_get", func(ctx context.Context, actor app.Actor, in nameIn) (any, error) {
		_ = actor
		if err := requireName(in.Name); err != nil {
			return nil, err
		}
		p, err := s.svc.GetProfile(ctx, in.Name)
		if err != nil {
			return nil, err
		}
		return fromProfile(p), nil
	})
	addTool(s, "netconf_users_list", func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		_ = actor
		list, err := s.svc.ListUsers(ctx)
		if err != nil {
			return nil, err
		}
		return fromUsers(list), nil
	})
	addTool(s, "netconf_datastore_get", func(ctx context.Context, actor app.Actor, in profileStoreIn) (any, error) {
		_ = actor
		if err := requireProfile(in.Profile); err != nil {
			return nil, err
		}
		raw, err := s.svc.GetDatastore(ctx, in.Profile, in.Store)
		if err != nil {
			return nil, err
		}
		return treeJSON(raw)
	})
	addTool(s, "netconf_datastore_set", func(ctx context.Context, actor app.Actor, in datastoreSetIn) (any, error) {
		_ = actor
		if err := requireProfile(in.Profile); err != nil {
			return nil, err
		}
		overlay, err := in.overlay()
		if err != nil {
			return nil, err
		}
		if err := s.svc.SetDatastore(ctx, in.Profile, in.Store, overlay); err != nil {
			return nil, err
		}
		return okResult(), nil
	})
	addTool(s, "netconf_datastore_commit", func(ctx context.Context, actor app.Actor, in profileIn) (any, error) {
		_ = actor
		if err := requireProfile(in.Profile); err != nil {
			return nil, err
		}
		if err := s.svc.Commit(ctx, in.Profile); err != nil {
			return nil, err
		}
		return okResult(), nil
	})
	addTool(s, "netconf_datastore_discard", func(ctx context.Context, actor app.Actor, in profileIn) (any, error) {
		_ = actor
		if err := requireProfile(in.Profile); err != nil {
			return nil, err
		}
		if err := s.svc.Discard(ctx, in.Profile); err != nil {
			return nil, err
		}
		return okResult(), nil
	})
	addTool(s, "netconf_sessions_list", func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		_ = actor
		list, err := s.svc.ListSessions(ctx)
		if err != nil {
			return nil, err
		}
		return fromSessions(list), nil
	})
	addTool(s, "netconf_session_kill", func(ctx context.Context, actor app.Actor, in idIn) (any, error) {
		_ = actor
		if err := requireID(in.ID); err != nil {
			return nil, err
		}
		if err := s.svc.KillSession(ctx, in.ID); err != nil {
			return nil, err
		}
		return okResult(), nil
	})
	addTool(s, "netconf_notifications_list", func(ctx context.Context, actor app.Actor, in notifListIn) (any, error) {
		_ = actor
		list, err := s.svc.ListNotifications(ctx, app.NotifQuery{Profile: in.Profile})
		if err != nil {
			return nil, err
		}
		return fromNotifications(list), nil
	})
	addTool(s, "netconf_notification_get", func(ctx context.Context, actor app.Actor, in idIn) (any, error) {
		_ = actor
		if err := requireID(in.ID); err != nil {
			return nil, err
		}
		n, err := s.svc.GetNotification(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		return fromNotification(n), nil
	})
	addTool(s, "netconf_notifications_wait", func(ctx context.Context, actor app.Actor, in waitIn) (any, error) {
		_ = actor
		waitCtx := ctx
		var cancel context.CancelFunc
		if in.Timeout != "" {
			d, err := time.ParseDuration(in.Timeout)
			if err != nil || d < 0 {
				return nil, domainerr.ValidationFailed("invalid timeout",
					domainerr.FieldViolation{Path: "timeout", Code: "invalid_value", Message: "timeout must be a Go duration"})
			}
			waitCtx, cancel = context.WithTimeout(ctx, d)
			defer cancel()
		}
		n, err := s.svc.WaitNotification(waitCtx, app.WaitQuery{Profile: in.Profile})
		if err != nil {
			if waitCtx.Err() != nil {
				return nil, domainerr.WaitTimeout("wait_timeout")
			}
			return nil, err
		}
		return fromNotification(n), nil
	})
	addTool(s, "netconf_notifications_clear", func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		_ = actor
		if err := s.svc.ClearNotifications(ctx); err != nil {
			return nil, err
		}
		return okResult(), nil
	})
	addTool(s, "netconf_preview_get", func(ctx context.Context, actor app.Actor, in previewIn) (any, error) {
		_ = actor
		raw, err := s.svc.PreviewGet(ctx, in.User, in.Path)
		if err != nil {
			return nil, err
		}
		return treeJSON(raw)
	})
	addTool(s, "netconf_audit_query", func(ctx context.Context, actor app.Actor, in auditQueryIn) (any, error) {
		_ = actor
		if in.Limit < 0 {
			return nil, domainerr.ValidationFailed("invalid limit",
				domainerr.FieldViolation{Path: "limit", Code: "invalid_value", Message: "limit must be a non-negative integer"})
		}
		list, err := s.svc.QueryAudit(ctx, app.AuditQuery{Limit: in.Limit})
		if err != nil {
			return nil, err
		}
		return fromAudit(list), nil
	})
}

func addTool[In any](s *Server, name string, h func(context.Context, app.Actor, In) (any, error)) {
	caps := capabilities.LookupTool(name)
	title := name
	desc := ""
	mutating := false
	idempotent := true
	if len(caps) > 0 {
		if caps[0].Title != "" {
			title = caps[0].Title
		}
		desc = caps[0].Description
		mutating = caps[0].Mutating
		idempotent = caps[0].Idempotent
	}
	readOnly := !mutating
	ann := &sdk.ToolAnnotations{
		Title:           title,
		ReadOnlyHint:    readOnly,
		IdempotentHint:  idempotent,
		DestructiveHint: boolPtr(mutating && !idempotent),
		OpenWorldHint:   boolPtr(false),
	}
	sdk.AddTool(s.sdk, &sdk.Tool{
		Name:        name,
		Title:       title,
		Description: desc,
		Annotations: ann,
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in In) (*sdk.CallToolResult, any, error) {
		if err := ctx.Err(); err != nil {
			return toolErrorResult(canceledError(err)), nil, nil
		}
		actor := s.actorFrom(ctx)
		if err := s.authorizeTool(actor, name); err != nil {
			return toolErrorResult(err), nil, nil
		}
		// recordAudit reads app.ActorFrom, not the adapter-private key.
		ctx = app.WithActor(ctx, actor)
		out, err := h(ctx, actor, in)
		if err != nil {
			return toolErrorResult(err), nil, nil
		}
		structured, err := asStructured(out)
		if err != nil {
			return nil, nil, rpcError(domainerr.ValidationFailed("internal error"))
		}
		return nil, structured, nil
	})
}

func boolPtr(v bool) *bool { return &v }

func canceledError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return domainerr.WaitTimeout("request deadline exceeded")
	}
	return domainerr.WaitTimeout("request canceled")
}
