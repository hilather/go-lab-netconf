package ncserver

import (
	"context"
	"fmt"
	"io"

	"github.com/hilather/go-lab-netconf/internal/ncframing"
	"github.com/hilather/go-lab-netconf/internal/ncrpc"
)

type session struct {
	id         string
	user       User
	remoteAddr string
	rw         io.ReadWriteCloser
	cancel     context.CancelFunc
	server     *Server
	ver        ncframing.Version
	r          *ncframing.Reader
}

func (s *session) run(ctx context.Context) error {
	s.ver = ncframing.Version10
	s.r = ncframing.NewReader(s.rw, ncframing.Version10)
	hello, err := ncrpc.EncodeHello(ncrpc.Hello{
		Capabilities: ncrpc.AdvertisedCapabilities(),
		SessionID:    s.id,
	})
	if err != nil {
		return err
	}
	if err := ncframing.Write1_0(s.rw, hello); err != nil {
		return err
	}
	raw, err := s.r.ReadMessage()
	if err != nil {
		return err
	}
	msg, err := ncrpc.Decode(raw)
	if err != nil {
		return err
	}
	if msg.Hello == nil {
		return fmt.Errorf("ncserver: expected hello")
	}
	if hasCap(ncrpc.AdvertisedCapabilities(), ncrpc.CapBase11) && hasCap(msg.Hello.Capabilities, ncrpc.CapBase11) {
		s.ver = ncframing.Version11
		s.r.SetVersion(ncframing.Version11)
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		raw, err := s.r.ReadMessage()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		msg, err := ncrpc.Decode(raw)
		if err != nil {
			if werr := s.writeReply(ncrpc.Reply{
				MessageID: "0",
				Errors:    []ncrpc.Error{rpcErr("rpc", "malformed-message", err.Error())},
			}); werr != nil {
				return werr
			}
			continue
		}
		if msg.RPC == nil {
			if werr := s.writeReply(ncrpc.Reply{
				MessageID: "0",
				Errors:    []ncrpc.Error{rpcErr("rpc", "malformed-message", "expected rpc")},
			}); werr != nil {
				return werr
			}
			continue
		}
		reply := s.dispatch(ctx, *msg.RPC)
		s.server.observeRPC(msg.RPC.Name, len(reply.Errors) == 0)
		if err := s.writeReply(reply); err != nil {
			return err
		}
		if msg.RPC.Name == ncrpc.OpCloseSession {
			return nil
		}
	}
}

func (s *session) writeReply(r ncrpc.Reply) error {
	raw, err := ncrpc.EncodeReply(r)
	if err != nil {
		return err
	}
	return ncframing.Write(s.rw, s.ver, raw)
}

func hasCap(caps []string, want string) bool {
	for _, c := range caps {
		if c == want {
			return true
		}
	}
	return false
}
