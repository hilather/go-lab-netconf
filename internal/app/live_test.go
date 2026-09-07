package app

import (
	"context"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/datastore"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
	"github.com/hilather/go-lab-netconf/internal/observability"
	"github.com/hilather/go-lab-netconf/internal/yangtree"
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

func TestApplyAndResetMetrics(t *testing.T) {
	reg := observability.NewRegistry()
	path := copyFixture(t, "defaults.yaml")
	svc, err := Boot(context.Background(), Options{BootstrapPath: path, Metrics: reg})
	if err != nil {
		t.Fatal(err)
	}
	snap := svc.Active()
	_, err = svc.Apply(context.Background(), []ApplyOp{{
		Op:        OpReplaceAdmission,
		Admission: &model.AdmissionSpec{AllowClientCidrs: []string{"10.99.42.0/24"}},
	}}, string(snap.Revision), "k-metrics")
	if err != nil {
		t.Fatal(err)
	}
	ok, found := reg.Get(observability.MetricApplyTotal, map[string]string{"result": "ok"})
	if !found || ok < 1 {
		t.Fatalf("apply ok %v %v", ok, found)
	}
	_, err = svc.Apply(context.Background(), []ApplyOp{{
		Op:        OpReplaceAdmission,
		Admission: &model.AdmissionSpec{AllowClientCidrs: []string{"10.0.0.0/8"}},
	}}, "sha256:deadbeef", "k-conflict")
	if err == nil {
		t.Fatal("expected conflict")
	}
	conflict, found := reg.Get(observability.MetricApplyTotal, map[string]string{"result": "conflict"})
	if !found || conflict < 1 {
		t.Fatalf("apply conflict %v %v", conflict, found)
	}
	if err := svc.Reset(context.Background()); err != nil {
		t.Fatal(err)
	}
	ok, found = reg.Get(observability.MetricApplyTotal, map[string]string{"result": "ok"})
	if !found || ok < 2 {
		t.Fatalf("reset apply ok %v %v", ok, found)
	}
	locks, _ := reg.Get(observability.MetricLocks, nil)
	if locks != 0 {
		t.Fatalf("locks %v", locks)
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

func TestApplyUpsertUserKeepsCommittedHostname(t *testing.T) {
	const committedHost = "after-commit"
	svc, _ := mustBoot(t)
	ctx := context.Background()
	setHostname(t, svc, "router-a", "candidate", committedHost)
	if err := svc.Commit(ctx, "router-a"); err != nil {
		t.Fatal(err)
	}
	if got := hostname(t, svc, "router-a", datastore.Running); got != committedHost {
		t.Fatalf("running after commit = %q, want %q", got, committedHost)
	}
	before, ok := svc.Datastore("router-a")
	if !ok {
		t.Fatal("missing profile handle")
	}
	_, err := svc.Apply(ctx, []ApplyOp{{
		Op: OpUpsertUser,
		User: &model.UserSpec{
			Name:         "bob",
			PasswordFile: "/run/secrets/netconf-bob",
			Profile:      "router-a",
			Access:       model.UserAccessReadWrite,
		},
	}}, string(svc.Active().Revision), "k-bob")
	if err != nil {
		t.Fatal(err)
	}
	after, ok := svc.Datastore("router-a")
	if !ok {
		t.Fatal("missing profile handle after apply")
	}
	if before != after {
		t.Fatal("upsertUser must keep router-a handle")
	}
	if got := hostname(t, svc, "router-a", datastore.Running); got != committedHost {
		t.Fatalf("running hostname after upsertUser = %q, want %q", got, committedHost)
	}
	bob, ok := svc.UserDatastore("bob")
	if !ok {
		t.Fatal("bob handle")
	}
	n, err := bob.Get(ctx, datastore.Running, datastore.Subtree{Path: hostnamePath})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := n.Lookup(hostnamePath)
	// With sharedProfileDatastore default true, new users share the
	// profile handle and see committed state (not bootstrap).
	if !ok || got != committedHost {
		t.Fatalf("bob running = %v, want %q (shared=true couples to committed state)", got, committedHost)
	}
}

func TestUpsertProfileSchemaRebuildsHandle(t *testing.T) {
	svc, _ := mustBoot(t)
	ctx := context.Background()
	copied, err := cloneState(svc.Active().Canonical)
	if err != nil {
		t.Fatal(err)
	}
	var p model.ProfileSpec
	for _, sp := range copied.Spec.Profiles {
		if sp.Name == "router-a" {
			p = sp
			break
		}
	}
	if p.Name == "" {
		t.Fatal("router-a missing")
	}
	found := false
	for i := range p.Schema {
		if p.Schema[i].Path == hostnamePath {
			p.Schema[i].Access = model.SchemaAccessRead
			found = true
		}
	}
	if !found {
		t.Fatal("hostname schema missing")
	}
	_, err = svc.Apply(ctx, []ApplyOp{{Op: OpUpsertProfile, Profile: &p}}, string(svc.Active().Revision), "k-ro")
	if err != nil {
		t.Fatal(err)
	}
	h, ok := svc.Datastore("router-a")
	if !ok {
		t.Fatal("missing profile handle")
	}
	err = h.Edit(ctx, datastore.Candidate, datastore.EditOp{
		Op:    yangtree.OpMerge,
		Path:  hostnamePath,
		Value: "nope",
	})
	requireCode(t, err, domainerr.CodeNotWritable)
	if hostname(t, svc, "router-a", datastore.Candidate) != "lab-rtr-a" {
		t.Fatalf("read-only merge mutated candidate = %q", hostname(t, svc, "router-a", datastore.Candidate))
	}
}
