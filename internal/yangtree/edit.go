package yangtree

import (
	"github.com/hilather/go-lab-netconf/internal/domainerr"
)

// Operation is a tree mutation verb.
type Operation string

const (
	OpMerge   Operation = "merge"
	OpReplace Operation = "replace"
	OpDelete  Operation = "delete"
)

type cursor struct {
	get func() any
	set func(any)
}

// Apply mutates n at path. Unknown paths and read-only leaves fail.
func (n *Node) Apply(op Operation, path string, value any) error {
	if n == nil {
		return domainerr.ValidationFailed("nil node")
	}
	p, err := ParsePath(path)
	if err != nil {
		return err
	}
	return n.apply(op, p, value)
}

func (n *Node) apply(op Operation, p Path, value any) error {
	if vn, ok := value.(Node); ok {
		if p.IsRoot() {
			value = vn.Map()
		} else if got, ok := vn.lookup(p); ok {
			value = got
		} else {
			value = vn.Map()
		}
	}
	if n.schema != nil && !n.schema.known(p) {
		return unknownPath(p)
	}
	if op != OpDelete && n.schema != nil && n.schema.readOnly(p) {
		return domainerr.NotWritable("leaf is not writable")
	}
	tmp := n.Clone()
	if tmp.root == nil {
		tmp.root = map[string]any{}
	}
	var err error
	switch op {
	case OpMerge, "":
		err = tmp.put(p, value, true)
	case OpReplace:
		err = tmp.put(p, value, false)
	case OpDelete:
		err = tmp.deleteAt(p)
	default:
		return domainerr.ValidationFailed("unsupported operation")
	}
	if err != nil {
		return err
	}
	n.root = tmp.root
	return nil
}

func (n *Node) put(p Path, value any, merge bool) error {
	if p.IsRoot() {
		m, ok := asMap(value)
		if !ok {
			return domainerr.ValidationFailed("root write requires a container")
		}
		if merge {
			n.root = mergeMaps(n.root, m)
		} else {
			n.root, _ = asMap(clone(m))
			if n.root == nil {
				n.root = map[string]any{}
			}
		}
		return n.checkWritten(p, n.root)
	}
	cur := n.moduleCursor(p.Module, true)
	if len(p.Segments) == 0 {
		var next any
		if merge {
			next = mergeValues(cur.get(), value)
		} else {
			next = clone(value)
		}
		if err := n.checkWritten(p, next); err != nil {
			return err
		}
		cur.set(next)
		return nil
	}
	for i := 0; i < len(p.Segments)-1; i++ {
		next, err := descend(cur, p.Segments[i], true)
		if err != nil {
			return err
		}
		cur = next
	}
	return n.applyLast(cur, p, value, merge)
}

func (n *Node) applyLast(parent cursor, p Path, value any, merge bool) error {
	seg, _ := p.last()
	if len(seg.Keys) > 0 {
		entryCur, err := descend(parent, seg, true)
		if err != nil {
			return err
		}
		var next any
		if merge {
			next = mergeValues(entryCur.get(), value)
		} else {
			next = clone(value)
		}
		if m, ok := asMap(next); ok {
			for _, k := range seg.Keys {
				m[k.Name] = k.Value
			}
			next = m
		} else if !isScalarLeaf(n, p) {
			return domainerr.ValidationFailed("list entry requires a container")
		}
		if err := n.checkWritten(p, next); err != nil {
			return err
		}
		entryCur.set(next)
		return nil
	}
	m, ok := asMap(parent.get())
	if !ok || m == nil {
		m = map[string]any{}
		parent.set(m)
	}
	var next any
	if merge {
		next = mergeValues(m[seg.Name], value)
	} else {
		next = clone(value)
	}
	if err := n.checkWritten(p, next); err != nil {
		return err
	}
	m[seg.Name] = next
	return nil
}

func isScalarLeaf(n *Node, p Path) bool {
	if n.schema == nil {
		return false
	}
	e, ok := n.schema.lookup(p)
	if !ok {
		return false
	}
	switch e.typ {
	case "list", "container", "leaf-list":
		return false
	default:
		return true
	}
}

