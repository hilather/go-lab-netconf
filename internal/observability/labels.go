package observability

import (
	"strconv"
	"strings"
)

var (
	forbiddenSet = indexStrings(ForbiddenLabels)
	allowedSet   = indexStrings(AllowedLabels)
)

func indexStrings(in []string) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for _, s := range in {
		out[strings.ToLower(s)] = struct{}{}
	}
	return out
}

// ForbiddenLabel reports whether key is a prohibited default label.
func ForbiddenLabel(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return false
	}
	if _, ok := forbiddenSet[k]; ok {
		return true
	}
	if strings.Contains(k, "client_ip") || strings.Contains(k, "remote_addr") ||
		strings.Contains(k, "password") || strings.Contains(k, "cookie") {
		return true
	}
	return false
}

// AllowedLabel reports whether key is in the global allowlist.
func AllowedLabel(key string) bool {
	_, ok := allowedSet[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

func checkLabelsDef(def MetricDef, labels map[string]string) error {
	allowed := make(map[string]struct{}, len(def.Labels))
	for _, l := range def.Labels {
		allowed[l] = struct{}{}
	}
	for k := range labels {
		if ForbiddenLabel(k) {
			return labelError("forbidden_label")
		}
		if _, ok := allowed[k]; !ok {
			return labelError("unknown_label")
		}
	}
	return nil
}

type labelError string

func (e labelError) Error() string { return string(e) }

// LabelReason is the bounded drop reason for a rejected sample.
func LabelReason(err error) string {
	if err == nil {
		return ""
	}
	if r, ok := err.(labelError); ok {
		return string(r)
	}
	return "invalid"
}

// RPCName collapses an RPC local-name to a catalog label.
func RPCName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "get", "get-config", "edit-config", "copy-config", "delete-config",
		"lock", "unlock", "commit", "discard-changes", "validate",
		"close-session", "kill-session", "create-subscription":
		return strings.ToLower(strings.TrimSpace(name))
	default:
		return "other"
	}
}

// RPCDecision collapses an RPC outcome to a catalog decision label.
func RPCDecision(ok bool) string {
	if ok {
		return "ok"
	}
	return "error"
}

// RESTCONFMethod collapses an HTTP method to a catalog label.
func RESTCONFMethod(method string) string {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return strings.ToUpper(strings.TrimSpace(method))
	default:
		return "other"
	}
}

// HTTPCode collapses an HTTP status to a catalog code label.
func HTTPCode(status int) string {
	if status < 100 || status > 599 {
		return "unknown"
	}
	return strconv.Itoa(status)
}

// HTTPRoute collapses a management path template to a catalog route label.
func HTTPRoute(route string) string {
	r := strings.TrimSpace(route)
	if r == "" {
		return "unknown"
	}
	return r
}

// ApplyResult collapses a mutation outcome to a bounded label.
func ApplyResult(result string) string {
	switch strings.ToLower(strings.TrimSpace(result)) {
	case "ok", "error", "conflict":
		return strings.ToLower(strings.TrimSpace(result))
	default:
		if result == "" {
			return "ok"
		}
		return "error"
	}
}
