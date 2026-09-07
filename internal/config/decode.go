package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
	"gopkg.in/yaml.v3"
)

// Decode auto-detects YAML vs JSON and rejects unknown fields. It does not
// normalize or validate.
func Decode(data []byte) (*model.State, error) {
	if len(data) > MaxDocumentBytes {
		return nil, domainerr.ValidationFailed("document exceeds size limit",
			domainerr.FieldViolation{Path: "", Code: violationTooLarge, Message: fmt.Sprintf("document is %d bytes; max is %d", len(data), MaxDocumentBytes)})
	}
	data = stripBOM(bytes.TrimSpace(data))
	if len(data) == 0 {
		return nil, domainerr.ValidationFailed("empty document",
			domainerr.FieldViolation{Path: "", Code: violationRequired, Message: "document is empty"})
	}
	if !utf8.Valid(data) {
		return nil, domainerr.ValidationFailed("document is not UTF-8",
			domainerr.FieldViolation{Path: "", Code: violationInvalidValue, Message: "document must be UTF-8"})
	}
	if looksLikeJSON(data) {
		return decodeJSON(data)
	}
	return decodeYAML(data)
}

func decodeYAML(data []byte) (*model.State, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var node yaml.Node
	if err := dec.Decode(&node); err != nil {
		if err == io.EOF {
			return nil, domainerr.ValidationFailed("empty document",
				domainerr.FieldViolation{Path: "", Code: violationRequired, Message: "document is empty"})
		}
		return nil, mapYAMLKnownFieldsError(err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, domainerr.ValidationFailed("trailing YAML document",
			domainerr.FieldViolation{Path: "", Code: violationInvalidValue, Message: "document contains more than one YAML value"})
	}
	if vs := inspectYAMLNode(&node, ""); len(vs) > 0 {
		return nil, classifyViolations("invalid YAML document", vs)
	}

	var raw any
	if err := node.Decode(&raw); err != nil {
		return nil, domainerr.ValidationFailed("YAML decode failed",
			domainerr.FieldViolation{Path: "", Code: violationInvalidValue, Message: err.Error()})
	}
	raw = stringifyKeys(raw)
	return decodeRaw(raw)
}

func decodeJSON(data []byte) (*model.State, error) {
	var raw any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return nil, domainerr.ValidationFailed("JSON decode failed",
			domainerr.FieldViolation{Path: "", Code: violationInvalidValue, Message: err.Error()})
	}
	if dec.More() {
		return nil, domainerr.ValidationFailed("trailing JSON value",
			domainerr.FieldViolation{Path: "", Code: violationInvalidValue, Message: "document contains more than one JSON value"})
	}
	return decodeRaw(raw)
}

func decodeRaw(raw any) (*model.State, error) {
	if vs := reservedFields(raw, ""); len(vs) > 0 {
		return nil, domainerr.ReservedKey("reserved fields", vs...)
	}
	applyDecodeDefaults(raw)
	if vs := unknownFields(raw, reflect.TypeOf(model.State{}), ""); len(vs) > 0 {
		return nil, domainerr.UnknownField("unknown fields", vs...)
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, domainerr.ValidationFailed("re-encode failed",
			domainerr.FieldViolation{Path: "", Code: violationInvalidValue, Message: err.Error()})
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var st model.State
	if err := dec.Decode(&st); err != nil {
		return nil, mapJSONDecodeError(err)
	}
	return &st, nil
}

func classifyViolations(message string, vs []domainerr.FieldViolation) error {
	hasReserved := false
	hasUnknown := false
	for _, v := range vs {
		switch v.Code {
		case violationReservedKey:
			hasReserved = true
		case violationUnknownField:
			hasUnknown = true
		}
	}
	switch {
	case hasReserved:
		return domainerr.ReservedKey(message, vs...)
	case hasUnknown:
		return domainerr.UnknownField(message, vs...)
	default:
		return domainerr.ValidationFailed(message, vs...)
	}
}

func mapJSONDecodeError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "unknown field") {
		field := msg
		if i := strings.Index(msg, `"`); i >= 0 {
			if j := strings.LastIndex(msg, `"`); j > i {
				field = msg[i+1 : j]
			}
		}
		return domainerr.UnknownField("unknown fields",
			domainerr.FieldViolation{Path: field, Code: violationUnknownField, Message: "unknown field"})
	}
	return domainerr.ValidationFailed("JSON decode failed",
		domainerr.FieldViolation{Path: "", Code: violationInvalidValue, Message: msg})
}

