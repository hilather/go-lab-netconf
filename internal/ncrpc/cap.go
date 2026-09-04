package ncrpc

// XML namespaces.
const (
	BaseNamespace         = "urn:ietf:params:xml:ns:netconf:base:1.0"
	NotificationNamespace = "urn:ietf:params:xml:ns:netconf:notification:1.0"
)

// Advertised capability URIs (RFC 6241 / RFC 5277).
const (
	CapBase10       = "urn:ietf:params:netconf:base:1.0"
	CapBase11       = "urn:ietf:params:netconf:base:1.1"
	CapCandidate    = "urn:ietf:params:netconf:capability:candidate:1.0"
	CapStartup      = "urn:ietf:params:netconf:capability:startup:1.0"
	CapValidate     = "urn:ietf:params:netconf:capability:validate:1.0"
	CapNotification = "urn:ietf:params:netconf:capability:notification:1.0"
)

var advertised = []string{
	CapBase10,
	CapBase11,
	CapCandidate,
	CapStartup,
	CapValidate,
	CapNotification,
}

// AdvertisedCapabilities returns base 1.0/1.1, candidate, startup, validate,
// and notification. writable-running and xpath are not included.
func AdvertisedCapabilities() []string {
	out := make([]string, len(advertised))
	copy(out, advertised)
	return out
}
