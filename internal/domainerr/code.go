package domainerr

// Code is a stable, transport-independent domain error code.
type Code string

const (
	CodeValidationFailed    Code = "validation_failed"
	CodeUnknownField        Code = "unknown_field"
	CodeReservedKey         Code = "reserved_key"
	CodeImmutableField      Code = "immutable_field"
	CodeRevisionMismatch    Code = "revision_mismatch"
	CodeNotFound            Code = "not_found"
	CodeLockDenied          Code = "lock_denied"
	CodeNotWritable         Code = "not_writable"
	CodeWaitTimeout         Code = "wait_timeout"
	CodeStoreWiped          Code = "store_wiped"
	CodeUnauthorized        Code = "unauthorized"
	CodeForbidden           Code = "forbidden"
	CodeOriginNotAllowed    Code = "origin_not_allowed"
	CodeTLSUnsupported      Code = "tls_unsupported"
	CodeCallHomeUnsupported Code = "callhome_unsupported"
	CodeCandidateDirty      Code = "candidate_dirty"
)

var catalog = []struct {
	Code      Code
	Retryable bool
}{
	{CodeValidationFailed, false},
	{CodeUnknownField, false},
	{CodeReservedKey, false},
	{CodeImmutableField, false},
	{CodeRevisionMismatch, true},
	{CodeNotFound, false},
	{CodeLockDenied, true},
	{CodeNotWritable, false},
	{CodeWaitTimeout, true},
	{CodeStoreWiped, false},
	{CodeUnauthorized, false},
	{CodeForbidden, false},
	{CodeOriginNotAllowed, false},
	{CodeTLSUnsupported, false},
	{CodeCallHomeUnsupported, false},
	{CodeCandidateDirty, true},
}

// Codes returns the stable catalog in documented order.
func Codes() []Code {
	out := make([]Code, len(catalog))
	for i, e := range catalog {
		out[i] = e.Code
	}
	return out
}

// Retryable reports the catalog default for code. Unknown codes are not retryable.
func Retryable(code Code) bool {
	for _, e := range catalog {
		if e.Code == code {
			return e.Retryable
		}
	}
	return false
}
