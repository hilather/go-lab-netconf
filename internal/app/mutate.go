package app

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strconv"

	"github.com/hilather/go-lab-netconf/internal/compiler"
	"github.com/hilather/go-lab-netconf/internal/config"
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
	"github.com/hilather/go-lab-netconf/internal/snapshot"
)

type candidate struct {
	prev *snapshot.Snapshot
	next *snapshot.Snapshot
	ops  []ApplyOp
	diff []DiffEntry
}

// Plan dry-runs the mutation pipeline. expectedRevision is required.
func (s *App) Plan(ctx context.Context, ops []ApplyOp, expectedRev string) (Plan, error) {
	if err := s.requireCtx(ctx); err != nil {
		return Plan{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cand, err := s.buildCandidate(ops, expectedRev, true)
	if err != nil {
		return Plan{}, err
	}
	return s.planFrom(cand), nil
}

// Apply compiles the candidate and atomically swaps only after success.
func (s *App) Apply(ctx context.Context, ops []ApplyOp, expectedRev, idempotencyKey string) (ApplyResult, error) {
	if err := s.requireCtx(ctx); err != nil {
		return ApplyResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if idempotencyKey == "" {
		return ApplyResult{}, domainerr.ValidationFailed("Idempotency-Key is required",
			domainerr.FieldViolation{Path: "idempotencyKey", Code: "required", Message: "Idempotency-Key is required for apply"})
	}
	fp, err := fingerprintOps(ops)
	if err != nil {
		return ApplyResult{}, err
	}
	if hit, err := s.idemp.lookup(idempotencyKey, fp); err != nil {
		return ApplyResult{}, err
	} else if hit != nil && hit.apply != nil {
		return *cloneApply(hit.apply), nil
	}
	cand, err := s.buildCandidate(ops, expectedRev, true)
	if err != nil {
		s.forgetIdempOnConflict(idempotencyKey, err)
		return ApplyResult{}, err
	}
	s.snaps.Swap(cand.next)
	s.syncHandles(cand.prev, cand.next)
	res := ApplyResult{
		Plan:            s.planFrom(cand),
		Applied:         true,
		Generation:      cand.next.Generation,
		RuntimeRevision: cand.next.Revision,
	}
	s.idemp.storeApply(idempotencyKey, fp, &res)
	return *cloneApply(&res), nil
}

func (s *App) buildCandidate(ops []ApplyOp, expectedRev string, requireRev bool) (*candidate, error) {
	prev, err := s.active()
	if err != nil {
		return nil, err
	}
	if requireRev {
		if expectedRev == "" {
			return nil, domainerr.ValidationFailed("expectedRevision is required",
				domainerr.FieldViolation{Path: "expectedRevision", Code: "required", Message: "expectedRevision is required for plan and apply"})
		}
		if expectedRev != string(prev.Revision) {
			return nil, domainerr.RevisionMismatch("active revision does not match expectedRevision", string(prev.Revision)).
				WithRemediation("Re-read GET state and re-plan against the current revision.")
		}
	}
	copied, err := cloneState(prev.Canonical)
	if err != nil {
		return nil, err
	}
	if err := applyOperations(copied, ops); err != nil {
		return nil, err
	}
	if err := rejectResetOnly(prev.Canonical, copied); err != nil {
		return nil, err
	}
	next, err := compileCandidate(copied, prev)
	if err != nil {
		return nil, asDomain(err)
	}
	diff, err := diffStates(prev.Canonical, next.Canonical)
	if err != nil {
		return nil, err
	}
	return &candidate{
		prev: prev,
		next: next,
		ops:  append([]ApplyOp(nil), ops...),
		diff: diff,
	}, nil
}

func compileCandidate(st *model.State, prev *snapshot.Snapshot) (*snapshot.Snapshot, error) {
	gen := model.Generation(1)
	boot := model.Revision("")
	if prev != nil {
		gen = prev.Generation + 1
		boot = prev.BootstrapRevision
	}
	return compiler.Compile(st, compiler.CompileOpts{
		Generation:        gen,
		BootstrapRevision: boot,
	})
}

func (s *App) planFrom(c *candidate) Plan {
	prevRev := model.Revision("")
	if c.prev != nil {
		prevRev = c.prev.Revision
	}
	return Plan{
		PreviousRevision:  prevRev,
		CandidateRevision: c.next.Revision,
		Drifted:           c.next.Drifted(),
		Diff:              c.diff,
		Operations:        append([]ApplyOp(nil), c.ops...),
	}
}

func (s *App) forgetIdempOnConflict(key string, err error) {
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeRevisionMismatch {
		return
	}
	s.idemp.evict(key)
}

func cloneState(st *model.State) (*model.State, error) {
	if st == nil {
		return nil, domainerr.ValidationFailed("nil state")
	}
	b, err := json.Marshal(st)
	if err != nil {
		return nil, domainerr.ValidationFailed("clone: " + err.Error())
	}
	var out model.State
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, domainerr.ValidationFailed("clone: " + err.Error())
	}
	return &out, nil
}

func diffStates(before, after *model.State) ([]DiffEntry, error) {
	bt, err := jsonTree(before)
	if err != nil {
		return nil, err
	}
	at, err := jsonTree(after)
	if err != nil {
		return nil, err
	}
	var out []DiffEntry
	walkDiff(bt, at, "", &out)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func jsonTree(st *model.State) (any, error) {
	if st == nil {
		return map[string]any{}, nil
	}
	raw, err := config.CanonicalJSON(st)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	return tree, nil
}

func walkDiff(before, after any, path string, out *[]DiffEntry) {
	if bytes.Equal(mustJSON(before), mustJSON(after)) {
		return
	}
	bm, bMap := before.(map[string]any)
	am, aMap := after.(map[string]any)
	if bMap && aMap {
		keys := map[string]struct{}{}
		for k := range bm {
			keys[k] = struct{}{}
		}
		for k := range am {
			keys[k] = struct{}{}
		}
		ks := make([]string, 0, len(keys))
		for k := range keys {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			bv, bOk := bm[k]
			av, aOk := am[k]
			p := k
			if path != "" {
				p = path + "." + k
			}
			switch {
			case !bOk:
				*out = append(*out, DiffEntry{Path: p, Op: "add", After: mustJSON(av)})
			case !aOk:
				*out = append(*out, DiffEntry{Path: p, Op: "remove", Before: mustJSON(bv)})
			default:
				walkDiff(bv, av, p, out)
			}
		}
		return
	}
	bl, bArr := before.([]any)
	al, aArr := after.([]any)
	if bArr && aArr {
		n := len(bl)
		if len(al) > n {
			n = len(al)
		}
		for i := 0; i < n; i++ {
			p := path + "[" + strconv.Itoa(i) + "]"
			switch {
			case i >= len(bl):
				*out = append(*out, DiffEntry{Path: p, Op: "add", After: mustJSON(al[i])})
			case i >= len(al):
				*out = append(*out, DiffEntry{Path: p, Op: "remove", Before: mustJSON(bl[i])})
			default:
				walkDiff(bl[i], al[i], p, out)
			}
		}
		return
	}
	*out = append(*out, DiffEntry{Path: path, Op: "replace", Before: mustJSON(before), After: mustJSON(after)})
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}
