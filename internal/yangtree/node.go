package yangtree

import (
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
)

// Change is one leaf-level difference between two trees.
type Change struct {
	Path      string
	Operation string
}

// Node is a path-addressable configuration tree.
type Node struct {
	schema *schema
	root   map[string]any
}

// Compile builds a tree from an instance map and compact schema.
// Values are not type-checked here; call Validate or edit to check types.
func Compile(instance map[string]any, leaves []model.SchemaLeaf) (Node, error) {
	schema, err := newSchema(leaves)
	if err != nil {
		return Node{}, err
	}
	root, _ := asMap(clone(instance))
	if root == nil {
		root = map[string]any{}
	}
	return Node{schema: schema, root: root}, nil
}

// IsZero reports whether n is an uninitialized node.
func (n Node) IsZero() bool {
	return n.schema == nil && n.root == nil
}

// IsEmpty reports whether n has no children.
func (n Node) IsEmpty() bool {
	return len(n.root) == 0
}

// Clone returns a deep copy that does not alias n.
func (n Node) Clone() Node {
	root, _ := asMap(clone(n.root))
	if root == nil {
		root = map[string]any{}
	}
	return Node{schema: n.schema, root: root}
}

// Map returns a deep copy of the stored tree.
func (n Node) Map() map[string]any {
	m, _ := asMap(clone(n.root))
	if m == nil {
		return map[string]any{}
	}
	return m
}

// Equal reports whether the stored trees are equal (getters excluded).
func (n Node) Equal(other Node) bool {
	return equal(n.root, other.root)
}

// Lookup returns the value at path after applying dynamic getters.
// Missing or unknown paths return false.
func (n Node) Lookup(path string) (any, bool) {
	p, err := ParsePath(path)
	if err != nil {
		return nil, false
	}
	return n.lookup(p)
}

func (n Node) lookup(p Path) (any, bool) {
	if p.IsRoot() {
		return n.materializeRoot(), true
	}
	if n.schema != nil && !n.schema.known(p) {
		return nil, false
	}
	v, ok := n.walkGet(p)
	if !ok {
		if e, found := n.schema.lookup(p); found && e.valueFrom == model.ValueFromProcessUptime {
			return clone(getterValue(e)), true
		}
		return nil, false
	}
	if e, found := n.schema.lookup(p); found && e.valueFrom == model.ValueFromProcessUptime {
		return clone(getterValue(e)), true
	}
	return clone(v), true
}

// Subtree returns a copy of the tree selected by p. Unknown or missing
// paths yield an empty node. The empty path selects the whole tree.
func (n Node) Subtree(p Path) Node {
	out := Node{schema: n.schema, root: map[string]any{}}
	if p.IsRoot() {
		out.root, _ = asMap(n.materializeRoot())
		if out.root == nil {
			out.root = map[string]any{}
		}
		return out
	}
	v, ok := n.lookup(p)
	if !ok {
		return out
	}
	out.root = graft(p, v)
	return out
}

func (n Node) materializeRoot() map[string]any {
	root, _ := asMap(clone(n.root))
	if root == nil {
		root = map[string]any{}
	}
	n.applyGetters(Path{}, root)
	return root
}

func (n Node) applyGetters(prefix Path, v any) {
	if n.schema == nil {
		return
	}
	if e, ok := n.schema.lookup(prefix); ok && e.valueFrom == model.ValueFromProcessUptime {
		return
	}
	m, ok := asMap(v)
	if !ok {
		arr, ok := asSlice(v)
		if !ok {
			return
		}
		for _, item := range arr {
			im, ok := asMap(item)
			if !ok {
				continue
			}
			itemPath := prefix
			if keys := keysForItem(im); len(keys) > 0 && len(itemPath.Segments) > 0 {
				last := itemPath.Segments[len(itemPath.Segments)-1]
				last.Keys = keys
				itemPath.Segments[len(itemPath.Segments)-1] = last
			}
			n.applyGetters(itemPath, im)
		}
		return
	}
	for k, child := range m {
		np := prefix
		if np.Module == "" {
			np.Module = k
			n.applyGetters(np, child)
			continue
		}
		np = np.append(Segment{Name: k})
		if e, ok := n.schema.lookup(np); ok && e.valueFrom == model.ValueFromProcessUptime {
			m[k] = getterValue(e)
			continue
		}
		n.applyGetters(np, child)
	}
}

