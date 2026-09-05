package capabilities

// FeatureApplyLive and FeatureApplyResetOnly are the only apply values.
const (
	FeatureApplyLive      = "live"
	FeatureApplyResetOnly = "reset-only"
)

// Feature is one frozen live vs reset-only row.
type Feature struct {
	ID    string `json:"id"`
	Apply string `json:"apply"`
	Path  string `json:"path"`
}

// Features is the frozen operator catalog.
func Features() []Feature {
	return []Feature{
		{ID: "replaceProfiles", Apply: FeatureApplyLive, Path: "spec.profiles"},
		{ID: "upsertProfile", Apply: FeatureApplyLive, Path: "spec.profiles"},
		{ID: "removeProfile", Apply: FeatureApplyLive, Path: "spec.profiles"},
		{ID: "replaceUsers", Apply: FeatureApplyLive, Path: "spec.users"},
		{ID: "upsertUser", Apply: FeatureApplyLive, Path: "spec.users"},
		{ID: "removeUser", Apply: FeatureApplyLive, Path: "spec.users"},
		{ID: "replaceAdmission", Apply: FeatureApplyLive, Path: "spec.admission"},
		{ID: "replaceNetconfCaps", Apply: FeatureApplyLive, Path: "spec.netconf.versions"},
		{ID: "replaceObservability", Apply: FeatureApplyLive, Path: "spec.observability"},
		{ID: "listeners.netconf.address", Apply: FeatureApplyResetOnly, Path: "spec.listeners.netconf.address"},
		{ID: "listeners.restconf.address", Apply: FeatureApplyResetOnly, Path: "spec.listeners.restconf.address"},
		{ID: "listeners.management.address", Apply: FeatureApplyResetOnly, Path: "spec.listeners.management.address"},
		{ID: "listeners.netconf.hostKeyFile", Apply: FeatureApplyResetOnly, Path: "spec.listeners.netconf.hostKeyFile"},
		{ID: "auth", Apply: FeatureApplyResetOnly, Path: "spec.auth"},
		{ID: "tls", Apply: FeatureApplyResetOnly, Path: "spec.listeners.restconf.tls"},
		{ID: "callHome", Apply: FeatureApplyResetOnly, Path: "spec.listeners.callHome"},
		{ID: "netconfTls", Apply: FeatureApplyResetOnly, Path: "spec.listeners.netconfTls"},
		{ID: "ui.enabled", Apply: FeatureApplyResetOnly, Path: "spec.ui.enabled"},
	}
}

// FeatureIDs is the frozen id list in catalog order.
func FeatureIDs() []string {
	fs := Features()
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.ID
	}
	return out
}
