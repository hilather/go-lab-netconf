package restconf

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hilather/go-lab-netconf/internal/datastore"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/yangtree"
)

func (s *Server) getData(w http.ResponseWriter, r *http.Request, h datastore.Handle, path string) {
	if err := requireJSONAccept(r); err != nil {
		writeError(w, err)
		return
	}
	n, err := h.Get(r.Context(), datastore.Running, datastore.Subtree{Path: path})
	if err != nil {
		writeError(w, err)
		return
	}
	var payload any
	if path == "" {
		payload = encodeRoot(n.Map())
	} else {
		v, ok := n.Lookup(path)
		if !ok {
			payload = map[string]any{}
		} else {
			payload = wrapResource(path, v)
		}
	}
	writeYangJSON(w, http.StatusOK, payload)
}

func (s *Server) writeData(w http.ResponseWriter, r *http.Request, h datastore.Handle, path string) {
	op := datastore.EditOp{Path: path}
	switch r.Method {
	case http.MethodPatch, http.MethodPost:
		op.Op = yangtree.OpMerge
	case http.MethodPut:
		op.Op = yangtree.OpReplace
	case http.MethodDelete:
		op.Op = yangtree.OpDelete
	}
	if r.Method != http.MethodDelete {
		if err := requireYangJSONContent(r); err != nil {
			writeError(w, err)
			return
		}
		val, err := readResourceBody(r, path)
		if err != nil {
			writeError(w, err)
			return
		}
		op.Value = val
	}
	if err := h.WriteRunningIfCandidateClean(r.Context(), op); err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusNoContent
	if r.Method == http.MethodPost {
		status = http.StatusCreated
	}
	w.WriteHeader(status)
}

func dataPath(escapedURLPath string) (string, error) {
	const prefix = "/restconf/data"
	if escapedURLPath != prefix && !strings.HasPrefix(escapedURLPath, prefix+"/") {
		return "", domainerr.NotFound("not a data resource")
	}
	rest := strings.Trim(strings.TrimPrefix(escapedURLPath, prefix), "/")
	if rest == "" {
		return "", nil
	}
	var b strings.Builder
	for i, seg := range strings.Split(rest, "/") {
		dec, err := url.PathUnescape(seg)
		if err != nil {
			return "", domainerr.ValidationFailed("invalid RESTCONF path")
		}
		if dec == "" {
			return "", domainerr.ValidationFailed("invalid RESTCONF path")
		}
		ident, keys := splitSegment(dec)
		if i == 0 {
			if !strings.Contains(ident, ":") {
				return "", domainerr.ValidationFailed("path must be module-qualified")
			}
			b.WriteString(ident)
		} else {
			b.WriteByte('/')
			b.WriteString(ident)
		}
		for _, k := range keys {
			b.WriteByte('[')
			b.WriteString(k.Name)
			b.WriteByte('=')
			b.WriteString(k.Value)
			b.WriteByte(']')
		}
	}
	p, err := yangtree.ParsePath(b.String())
	if err != nil {
		return "", err
	}
	return p.String(), nil
}

func splitSegment(seg string) (string, []yangtree.Key) {
	if i := strings.IndexByte(seg, '['); i >= 0 {
		return seg[:i], parseBracketKeys(seg[i:])
	}
	eq := strings.IndexByte(seg, '=')
	if eq < 0 {
		return seg, nil
	}
	name := seg[:eq]
	vals := strings.Split(seg[eq+1:], ",")
	keys := make([]yangtree.Key, 0, len(vals))
	if len(vals) == 1 {
		keys = append(keys, yangtree.Key{Name: "name", Value: vals[0]})
		return name, keys
	}
	for i, v := range vals {
		keys = append(keys, yangtree.Key{Name: "name" + strconv.Itoa(i+1), Value: v})
	}
	return name, keys
}

func parseBracketKeys(s string) []yangtree.Key {
	var keys []yangtree.Key
	for s != "" && s[0] == '[' {
		end := strings.IndexByte(s, ']')
		if end < 0 {
			return keys
		}
		inner := s[1:end]
		k, v, ok := strings.Cut(inner, "=")
		if ok {
			v = strings.Trim(v, `"'`)
			keys = append(keys, yangtree.Key{Name: k, Value: v})
		}
		s = s[end+1:]
	}
	return keys
}

func encodeRoot(m map[string]any) map[string]any {
	out := map[string]any{}
	for mod, children := range m {
		cm, ok := children.(map[string]any)
		if !ok {
			out[mod] = children
			continue
		}
		for name, val := range cm {
			out[mod+":"+name] = val
		}
	}
	return out
}

func decodeRoot(m map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		mod, name, ok := strings.Cut(k, ":")
		if !ok {
			out[k] = v
			continue
		}
		parent, _ := out[mod].(map[string]any)
		if parent == nil {
			parent = map[string]any{}
			out[mod] = parent
		}
		parent[name] = v
	}
	return out
}

func wrapResource(path string, value any) map[string]any {
	p, err := yangtree.ParsePath(path)
	if err != nil {
		return map[string]any{}
	}
	key := p.Module
	if len(p.Segments) > 0 {
		key = p.Module + ":" + p.Segments[len(p.Segments)-1].Name
	}
	return map[string]any{key: value}
}

func readResourceBody(r *http.Request, path string) (any, error) {
	limited := io.LimitReader(r.Body, maxBodyBytes+1)
	b, err := io.ReadAll(limited)
	if err != nil {
		return nil, domainerr.ValidationFailed("failed to read request body")
	}
	if len(b) > maxBodyBytes {
		return nil, domainerr.ValidationFailed("request body too large")
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return nil, domainerr.ValidationFailed("JSON object required")
	}
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, domainerr.ValidationFailed("invalid JSON")
	}
	return unwrapResource(path, raw)
}

func unwrapResource(path string, raw any) (any, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, domainerr.ValidationFailed("JSON object required")
	}
	if path == "" {
		return decodeRoot(m), nil
	}
	p, err := yangtree.ParsePath(path)
	if err != nil {
		return nil, err
	}
	for _, k := range resourceKeys(p) {
		if v, ok := m[k]; ok {
			return v, nil
		}
	}
	if len(m) == 1 {
		for _, v := range m {
			return v, nil
		}
	}
	return nil, domainerr.ValidationFailed("request body does not match the target resource")
}

func resourceKeys(p yangtree.Path) []string {
	if len(p.Segments) == 0 {
		return []string{p.Module}
	}
	last := p.Segments[len(p.Segments)-1].Name
	return []string{p.Module + ":" + last, last}
}
