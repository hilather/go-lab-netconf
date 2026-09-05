package capabilities

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
)

// ProblemContentType is the RFC 9457 media type adapters must emit.
const ProblemContentType = "application/problem+json"

// ProblemTypePrefix is the URN namespace for domain problem types.
const ProblemTypePrefix = "urn:labnetconf:error:"

// ErrorCatalogRelPath is the generated problem+json code catalog.
const ErrorCatalogRelPath = "api/errors/v1.json"

// ErrorCatalogAPIVersion identifies the error catalog document shape.
const ErrorCatalogAPIVersion = "labnetconf.dev/errors/v1"

// ErrorCatalogGeneratedBy is embedded so verify-generated can treat the file as generated.
const ErrorCatalogGeneratedBy = "scripts/generate; DO NOT EDIT."

// JSON-RPC 2.0 reserved codes plus the application range.
const (
	JSONRPCParseError     = -32700
	JSONRPCInvalidRequest = -32600
	JSONRPCMethodNotFound = -32601
	JSONRPCInvalidParams  = -32602
	JSONRPCInternalError  = -32603

	JSONRPCApplication     = -32000
	JSONRPCUnauthenticated = -32001
	JSONRPCForbidden       = -32003
	JSONRPCNotFound        = -32004
	JSONRPCConflict        = -32009
	JSONRPCTimeout         = -32010
)

// Problem is an RFC 9457 problem+json document with domainerr extensions.
type Problem struct {
	Type            string                     `json:"type"`
	Title           string                     `json:"title"`
	Status          int                        `json:"status"`
	Detail          string                     `json:"detail,omitempty"`
	Instance        string                     `json:"instance,omitempty"`
	Code            domainerr.Code             `json:"code,omitempty"`
	Retryable       bool                       `json:"retryable"`
	FieldViolations []domainerr.FieldViolation `json:"fieldViolations,omitempty"`
	CurrentRevision string                     `json:"currentRevision,omitempty"`
	Remediation     string                     `json:"remediation,omitempty"`
}

// JSONRPCError is a JSON-RPC 2.0 error object.
type JSONRPCError struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    *domainerr.Error `json:"data,omitempty"`
}

type mapping struct {
	Status int
	RPC    int
	Title  string
}

var errorMap = map[domainerr.Code]mapping{
	domainerr.CodeValidationFailed:    {http.StatusBadRequest, JSONRPCInvalidParams, "Validation failed"},
	domainerr.CodeUnknownField:        {http.StatusBadRequest, JSONRPCInvalidParams, "Unknown field"},
	domainerr.CodeReservedKey:         {http.StatusBadRequest, JSONRPCInvalidParams, "Reserved key"},
	domainerr.CodeImmutableField:      {http.StatusBadRequest, JSONRPCInvalidParams, "Immutable field"},
	domainerr.CodeRevisionMismatch:    {http.StatusConflict, JSONRPCConflict, "Revision mismatch"},
	domainerr.CodeNotFound:            {http.StatusNotFound, JSONRPCNotFound, "Not Found"},
	domainerr.CodeLockDenied:          {http.StatusConflict, JSONRPCConflict, "Lock denied"},
	domainerr.CodeNotWritable:         {http.StatusForbidden, JSONRPCForbidden, "Not writable"},
	domainerr.CodeWaitTimeout:         {http.StatusGatewayTimeout, JSONRPCTimeout, "Wait timeout"},
	domainerr.CodeStoreWiped:          {http.StatusConflict, JSONRPCConflict, "Store wiped"},
	domainerr.CodeUnauthorized:        {http.StatusUnauthorized, JSONRPCUnauthenticated, "Unauthorized"},
	domainerr.CodeForbidden:           {http.StatusForbidden, JSONRPCForbidden, "Forbidden"},
	domainerr.CodeOriginNotAllowed:    {http.StatusForbidden, JSONRPCForbidden, "Origin not allowed"},
	domainerr.CodeTLSUnsupported:      {http.StatusBadRequest, JSONRPCInvalidParams, "TLS unsupported"},
	domainerr.CodeCallHomeUnsupported: {http.StatusBadRequest, JSONRPCInvalidParams, "Call-home unsupported"},
	domainerr.CodeCandidateDirty:      {http.StatusConflict, JSONRPCConflict, "Candidate dirty"},
}

