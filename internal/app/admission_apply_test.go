package app

import (
	"context"
	"net/netip"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/model"
)

// A present empty allowClientCidrs is deny-all. Apply must not turn it
// back into the loopback default.
func TestApplyEmptyAdmissionIsDenyAll(t *testing.T) {
	svc, snap := mustBoot(t)
	if _, err := svc.Apply(context.Background(), []ApplyOp{{
		Op:        OpReplaceAdmission,
		Admission: &model.AdmissionSpec{AllowClientCidrs: []string{}},
	}}, string(snap.Revision), "deny-all"); err != nil {
		t.Fatal(err)
	}
	live := svc.Active()
	cidrs := live.Canonical.Spec.Admission.AllowClientCidrs
	if cidrs == nil || len(cidrs) != 0 {
		t.Fatalf("canonical allowClientCidrs = %#v, want non-nil empty", cidrs)
	}
	if !live.AllowDenyAll || live.Allowed(netip.MustParseAddr("127.0.0.1")) {
		t.Fatalf("empty allowClientCidrs denyAll=%v cidrs=%#v allow=%v", live.AllowDenyAll, cidrs, live.Allow)
	}
}

// An omitted or null allowClientCidrs stays the loopback default.
// This guard is true before and after the empty-slice fix.
func TestApplyNilAdmissionStaysLoopback(t *testing.T) {
	svc, snap := mustBoot(t)
	if _, err := svc.Apply(context.Background(), []ApplyOp{{
		Op:        OpReplaceAdmission,
		Admission: &model.AdmissionSpec{},
	}}, string(snap.Revision), "nil-admission"); err != nil {
		t.Fatal(err)
	}
	live := svc.Active()
	if live.AllowDenyAll || !live.Allowed(netip.MustParseAddr("127.0.0.1")) {
		t.Fatalf("nil allowClientCidrs denyAll=%v allow=%v, want loopback", live.AllowDenyAll, live.Allow)
	}
	cidrs := live.Canonical.Spec.Admission.AllowClientCidrs
	if len(cidrs) != 2 || cidrs[0] != "127.0.0.0/8" || cidrs[1] != "::1/128" {
		t.Fatalf("nil allowClientCidrs canonical = %#v, want loopback", cidrs)
	}
}
