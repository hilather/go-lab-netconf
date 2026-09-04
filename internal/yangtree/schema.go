package yangtree

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
)

var processStart = time.Now()

type schema struct {
	byPath map[string]schemaEntry
}

type schemaEntry struct {
	path      Path
	typ       string
	access    string
	rangeMin  *float64
	rangeMax  *float64
	pattern   *regexp.Regexp
	valueFrom string
}

func newSchema(leaves []model.SchemaLeaf) (*schema, error) {
	s := &schema{byPath: make(map[string]schemaEntry, len(leaves))}
	for i, leaf := range leaves {
		if strings.TrimSpace(leaf.Path) == "" {
			return nil, domainerr.ValidationFailed("schema path is required",
				domainerr.FieldViolation{Path: fmt.Sprintf("schema[%d].path", i), Code: "required", Message: "schema path is required"})
		}
		p, err := ParsePath(leaf.Path)
		if err != nil {
			return nil, err
		}
		key := p.SchemaString()
		if _, dup := s.byPath[key]; dup {
			return nil, domainerr.ValidationFailed("duplicate schema path",
				domainerr.FieldViolation{Path: leaf.Path, Code: "duplicate_id", Message: "duplicate schema path"})
		}
		if !model.KnownSchemaType(leaf.Type) {
			return nil, domainerr.ValidationFailed("unsupported compact schema type",
				domainerr.FieldViolation{Path: leaf.Path, Code: "invalid_value", Message: "unsupported compact schema type"})
		}
		e := schemaEntry{
			path:      p,
			typ:       leaf.Type,
			access:    leaf.Access,
			valueFrom: leaf.ValueFrom,
		}
		if leaf.ValueFrom != "" && leaf.ValueFrom != model.ValueFromProcessUptime {
			return nil, domainerr.ValidationFailed("valueFrom must be processUptime",
				domainerr.FieldViolation{Path: leaf.Path, Code: "invalid_value", Message: "valueFrom must be processUptime"})
		}
		if leaf.Range != "" {
			min, max, err := parseRange(leaf.Range)
			if err != nil {
				return nil, domainerr.ValidationFailed("invalid range",
					domainerr.FieldViolation{Path: leaf.Path, Code: "invalid_value", Message: err.Error()})
			}
			e.rangeMin, e.rangeMax = min, max
		}
		if leaf.Pattern != "" {
			re, err := regexp.Compile(leaf.Pattern)
			if err != nil {
				return nil, domainerr.ValidationFailed("invalid pattern",
					domainerr.FieldViolation{Path: leaf.Path, Code: "invalid_value", Message: err.Error()})
			}
			e.pattern = re
		}
		s.byPath[key] = e
	}
	return s, nil
}

func (s *schema) lookup(p Path) (schemaEntry, bool) {
	if s == nil {
		return schemaEntry{}, false
	}
	e, ok := s.byPath[p.SchemaString()]
	return e, ok
}

func (s *schema) known(p Path) bool {
	if p.IsRoot() {
		return true
	}
	if s == nil || len(s.byPath) == 0 {
		return false
	}
	sp := p.SchemaString()
	if _, ok := s.byPath[sp]; ok {
		return true
	}
	for key := range s.byPath {
		if key == sp {
			return true
		}
		// ancestor of a declared path (implicit container)
		if strings.HasPrefix(key, sp+"/") {
			return true
		}
		// module root "mod:" prefixes "mod:container/..."
		if len(p.Segments) == 0 && strings.HasPrefix(key, sp) {
			return true
		}
	}
	return false
}

func (s *schema) readOnly(p Path) bool {
	e, ok := s.lookup(p)
	if !ok {
		return false
	}
	if e.valueFrom != "" {
		return true
	}
	return e.access == model.SchemaAccessRead
}