func lookupMapping(code domainerr.Code) mapping {
	if m, ok := errorMap[code]; ok {
		return m
	}
	return mapping{Status: http.StatusInternalServerError, RPC: JSONRPCInternalError, Title: "Internal error"}
}

// ProblemTypeURN is the RFC 9457 type for code (underscores become hyphens).
func ProblemTypeURN(code domainerr.Code) string {
	if code == "" {
		return ProblemTypePrefix + "internal"
	}
	return ProblemTypePrefix + strings.ReplaceAll(string(code), "_", "-")
}

// HTTPStatus is the REST status hint for code. Unknown codes are 500.
func HTTPStatus(code domainerr.Code) int {
	return lookupMapping(code).Status
}

func domainOf(err error) *domainerr.Error {
	if de, ok := domainerr.As(err); ok && de != nil {
		return de
	}
	return nil
}

// ProblemFrom maps err to a problem+json document.
func ProblemFrom(err error, instance string) Problem {
	de := domainOf(err)
	if de == nil {
		return Problem{
			Type:     ProblemTypeURN(""),
			Title:    "Internal error",
			Status:   http.StatusInternalServerError,
			Detail:   "internal error",
			Instance: instance,
		}
	}
	m := lookupMapping(de.Code)
	p := Problem{
		Type:            ProblemTypeURN(de.Code),
		Title:           m.Title,
		Status:          m.Status,
		Detail:          de.Message,
		Instance:        instance,
		Code:            de.Code,
		Retryable:       de.Retryable,
		CurrentRevision: de.CurrentRevision,
		Remediation:     de.Remediation,
	}
	if len(de.FieldViolations) > 0 {
		p.FieldViolations = append([]domainerr.FieldViolation(nil), de.FieldViolations...)
	}
	return p
}

// JSONRPCFrom maps err to a JSON-RPC error.
func JSONRPCFrom(err error) JSONRPCError {
	de := domainOf(err)
	if de == nil {
		return JSONRPCError{Code: JSONRPCInternalError, Message: "internal error"}
	}
	cp := *de
	if de.FieldViolations != nil {
		cp.FieldViolations = append([]domainerr.FieldViolation(nil), de.FieldViolations...)
	}
	msg := de.Message
	if msg == "" {
		msg = lookupMapping(de.Code).Title
	}
	return JSONRPCError{
		Code:    lookupMapping(de.Code).RPC,
		Message: msg,
		Data:    &cp,
	}
}

// ErrorCatalogEntry is one generated problem+json code row.
type ErrorCatalogEntry struct {
	Code      domainerr.Code `json:"code"`
	Status    int            `json:"status"`
	Retryable bool           `json:"retryable"`
	Title     string         `json:"title"`
}

// ErrorCatalog is the generated problem+json code document.
type ErrorCatalog struct {
	APIVersion  string              `json:"apiVersion"`
	GeneratedBy string              `json:"generatedBy"`
	Errors      []ErrorCatalogEntry `json:"errors"`
}

// RenderErrorCatalog returns pretty-printed JSON for api/errors/v1.json.
func RenderErrorCatalog() ([]byte, error) {
	codes := domainerr.Codes()
	entries := make([]ErrorCatalogEntry, 0, len(codes))
	for _, c := range codes {
		m := lookupMapping(c)
		entries = append(entries, ErrorCatalogEntry{
			Code:      c,
			Status:    m.Status,
			Retryable: domainerr.Retryable(c),
			Title:     m.Title,
		})
	}
	doc := ErrorCatalog{
		APIVersion:  ErrorCatalogAPIVersion,
		GeneratedBy: ErrorCatalogGeneratedBy,
		Errors:      entries,
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}
