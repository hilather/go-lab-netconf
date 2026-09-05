package observability

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/hilather/go-lab-netconf/internal/buildinfo"
)

// MaxSeriesPerMetric bounds cardinality.
const MaxSeriesPerMetric = 256

// OpenMetricsContentType is the scrape Content-Type.
const OpenMetricsContentType = "application/openmetrics-text; version=1.0.0; charset=utf-8"

// Sample is one scraped series.
type Sample struct {
	Name   string
	Kind   Kind
	Labels map[string]string
	Value  float64
}

type seriesKey struct {
	name   string
	labels string
}

// Registry is an in-process metric store.
type Registry struct {
	mu       sync.Mutex
	hookMu   sync.Mutex
	defs     map[string]MetricDef
	counters map[seriesKey]float64
	gauges   map[seriesKey]float64
	seriesN  map[string]int
	hooks    []func()
}

// NewRegistry returns an empty store keyed by the frozen catalog.
func NewRegistry() *Registry {
	defs := Metrics()
	m := make(map[string]MetricDef, len(defs))
	for _, d := range defs {
		m[d.Name] = d
	}
	r := &Registry{
		defs:     m,
		counters: map[seriesKey]float64{},
		gauges:   map[seriesKey]float64{},
		seriesN:  map[string]int{},
	}
	r.Set(MetricSessions, nil, 0)
	r.Set(MetricLocks, nil, 0)
	r.Set(MetricNotifications, nil, 0)
	r.setBuildInfo()
	return r
}

func (r *Registry) setBuildInfo() {
	info := buildinfo.Current()
	r.Set(MetricBuildInfo, map[string]string{
		"version": info.Version,
		"commit":  info.Commit,
	}, 1)
}

// OnScrape registers a hook invoked before each scrape. Hooks must only
// call Inc/Set; they must not call Snapshot or Write*.
func (r *Registry) OnScrape(fn func()) {
	if r == nil || fn == nil {
		return
	}
	r.hookMu.Lock()
	r.hooks = append(r.hooks, fn)
	r.hookMu.Unlock()
}

func (r *Registry) runHooks() {
	if r == nil {
		return
	}
	r.hookMu.Lock()
	hooks := append([]func(){}, r.hooks...)
	r.hookMu.Unlock()
	for _, fn := range hooks {
		fn()
	}
}

// Inc adds n to a counter. Invalid labels are dropped, never recorded.
func (r *Registry) Inc(name string, labels map[string]string, n float64) {
	if r == nil || n == 0 {
		return
	}
	r.observe(name, KindCounter, labels, n, false)
}

// Set writes a gauge.
func (r *Registry) Set(name string, labels map[string]string, v float64) {
	if r == nil {
		return
	}
	r.observe(name, KindGauge, labels, v, true)
}

func (r *Registry) observe(name string, want Kind, labels map[string]string, v float64, set bool) {
	def, ok := r.defs[name]
	if !ok || def.Kind != want {
		return
	}
	if err := checkLabelsDef(def, labels); err != nil {
		return
	}
	clean := filterLabels(def, labels)
	key := seriesKey{name: name, labels: encodeLabels(def.Labels, clean)}

	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.ensureSeries(name, key, def.Kind) {
		return
	}
	switch def.Kind {
	case KindCounter:
		r.counters[key] += v
	case KindGauge:
		if set {
			r.gauges[key] = v
		} else {
			r.gauges[key] += v
		}
	}
}

func (r *Registry) ensureSeries(name string, key seriesKey, kind Kind) bool {
	switch kind {
	case KindCounter:
		if _, ok := r.counters[key]; ok {
			return true
		}
	case KindGauge:
		if _, ok := r.gauges[key]; ok {
			return true
		}
	}
	if r.seriesN[name] >= MaxSeriesPerMetric {
		return false
	}
	r.seriesN[name]++
	return true
}

