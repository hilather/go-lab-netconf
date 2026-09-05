package rest

import (
	"encoding/json"

	"github.com/hilather/go-lab-netconf/internal/app"
	"github.com/hilather/go-lab-netconf/internal/buildinfo"
	"github.com/hilather/go-lab-netconf/internal/capabilities"
	"github.com/hilather/go-lab-netconf/internal/notif"
)

type healthResponse struct {
	Status string `json:"status"`
}

type versionResponse struct {
	Version   string           `json:"version"`
	Commit    string           `json:"commit"`
	BuildTime string           `json:"buildTime"`
	Protocols versionProtocols `json:"protocols"`
}

type versionProtocols struct {
	ConfigAPI string `json:"configAPI"`
	REST      string `json:"rest"`
	MCP       string `json:"mcp"`
}

type capabilityViewResponse struct {
	Capabilities []capabilityInfo `json:"capabilities"`
}

type capabilityInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Mutating    bool   `json:"mutating"`
	Idempotent  bool   `json:"idempotent"`
}

type statusResponse struct {
	Ready     bool           `json:"ready"`
	Revision  string         `json:"revision"`
	Listeners []listenerJSON `json:"listeners"`
}

type listenerJSON struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type stateViewJSON struct {
	BootstrapRevision string          `json:"bootstrapRevision"`
	RuntimeRevision   string          `json:"runtimeRevision"`
	Generation        uint64          `json:"generation"`
	Drifted           bool            `json:"drifted"`
	LoadedAt          string          `json:"loadedAt,omitempty"`
	Canonical         json.RawMessage `json:"canonical"`
}

type changeRequest struct {
	ExpectedRevision string        `json:"expectedRevision"`
	IdempotencyKey   string        `json:"idempotencyKey"`
	Operations       []app.ApplyOp `json:"operations"`
}

type planJSON struct {
	PreviousRevision  string          `json:"previousRevision"`
	CandidateRevision string          `json:"candidateRevision"`
	Drifted           bool            `json:"drifted"`
	Diff              []app.DiffEntry `json:"diff"`
	Operations        []app.ApplyOp   `json:"operations,omitempty"`
	Applied           bool            `json:"applied,omitempty"`
	Generation        uint64          `json:"generation,omitempty"`
	RuntimeRevision   string          `json:"runtimeRevision,omitempty"`
}

type waitRequest struct {
	Profile string `json:"profile"`
	Timeout string `json:"timeout"`
}

type notificationJSON struct {
	ID      string         `json:"id"`
	Profile string         `json:"profile"`
	Changes []notif.Change `json:"changes"`
}

type profileJSON struct {
	Name     string          `json:"name"`
	Modules  json.RawMessage `json:"modules,omitempty"`
	Schema   json.RawMessage `json:"schema,omitempty"`
	Instance json.RawMessage `json:"instance,omitempty"`
}

type userJSON struct {
	Name               string `json:"name"`
	Profile            string `json:"profile"`
	Access             string `json:"access"`
	PasswordFile       string `json:"passwordFile,omitempty"`
	AuthorizedKeysFile string `json:"authorizedKeysFile,omitempty"`
}

type sessionJSON struct {
	ID      string `json:"id"`
	User    string `json:"user"`
	Profile string `json:"profile"`
}

type sessionCreateJSON struct {
	CSRF      string `json:"csrf"`
	ExpiresAt string `json:"expiresAt"`
}

type sessionViewJSON struct {
	ID        string   `json:"id"`
	Role      string   `json:"role"`
	Scopes    []string `json:"scopes"`
	CSRF      string   `json:"csrf,omitempty"`
	ExpiresAt string   `json:"expiresAt,omitempty"`
}

type auditJSON struct {
	ID         string `json:"id"`
	Time       string `json:"time,omitempty"`
	ActorID    string `json:"actorId,omitempty"`
	Capability string `json:"capability,omitempty"`
	Result     string `json:"result,omitempty"`
	ErrorCode  string `json:"errorCode,omitempty"`
}

func fromVersion(info buildinfo.Info) versionResponse {
	return versionResponse{
		Version:   info.Version,
		Commit:    info.Commit,
		BuildTime: info.BuildTime,
		Protocols: versionProtocols{
			ConfigAPI: info.Protocols.ConfigAPI,
			REST:      info.Protocols.REST,
			MCP:       info.Protocols.MCP,
		},
	}
}

func fromCapabilities() capabilityViewResponse {
	src := capabilities.DiscoveryList()
	out := make([]capabilityInfo, 0, len(src))
	for _, d := range src {
		out = append(out, capabilityInfo{
			Name: d.Name, Version: d.Version, Description: d.Description,
			Mutating: d.Mutating, Idempotent: d.Idempotent,
		})
	}
	return capabilityViewResponse{Capabilities: out}
}

func fromPlan(p app.Plan) planJSON {
	diff := p.Diff
	if diff == nil {
		diff = []app.DiffEntry{}
	}
	return planJSON{
		PreviousRevision:  string(p.PreviousRevision),
		CandidateRevision: string(p.CandidateRevision),
		Drifted:           p.Drifted,
		Diff:              diff,
		Operations:        p.Operations,
	}
}

func fromApply(r app.ApplyResult) planJSON {
	out := fromPlan(r.Plan)
	out.Applied = r.Applied
	out.Generation = uint64(r.Generation)
	out.RuntimeRevision = string(r.RuntimeRevision)
	return out
}

func fromNotification(n notif.Notification) notificationJSON {
	ch := n.Changes
	if ch == nil {
		ch = []notif.Change{}
	}
	return notificationJSON{ID: n.ID, Profile: n.Profile, Changes: ch}
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
