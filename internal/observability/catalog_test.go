package observability

import (
	"strings"
	"testing"
)

func TestFrozenMetricNames(t *testing.T) {
	want := []string{
		"labnetconf_apply_total",
		"labnetconf_build_info",
		"labnetconf_http_requests_total",
		"labnetconf_locks",
		"labnetconf_notifications",
		"labnetconf_restconf_requests_total",
		"labnetconf_rpcs_total",
		"labnetconf_sessions",
	}
	got := FrozenNames()
	if len(got) != len(want) {
		t.Fatalf("names %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("name[%d]=%s want %s", i, got[i], want[i])
		}
	}
}

func TestCatalogNoClientIPLabels(t *testing.T) {
	for _, m := range Metrics() {
		for _, l := range m.Labels {
			if ForbiddenLabel(l) || strings.Contains(strings.ToLower(l), "ip") {
				t.Errorf("metric %s has forbidden label %s", m.Name, l)
			}
		}
	}
	for _, f := range ForbiddenLabels {
		if f == "client_ip" {
			return
		}
	}
	t.Fatal("ForbiddenLabels must include client_ip")
}

func TestRegistryOpenMetrics(t *testing.T) {
	r := NewRegistry()
	ObserveRPC(r, "get-config", "ok")
	ObserveRPC(r, "edit-config", "error")
	ObserveRESTCONF(r, "GET", 200)
	ObserveHTTP(r, 200, "/v1/health/ready")
	ObserveApply(r, "ok")
	SetSessions(r, 2)
	SetLocks(r, 1)
	SetNotifications(r, 3)
	var b strings.Builder
	if err := r.WriteOpenMetrics(&b); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, name := range FrozenNames() {
		if !strings.Contains(out, name) {
			t.Errorf("missing %s\n%s", name, out)
		}
	}
	if !strings.Contains(out, `rpc="get-config"`) {
		t.Fatal(out)
	}
	if !strings.Contains(out, `decision="ok"`) {
		t.Fatal(out)
	}
	if !strings.HasSuffix(out, "# EOF\n") {
		t.Fatal(out)
	}
	if strings.Contains(out, "client_ip") {
		t.Fatal("client IP leaked into scrape")
	}
}

func TestEmptyRegistryEmitsCatalogNames(t *testing.T) {
	r := NewRegistry()
	var b strings.Builder
	if err := r.WriteOpenMetrics(&b); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, name := range FrozenNames() {
		if !strings.Contains(out, "# TYPE "+name+" ") {
			t.Errorf("missing TYPE %s", name)
		}
	}
	if !strings.Contains(out, "labnetconf_build_info") {
		t.Fatal(out)
	}
}

func TestEvaluateReady(t *testing.T) {
	p := Evaluate(Facts{
		SnapshotUp: true, NetconfEnabled: true, NetconfBound: true,
		RestconfEnabled: true, RestconfBound: true, MgmtOff: true,
	})
	if !p.Live || !p.Ready {
		t.Fatalf("%+v", p)
	}
	p = Evaluate(Facts{
		SnapshotUp: true, NetconfEnabled: true, NetconfBound: true,
		RestconfEnabled: true, RestconfBound: true,
	})
	if p.Ready {
		t.Fatal("mgmt unbound should not be ready")
	}
	p = Evaluate(Facts{
		SnapshotUp: true, NetconfEnabled: true, NetconfBound: false,
		RestconfEnabled: false, MgmtOff: true,
	})
	if p.Ready {
		t.Fatal("enabled NETCONF unbound should not be ready")
	}
	p = Evaluate(Facts{
		SnapshotUp: true, NetconfEnabled: false,
		RestconfEnabled: true, RestconfBound: true, MgmtOff: true,
	})
	if !p.Ready {
		t.Fatalf("disabled NETCONF should not block ready: %+v", p)
	}
	p = Evaluate(Facts{
		NetconfEnabled: true, NetconfBound: true,
		RestconfEnabled: true, RestconfBound: true, MgmtOff: true,
	})
	if p.Ready {
		t.Fatal("missing snapshot should not be ready")
	}
}

func TestRPCNameBounded(t *testing.T) {
	if RPCName("get-config") != "get-config" {
		t.Fatal("get-config")
	}
	if RPCName("invented") != "other" {
		t.Fatal("other")
	}
}
