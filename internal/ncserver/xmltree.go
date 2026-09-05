package ncserver

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hilather/go-lab-netconf/internal/yangtree"
)

func xmlnsFor(module string, ns map[string]string) string {
	if ns != nil {
		if v := ns[module]; v != "" {
			return v
		}
	}
	return "urn:ietf:params:xml:ns:yang:" + module
}

func moduleForNS(space string, ns map[string]string) string {
	if space == "" {
		return ""
	}
	for mod, uri := range ns {
		if uri == space {
			return mod
		}
	}
	const prefix = "urn:ietf:params:xml:ns:yang:"
	if strings.HasPrefix(space, prefix) {
		return strings.TrimPrefix(space, prefix)
	}
	return ""
}

func encodeTree(m map[string]any, ns map[string]string) []byte {
	var b bytes.Buffer
	mods := sortedKeys(m)
	for _, mod := range mods {
		body, ok := m[mod].(map[string]any)
		if !ok {
			continue
		}
		xmlns := xmlnsFor(mod, ns)
		for _, k := range sortedKeys(body) {
			encodeElement(&b, k, body[k], xmlns, true)
		}
	}
	out := b.Bytes()
	if out == nil {
		out = []byte{}
	}
	return out
}

func encodeElement(b *bytes.Buffer, name string, v any, ns string, withNS bool) {
	switch x := v.(type) {
	case map[string]any:
		openElement(b, name, ns, withNS)
		if len(x) == 0 {
			b.WriteString("/>")
			return
		}
		b.WriteByte('>')
		for _, k := range sortedKeys(x) {
			encodeElement(b, k, x[k], ns, false)
		}
		closeElement(b, name)
	case []any:
		for i, item := range x {
			encodeElement(b, name, item, ns, withNS && i == 0)
		}
	default:
		openElement(b, name, ns, withNS)
		if v == nil {
			b.WriteString("/>")
			return
		}
		b.WriteByte('>')
		_ = xml.EscapeText(b, []byte(formatScalar(v)))
		closeElement(b, name)
	}
}

func openElement(b *bytes.Buffer, name, ns string, withNS bool) {
	b.WriteByte('<')
	b.WriteString(name)
	if withNS && ns != "" {
		b.WriteString(` xmlns="`)
		_ = xml.EscapeText(b, []byte(ns))
		b.WriteByte('"')
	}
}

func closeElement(b *bytes.Buffer, name string) {
	b.WriteString("</")
	b.WriteString(name)
	b.WriteByte('>')
}

func formatScalar(v any) string {
	switch x := v.(type) {
	case bool:
		if x {
			return "true"
		}
		return "false"
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	case uint32:
		return strconv.FormatUint(uint64(x), 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'g', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

type xmlNode struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Text    string     `xml:",chardata"`
	Nodes   []xmlNode  `xml:",any"`
}

func decodeForest(inner []byte) ([]xmlNode, error) {
	inner = bytes.TrimSpace(inner)
	if len(inner) == 0 {
		return nil, nil
	}
	wrapped := append([]byte("<x>"), inner...)
	wrapped = append(wrapped, "</x>"...)
	dec := xml.NewDecoder(bytes.NewReader(wrapped))
	var root xmlNode
	if err := dec.Decode(&root); err != nil {
		return nil, err
	}
	return root.Nodes, nil
}

func decodeConfig(inner []byte, ns map[string]string) (map[string]any, error) {
	nodes, err := decodeForest(inner)
	if err != nil {
		return nil, err
	}
	root := map[string]any{}
	for _, n := range nodes {
		mod := moduleForNS(n.XMLName.Space, ns)
		if mod == "" {
			return nil, fmt.Errorf("unknown-element: namespace %q", n.XMLName.Space)
		}
		body, _ := root[mod].(map[string]any)
		if body == nil {
			body = map[string]any{}
			root[mod] = body
		}
		putChild(body, n.XMLName.Local, nodeValue(n))
	}
	return root, nil
}

func putChild(m map[string]any, name string, v any) {
	if existing, ok := m[name]; ok {
		switch e := existing.(type) {
		case []any:
			m[name] = append(e, v)
		default:
			m[name] = []any{e, v}
		}
		return
	}
	m[name] = v
}

func nodeValue(n xmlNode) any {
	text := strings.TrimSpace(n.Text)
	if len(n.Nodes) == 0 {
		if text == "" {
			return map[string]any{}
		}
		return parseScalar(text)
	}
	m := map[string]any{}
	for _, c := range n.Nodes {
		putChild(m, c.XMLName.Local, nodeValue(c))
	}
	return m
}

func parseScalar(s string) any {
	switch s {
	case "true":
		return true
	case "false":
		return false
	default:
		return s
	}
}

func filterPath(inner []byte, ns map[string]string) (string, error) {
	nodes, err := decodeForest(inner)
	if err != nil {
		return "", err
	}
	if len(nodes) == 0 {
		return "", nil
	}
	n := nodes[0]
	mod := moduleForNS(n.XMLName.Space, ns)
	if mod == "" {
		return "", fmt.Errorf("unknown-element")
	}
	p := yangtree.Path{Module: mod, Segments: []yangtree.Segment{{Name: n.XMLName.Local}}}
	cur := n
	for {
		var next *xmlNode
		for i := range cur.Nodes {
			c := &cur.Nodes[i]
			if c.XMLName.Local == "" {
				continue
			}
			if strings.TrimSpace(c.Text) != "" && len(c.Nodes) == 0 {
				last := p.Segments[len(p.Segments)-1]
				last.Keys = append(last.Keys, yangtree.Key{Name: c.XMLName.Local, Value: strings.TrimSpace(c.Text)})
				p.Segments[len(p.Segments)-1] = last
				continue
			}
			if next == nil {
				next = c
			}
		}
		if next == nil {
			break
		}
		p.Segments = append(p.Segments, yangtree.Segment{Name: next.XMLName.Local})
		cur = *next
	}
	return p.String(), nil
}
