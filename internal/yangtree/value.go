package yangtree

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

func clone(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, val := range x {
			m[k] = clone(val)
		}
		return m
	case map[any]any:
		m := make(map[string]any, len(x))
		for k, val := range x {
			m[fmt.Sprint(k)] = clone(val)
		}
		return m
	case []any:
		s := make([]any, len(x))
		for i, val := range x {
			s[i] = clone(val)
		}
		return s
	case []map[string]any:
		s := make([]any, len(x))
		for i, val := range x {
			s[i] = clone(val)
		}
		return s
	default:
		return v
	}
}

func asMap(v any) (map[string]any, bool) {
	switch x := v.(type) {
	case map[string]any:
		return x, true
	case map[any]any:
		m := make(map[string]any, len(x))
		for k, val := range x {
			m[fmt.Sprint(k)] = val
		}
		return m, true
	default:
		return nil, false
	}
}

func asSlice(v any) ([]any, bool) {
	switch x := v.(type) {
	case []any:
		return x, true
	case []map[string]any:
		s := make([]any, len(x))
		for i, val := range x {
			s[i] = val
		}
		return s, true
	default:
		return nil, false
	}
}

func isMap(v any) bool {
	_, ok := asMap(v)
	return ok
}

func isSlice(v any) bool {
	_, ok := asSlice(v)
	return ok
}

func isEmptyType(v any) bool {
	if v == nil {
		return true
	}
	if b, ok := v.(bool); ok {
		return b
	}
	if m, ok := asMap(v); ok {
		return len(m) == 0
	}
	if s, ok := asSlice(v); ok {
		return len(s) == 0 || (len(s) == 1 && s[0] == nil)
	}
	return false
}

func asInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	case uint:
		if uint64(x) > math.MaxInt64 {
			return 0, false
		}
		return int64(x), true
	case uint32:
		return int64(x), true
	case uint64:
		if x > math.MaxInt64 {
			return 0, false
		}
		return int64(x), true
	case float64:
		if x == math.Trunc(x) && x >= math.MinInt64 && x <= math.MaxInt64 {
			return int64(x), true
		}
		return 0, false
	case json.Number:
		i, err := x.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}

func asUint64(v any) (uint64, bool) {
	switch x := v.(type) {
	case uint:
		return uint64(x), true
	case uint32:
		return uint64(x), true
	case uint64:
		return x, true
	case int:
		if x < 0 {
			return 0, false
		}
		return uint64(x), true
	case int32:
		if x < 0 {
			return 0, false
		}
		return uint64(x), true
	case int64:
		if x < 0 {
			return 0, false
		}
		return uint64(x), true
	case float64:
		if x < 0 || x != math.Trunc(x) || x > math.MaxUint64 {
			return 0, false
		}
		return uint64(x), true
	case json.Number:
		u, err := strconv.ParseUint(string(x), 10, 64)
		return u, err == nil
	default:
		return 0, false
	}
}

func asFloat64(v any) (float64, bool) {
	if n, ok := asInt64(v); ok {
		return float64(n), true
	}
	if n, ok := asUint64(v); ok {
		return float64(n), true
	}
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func stringify(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(v)
	}
}

func equal(a, b any) bool {
	if am, ok := asMap(a); ok {
		bm, ok := asMap(b)
		if !ok || len(am) != len(bm) {
			return false
		}
		for k, av := range am {
			if !equal(av, bm[k]) {
				return false
			}
		}
		return true
	}
	if as, ok := asSlice(a); ok {
		bs, ok := asSlice(b)
		if !ok || len(as) != len(bs) {
			return false
		}
		for i := range as {
			if !equal(as[i], bs[i]) {
				return false
			}
		}
		return true
	}
	if ai, aok := asInt64(a); aok {
		if bi, bok := asInt64(b); bok {
			return ai == bi
		}
	}
	if au, aok := asUint64(a); aok {
		if bu, bok := asUint64(b); bok {
			return au == bu
		}
	}
	if af, aok := asFloat64(a); aok {
		if bf, bok := asFloat64(b); bok {
			return af == bf
		}
	}
	return a == b
}

func matchKeys(m map[string]any, keys []Key) bool {
	if m == nil {
		return false
	}
	for _, k := range keys {
		v, ok := m[k.Name]
		if !ok || stringify(v) != k.Value {
			return false
		}
	}
	return true
}

func listIndex(arr []any, keys []Key) (int, map[string]any, bool) {
	for i, item := range arr {
		m, ok := asMap(item)
		if !ok {
			continue
		}
		if matchKeys(m, keys) {
			return i, m, true
		}
	}
	return -1, nil, false
}

func keysForItem(item map[string]any) []Key {
	if item == nil {
		return nil
	}
	for _, kn := range []string{"name", "id", "index", "key"} {
		if v, ok := item[kn]; ok {
			return []Key{{Name: kn, Value: stringify(v)}}
		}
	}
	for k, v := range item {
		if _, isM := asMap(v); isM {
			continue
		}
		if _, isS := asSlice(v); isS {
			continue
		}
		if _, ok := v.(string); ok {
			return []Key{{Name: k, Value: stringify(v)}}
		}
	}
	return nil
}

func entryWithKeys(keys []Key) map[string]any {
	m := make(map[string]any, len(keys))
	for _, k := range keys {
		m[k.Name] = k.Value
	}
	return m
}