func (e schemaEntry) checkValue(v any) error {
	if err := checkType(e.typ, v); err != nil {
		return err
	}
	if e.pattern != nil {
		str, ok := v.(string)
		if !ok {
			return domainerr.ValidationFailed("pattern applies to strings",
				domainerr.FieldViolation{Path: e.path.String(), Code: "invalid_value", Message: "pattern applies to strings"})
		}
		if !e.pattern.MatchString(str) {
			return domainerr.ValidationFailed("value does not match pattern",
				domainerr.FieldViolation{Path: e.path.String(), Code: "invalid_value", Message: "value does not match pattern"})
		}
	}
	if e.rangeMin != nil || e.rangeMax != nil {
		n, ok := asFloat64(v)
		if !ok {
			return domainerr.ValidationFailed("range applies to numeric values",
				domainerr.FieldViolation{Path: e.path.String(), Code: "invalid_value", Message: "range applies to numeric values"})
		}
		if e.rangeMin != nil && n < *e.rangeMin {
			return domainerr.ValidationFailed("value is below range",
				domainerr.FieldViolation{Path: e.path.String(), Code: "invalid_value", Message: "value is below range"})
		}
		if e.rangeMax != nil && n > *e.rangeMax {
			return domainerr.ValidationFailed("value is above range",
				domainerr.FieldViolation{Path: e.path.String(), Code: "invalid_value", Message: "value is above range"})
		}
	}
	return nil
}

func checkType(typ string, v any) error {
	switch typ {
	case "string", "identityref", "enumeration":
		if _, ok := v.(string); !ok {
			return typeError(typ)
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return typeError(typ)
		}
	case "int32":
		n, ok := asInt64(v)
		if !ok || n < math.MinInt32 || n > math.MaxInt32 {
			return typeError(typ)
		}
	case "int64":
		if _, ok := asInt64(v); !ok {
			return typeError(typ)
		}
	case "uint32":
		n, ok := asUint64(v)
		if !ok || n > math.MaxUint32 {
			return typeError(typ)
		}
	case "uint64":
		if _, ok := asUint64(v); !ok {
			return typeError(typ)
		}
	case "decimal64":
		if _, ok := asFloat64(v); !ok {
			return typeError(typ)
		}
	case "empty":
		if !isEmptyType(v) {
			return typeError(typ)
		}
	case "leaf-list":
		arr, ok := v.([]any)
		if !ok {
			return typeError(typ)
		}
		for _, item := range arr {
			if isMap(item) || isSlice(item) {
				return typeError(typ)
			}
		}
	case "list":
		if _, ok := v.([]any); ok {
			return nil
		}
		if isMap(v) {
			return nil
		}
		return typeError(typ)
	case "container":
		if !isMap(v) && v != nil {
			return typeError(typ)
		}
	default:
		return typeError(typ)
	}
	return nil
}

func typeError(typ string) error {
	return domainerr.ValidationFailed("value does not match compact type "+typ,
		domainerr.FieldViolation{Code: "invalid_value", Message: "value does not match type " + typ})
}

func parseRange(s string) (*float64, *float64, error) {
	parts := strings.Split(s, "..")
	if len(parts) != 2 {
		return nil, nil, fmt.Errorf("range must be min..max")
	}
	var min, max *float64
	if strings.TrimSpace(parts[0]) != "min" {
		n, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid range min")
		}
		min = &n
	}
	if strings.TrimSpace(parts[1]) != "max" {
		n, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid range max")
		}
		max = &n
	}
	return min, max, nil
}

func processUptimeSeconds() int64 {
	return int64(time.Since(processStart).Seconds())
}

func getterValue(e schemaEntry) any {
	sec := processUptimeSeconds()
	switch e.typ {
	case "uint32", "uint64":
		if sec < 0 {
			sec = 0
		}
		return uint64(sec)
	case "int32":
		return int32(sec)
	default:
		return sec
	}
}

func unknownPath(p Path) error {
	return domainerr.ValidationFailed("unknown path",
		domainerr.FieldViolation{Path: p.String(), Code: "unknown-element", Message: "path is not in the schema"})
}
