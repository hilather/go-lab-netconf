package yangtree

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
)

// Path is a module-qualified instance identifier.
type Path struct {
	Module   string
	Segments []Segment
}

// Segment is one path step, optionally with list key predicates.
type Segment struct {
	Name string
	Keys []Key
}

// Key is one list predicate [name=value].
type Key struct {
	Name  string
	Value string
}

// ParsePath parses module:container/list[k=v]/leaf.
// The empty string is the tree root.
func ParsePath(s string) (Path, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Path{}, nil
	}
	colon := strings.IndexByte(s, ':')
	if colon <= 0 {
		return Path{}, domainerr.ValidationFailed("path must be module-qualified",
			domainerr.FieldViolation{Path: s, Code: "unknown-element", Message: "missing module:"})
	}
	mod := s[:colon]
	if !isIdent(mod) {
		return Path{}, domainerr.ValidationFailed("invalid module name",
			domainerr.FieldViolation{Path: s, Code: "unknown-element", Message: "invalid module name"})
	}
	p := Path{Module: mod}
	rest := s[colon+1:]
	if rest == "" {
		return p, nil
	}
	i := 0
	for i < len(rest) {
		if rest[i] == '/' {
			return Path{}, domainerr.ValidationFailed("invalid path",
				domainerr.FieldViolation{Path: s, Code: "unknown-element", Message: "empty path segment"})
		}
		name, n, err := readIdentAt(rest, i)
		if err != nil {
			return Path{}, domainerr.ValidationFailed("invalid path",
				domainerr.FieldViolation{Path: s, Code: "unknown-element", Message: err.Error()})
		}
		i = n
		seg := Segment{Name: name}
		for i < len(rest) && rest[i] == '[' {
			k, n, err := readKeyAt(rest, i)
			if err != nil {
				return Path{}, domainerr.ValidationFailed("invalid path",
					domainerr.FieldViolation{Path: s, Code: "unknown-element", Message: err.Error()})
			}
			seg.Keys = append(seg.Keys, k)
			i = n
		}
		p.Segments = append(p.Segments, seg)
		if i == len(rest) {
			break
		}
		if rest[i] != '/' {
			return Path{}, domainerr.ValidationFailed("invalid path",
				domainerr.FieldViolation{Path: s, Code: "unknown-element", Message: "expected '/' between segments"})
		}
		i++
		if i == len(rest) {
			return Path{}, domainerr.ValidationFailed("invalid path",
				domainerr.FieldViolation{Path: s, Code: "unknown-element", Message: "trailing slash"})
		}
	}
	return p, nil
}

// String renders the instance identifier.
func (p Path) String() string {
	if p.Module == "" && len(p.Segments) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(p.Module)
	b.WriteByte(':')
	for i, seg := range p.Segments {
		if i > 0 {
			b.WriteByte('/')
		}
		b.WriteString(seg.Name)
		for _, k := range seg.Keys {
			b.WriteByte('[')
			b.WriteString(k.Name)
			b.WriteByte('=')
			b.WriteString(quoteKeyValue(k.Value))
			b.WriteByte(']')
		}
	}
	return b.String()
}

// IsRoot reports whether p addresses the whole tree.
func (p Path) IsRoot() bool {
	return p.Module == "" && len(p.Segments) == 0
}

// SchemaString is the path with list predicates stripped.
func (p Path) SchemaString() string {
	if p.Module == "" && len(p.Segments) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(p.Module)
	b.WriteByte(':')
	for i, seg := range p.Segments {
		if i > 0 {
			b.WriteByte('/')
		}
		b.WriteString(seg.Name)
	}
	return b.String()
}

func (p Path) append(seg Segment) Path {
	out := Path{Module: p.Module, Segments: make([]Segment, len(p.Segments)+1)}
	copy(out.Segments, p.Segments)
	out.Segments[len(p.Segments)] = seg
	return out
}

func (p Path) last() (Segment, bool) {
	if len(p.Segments) == 0 {
		return Segment{}, false
	}
	return p.Segments[len(p.Segments)-1], true
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' && r != '.' {
			return false
		}
	}
	return true
}

func readIdentAt(s string, i int) (string, int, error) {
	if i >= len(s) {
		return "", i, fmt.Errorf("expected identifier")
	}
	j := i
	for j < len(s) {
		r := rune(s[j])
		if j == i {
			if !unicode.IsLetter(r) && r != '_' {
				return "", i, fmt.Errorf("expected identifier")
			}
		} else if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' && r != '.' {
			break
		}
		j++
	}
	return s[i:j], j, nil
}

func readKeyAt(s string, i int) (Key, int, error) {
	if i >= len(s) || s[i] != '[' {
		return Key{}, i, fmt.Errorf("expected '['")
	}
	i++
	name, n, err := readIdentAt(s, i)
	if err != nil {
		return Key{}, i, fmt.Errorf("invalid list key name")
	}
	i = n
	if i >= len(s) || s[i] != '=' {
		return Key{}, i, fmt.Errorf("expected '=' in list predicate")
	}
	i++
	val, n, err := readKeyValueAt(s, i)
	if err != nil {
		return Key{}, i, err
	}
	i = n
	if i >= len(s) || s[i] != ']' {
		return Key{}, i, fmt.Errorf("unterminated list predicate")
	}
	return Key{Name: name, Value: val}, i + 1, nil
}

func readKeyValueAt(s string, i int) (string, int, error) {
	if i >= len(s) {
		return "", i, fmt.Errorf("missing list key value")
	}
	switch s[i] {
	case '\'', '"':
		q := s[i]
		i++
		j := i
		for j < len(s) && s[j] != q {
			j++
		}
		if j >= len(s) {
			return "", i, fmt.Errorf("unterminated quoted list key")
		}
		return s[i:j], j + 1, nil
	default:
		j := i
		for j < len(s) && s[j] != ']' {
			j++
		}
		if j == i {
			return "", i, fmt.Errorf("empty list key value")
		}
		return s[i:j], j, nil
	}
}

func quoteKeyValue(v string) string {
	if v == "" || strings.ContainsAny(v, "/[]'\" \t") {
		return "'" + strings.ReplaceAll(v, "'", "") + "'"
	}
	return v
}
