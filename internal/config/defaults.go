package config

const (
	DefaultNetconfAddress  = ":830"
	DefaultRestconfAddress = ":8303"
	DefaultMgmtAddress     = ":8088"
	DefaultRESTPath        = "/v1"
	DefaultMCPPath         = "/mcp"
	MaxDocumentBytes       = 1 << 20
	MinTokenBytes          = 32

	violationUnknownField        = "unknown_field"
	violationRequired            = "required"
	violationInvalidValue        = "invalid_value"
	violationReservedKey         = "reserved_key"
	violationDuplicateKey        = "duplicate_key"
	violationTooLarge            = "document_too_large"
	violationUnsupportedVersion  = "unsupported_version"
	violationDuplicateID         = "duplicate_id"
	violationEmptyID             = "empty_id"
	violationTLSUnsupported      = "tls_unsupported"
	violationCallHomeUnsupported = "callhome_unsupported"
)

// DefaultLoopbackCIDRs is applied when admission.allowClientCidrs is omitted.
var DefaultLoopbackCIDRs = []string{"127.0.0.0/8", "::1/128"}
