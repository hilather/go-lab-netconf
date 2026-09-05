package nctest

import (
	"fmt"
	"io"
	"sync"

	"github.com/hilather/go-lab-netconf/internal/ncframing"
	"github.com/hilather/go-lab-netconf/internal/ncrpc"
)

// Client is a NETCONF peer over an already-opened stream (typically
// SSH subsystem netconf). It does not Dial and does not import ssh.
type Client struct {
	mu  sync.Mutex
	rw  io.ReadWriteCloser
	r   *ncframing.Reader
	ver ncframing.Version
}

// New wraps rw. Hello is 1.0-framed until Handshake sees base:1.1 on both sides.
func New(rw io.ReadWriteCloser) *Client {
	return &Client{
		rw:  rw,
		r:   ncframing.NewReader(rw, ncframing.Version10),
		ver: ncframing.Version10,
	}
}

// Handshake writes a client hello and reads the server hello.
func (c *Client) Handshake(caps []string) (ncrpc.Hello, error) {
	if len(caps) == 0 {
		caps = []string{ncrpc.CapBase10, ncrpc.CapBase11}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	msg, err := c.r.ReadMessage()
	if err != nil {
		return ncrpc.Hello{}, err
	}
	decoded, err := ncrpc.Decode(msg)
	if err != nil {
		return ncrpc.Hello{}, err
	}
	if decoded.Hello == nil {
		return ncrpc.Hello{}, fmt.Errorf("nctest: expected hello")
	}
	raw, err := ncrpc.EncodeHello(ncrpc.Hello{Capabilities: caps})
	if err != nil {
		return ncrpc.Hello{}, err
	}
	if err := ncframing.Write1_0(c.rw, raw); err != nil {
		return ncrpc.Hello{}, err
	}
	if hasCap(caps, ncrpc.CapBase11) && hasCap(decoded.Hello.Capabilities, ncrpc.CapBase11) {
		c.ver = ncframing.Version11
		c.r.SetVersion(ncframing.Version11)
	}
	return *decoded.Hello, nil
}

// RPC sends one request and returns the matching reply.
func (c *Client) RPC(r ncrpc.RPC) (ncrpc.Reply, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	raw, err := ncrpc.EncodeRPC(r)
	if err != nil {
		return ncrpc.Reply{}, err
	}
	if err := ncframing.Write(c.rw, c.ver, raw); err != nil {
		return ncrpc.Reply{}, err
	}
	msg, err := c.r.ReadMessage()
	if err != nil {
		return ncrpc.Reply{}, err
	}
	decoded, err := ncrpc.Decode(msg)
	if err != nil {
		return ncrpc.Reply{}, err
	}
	if decoded.Reply == nil {
		return ncrpc.Reply{}, fmt.Errorf("nctest: expected rpc-reply, got %#v", decoded)
	}
	return *decoded.Reply, nil
}

// Close sends close-session when possible and closes the stream.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	raw, err := ncrpc.EncodeRPC(ncrpc.RPC{MessageID: "close", Name: ncrpc.OpCloseSession})
	if err == nil {
		_ = ncframing.Write(c.rw, c.ver, raw)
		_, _ = c.r.ReadMessage()
	}
	return c.rw.Close()
}

func hasCap(caps []string, want string) bool {
	for _, c := range caps {
		if c == want {
			return true
		}
	}
	return false
}
