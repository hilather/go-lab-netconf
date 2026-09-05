package app

func featureCatalog() []Feature {
	return []Feature{
		{ID: "replaceProfiles", Apply: ApplyLive, Path: "spec.profiles"},
		{ID: "upsertProfile", Apply: ApplyLive, Path: "spec.profiles"},
		{ID: "removeProfile", Apply: ApplyLive, Path: "spec.profiles"},
		{ID: "replaceUsers", Apply: ApplyLive, Path: "spec.users"},
		{ID: "upsertUser", Apply: ApplyLive, Path: "spec.users"},
		{ID: "removeUser", Apply: ApplyLive, Path: "spec.users"},
		{ID: "replaceAdmission", Apply: ApplyLive, Path: "spec.admission"},
		{ID: "replaceNetconfCaps", Apply: ApplyLive, Path: "spec.netconf.versions"},
		{ID: "replaceObservability", Apply: ApplyLive, Path: "spec.observability"},
		{ID: "listeners.netconf.address", Apply: ApplyResetOnly, Path: "spec.listeners.netconf.address"},
		{ID: "listeners.restconf.address", Apply: ApplyResetOnly, Path: "spec.listeners.restconf.address"},
		{ID: "listeners.management.address", Apply: ApplyResetOnly, Path: "spec.listeners.management.address"},
		{ID: "listeners.netconf.hostKeyFile", Apply: ApplyResetOnly, Path: "spec.listeners.netconf.hostKeyFile"},
		{ID: "auth", Apply: ApplyResetOnly, Path: "spec.auth"},
		{ID: "tls", Apply: ApplyResetOnly, Path: "spec.listeners.restconf.tls"},
		{ID: "callHome", Apply: ApplyResetOnly, Path: "spec.listeners.callHome"},
		{ID: "netconfTls", Apply: ApplyResetOnly, Path: "spec.listeners.netconfTls"},
		{ID: "ui.enabled", Apply: ApplyResetOnly, Path: "spec.ui.enabled"},
	}
}