// Snapshot copies current series.
func (r *Registry) Snapshot() []Sample {
	if r == nil {
		return nil
	}
	r.runHooks()
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Sample
	for key, v := range r.counters {
		out = append(out, Sample{Name: key.name, Kind: KindCounter, Labels: scrapeLabels(key.name, key.labels), Value: v})
	}
	for key, v := range r.gauges {
		out = append(out, Sample{Name: key.name, Kind: KindGauge, Labels: scrapeLabels(key.name, key.labels), Value: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return encodeLabels(sortedKeys(out[i].Labels), out[i].Labels) < encodeLabels(sortedKeys(out[j].Labels), out[j].Labels)
	})
	return out
}

// Get returns the counter/gauge value for an exact label set.
func (r *Registry) Get(name string, labels map[string]string) (float64, bool) {
	if r == nil {
		return 0, false
	}
	def, ok := r.defs[name]
	if !ok {
		return 0, false
	}
	key := seriesKey{name: name, labels: encodeLabels(def.Labels, filterLabels(def, labels))}
	r.mu.Lock()
	defer r.mu.Unlock()
	switch def.Kind {
	case KindCounter:
		v, ok := r.counters[key]
		return v, ok
	case KindGauge:
		v, ok := r.gauges[key]
		return v, ok
	default:
		return 0, false
	}
}

// WritePrometheus writes the in-memory scrape in Prometheus text format.
func (r *Registry) WritePrometheus(w io.Writer) error {
	if r == nil || w == nil {
		return nil
	}
	samples := r.Snapshot()
	byName := make(map[string][]Sample, len(samples))
	for _, s := range samples {
		byName[s.Name] = append(byName[s.Name], s)
	}
	for _, def := range Metrics() {
		if _, err := fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", def.Name, def.Help, def.Name, def.Kind); err != nil {
			return err
		}
		for _, s := range byName[def.Name] {
			if _, err := fmt.Fprintf(w, "%s%s %s\n", s.Name, promLabels(s.Labels), trimFloat(s.Value)); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteOpenMetrics writes Prometheus text plus the OpenMetrics EOF marker.
func (r *Registry) WriteOpenMetrics(w io.Writer) error {
	if err := r.WritePrometheus(w); err != nil {
		return err
	}
	if w == nil {
		return nil
	}
	_, err := io.WriteString(w, "# EOF\n")
	return err
}

func filterLabels(def MetricDef, in map[string]string) map[string]string {
	if len(def.Labels) == 0 {
		return nil
	}
	out := make(map[string]string, len(def.Labels))
	for _, k := range def.Labels {
		if v, ok := in[k]; ok {
			out[k] = v
		} else {
			out[k] = ""
		}
	}
	return out
}

func encodeLabels(order []string, labels map[string]string) string {
	if len(order) == 0 {
		return ""
	}
	var b strings.Builder
	for i, k := range order {
		if i > 0 {
			b.WriteByte(1)
		}
		b.WriteString(k)
		b.WriteByte(0)
		if labels != nil {
			b.WriteString(labels[k])
		}
	}
	return b.String()
}

func decodeLabels(enc string) map[string]string {
	if enc == "" {
		return nil
	}
	out := map[string]string{}
	for _, part := range strings.Split(enc, "\x01") {
		k, v, ok := strings.Cut(part, "\x00")
		if !ok {
			continue
		}
		out[k] = v
	}
	return out
}

func scrapeLabels(name, enc string) map[string]string {
	raw := decodeLabels(enc)
	if len(raw) == 0 {
		return raw
	}
	def, ok := LookupMetric(name)
	if !ok {
		return nil
	}
	if checkLabelsDef(def, raw) == nil {
		return raw
	}
	out := make(map[string]string, len(def.Labels))
	for _, k := range def.Labels {
		if ForbiddenLabel(k) {
			continue
		}
		if v, ok := raw[k]; ok {
			out[k] = v
		}
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func promLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := sortedKeys(labels)
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteString(`="`)
		b.WriteString(escapeLabel(labels[k]))
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}

func escapeLabel(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

func trimFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
