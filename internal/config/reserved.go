package config

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
)

var reservedExact = map[string]string{
	"callhome": "implies RFC 8071 call-home",
	"manager":  "implies an outbound manager",
	"remote":   "implies outbound Dial",
}

var reservedPrefixes = []struct {
	prefix string
	why    string
}{
	{"callhome", "implies RFC 8071 call-home"},
	{"manager", "implies an outbound manager"},
	{"remote", "implies outbound Dial"},
}

func normalizeKey(k string) string {
	k = strings.TrimLeft(k, "-")
	var b strings.Builder
	b.Grow(len(k))
	for _, r := range k {
		if r == '-' || r == '_' {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func reservedReason(normalized string) string {
	// spec.management / listeners.management must not match manager*.
	if normalized == "management" || strings.HasPrefix(normalized, "management") {
		return ""
	}
	if why, ok := reservedExact[normalized]; ok {
		return why
	}
	for _, p := range reservedPrefixes {
		if strings.HasPrefix(normalized, p.prefix) {
			return p.why
		}
	}
	return ""
}

func allowReservedPath(path string) bool {
	switch path {
	case "spec.listeners.callHome", "spec.listeners.netconfTls", "spec.listeners.restconf.tls":
		return true
	default:
		return false
	}
}

func isFreeFormPath(path string) bool {
	if path == "metadata.labels" || strings.HasSuffix(path, ".labels") {
		return true
	}
	if strings.Contains(path, ".instance") || strings.Contains(path, ".startup") {
		return true
	}
	return false
}

func reservedFields(v any, path string) []domainerr.FieldViolation {
	switch x := v.(type) {
	case map[string]any:
		if isFreeFormPath(path) {
			return nil
		}
		var vs []domainerr.FieldViolation
		for k, child := range x {
			p := joinPath(path, k)
			if allowReservedPath(p) {
				vs = append(vs, reservedFields(child, p)...)
				continue
			}
			if why := reservedReason(normalizeKey(k)); why != "" {
				vs = append(vs, domainerr.FieldViolation{
					Path:    p,
					Code:    violationReservedKey,
					Message: fmt.Sprintf("reserved key %q — not a LabNETCONF surface", k),
				})
				continue
			}
			vs = append(vs, reservedFields(child, p)...)
		}
		return vs
	case []any:
		var vs []domainerr.FieldViolation
		for i, child := range x {
			vs = append(vs, reservedFields(child, indexPath(path, i))...)
		}
		return vs
	default:
		return nil
	}
}
