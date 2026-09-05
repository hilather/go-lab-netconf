package app

import (
	"context"
	"encoding/json"

	"github.com/hilather/go-lab-netconf/internal/buildinfo"
	"github.com/hilather/go-lab-netconf/internal/config"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
)

func (s *App) Version(ctx context.Context) (Version, error) {
	if err := s.requireCtx(ctx); err != nil {
		return Version{}, err
	}
	return buildinfo.Current(), nil
}

func (s *App) Capabilities(ctx context.Context) ([]Capability, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *App) Status(ctx context.Context) (Status, error) {
	if err := s.requireCtx(ctx); err != nil {
		return Status{}, err
	}
	snap, err := s.active()
	if err != nil {
		return Status{}, err
	}
	return Status{
		Ready:    true,
		Revision: snap.Revision,
		Listeners: []ListenerStatus{
			{Name: "netconf", Address: snap.NetconfAddress},
			{Name: "restconf", Address: snap.RestconfAddress},
			{Name: "management", Address: snap.ManagementAddress},
		},
	}, nil
}

func (s *App) Schema(ctx context.Context) (json.RawMessage, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	return json.RawMessage(`{}`), nil
}

func (s *App) Features(ctx context.Context) (Features, error) {
	if err := s.requireCtx(ctx); err != nil {
		return Features{}, err
	}
	return Features{Items: featureCatalog()}, nil
}

func (s *App) State(ctx context.Context) (StateView, error) {
	if err := s.requireCtx(ctx); err != nil {
		return StateView{}, err
	}
	snap, err := s.active()
	if err != nil {
		return StateView{}, err
	}
	copied, err := cloneState(snap.Canonical)
	if err != nil {
		return StateView{}, err
	}
	return StateView{
		BootstrapRevision: snap.BootstrapRevision,
		RuntimeRevision:   snap.Revision,
		Generation:        snap.Generation,
		Drifted:           snap.Drifted(),
		LoadedAt:          snap.CompiledAt,
		Canonical:         copied,
	}, nil
}

func (s *App) ValidateState(ctx context.Context, body []byte) error {
	if err := s.requireCtx(ctx); err != nil {
		return err
	}
	_, err := config.Load(body)
	return err
}

func (s *App) ExportState(ctx context.Context) ([]byte, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	return config.CanonicalYAML(snap.Canonical)
}

func (s *App) ListProfiles(ctx context.Context) ([]ProfileSummary, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	out := make([]ProfileSummary, 0, len(snap.Profiles))
	for _, p := range snap.Profiles {
		out = append(out, ProfileSummary{Name: p.Name})
	}
	return out, nil
}

func (s *App) GetProfile(ctx context.Context, name string) (ProfileView, error) {
	if err := s.requireCtx(ctx); err != nil {
		return ProfileView{}, err
	}
	if name == "" {
		return ProfileView{}, domainerr.ValidationFailed("name is required",
			domainerr.FieldViolation{Path: "name", Code: "required", Message: "name is required"})
	}
	snap, err := s.active()
	if err != nil {
		return ProfileView{}, err
	}
	p, ok := snap.ProfileNamed(name)
	if !ok {
		return ProfileView{}, domainerr.NotFound("profile " + name + " not found")
	}
	copied, err := cloneState(snap.Canonical)
	if err != nil {
		return ProfileView{}, err
	}
	var instance map[string]any
	var modules []model.ModuleSpec
	var schema []model.SchemaLeaf
	for _, sp := range copied.Spec.Profiles {
		if sp.Name == name {
			instance = sp.Instance
			modules = sp.Modules
			schema = sp.Schema
			break
		}
	}
	if instance == nil {
		instance = p.Running.Map()
	}
	return ProfileView{Name: p.Name, Modules: modules, Schema: schema, Instance: instance}, nil
}

func (s *App) ListUsers(ctx context.Context) ([]UserView, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	snap, err := s.active()
	if err != nil {
		return nil, err
	}
	out := make([]UserView, 0, len(snap.Users))
	for _, u := range snap.Users {
		out = append(out, UserView{
			Name:               u.Name,
			Profile:            u.Profile,
			Access:             u.Access,
			PasswordFile:       u.PasswordFile,
			AuthorizedKeysFile: u.AuthorizedKeysFile,
		})
	}
	return out, nil
}

func (s *App) ListSessions(ctx context.Context) ([]Session, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *App) KillSession(ctx context.Context, id string) error {
	if err := s.requireCtx(ctx); err != nil {
		return err
	}
	if id == "" {
		return domainerr.ValidationFailed("id is required",
			domainerr.FieldViolation{Path: "id", Code: "required", Message: "id is required"})
	}
	return domainerr.NotFound("session " + id + " not found")
}

func (s *App) QueryAudit(ctx context.Context, q AuditQuery) ([]AuditEvent, error) {
	if err := s.requireCtx(ctx); err != nil {
		return nil, err
	}
	_ = q
	return nil, nil
}
