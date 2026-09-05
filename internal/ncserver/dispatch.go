package ncserver

import (
	"context"

	"github.com/hilather/go-lab-netconf/internal/datastore"
	"github.com/hilather/go-lab-netconf/internal/model"
	"github.com/hilather/go-lab-netconf/internal/ncrpc"
	"github.com/hilather/go-lab-netconf/internal/yangtree"
)

func (s *session) dispatch(ctx context.Context, rpc ncrpc.RPC) ncrpc.Reply {
	id := rpc.MessageID
	if id == "" {
		return errReply("0", rpcErr("rpc", "missing-attribute", "message-id is required"))
	}
	if s.user.Access != model.UserAccessReadWrite && mutating(rpc.Name) {
		return errReply(id, rpcErr("protocol", "access-denied", "user is read-only"))
	}
	ctx = datastore.WithSessionID(ctx, s.id)
	switch rpc.Name {
	case ncrpc.OpGet:
		return s.doGet(ctx, rpc, datastore.Running)
	case ncrpc.OpGetConfig:
		store, rpcErrv := storeName(rpc.Source, true)
		if rpcErrv.Tag != "" {
			return errReply(id, rpcErrv)
		}
		return s.doGet(ctx, rpc, store)
	case ncrpc.OpEditConfig:
		return s.doEdit(ctx, rpc)
	case ncrpc.OpCopyConfig:
		return s.doCopy(ctx, rpc)
	case ncrpc.OpDeleteConfig:
		return s.doDelete(ctx, rpc)
	case ncrpc.OpLock:
		return s.doLock(ctx, rpc, true)
	case ncrpc.OpUnlock:
		return s.doLock(ctx, rpc, false)
	case ncrpc.OpCommit:
		if err := s.user.Handle.Commit(ctx); err != nil {
			return errReply(id, mapErr(err))
		}
		return okReply(id)
	case ncrpc.OpDiscardChanges:
		if err := s.user.Handle.Discard(ctx); err != nil {
			return errReply(id, mapErr(err))
		}
		return okReply(id)
	case ncrpc.OpValidate:
		return s.doValidate(ctx, rpc)
	case ncrpc.OpCloseSession:
		return okReply(id)
	case ncrpc.OpKillSession:
		return s.doKill(rpc)
	case ncrpc.OpCreateSubscription:
		return s.doSubscribe(rpc)
	default:
		return errReply(id, rpcErr("protocol", "operation-not-supported", "unknown rpc "+rpc.Name))
	}
}

func mutating(op string) bool {
	switch op {
	case ncrpc.OpEditConfig, ncrpc.OpCopyConfig, ncrpc.OpDeleteConfig,
		ncrpc.OpLock, ncrpc.OpUnlock, ncrpc.OpCommit, ncrpc.OpDiscardChanges,
		ncrpc.OpKillSession:
		return true
	default:
		return false
	}
}

func storeName(name string, required bool) (datastore.Name, ncrpc.Error) {
	if name == "" {
		if required {
			return "", rpcErr("protocol", "missing-element", "datastore is required")
		}
		return datastore.Running, ncrpc.Error{}
	}
	if name == "url" {
		return "", rpcErr("protocol", "operation-not-supported", "url capability is not advertised")
	}
	switch datastore.Name(name) {
	case datastore.Running, datastore.Candidate, datastore.Startup:
		return datastore.Name(name), ncrpc.Error{}
	default:
		return "", rpcErr("protocol", "unknown-element", "unknown datastore")
	}
}

func (s *session) doGet(ctx context.Context, rpc ncrpc.RPC, store datastore.Name) ncrpc.Reply {
	if rpc.Filter != nil && rpc.Filter.Type == ncrpc.FilterXPath {
		return errReply(rpc.MessageID, rpcErr("protocol", "operation-not-supported", "xpath is not supported"))
	}
	path := ""
	if rpc.Filter != nil && len(bytesTrim(rpc.Filter.Inner)) > 0 {
		p, err := filterPath(rpc.Filter.Inner, s.user.Namespaces)
		if err != nil {
			return errReply(rpc.MessageID, rpcErr("application", "unknown-element", err.Error()))
		}
		path = p
	}
	n, err := s.user.Handle.Get(ctx, store, datastore.Subtree{Path: path})
	if err != nil {
		return errReply(rpc.MessageID, mapErr(err))
	}
	data := encodeTree(n.Map(), s.user.Namespaces)
	return ncrpc.Reply{MessageID: rpc.MessageID, Data: data}
}