func (n Node) walkGet(p Path) (any, bool) {
	if n.root == nil {
		return nil, false
	}
	cur, ok := n.root[p.Module]
	if !ok {
		return nil, false
	}
	if len(p.Segments) == 0 {
		return cur, true
	}
	for i, seg := range p.Segments {
		if len(seg.Keys) > 0 {
			if m, ok := asMap(cur); ok {
				cur = m[seg.Name]
			}
			arr, ok := asSlice(cur)
			if !ok {
				return nil, false
			}
			_, item, ok := listIndex(arr, seg.Keys)
			if !ok {
				return nil, false
			}
			cur = item
			if i == len(p.Segments)-1 {
				return cur, true
			}
			continue
		}
		m, ok := asMap(cur)
		if !ok {
			return nil, false
		}
		next, ok := m[seg.Name]
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

// Validate type-checks stored values against the compact schema.
func (n Node) Validate() error {
	if n.schema == nil {
		return nil
	}
	return n.validateValue(Path{}, n.root)
}

func (n Node) validateValue(p Path, v any) error {
	if e, ok := n.schema.lookup(p); ok {
		switch e.typ {
		case "list":
			if arr, ok := asSlice(v); ok {
				for _, item := range arr {
					if err := n.validateChildren(p, item); err != nil {
						return err
					}
				}
				return nil
			}
			if isMap(v) {
				return n.validateChildren(p, v)
			}
			return e.checkValue(v)
		case "container":
			if err := e.checkValue(v); err != nil {
				return err
			}
			return n.validateChildren(p, v)
		case "leaf-list":
			return e.checkValue(v)
		default:
			if e.valueFrom == model.ValueFromProcessUptime {
				return nil
			}
			if err := e.checkValue(v); err != nil {
				de, ok := domainerr.As(err)
				if ok {
					return de.WithViolations(domainerr.FieldViolation{Path: p.String(), Code: "invalid_value", Message: de.Message})
				}
				return err
			}
			return nil
		}
	}
	return n.validateChildren(p, v)
}

func (n Node) validateChildren(p Path, v any) error {
	if m, ok := asMap(v); ok {
		for k, child := range m {
			np := p
			if np.Module == "" {
				np.Module = k
			} else {
				np = np.append(Segment{Name: k})
			}
			if err := n.validateValue(np, child); err != nil {
				return err
			}
		}
		return nil
	}
	if arr, ok := asSlice(v); ok {
		for _, item := range arr {
			if err := n.validateValue(p, item); err != nil {
				return err
			}
		}
	}
	return nil
}

// Diff returns leaf-level create/replace/delete operations from n to other.
func (n Node) Diff(other Node) []Change {
	var out []Change
	diffWalk(Path{}, n.root, other.root, &out)
	return out
}

func diffWalk(p Path, a, b any, out *[]Change) {
	if equal(a, b) {
		return
	}
	am, aMap := asMap(a)
	bm, bMap := asMap(b)
	if aMap && bMap {
		seen := map[string]bool{}
		for k := range am {
			seen[k] = true
		}
		for k := range bm {
			seen[k] = true
		}
		for k := range seen {
			np := p
			if np.Module == "" {
				np.Module = k
			} else {
				np = np.append(Segment{Name: k})
			}
			diffWalk(np, am[k], bm[k], out)
		}
		return
	}
	as, aSlice := asSlice(a)
	bs, bSlice := asSlice(b)
	if aSlice && bSlice {
		type pair struct {
			keys []Key
			a    any
			b    any
		}
		index := map[string]*pair{}
		order := []string{}
		add := func(item any, fromA bool) {
			m, ok := asMap(item)
			keys := keysForItem(m)
			id := p.String()
			if ok && len(keys) > 0 {
				tmp := p
				if len(tmp.Segments) > 0 {
					last := tmp.Segments[len(tmp.Segments)-1]
					last.Keys = keys
					tmp.Segments[len(tmp.Segments)-1] = last
				}
				id = tmp.String()
			} else {
				id = p.String() + "#" + stringify(item)
			}
			pr, ok := index[id]
			if !ok {
				pr = &pair{keys: keys}
				index[id] = pr
				order = append(order, id)
			}
			if fromA {
				pr.a = item
			} else {
				pr.b = item
			}
		}
		for _, item := range as {
			add(item, true)
		}
		for _, item := range bs {
			add(item, false)
		}
		for _, id := range order {
			pr := index[id]
			np := p
			if len(pr.keys) > 0 && len(np.Segments) > 0 {
				last := np.Segments[len(np.Segments)-1]
				last.Keys = pr.keys
				np.Segments[len(np.Segments)-1] = last
			}
			diffWalk(np, pr.a, pr.b, out)
		}
		return
	}
	op := "replace"
	if a == nil {
		op = "create"
	} else if b == nil {
		op = "delete"
	}
	*out = append(*out, Change{Path: p.String(), Operation: op})
}

func graft(p Path, v any) map[string]any {
	if p.Module == "" {
		if m, ok := asMap(v); ok {
			return m
		}
		return map[string]any{}
	}
	if len(p.Segments) == 0 {
		return map[string]any{p.Module: v}
	}
	var cur any = map[string]any{}
	root := map[string]any{p.Module: cur}
	for i, seg := range p.Segments {
		last := i == len(p.Segments)-1
		m, _ := asMap(cur)
		if len(seg.Keys) > 0 {
			entry := entryWithKeys(seg.Keys)
			if last {
				if body, ok := asMap(v); ok {
					for k, val := range body {
						entry[k] = val
					}
				}
			}
			m[seg.Name] = []any{entry}
			if last {
				return root
			}
			cur = entry
			continue
		}
		if last {
			m[seg.Name] = v
			return root
		}
		next := map[string]any{}
		m[seg.Name] = next
		cur = next
	}
	return root
}
