package config

import (
	"encoding/json"
	"strings"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
)

// Normalize returns a copy of st with defaults materialized. allowClientCidrs
// nil (omitted/null) becomes loopback; a present empty list stays deny-all.
func Normalize(st *model.State) (*model.State, error) {
	if st == nil {
		return nil, domainerr.ValidationFailed("nil state",
			domainerr.FieldViolation{Path: "", Code: violationRequired, Message: "state is nil"})
	}
	out, err := cloneState(st)
	if err != nil {
		return nil, err
	}
	materializeDefaults(&out.Spec)
	return out, nil
}

func cloneState(st *model.State) (*model.State, error) {
	b, err := json.Marshal(st)
	if err != nil {
		return nil, domainerr.New(domainerr.CodeValidationFailed, "clone marshal: "+err.Error())
	}
	var out model.State
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, domainerr.New(domainerr.CodeValidationFailed, "clone unmarshal: "+err.Error())
	}
	return &out, nil
}

func materializeDefaults(sp *model.Spec) {
	if strings.TrimSpace(sp.Listeners.Netconf.Address) == "" {
		sp.Listeners.Netconf.Address = DefaultNetconfAddress
	}
	if strings.TrimSpace(sp.Listeners.Restconf.Address) == "" {
		sp.Listeners.Restconf.Address = DefaultRestconfAddress
	}
	if strings.TrimSpace(sp.Listeners.Management.Address) == "" {
		sp.Listeners.Management.Address = DefaultMgmtAddress
	}
	if strings.TrimSpace(sp.Listeners.Management.RESTPath) == "" {
		sp.Listeners.Management.RESTPath = DefaultRESTPath
	}
	if strings.TrimSpace(sp.Listeners.Management.MCPPath) == "" {
		sp.Listeners.Management.MCPPath = DefaultMCPPath
	}
	if strings.TrimSpace(sp.Auth.Mode) == "" {
		sp.Auth.Mode = model.MgmtAuthBearer
	}
	if sp.Auth.Tokens == nil {
		sp.Auth.Tokens = []model.TokenSpec{}
	}
	if len(sp.Netconf.Versions) == 0 {
		sp.Netconf.Versions = []string{model.NetconfVersion10, model.NetconfVersion11}
	}
	if sp.Admission.AllowClientCidrs == nil {
		sp.Admission.AllowClientCidrs = append([]string(nil), DefaultLoopbackCIDRs...)
	}
	if sp.Management.AllowedOrigins == nil {
		sp.Management.AllowedOrigins = []string{}
	}
	if sp.Profiles == nil {
		sp.Profiles = []model.ProfileSpec{}
	}
	if sp.Users == nil {
		sp.Users = []model.UserSpec{}
	}
	for i := range sp.Profiles {
		if sp.Profiles[i].Instance == nil {
			sp.Profiles[i].Instance = map[string]any{}
		}
	}
}
