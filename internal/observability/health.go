package observability

// Warning codes are a bounded, stable Status DTO surface.
const (
	WarnSnapshotMissing = "snapshot_missing"
	WarnNetconfUnbound  = "netconf_unbound"
	WarnRestconfUnbound = "restconf_unbound"
	WarnMgmtUnbound     = "management_unbound"
	WarnListenerUnbound = "listener_unbound"
)

// MaxWarnings caps the Status warning list.
const MaxWarnings = 16

// Warning is one agent-readable operational note.
type Warning struct {
	Code    string
	Message string
}

// Facts are process observations used to evaluate health.
type Facts struct {
	ProcessDown bool
	SnapshotUp  bool
	// NetconfEnabled is spec.listeners.netconf.enabled.
	NetconfEnabled bool
	NetconfBound   bool
	// RestconfEnabled is spec.listeners.restconf.enabled.
	RestconfEnabled bool
	RestconfBound   bool
	// MgmtBound is true when the management listener is accepting.
	MgmtBound bool
	// MgmtOff is true when management was explicitly disabled (off/none/-).
	MgmtOff bool
}

// Probe is liveness and readiness plus bounded warnings.
type Probe struct {
	Live     bool
	Ready    bool
	Warnings []Warning
}

// Evaluate implements Ready = snapshot loaded AND every enabled
// NETCONF/RESTCONF listener is bound AND (management bound or off).
func Evaluate(in Facts) Probe {
	p := Probe{Live: !in.ProcessDown}
	netconfOK := !in.NetconfEnabled || in.NetconfBound
	restconfOK := !in.RestconfEnabled || in.RestconfBound
	mgmtOK := in.MgmtBound || in.MgmtOff
	p.Ready = p.Live && in.SnapshotUp && netconfOK && restconfOK && mgmtOK

	add := func(code, msg string) {
		if len(p.Warnings) >= MaxWarnings {
			return
		}
		p.Warnings = append(p.Warnings, Warning{Code: code, Message: msg})
	}
	listenerUnbound := false
	if in.NetconfEnabled && !in.NetconfBound {
		add(WarnNetconfUnbound, "NETCONF SSH listener is not bound")
		listenerUnbound = true
	}
	if in.RestconfEnabled && !in.RestconfBound {
		add(WarnRestconfUnbound, "RESTCONF HTTP listener is not bound")
		listenerUnbound = true
	}
	if !in.SnapshotUp {
		add(WarnSnapshotMissing, "compiled snapshot is not installed")
	}
	if !mgmtOK {
		add(WarnMgmtUnbound, "management listener is not bound")
		listenerUnbound = true
	}
	if listenerUnbound {
		add(WarnListenerUnbound, "a required listener is not bound")
	}
	return p
}