func (s *session) doEdit(ctx context.Context, rpc ncrpc.RPC) ncrpc.Reply {
	if rpc.Target != ncrpc.StoreCandidate {
		return errReply(rpc.MessageID, rpcErr("protocol", "operation-not-supported", "edit-config target must be candidate"))
	}
	var op yangtree.Operation
	switch rpc.DefaultOp {
	case "", "merge":
		op = yangtree.OpMerge
	case "replace":
		op = yangtree.OpReplace
	default:
		return errReply(rpc.MessageID, rpcErr("protocol", "invalid-value", "unsupported default-operation"))
	}
	tree, err := decodeConfig(rpc.Config, s.user.Namespaces)
	if err != nil {
		return errReply(rpc.MessageID, rpcErr("application", "unknown-element", err.Error()))
	}
	if err := s.user.Handle.Edit(ctx, datastore.Candidate, datastore.EditOp{Op: op, Path: "", Value: tree}); err != nil {
		return errReply(rpc.MessageID, mapErr(err))
	}
	return okReply(rpc.MessageID)
}

func (s *session) doCopy(ctx context.Context, rpc ncrpc.RPC) ncrpc.Reply {
	src, err := storeName(rpc.Source, true)
	if err.Tag != "" {
		return errReply(rpc.MessageID, err)
	}
	dst, err := storeName(rpc.Target, true)
	if err.Tag != "" {
		return errReply(rpc.MessageID, err)
	}
	if e := s.user.Handle.Copy(ctx, src, dst); e != nil {
		return errReply(rpc.MessageID, mapErr(e))
	}
	return okReply(rpc.MessageID)
}

func (s *session) doDelete(ctx context.Context, rpc ncrpc.RPC) ncrpc.Reply {
	store, err := storeName(rpc.Target, true)
	if err.Tag != "" {
		return errReply(rpc.MessageID, err)
	}
	if e := s.user.Handle.Delete(ctx, store); e != nil {
		return errReply(rpc.MessageID, mapErr(e))
	}
	return okReply(rpc.MessageID)
}

func (s *session) doLock(ctx context.Context, rpc ncrpc.RPC, take bool) ncrpc.Reply {
	store, err := storeName(rpc.Target, true)
	if err.Tag != "" {
		return errReply(rpc.MessageID, err)
	}
	var e error
	if take {
		e = s.user.Handle.Lock(ctx, store, s.id)
	} else {
		e = s.user.Handle.Unlock(ctx, store, s.id)
	}
	if e != nil {
		return errReply(rpc.MessageID, mapErr(e))
	}
	return okReply(rpc.MessageID)
}

func (s *session) doValidate(ctx context.Context, rpc ncrpc.RPC) ncrpc.Reply {
	store := datastore.Candidate
	if rpc.Source != "" {
		var err ncrpc.Error
		store, err = storeName(rpc.Source, true)
		if err.Tag != "" {
			return errReply(rpc.MessageID, err)
		}
	}
	n, e := s.user.Handle.Get(ctx, store, datastore.Subtree{})
	if e != nil {
		return errReply(rpc.MessageID, mapErr(e))
	}
	if e := n.Validate(); e != nil {
		return errReply(rpc.MessageID, mapErr(e))
	}
	return okReply(rpc.MessageID)
}

func (s *session) doKill(rpc ncrpc.RPC) ncrpc.Reply {
	if rpc.SessionID == "" {
		return errReply(rpc.MessageID, rpcErr("protocol", "missing-element", "session-id is required"))
	}
	if rpc.SessionID == s.id {
		s.cancel()
		return okReply(rpc.MessageID)
	}
	if err := s.server.Kill(rpc.SessionID); err != nil {
		return errReply(rpc.MessageID, rpcErr("application", "invalid-value", "no such session"))
	}
	return okReply(rpc.MessageID)
}

func (s *session) doSubscribe(rpc ncrpc.RPC) ncrpc.Reply {
	stream := rpc.Stream
	if stream == "" {
		stream = "NETCONF"
	}
	if stream != "NETCONF" {
		return errReply(rpc.MessageID, rpcErr("application", "invalid-value", "unknown stream"))
	}
	return okReply(rpc.MessageID)
}

func bytesTrim(b []byte) []byte {
	i, j := 0, len(b)
	for i < j && (b[i] == ' ' || b[i] == '\n' || b[i] == '\r' || b[i] == '\t') {
		i++
	}
	for j > i && (b[j-1] == ' ' || b[j-1] == '\n' || b[j-1] == '\r' || b[j-1] == '\t') {
		j--
	}
	return b[i:j]
}