func mapYAMLKnownFieldsError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "not found") || strings.Contains(msg, "unknown field") {
		field := msg
		if i := strings.Index(msg, `"`); i >= 0 {
			if j := strings.Index(msg[i+1:], `"`); j >= 0 {
				field = msg[i+1 : i+1+j]
			}
		}
		return domainerr.UnknownField("unknown fields",
			domainerr.FieldViolation{Path: field, Code: violationUnknownField, Message: fmt.Sprintf("unknown field %q", field)})
	}
	return domainerr.ValidationFailed("YAML decode failed",
		domainerr.FieldViolation{Path: "", Code: violationInvalidValue, Message: msg})
}

func looksLikeJSON(data []byte) bool {
	data = bytes.TrimSpace(data)
	return len(data) > 0 && (data[0] == '{' || data[0] == '[')
}

func stripBOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
}

func stringifyKeys(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, child := range x {
			out[k] = stringifyKeys(child)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, child := range x {
			out[fmt.Sprint(k)] = stringifyKeys(child)
		}
		return out
	case []any:
		for i, child := range x {
			x[i] = stringifyKeys(child)
		}
		return x
	default:
		return v
	}
}

func applyDecodeDefaults(v any) {
	root, ok := v.(map[string]any)
	if !ok {
		return
	}
	spec := ensureMap(root, "spec")
	if spec == nil {
		return
	}

	listeners := ensureMap(spec, "listeners")
	netconfL := ensureMap(listeners, "netconf")
	setDefault(netconfL, "enabled", true)
	setDefaultAddress(netconfL, DefaultNetconfAddress)

	restconfL := ensureMap(listeners, "restconf")
	setDefault(restconfL, "enabled", true)
	setDefaultAddress(restconfL, DefaultRestconfAddress)
	tls := ensureMap(restconfL, "tls")
	setDefault(tls, "enabled", false)

	mgmtL := ensureMap(listeners, "management")
	setDefaultAddress(mgmtL, DefaultMgmtAddress)
	setDefault(mgmtL, "restPath", DefaultRESTPath)
	setDefault(mgmtL, "mcpPath", DefaultMCPPath)

	callHome := ensureMap(listeners, "callHome")
	setDefault(callHome, "enabled", false)
	netconfTLS := ensureMap(listeners, "netconfTls")
	setDefault(netconfTLS, "enabled", false)

	auth := ensureMap(spec, "auth")
	setDefault(auth, "mode", model.MgmtAuthBearer)
	setDefault(auth, "tokens", []any{})

	ui := ensureMap(spec, "ui")
	setDefault(ui, "enabled", true)

	netconf := ensureMap(spec, "netconf")
	if _, exists := netconf["versions"]; !exists {
		netconf["versions"] = []any{model.NetconfVersion10, model.NetconfVersion11}
	}
	setDefault(netconf, "writableRunning", false)
	notifs := ensureMap(netconf, "notifications")
	setDefault(notifs, "enabled", true)
	setDefault(netconf, "sharedProfileDatastore", true)

	restconf := ensureMap(spec, "restconf")
	setDefault(restconf, "json", true)
	setDefault(restconf, "xml", false)

	// admission.allowClientCidrs: omitted/null stays nil (materialized in Normalize).
	_ = ensureMap(spec, "admission")

	mgmt := ensureMap(spec, "management")
	setDefault(mgmt, "allowedOrigins", []any{})
	mcp := ensureMap(mgmt, "mcp")
	setDefault(mcp, "allowLegacyClients", false)

	if profiles, ok := spec["profiles"].([]any); ok {
		for _, item := range profiles {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if _, exists := m["instance"]; !exists {
				m["instance"] = map[string]any{}
			}
		}
	}
	if _, exists := spec["profiles"]; !exists {
		spec["profiles"] = []any{}
	}
	if _, exists := spec["users"]; !exists {
		spec["users"] = []any{}
	}
}

