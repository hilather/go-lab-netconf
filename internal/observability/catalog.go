package observability

import (
	"encoding/json"
	"sort"
)

// CatalogID is the versioned metrics/events document identifier.
const CatalogID = "labnetconf.dev/metrics/v1alpha1"

// CatalogRelPath is the generated catalog artifact.
const CatalogRelPath = "api/metrics/v1alpha1.json"

// Kind is a catalog metric type.
type Kind string

const (
	KindCounter Kind = "counter"
	KindGauge   Kind = "gauge"
)

// Frozen metric names from docs/09.
const (
	MetricRPCsTotal             = "labnetconf_rpcs_total"
	MetricRESTCONFRequestsTotal = "labnetconf_restconf_requests_total"
	MetricSessions              = "labnetconf_sessions"
	MetricLocks                 = "labnetconf_locks"
	MetricNotifications         = "labnetconf_notifications"
	MetricApplyTotal            = "labnetconf_apply_total"
	MetricHTTPRequestsTotal     = "labnetconf_http_requests_total"
	MetricBuildInfo             = "labnetconf_build_info"
)

// FrozenNames is the docs/09 series list in catalog order.
func FrozenNames() []string {
	out := make([]string, 0, len(Metrics()))
	for _, m := range Metrics() {
		out = append(out, m.Name)
	}
	return out
}

// Frozen structured-log event names.
const (
	EventNETCONFRPC      = "netconf.rpc"
	EventRESTCONFRequest = "restconf.request"
	EventStateApply      = "state.apply"
	EventStateReset      = "state.reset"
	EventHTTPRequest     = "http.request"
)

// AllowedLabels is the default bounded label set. Client IPs are never allowed.
var AllowedLabels = []string{
	"code",
	"commit",
	"component",
	"decision",
	"event",
	"method",
	"result",
	"route",
	"rpc",
	"version",
}

// ForbiddenLabels must never appear on a catalog metric or recorded sample.
var ForbiddenLabels = []string{
	"actor",
	"actor_id",
	"address",
	"authorization",
	"body",
	"client",
	"client_ip",
	"cookie",
	"data",
	"detail",
	"err",
	"error",
	"error_text",
	"from",
	"host",
	"idempotency",
	"idempotency_key",
	"message",
	"password",
	"peer",
	"raw",
	"remote_addr",
	"set_cookie",
	"source_ip",
	"src",
	"src_ip",
	"subject",
	"to",
}

// MetricDef is one catalog row.
type MetricDef struct {
	Name   string   `json:"name"`
	Kind   Kind     `json:"kind"`
	Help   string   `json:"help"`
	Labels []string `json:"labels"`
	Unit   string   `json:"unit,omitempty"`
}

// EventDef is one stable structured-log event.
type EventDef struct {
	Name   string   `json:"name"`
	Fields []string `json:"fields"`
}

// Document is the versioned catalog artifact.
type Document struct {
	ID              string      `json:"id"`
	Version         string      `json:"version"`
	AllowedLabels   []string    `json:"allowedLabels"`
	ForbiddenLabels []string    `json:"forbiddenLabels"`
	Metrics         []MetricDef `json:"metrics"`
	Events          []EventDef  `json:"events"`
}

// EventFields is the frozen slog JSON field set.
var EventFields = []string{
	"timestamp", "level", "event", "component", "request_id",
	"capability", "result", "error_code", "duration_ms",
}

// Metrics returns the frozen first-GA catalog in stable name order.
func Metrics() []MetricDef {
	defs := []MetricDef{
		{Name: MetricRPCsTotal, Kind: KindCounter, Help: "NETCONF RPCs by name and decision.", Labels: []string{"rpc", "decision"}},
		{Name: MetricRESTCONFRequestsTotal, Kind: KindCounter, Help: "RESTCONF data-plane HTTP requests.", Labels: []string{"method", "code"}},
		{Name: MetricSessions, Kind: KindGauge, Help: "Active NETCONF and RESTCONF sessions.", Labels: nil},
		{Name: MetricLocks, Kind: KindGauge, Help: "Held datastore locks across profile-instances.", Labels: nil},
		{Name: MetricNotifications, Kind: KindGauge, Help: "In-process config-change notifications stored.", Labels: nil},
		{Name: MetricApplyTotal, Kind: KindCounter, Help: "Plan/apply/reset commits.", Labels: []string{"result"}},
		{Name: MetricHTTPRequestsTotal, Kind: KindCounter, Help: "Management HTTP requests.", Labels: []string{"code", "route"}},
		{Name: MetricBuildInfo, Kind: KindGauge, Help: "Build metadata. Always 1.", Labels: []string{"version", "commit"}},
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	for i := range defs {
		defs[i].Labels = append([]string(nil), defs[i].Labels...)
		sort.Strings(defs[i].Labels)
	}
	return defs
}

// Events returns the frozen structured-log event catalog.
func Events() []EventDef {
	names := []string{
		EventNETCONFRPC, EventRESTCONFRequest, EventStateApply,
		EventStateReset, EventHTTPRequest,
	}
	out := make([]EventDef, len(names))
	for i, n := range names {
		out[i] = EventDef{Name: n, Fields: append([]string(nil), EventFields...)}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// LookupMetric returns the catalog definition for name.
func LookupMetric(name string) (MetricDef, bool) {
	def, ok := metricIndex[name]
	return def, ok
}

var metricIndex = func() map[string]MetricDef {
	defs := Metrics()
	m := make(map[string]MetricDef, len(defs))
	for _, d := range defs {
		m[d.Name] = d
	}
	return m
}()

// Catalog returns the versioned document.
func Catalog() Document {
	return Document{
		ID:              CatalogID,
		Version:         "v1alpha1",
		AllowedLabels:   append([]string(nil), AllowedLabels...),
		ForbiddenLabels: append([]string(nil), ForbiddenLabels...),
		Metrics:         Metrics(),
		Events:          Events(),
	}
}

// RenderCatalog is the generated JSON artifact.
func RenderCatalog() ([]byte, error) {
	b, err := json.MarshalIndent(Catalog(), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
