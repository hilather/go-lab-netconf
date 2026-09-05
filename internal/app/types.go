package app

import (
	"encoding/json"
	"time"

	"github.com/hilather/go-lab-netconf/internal/audit"
	"github.com/hilather/go-lab-netconf/internal/model"
)

// Actor is the caller identity recorded on audit events.
type Actor struct {
	ID        string
	Class     string
	Role      string
	Scopes    []string
	Transport string
}

const (
	ApplyLive      = "live"
	ApplyResetOnly = "reset-only"
)

// Closed plan/apply verbs from docs/04. Listener addresses, host keys,
// tokens, tls/callHome/netconfTls, ui.enabled, and management.address are
// reset-only and are not verbs.
const (
	OpReplaceProfiles      = "replaceProfiles"
	OpUpsertProfile        = "upsertProfile"
	OpRemoveProfile        = "removeProfile"
	OpReplaceUsers         = "replaceUsers"
	OpUpsertUser           = "upsertUser"
	OpRemoveUser           = "removeUser"
	OpReplaceAdmission     = "replaceAdmission"
	OpReplaceNetconfCaps   = "replaceNetconfCaps"
	OpReplaceObservability = "replaceObservability"
)

// ApplyOp is one typed live mutation. datastore:set/commit/discard are
// Service methods, not apply verbs.
type ApplyOp struct {
	Op            string               `json:"op"`
	Profiles      []model.ProfileSpec  `json:"profiles,omitempty"`
	Profile       *model.ProfileSpec   `json:"profile,omitempty"`
	Users         []model.UserSpec     `json:"users,omitempty"`
	User          *model.UserSpec      `json:"user,omitempty"`
	Name          string               `json:"name,omitempty"`
	Admission     *model.AdmissionSpec `json:"admission,omitempty"`
	Netconf       *model.NetconfSpec   `json:"netconf,omitempty"`
	Observability json.RawMessage      `json:"observability,omitempty"`
}

// KnownOp reports whether op is a v1alpha1 plan/apply verb.
func KnownOp(op string) bool {
	switch op {
	case OpReplaceProfiles, OpUpsertProfile, OpRemoveProfile,
		OpReplaceUsers, OpUpsertUser, OpRemoveUser,
		OpReplaceAdmission, OpReplaceNetconfCaps, OpReplaceObservability:
		return true
	default:
		return false
	}
}

// Plan is the dry-run result of plan (and the body of apply).
type Plan struct {
	PreviousRevision  model.Revision
	CandidateRevision model.Revision
	Drifted           bool
	Diff              []DiffEntry
	Operations        []ApplyOp
}

// ApplyResult is a committed mutation result.
type ApplyResult struct {
	Plan
	Applied         bool
	Generation      model.Generation
	RuntimeRevision model.Revision
}

// DiffEntry is one canonical-path change.
type DiffEntry struct {
	Path   string          `json:"path"`
	Op     string          `json:"op"`
	Before json.RawMessage `json:"before,omitempty"`
	After  json.RawMessage `json:"after,omitempty"`
}

// StateView is GET /v1/state. Canonical is a copy.
type StateView struct {
	BootstrapRevision model.Revision
	RuntimeRevision   model.Revision
	Generation        model.Generation
	Drifted           bool
	LoadedAt          time.Time
	Canonical         *model.State
}

// Status is the agent-readable process DTO.
type Status struct {
	Ready     bool
	Revision  model.Revision
	Listeners []ListenerStatus
}

// ListenerStatus is one configured listener.
type ListenerStatus struct {
	Name    string
	Address string
}

// Features is GET /v1/features.
type Features struct {
	Items []Feature
}

// Feature is one frozen live vs reset-only row.
type Feature struct {
	ID    string `json:"id"`
	Apply string `json:"apply"`
	Path  string `json:"path"`
}

// Capability is one discovery row. The catalog is filled by a later package.
type Capability struct {
	Name string
}

// ProfileSummary is one row of GET /v1/profiles.
type ProfileSummary struct {
	Name string
}

// ProfileView is GET /v1/profiles/{name}.
type ProfileView struct {
	Name     string
	Modules  []model.ModuleSpec
	Schema   []model.SchemaLeaf
	Instance map[string]any
}

// UserView is one data-plane user with secret bytes omitted.
type UserView struct {
	Name               string
	Profile            string
	Access             string
	PasswordFile       string
	AuthorizedKeysFile string
}

// Session is one NETCONF or RESTCONF session. Empty until those listeners exist.
type Session struct {
	ID      string
	User    string
	Profile string
}

// AuditQuery lists recent in-memory events.
type AuditQuery struct {
	Limit int
}

// AuditEvent is one mutation record from the in-process ring.
type AuditEvent = audit.Event
