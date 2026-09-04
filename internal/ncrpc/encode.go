package ncrpc

import (
	"bytes"
	"encoding/xml"
	"fmt"
)

// Encode serializes hello, rpc, or rpc-reply. Exactly one field of m must be set.
func Encode(m Message) ([]byte, error) {
	n := 0
	if m.Hello != nil {
		n++
	}
	if m.RPC != nil {
		n++
	}
	if m.Reply != nil {
		n++
	}
	if n != 1 {
		return nil, fmt.Errorf("ncrpc: message must be hello, rpc, or rpc-reply")
	}
	switch {
	case m.Hello != nil:
		return EncodeHello(*m.Hello)
	case m.RPC != nil:
		return EncodeRPC(*m.RPC)
	default:
		return EncodeReply(*m.Reply)
	}
}

// EncodeHello writes a <hello> document.
func EncodeHello(h Hello) ([]byte, error) {
	type helloXML struct {
		XMLName      xml.Name `xml:"urn:ietf:params:xml:ns:netconf:base:1.0 hello"`
		Capabilities []string `xml:"capabilities>capability"`
		SessionID    string   `xml:"session-id,omitempty"`
	}
	return xml.Marshal(helloXML{
		Capabilities: h.Capabilities,
		SessionID:    h.SessionID,
	})
}

// EncodeRPC writes a <rpc> document. MessageID is required.
func EncodeRPC(r RPC) ([]byte, error) {
	if r.MessageID == "" {
		return nil, fmt.Errorf("ncrpc: missing message-id")
	}
	if r.Name == "" {
		return nil, fmt.Errorf("ncrpc: missing rpc operation")
	}
	inner, err := encodeOp(r)
	if err != nil {
		return nil, err
	}
	type rpcXML struct {
		XMLName   xml.Name `xml:"urn:ietf:params:xml:ns:netconf:base:1.0 rpc"`
		MessageID string   `xml:"message-id,attr"`
		Inner     []byte   `xml:",innerxml"`
	}
	return xml.Marshal(rpcXML{MessageID: r.MessageID, Inner: inner})
}

// EncodeReply writes a <rpc-reply> document. MessageID is required.
func EncodeReply(r Reply) ([]byte, error) {
	if r.MessageID == "" {
		return nil, fmt.Errorf("ncrpc: missing message-id")
	}
	var inner bytes.Buffer
	switch {
	case len(r.Errors) > 0:
		for _, e := range r.Errors {
			encodeError(&inner, e)
		}
	case len(r.Data) > 0:
		inner.WriteString("<data>")
		inner.Write(r.Data)
		inner.WriteString("</data>")
	default:
		inner.WriteString("<ok/>")
	}
	type replyXML struct {
		XMLName   xml.Name `xml:"urn:ietf:params:xml:ns:netconf:base:1.0 rpc-reply"`
		MessageID string   `xml:"message-id,attr"`
		Inner     []byte   `xml:",innerxml"`
	}
	return xml.Marshal(replyXML{MessageID: r.MessageID, Inner: inner.Bytes()})
}

func encodeOp(r RPC) ([]byte, error) {
	xmlns := ""
	if r.Name == OpCreateSubscription {
		xmlns = NotificationNamespace
	}
	var body bytes.Buffer
	switch r.Name {
	case OpCopyConfig:
		encodeDatastore(&body, "target", r.Target)
		encodeDatastore(&body, "source", r.Source)
	case OpEditConfig:
		encodeDatastore(&body, "target", r.Target)
		if r.DefaultOp != "" {
			encodeLeaf(&body, "default-operation", r.DefaultOp)
		}
		encodeFilter(&body, r.Filter)
		encodeConfig(&body, r.Config)
	case OpGet:
		encodeFilter(&body, r.Filter)
	case OpGetConfig, OpValidate:
		encodeDatastore(&body, "source", r.Source)
		encodeFilter(&body, r.Filter)
	case OpDeleteConfig, OpLock, OpUnlock:
		encodeDatastore(&body, "target", r.Target)
	case OpKillSession:
		if r.SessionID != "" {
			encodeLeaf(&body, "session-id", r.SessionID)
		}
	case OpCreateSubscription:
		if r.Stream != "" {
			encodeLeaf(&body, "stream", r.Stream)
		}
		encodeFilter(&body, r.Filter)
	default:
		encodeDatastore(&body, "source", r.Source)
		encodeDatastore(&body, "target", r.Target)
		if r.DefaultOp != "" {
			encodeLeaf(&body, "default-operation", r.DefaultOp)
		}
		encodeFilter(&body, r.Filter)
		encodeConfig(&body, r.Config)
		if r.SessionID != "" {
			encodeLeaf(&body, "session-id", r.SessionID)
		}
		if r.Stream != "" {
			encodeLeaf(&body, "stream", r.Stream)
		}
	}
	return encodeContainer(r.Name, xmlns, body.Bytes()), nil
}

func encodeContainer(name, xmlns string, inner []byte) []byte {
	var b bytes.Buffer
	b.WriteByte('<')
	b.WriteString(name)
	if xmlns != "" {
		b.WriteString(` xmlns="`)
		b.WriteString(xmlns)
		b.WriteByte('"')
	}
	if len(inner) == 0 {
		b.WriteString("/>")
		return b.Bytes()
	}
	b.WriteByte('>')
	b.Write(inner)
	b.WriteString("</")
	b.WriteString(name)
	b.WriteByte('>')
	return b.Bytes()
}

func encodeDatastore(b *bytes.Buffer, tag, store string) {
	if store == "" {
		return
	}
	b.WriteByte('<')
	b.WriteString(tag)
	b.WriteString("><")
	b.WriteString(store)
	b.WriteString("/></")
	b.WriteString(tag)
	b.WriteByte('>')
}

func encodeFilter(b *bytes.Buffer, f *Filter) {
	if f == nil {
		return
	}
	typ := f.Type
	if typ == "" {
		typ = FilterSubtree
	}
	b.WriteString(`<filter type="`)
	escape(b, typ)
	b.WriteByte('"')
	if f.Select != "" {
		b.WriteString(` select="`)
		escape(b, f.Select)
		b.WriteByte('"')
	}
	if len(f.Inner) == 0 {
		b.WriteString("/>")
		return
	}
	b.WriteByte('>')
	b.Write(f.Inner)
	b.WriteString("</filter>")
}

func encodeConfig(b *bytes.Buffer, config []byte) {
	if len(config) == 0 {
		return
	}
	b.WriteString("<config>")
	b.Write(config)
	b.WriteString("</config>")
}

func encodeLeaf(b *bytes.Buffer, name, value string) {
	b.WriteByte('<')
	b.WriteString(name)
	b.WriteByte('>')
	escape(b, value)
	b.WriteString("</")
	b.WriteString(name)
	b.WriteByte('>')
}

func encodeError(b *bytes.Buffer, e Error) {
	b.WriteString("<rpc-error>")
	if e.Type != "" {
		encodeLeaf(b, "error-type", e.Type)
	}
	if e.Tag != "" {
		encodeLeaf(b, "error-tag", e.Tag)
	}
	if e.Severity != "" {
		encodeLeaf(b, "error-severity", e.Severity)
	}
	if e.AppTag != "" {
		encodeLeaf(b, "error-app-tag", e.AppTag)
	}
	if e.Path != "" {
		encodeLeaf(b, "error-path", e.Path)
	}
	if e.Message != "" {
		encodeLeaf(b, "error-message", e.Message)
	}
	b.WriteString("</rpc-error>")
}

func escape(b *bytes.Buffer, s string) {
	_ = xml.EscapeText(b, []byte(s))
}
