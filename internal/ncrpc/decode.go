package ncrpc

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
)

// Decode reads a hello, rpc, or rpc-reply document.
func Decode(msg []byte) (Message, error) {
	dec := xml.NewDecoder(bytes.NewReader(msg))
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return Message{}, fmt.Errorf("ncrpc: missing root element")
			}
			return Message{}, fmt.Errorf("ncrpc: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "hello":
			h, err := decodeHello(dec, se)
			if err != nil {
				return Message{}, err
			}
			return Message{Hello: &h}, nil
		case "rpc":
			r, err := decodeRPC(dec, se)
			if err != nil {
				return Message{}, err
			}
			return Message{RPC: &r}, nil
		case "rpc-reply":
			r, err := decodeReply(dec, se)
			if err != nil {
				return Message{}, err
			}
			return Message{Reply: &r}, nil
		default:
			return Message{}, fmt.Errorf("ncrpc: unexpected element %q", se.Name.Local)
		}
	}
}

func decodeHello(dec *xml.Decoder, start xml.StartElement) (Hello, error) {
	var h Hello
	for {
		tok, err := dec.Token()
		if err != nil {
			return Hello{}, unexpectedEOF(err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "capabilities":
				caps, err := decodeCaps(dec, t)
				if err != nil {
					return Hello{}, err
				}
				h.Capabilities = caps
			case "session-id":
				s, err := decodeText(dec, t)
				if err != nil {
					return Hello{}, err
				}
				h.SessionID = s
			default:
				if err := dec.Skip(); err != nil {
					return Hello{}, err
				}
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return h, nil
			}
		}
	}
}

func decodeCaps(dec *xml.Decoder, start xml.StartElement) ([]string, error) {
	var caps []string
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, unexpectedEOF(err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "capability" {
				s, err := decodeText(dec, t)
				if err != nil {
					return nil, err
				}
				caps = append(caps, s)
			} else if err := dec.Skip(); err != nil {
				return nil, err
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return caps, nil
			}
		}
	}
}

func decodeRPC(dec *xml.Decoder, start xml.StartElement) (RPC, error) {
	var r RPC
	for _, a := range start.Attr {
		if a.Name.Local == "message-id" {
			r.MessageID = a.Value
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return RPC{}, unexpectedEOF(err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if r.Name == "" {
				if err := decodeOp(dec, t, &r); err != nil {
					return RPC{}, err
				}
			} else if err := dec.Skip(); err != nil {
				return RPC{}, err
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				if r.Name == "" {
					return RPC{}, fmt.Errorf("ncrpc: missing rpc operation")
				}
				return r, nil
			}
		}
	}
}

func decodeOp(dec *xml.Decoder, start xml.StartElement, r *RPC) error {
	r.Name = start.Name.Local
	r.Namespace = start.Name.Space
	for {
		tok, err := dec.Token()
		if err != nil {
			return unexpectedEOF(err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "source":
				name, err := decodeDatastore(dec, t)
				if err != nil {
					return err
				}
				r.Source = name
			case "target":
				name, err := decodeDatastore(dec, t)
				if err != nil {
					return err
				}
				r.Target = name
			case "filter":
				f, err := decodeFilter(dec, t)
				if err != nil {
					return err
				}
				r.Filter = &f
			case "default-operation":
				s, err := decodeText(dec, t)
				if err != nil {
					return err
				}
				r.DefaultOp = s
			case "config":
				inner, err := decodeInner(dec, t)
				if err != nil {
					return err
				}
				r.Config = inner
			case "session-id":
				s, err := decodeText(dec, t)
				if err != nil {
					return err
				}
				r.SessionID = s
			case "stream":
				s, err := decodeText(dec, t)
				if err != nil {
					return err
				}
				r.Stream = s
			default:
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

func decodeDatastore(dec *xml.Decoder, start xml.StartElement) (string, error) {
	name := ""
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", unexpectedEOF(err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case StoreRunning, StoreCandidate, StoreStartup:
				name = t.Name.Local
				if err := dec.Skip(); err != nil {
					return "", err
				}
			case "url":
				name = "url"
				if err := dec.Skip(); err != nil {
					return "", err
				}
			default:
				if err := dec.Skip(); err != nil {
					return "", err
				}
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return name, nil
			}
		}
	}
}

func decodeFilter(dec *xml.Decoder, start xml.StartElement) (Filter, error) {
	var x struct {
		Type   string `xml:"type,attr"`
		Select string `xml:"select,attr"`
		Inner  []byte `xml:",innerxml"`
	}
	if err := dec.DecodeElement(&x, &start); err != nil {
		return Filter{}, err
	}
	typ := x.Type
	if typ == "" {
		typ = FilterSubtree
	}
	return Filter{Type: typ, Select: x.Select, Inner: x.Inner}, nil
}

func decodeReply(dec *xml.Decoder, start xml.StartElement) (Reply, error) {
	var r Reply
	for _, a := range start.Attr {
		if a.Name.Local == "message-id" {
			r.MessageID = a.Value
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return Reply{}, unexpectedEOF(err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "ok":
				r.OK = true
				if err := dec.Skip(); err != nil {
					return Reply{}, err
				}
			case "data":
				inner, err := decodeInner(dec, t)
				if err != nil {
					return Reply{}, err
				}
				r.Data = inner
			case "rpc-error":
				e, err := decodeError(dec, t)
				if err != nil {
					return Reply{}, err
				}
				r.Errors = append(r.Errors, e)
			default:
				if err := dec.Skip(); err != nil {
					return Reply{}, err
				}
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return r, nil
			}
		}
	}
}

func decodeError(dec *xml.Decoder, start xml.StartElement) (Error, error) {
	var x struct {
		Type     string `xml:"error-type"`
		Tag      string `xml:"error-tag"`
		Severity string `xml:"error-severity"`
		AppTag   string `xml:"error-app-tag"`
		Path     string `xml:"error-path"`
		Message  string `xml:"error-message"`
	}
	if err := dec.DecodeElement(&x, &start); err != nil {
		return Error{}, err
	}
	return Error{
		Type:     x.Type,
		Tag:      x.Tag,
		Severity: x.Severity,
		AppTag:   x.AppTag,
		Path:     x.Path,
		Message:  x.Message,
	}, nil
}

func decodeInner(dec *xml.Decoder, start xml.StartElement) ([]byte, error) {
	var x struct {
		Inner []byte `xml:",innerxml"`
	}
	if err := dec.DecodeElement(&x, &start); err != nil {
		return nil, err
	}
	return x.Inner, nil
}

func decodeText(dec *xml.Decoder, start xml.StartElement) (string, error) {
	var s string
	if err := dec.DecodeElement(&s, &start); err != nil {
		return "", err
	}
	return s, nil
}

func unexpectedEOF(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}