func ensureMap(parent map[string]any, key string) map[string]any {
	if parent == nil {
		return nil
	}
	if m, ok := parent[key].(map[string]any); ok {
		return m
	}
	if v, exists := parent[key]; exists && v != nil {
		return nil
	}
	m := map[string]any{}
	parent[key] = m
	return m
}

func setDefault(obj map[string]any, key string, val any) {
	if obj == nil {
		return
	}
	if _, exists := obj[key]; !exists {
		obj[key] = val
	}
}

func setDefaultAddress(obj map[string]any, def string) {
	if obj == nil {
		return
	}
	if v, ok := obj["address"].(string); ok {
		if strings.TrimSpace(v) != "" {
			return
		}
	} else if _, exists := obj["address"]; exists {
		return
	}
	obj["address"] = def
}

func inspectYAMLNode(n *yaml.Node, path string) []domainerr.FieldViolation {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.AliasNode || n.Anchor != "" {
		return []domainerr.FieldViolation{{
			Path:    path,
			Code:    violationInvalidValue,
			Message: "YAML aliases and anchors are not allowed",
		}}
	}
	if n.Kind == yaml.DocumentNode {
		var vs []domainerr.FieldViolation
		for _, c := range n.Content {
			vs = append(vs, inspectYAMLNode(c, path)...)
		}
		return vs
	}
	if n.Kind == yaml.MappingNode {
		var vs []domainerr.FieldViolation
		seen := map[string]bool{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			v := n.Content[i+1]
			name := k.Value
			if name == "<<" {
				vs = append(vs, domainerr.FieldViolation{
					Path:    joinPath(path, name),
					Code:    violationInvalidValue,
					Message: "YAML merge keys are not allowed",
				})
				continue
			}
			if seen[name] {
				vs = append(vs, domainerr.FieldViolation{
					Path:    joinPath(path, name),
					Code:    violationDuplicateKey,
					Message: fmt.Sprintf("duplicate key %q", name),
				})
			}
			seen[name] = true
			childPath := joinPath(path, name)
			if !isFreeFormPath(path) && !isFreeFormPath(childPath) && !allowReservedPath(childPath) {
				if why := reservedReason(normalizeKey(name)); why != "" {
					vs = append(vs, domainerr.FieldViolation{
						Path:    childPath,
						Code:    violationReservedKey,
						Message: fmt.Sprintf("reserved key %q — not a LabNETCONF surface", name),
					})
				}
			}
			vs = append(vs, inspectYAMLNode(k, childPath)...)
			vs = append(vs, inspectYAMLNode(v, childPath)...)
		}
		return vs
	}
	if n.Kind == yaml.SequenceNode {
		var vs []domainerr.FieldViolation
		for i, c := range n.Content {
			vs = append(vs, inspectYAMLNode(c, indexPath(path, i))...)
		}
		return vs
	}
	return nil
}

func unknownFields(val any, typ reflect.Type, path string) []domainerr.FieldViolation {
	if val == nil || typ == nil {
		return nil
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Struct:
		obj, ok := val.(map[string]any)
		if !ok {
			return nil
		}
		fields := jsonFieldMap(typ)
		var vs []domainerr.FieldViolation
		for k, child := range obj {
			ft, ok := fields[k]
			if !ok {
				vs = append(vs, domainerr.FieldViolation{
					Path:    joinPath(path, k),
					Code:    violationUnknownField,
					Message: fmt.Sprintf("unknown field %q", k),
				})
				continue
			}
			vs = append(vs, unknownFields(child, ft, joinPath(path, k))...)
		}
		return vs
	case reflect.Slice, reflect.Array:
		arr, ok := val.([]any)
		if !ok {
			return nil
		}
		var vs []domainerr.FieldViolation
		for i, child := range arr {
			vs = append(vs, unknownFields(child, typ.Elem(), indexPath(path, i))...)
		}
		return vs
	default:
		return nil
	}
}

func jsonFieldMap(typ reflect.Type) map[string]reflect.Type {
	out := make(map[string]reflect.Type)
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.PkgPath != "" {
			continue
		}
		tag := f.Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if name == "" || name == "-" {
			continue
		}
		out[name] = f.Type
	}
	return out
}

func joinPath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}

func indexPath(base string, i int) string {
	return fmt.Sprintf("%s[%d]", base, i)
}