func (n *Node) deleteAt(p Path) error {
	if p.IsRoot() {
		n.root = map[string]any{}
		return nil
	}
	if len(p.Segments) == 0 {
		if _, ok := n.root[p.Module]; !ok {
			return domainerr.NotFound("path does not exist")
		}
		delete(n.root, p.Module)
		return nil
	}
	cur := n.moduleCursor(p.Module, false)
	if cur.get() == nil {
		return domainerr.NotFound("path does not exist")
	}
	for i := 0; i < len(p.Segments)-1; i++ {
		next, err := descend(cur, p.Segments[i], false)
		if err != nil {
			return err
		}
		if next.get == nil || next.get() == nil {
			return domainerr.NotFound("path does not exist")
		}
		cur = next
	}
	seg, _ := p.last()
	if len(seg.Keys) > 0 {
		m, ok := asMap(cur.get())
		if !ok {
			return domainerr.NotFound("path does not exist")
		}
		arr, ok := asSlice(m[seg.Name])
		if !ok {
			return domainerr.NotFound("path does not exist")
		}
		idx, _, found := listIndex(arr, seg.Keys)
		if !found {
			return domainerr.NotFound("path does not exist")
		}
		m[seg.Name] = append(arr[:idx], arr[idx+1:]...)
		return nil
	}
	m, ok := asMap(cur.get())
	if !ok {
		return domainerr.NotFound("path does not exist")
	}
	if _, ok := m[seg.Name]; !ok {
		return domainerr.NotFound("path does not exist")
	}
	delete(m, seg.Name)
	return nil
}

func (n *Node) checkWritten(p Path, value any) error {
	if n.schema == nil {
		return nil
	}
	e, ok := n.schema.lookup(p)
	if ok {
		if e.valueFrom != "" {
			return domainerr.NotWritable("leaf is not writable")
		}
		if err := e.checkValue(value); err != nil {
			return err
		}
	}
	return n.validateValue(p, value)
}

func (n *Node) moduleCursor(mod string, create bool) cursor {
	if _, ok := n.root[mod]; !ok && create {
		n.root[mod] = map[string]any{}
	}
	return cursor{
		get: func() any { return n.root[mod] },
		set: func(v any) { n.root[mod] = v },
	}
}

func descend(cur cursor, seg Segment, create bool) (cursor, error) {
	parent := cur.get()
	m, ok := asMap(parent)
	if !ok || m == nil {
		if !create {
			return cursor{}, domainerr.NotFound("path does not exist")
		}
		m = map[string]any{}
		cur.set(m)
	}
	if len(seg.Keys) == 0 {
		if _, ok := m[seg.Name]; !ok {
			if !create {
				return cursor{}, domainerr.NotFound("path does not exist")
			}
			m[seg.Name] = map[string]any{}
		}
		return cursor{
			get: func() any { return m[seg.Name] },
			set: func(v any) { m[seg.Name] = v },
		}, nil
	}
	arr, ok := asSlice(m[seg.Name])
	if !ok {
		if !create {
			return cursor{}, domainerr.NotFound("path does not exist")
		}
		arr = []any{}
		m[seg.Name] = arr
	}
	idx, _, found := listIndex(arr, seg.Keys)
	if !found {
		if !create {
			return cursor{}, domainerr.NotFound("path does not exist")
		}
		arr = append(arr, entryWithKeys(seg.Keys))
		idx = len(arr) - 1
		m[seg.Name] = arr
	}
	captured := idx
	return cursor{
		get: func() any {
			a, ok := asSlice(m[seg.Name])
			if !ok || captured >= len(a) {
				return nil
			}
			return a[captured]
		},
		set: func(v any) {
			a, ok := asSlice(m[seg.Name])
			if !ok {
				a = []any{}
			}
			if captured >= len(a) {
				return
			}
			a[captured] = v
			m[seg.Name] = a
		},
	}, nil
}

func mergeMaps(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = map[string]any{}
	}
	for k, sv := range src {
		if dv, ok := dst[k]; ok {
			dst[k] = mergeValues(dv, sv)
		} else {
			dst[k] = clone(sv)
		}
	}
	return dst
}

func mergeValues(dst, src any) any {
	if src == nil {
		return dst
	}
	dm, dMap := asMap(dst)
	sm, sMap := asMap(src)
	if dMap && sMap {
		return mergeMaps(dm, sm)
	}
	ds, dSlice := asSlice(dst)
	ss, sSlice := asSlice(src)
	if sSlice {
		if !dSlice {
			ds = nil
		}
		return mergeLists(ds, ss)
	}
	if sMap && !dMap {
		return clone(sm)
	}
	return clone(src)
}

func mergeLists(dst, src []any) []any {
	out := make([]any, len(dst))
	copy(out, dst)
	for _, item := range src {
		m, ok := asMap(item)
		if !ok {
			out = append(out, clone(item))
			continue
		}
		keys := keysForItem(m)
		if len(keys) == 0 {
			out = append(out, clone(item))
			continue
		}
		idx, existing, found := listIndex(out, keys)
		if !found {
			out = append(out, clone(m))
			continue
		}
		out[idx] = mergeValues(existing, m)
	}
	return out
}
