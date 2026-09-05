package app

import (
	"strconv"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
)

func applyOperations(st *model.State, ops []ApplyOp) error {
	if st == nil {
		return domainerr.ValidationFailed("nil state",
			domainerr.FieldViolation{Path: "", Code: "required", Message: "state is nil"})
	}
	for i, op := range ops {
		if err := applyOne(st, op, i); err != nil {
			return err
		}
	}
	return nil
}

func applyOne(st *model.State, op ApplyOp, i int) error {
	path := "operations[" + strconv.Itoa(i) + "]"
	if !KnownOp(op.Op) {
		return domainerr.ValidationFailed("unknown operation",
			domainerr.FieldViolation{Path: path + ".op", Code: "invalid_value", Message: "unknown op; listener addresses, host keys, tokens, tls/callHome/netconfTls, ui.enabled, and management.address are reset-only"}).
			WithRemediation("reset-only; rewrite bootstrap and POST /v1/state:reset")
	}
	switch op.Op {
	case OpReplaceProfiles:
		st.Spec.Profiles = append([]model.ProfileSpec(nil), op.Profiles...)
	case OpUpsertProfile:
		if op.Profile == nil {
			return domainerr.ValidationFailed("missing profile",
				domainerr.FieldViolation{Path: path + ".profile", Code: "required", Message: "upsertProfile requires profile"})
		}
		upsertProfile(&st.Spec.Profiles, *op.Profile)
	case OpRemoveProfile:
		if op.Name == "" {
			return domainerr.ValidationFailed("missing name",
				domainerr.FieldViolation{Path: path + ".name", Code: "required", Message: "removeProfile requires name"})
		}
		if !removeProfile(&st.Spec.Profiles, op.Name) {
			return domainerr.NotFound("profile " + op.Name + " not found")
		}
	case OpReplaceUsers:
		st.Spec.Users = append([]model.UserSpec(nil), op.Users...)
	case OpUpsertUser:
		if op.User == nil {
			return domainerr.ValidationFailed("missing user",
				domainerr.FieldViolation{Path: path + ".user", Code: "required", Message: "upsertUser requires user"})
		}
		upsertUser(&st.Spec.Users, *op.User)
	case OpRemoveUser:
		if op.Name == "" {
			return domainerr.ValidationFailed("missing name",
				domainerr.FieldViolation{Path: path + ".name", Code: "required", Message: "removeUser requires name"})
		}
		if !removeUser(&st.Spec.Users, op.Name) {
			return domainerr.NotFound("user " + op.Name + " not found")
		}
	case OpReplaceAdmission:
		if op.Admission == nil {
			return domainerr.ValidationFailed("missing admission",
				domainerr.FieldViolation{Path: path + ".admission", Code: "required", Message: "replaceAdmission requires admission"})
		}
		a := *op.Admission
		if a.AllowClientCidrs != nil {
			a.AllowClientCidrs = append([]string(nil), a.AllowClientCidrs...)
		}
		st.Spec.Admission = a
	case OpReplaceNetconfCaps:
		if op.Netconf == nil {
			return domainerr.ValidationFailed("missing netconf",
				domainerr.FieldViolation{Path: path + ".netconf", Code: "required", Message: "replaceNetconfCaps requires netconf"})
		}
		st.Spec.Netconf.Versions = append([]string(nil), op.Netconf.Versions...)
		st.Spec.Netconf.Notifications = op.Netconf.Notifications
	case OpReplaceObservability:
		// Live verb with no 1.0 spec field to persist.
	}
	return nil
}

func upsertProfile(list *[]model.ProfileSpec, p model.ProfileSpec) {
	for i := range *list {
		if (*list)[i].Name == p.Name {
			(*list)[i] = p
			return
		}
	}
	*list = append(*list, p)
}

func removeProfile(list *[]model.ProfileSpec, name string) bool {
	dst := (*list)[:0]
	found := false
	for _, p := range *list {
		if p.Name == name {
			found = true
			continue
		}
		dst = append(dst, p)
	}
	*list = dst
	return found
}

func upsertUser(list *[]model.UserSpec, u model.UserSpec) {
	for i := range *list {
		if (*list)[i].Name == u.Name {
			(*list)[i] = u
			return
		}
	}
	*list = append(*list, u)
}

func removeUser(list *[]model.UserSpec, name string) bool {
	dst := (*list)[:0]
	found := false
	for _, u := range *list {
		if u.Name == name {
			found = true
			continue
		}
		dst = append(dst, u)
	}
	*list = dst
	return found
}

func rejectResetOnly(before, after *model.State) error {
	if before == nil || after == nil {
		return nil
	}
	rem := "reset-only; rewrite bootstrap and POST /v1/state:reset"
	if !listenersEqual(before.Spec.Listeners, after.Spec.Listeners) {
		return domainerr.ValidationFailed("listeners are reset-only",
			domainerr.FieldViolation{Path: "spec.listeners", Code: "invalid_value", Message: "listener addresses, host keys, and tls/callHome/netconfTls cannot change via Apply"}).
			WithRemediation(rem)
	}
	if !authEqual(before.Spec.Auth, after.Spec.Auth) {
		return domainerr.ValidationFailed("spec.auth is reset-only",
			domainerr.FieldViolation{Path: "spec.auth", Code: "invalid_value", Message: "auth tokens cannot change via Apply"}).
			WithRemediation(rem)
	}
	if before.Spec.UI != after.Spec.UI {
		return domainerr.ValidationFailed("spec.ui is reset-only",
			domainerr.FieldViolation{Path: "spec.ui.enabled", Code: "invalid_value", Message: "ui.enabled cannot change via Apply"}).
			WithRemediation(rem)
	}
	if before.Spec.Netconf.SharedProfileDatastore != after.Spec.Netconf.SharedProfileDatastore {
		return domainerr.ValidationFailed("sharedProfileDatastore is reset-only",
			domainerr.FieldViolation{Path: "spec.netconf.sharedProfileDatastore", Code: "invalid_value", Message: "sharedProfileDatastore cannot change via Apply"}).
			WithRemediation(rem)
	}
	if before.Spec.Restconf != after.Spec.Restconf {
		return domainerr.ValidationFailed("spec.restconf is reset-only",
			domainerr.FieldViolation{Path: "spec.restconf", Code: "invalid_value", Message: "restconf encoding flags cannot change via Apply"}).
			WithRemediation(rem)
	}
	return nil
}

func listenersEqual(a, b model.ListenersSpec) bool {
	return a.Netconf == b.Netconf && a.Restconf == b.Restconf && a.Management == b.Management &&
		a.CallHome == b.CallHome && a.NetconfTLS == b.NetconfTLS
}

func authEqual(a, b model.AuthSpec) bool {
	if a.Mode != b.Mode || len(a.Tokens) != len(b.Tokens) {
		return false
	}
	for i := range a.Tokens {
		if a.Tokens[i].ID != b.Tokens[i].ID || a.Tokens[i].Role != b.Tokens[i].Role ||
			a.Tokens[i].SecretFile != b.Tokens[i].SecretFile {
			return false
		}
	}
	return true
}
