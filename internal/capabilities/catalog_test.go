package capabilities

import (
	"strings"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
)

func TestCatalogRowCountAndPrefixes(t *testing.T) {
	if err := ValidateCatalog(); err != nil {
		t.Fatal(err)
	}
	if len(All()) != TableRowCount {
		t.Fatalf("rows %d", len(All()))
	}
	for _, name := range Tools() {
		if !strings.HasPrefix(name, "netconf_") {
			t.Errorf("tool %s must start with netconf_", name)
		}
		if strings.HasPrefix(name, "labnetconf_") {
			t.Errorf("tool %s uses rejected labnetconf_ prefix", name)
		}
	}
	for _, r := range Resources() {
		if !strings.HasPrefix(r, "labnetconf://") {
			t.Errorf("resource %s must use labnetconf://", r)
		}
	}
}

func TestParityRequiredRESTBindings(t *testing.T) {
	want := []string{
		"GET /v1/version",
		"GET /v1/capabilities",
		"GET /v1/status",
		"GET /v1/schema/config",
		"GET /v1/features",
		"GET /v1/state",
		"POST /v1/state:validate",
		"GET /v1/state:export",
		"POST /v1/state:reset",
		"POST /v1/changes:plan",
		"POST /v1/changes:apply",
		"GET /v1/profiles",
		"GET /v1/profiles/{name}",
		"GET /v1/users",
		"GET /v1/datastores/{profile}/{store}",
		"POST /v1/datastores/{profile}/{store}:set",
		"POST /v1/datastores/{profile}:commit",
		"POST /v1/datastores/{profile}:discard",
		"GET /v1/sessions",
		"POST /v1/sessions/{id}:kill",
		"GET /v1/notifications",
		"GET /v1/notifications/{id}",
		"POST /v1/notifications:wait",
		"POST /v1/notifications:clear",
		"GET /v1/preview/get",
		"GET /v1/audit",
	}
	got := map[string]bool{}
	for _, c := range All() {
		if c.RESTOnly {
			continue
		}
		for _, b := range c.REST {
			got[b.RESTRef()] = true
		}
	}
	for _, row := range want {
		if !got[row] {
			t.Errorf("missing PARITY_REQUIRED REST row %s", row)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("parity REST rows %d, want %d", len(got), len(want))
	}
}

func TestHealthRESTOnly(t *testing.T) {
	for _, id := range []ID{HealthLive, HealthReady} {
		c, ok := Lookup(id)
		if !ok || !c.RESTOnly {
			t.Fatalf("%s must be REST-only", id)
		}
	}
	if _, ok := LookupREST("GET", "/v1/health/live"); !ok {
		t.Fatal("health live")
	}
	if _, ok := LookupREST("GET", "/v1/health/ready"); !ok {
		t.Fatal("health ready")
	}
}

func TestFrozenResources(t *testing.T) {
	want := []string{
		"labnetconf://state",
		"labnetconf://profiles/{name}",
		"labnetconf://datastores/{profile}/{store}",
		"labnetconf://notifications/{id}",
		"labnetconf://schema/config",
	}
	got := map[string]bool{}
	for _, r := range Resources() {
		got[r] = true
	}
	if len(got) != len(want) {
		t.Fatalf("resources %v", Resources())
	}
	for _, r := range want {
		if !got[r] {
			t.Errorf("missing resource %s", r)
		}
	}
}

func TestFeaturesFrozen(t *testing.T) {
	ids := FeatureIDs()
	want := []string{
		"replaceProfiles", "upsertProfile", "removeProfile",
		"replaceUsers", "upsertUser", "removeUser",
		"replaceAdmission", "replaceNetconfCaps", "replaceObservability",
		"listeners.netconf.address", "listeners.restconf.address",
		"listeners.management.address", "listeners.netconf.hostKeyFile",
		"auth", "tls", "callHome", "netconfTls", "ui.enabled",
	}
	if len(ids) != len(want) {
		t.Fatalf("%v", ids)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("id[%d]=%s want %s", i, ids[i], want[i])
		}
	}
	for _, f := range Features() {
		if f.Apply != FeatureApplyLive && f.Apply != FeatureApplyResetOnly {
			t.Errorf("%s apply=%s", f.ID, f.Apply)
		}
	}
}

func TestProblemJSONFrozenCodes(t *testing.T) {
	want := []domainerr.Code{
		domainerr.CodeValidationFailed, domainerr.CodeUnknownField, domainerr.CodeReservedKey,
		domainerr.CodeImmutableField, domainerr.CodeRevisionMismatch, domainerr.CodeNotFound,
		domainerr.CodeLockDenied, domainerr.CodeNotWritable, domainerr.CodeWaitTimeout,
		domainerr.CodeStoreWiped, domainerr.CodeUnauthorized, domainerr.CodeForbidden,
		domainerr.CodeOriginNotAllowed, domainerr.CodeTLSUnsupported, domainerr.CodeCallHomeUnsupported,
		domainerr.CodeCandidateDirty,
	}
	got := map[domainerr.Code]bool{}
	for _, c := range domainerr.Codes() {
		got[c] = true
	}
	for _, c := range want {
		if !got[c] {
			t.Errorf("catalog missing %s", c)
		}
		p := ProblemFrom(domainerr.New(c, string(c)), "urn:labnetconf:request:1")
		if p.Code != c {
			t.Errorf("%s problem code %s", c, p.Code)
		}
		if p.Status == 0 || p.Status == 500 && c != "" && errorMap[c].Status != 500 {
			t.Errorf("%s status %d", c, p.Status)
		}
		if !strings.HasPrefix(p.Type, ProblemTypePrefix) {
			t.Errorf("%s type %s", c, p.Type)
		}
	}
	p := ProblemFrom(domainerr.CandidateDirty("candidate is dirty"), "urn:labnetconf:request:1")
	if p.Status != 409 || p.Code != domainerr.CodeCandidateDirty {
		t.Fatalf("candidate_dirty %+v", p)
	}
}

func TestNoRestconfBinding(t *testing.T) {
	for _, c := range All() {
		for _, b := range c.REST {
			if strings.HasPrefix(b.Path, "/restconf") {
				t.Errorf("%s mounts %s", c.ID, b.Path)
			}
		}
	}
}
