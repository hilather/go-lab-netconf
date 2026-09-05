package app

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
)

func TestLiveVsResetOnly(t *testing.T) {
	svc, snap := mustBoot(t)
	ctx := context.Background()

	feats, err := svc.Features(ctx)
	if err != nil {
		t.Fatal(err)
	}
	live := map[string]bool{}
	resetOnly := map[string]bool{}
	for _, f := range feats.Items {
		switch f.Apply {
		case ApplyLive:
			live[f.ID] = true
		case ApplyResetOnly:
			resetOnly[f.ID] = true
		}
	}
	for _, id := range []string{
		"replaceProfiles", "upsertProfile", "removeProfile",
		"replaceUsers", "upsertUser", "removeUser",
		"replaceAdmission", "replaceNetconfCaps", "replaceObservability",
	} {
		if !live[id] {
			t.Errorf("expected live %s", id)
		}
	}
	for _, id := range []string{
		"listeners.netconf.address", "listeners.restconf.address",
		"listeners.management.address", "listeners.netconf.hostKeyFile",
		"auth", "tls", "callHome", "netconfTls", "ui.enabled",
	} {
		if !resetOnly[id] {
			t.Errorf("expected reset-only %s", id)
		}
	}

	res, err := svc.Apply(ctx, []ApplyOp{{
		Op:        OpReplaceAdmission,
		Admission: &model.AdmissionSpec{AllowClientCidrs: []string{"10.99.42.0/24"}},
	}}, string(snap.Revision), "k-admission")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Applied {
		t.Fatal("apply")
	}
	if svc.Active().NetconfAddress != snap.NetconfAddress {
		t.Fatalf("listener address changed via live apply: %q", svc.Active().NetconfAddress)
	}

	_, err = svc.Apply(ctx, []ApplyOp{{Op: "replaceListeners"}}, string(res.RuntimeRevision), "k-listeners")
	requireCode(t, err, domainerr.CodeValidationFailed)
	de, _ := domainerr.As(err)
	if de.Remediation == "" {
		t.Fatal("remediation")
	}
}

func TestRevisionMismatch(t *testing.T) {
	svc, snap := mustBoot(t)
	_, err := svc.Apply(context.Background(), []ApplyOp{{
		Op:        OpReplaceAdmission,
		Admission: &model.AdmissionSpec{AllowClientCidrs: []string{"10.0.0.0/8"}},
	}}, "sha256:deadbeef", "k-mismatch")
	requireCode(t, err, domainerr.CodeRevisionMismatch)
	de, _ := domainerr.As(err)
	if de.CurrentRevision != string(snap.Revision) {
		t.Fatalf("currentRevision = %q, want %q", de.CurrentRevision, snap.Revision)
	}
	if hostname(t, svc, "router-a", "running") != "lab-rtr-a" {
		t.Fatal("mismatch must not mutate running")
	}
}

func TestApplyRequiresExpectedRevisionAndIdempotencyKey(t *testing.T) {
	svc, snap := mustBoot(t)
	ops := []ApplyOp{{
		Op:        OpReplaceAdmission,
		Admission: &model.AdmissionSpec{AllowClientCidrs: []string{"10.0.0.0/8"}},
	}}
	_, err := svc.Apply(context.Background(), ops, "", "k-rev")
	requireCode(t, err, domainerr.CodeValidationFailed)
	_, err = svc.Apply(context.Background(), ops, string(snap.Revision), "")
	requireCode(t, err, domainerr.CodeValidationFailed)
}

func TestIdempotentApply(t *testing.T) {
	svc, snap := mustBoot(t)
	ops := []ApplyOp{{
		Op:        OpReplaceAdmission,
		Admission: &model.AdmissionSpec{AllowClientCidrs: []string{"10.99.42.0/24", "127.0.0.0/8"}},
	}}
	r1, err := svc.Apply(context.Background(), ops, string(snap.Revision), "k1")
	if err != nil {
		t.Fatal(err)
	}
	r2, err := svc.Apply(context.Background(), ops, string(snap.Revision), "k1")
	if err != nil {
		t.Fatal(err)
	}
	if r1.RuntimeRevision != r2.RuntimeRevision {
		t.Fatal("idempotent replay must return the same revision")
	}
	if r1.Generation != r2.Generation {
		t.Fatal("idempotent replay must return the same generation")
	}
	_, err = svc.Apply(context.Background(), []ApplyOp{{
		Op:        OpReplaceAdmission,
		Admission: &model.AdmissionSpec{AllowClientCidrs: []string{"192.0.2.0/24"}},
	}}, string(snap.Revision), "k1")
	requireCode(t, err, domainerr.CodeValidationFailed)
}

func TestDatastoreSetIsNotApplyVerb(t *testing.T) {
	svc, snap := mustBoot(t)
	_, err := svc.Apply(context.Background(), []ApplyOp{{Op: "datastore:set"}}, string(snap.Revision), "k-set")
	requireCode(t, err, domainerr.CodeValidationFailed)
	_, err = svc.Plan(context.Background(), []ApplyOp{{Op: "commit"}}, string(snap.Revision))
	requireCode(t, err, domainerr.CodeValidationFailed)
}
