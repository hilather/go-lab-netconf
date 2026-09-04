package ncframing

import (
	"bufio"
	"fmt"
	"io"
)

// Reader decodes one NETCONF message per ReadMessage call.
type Reader struct {
	br  *bufio.Reader
	ver Version
	max int
}

// NewReader returns a decoder for ver. Hello is 1.0-framed; call SetVersion
// after both peers advertise base:1.1 so buffered bytes stay on this reader.
func NewReader(r io.Reader, ver Version) *Reader {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	return &Reader{br: br, ver: ver, max: MaxMessageSize}
}

// SetVersion selects the framing used by subsequent ReadMessage calls.
func (r *Reader) SetVersion(ver Version) {
	r.ver = ver
}

// Version reports the framing currently used by ReadMessage.
func (r *Reader) Version() Version {
	return r.ver
}

// ReadMessage returns the next decoded payload, without framing bytes.
func (r *Reader) ReadMessage() ([]byte, error) {
	switch r.ver {
	case Version10:
		return r.read10()
	case Version11:
		return r.read11()
	default:
		return nil, fmt.Errorf("ncframing: unsupported version %d", r.ver)
	}
}

func (r *Reader) read10() ([]byte, error) {
	var msg []byte
	var window [6]byte
	wlen := 0
	got := false
	for {
		b, err := r.br.ReadByte()
		if err == io.EOF {
			if !got {
				return nil, io.EOF
			}
			return nil, io.ErrUnexpectedEOF
		}
		if err != nil {
			return nil, err
		}
		got = true
		if wlen < len(window) {
			window[wlen] = b
			wlen++
		} else {
			if len(msg)+1 > r.max {
				return nil, ErrTooLarge
			}
			msg = append(msg, window[0])
			copy(window[:], window[1:])
			window[len(window)-1] = b
		}
		if wlen == len(window) && string(window[:]) == EOM10 {
			return msg, nil
		}
	}
}

func (r *Reader) read11() ([]byte, error) {
	var msg []byte
	chunks := 0
	for {
		b, err := r.br.ReadByte()
		if err == io.EOF {
			if chunks == 0 {
				return nil, io.EOF
			}
			return nil, io.ErrUnexpectedEOF
		}
		if err != nil {
			return nil, err
		}
		if b != '\n' {
			return nil, fmt.Errorf("%w: expected LF", ErrInvalidFrame)
		}
		if err := r.expect('#'); err != nil {
			return nil, unexpectedEOF(err)
		}
		b, err = r.br.ReadByte()
		if err != nil {
			return nil, unexpectedEOF(err)
		}
		if b == '#' {
			if err := r.expect('\n'); err != nil {
				return nil, unexpectedEOF(err)
			}
			if chunks == 0 {
				return nil, fmt.Errorf("%w: missing chunk", ErrInvalidFrame)
			}
			return msg, nil
		}
		if b < '1' || b > '9' {
			return nil, fmt.Errorf("%w: chunk size", ErrInvalidFrame)
		}
		size := int(b - '0')
		digits := 1
		for {
			b, err = r.br.ReadByte()
			if err != nil {
				return nil, unexpectedEOF(err)
			}
			if b == '\n' {
				break
			}
			if b < '0' || b > '9' {
				return nil, fmt.Errorf("%w: chunk size", ErrInvalidFrame)
			}
			digits++
			if digits > 10 {
				return nil, fmt.Errorf("%w: chunk size", ErrInvalidFrame)
			}
			size = size*10 + int(b-'0')
			if size > r.max {
				return nil, ErrTooLarge
			}
		}
		if size < 1 {
			return nil, fmt.Errorf("%w: chunk size", ErrInvalidFrame)
		}
		if len(msg)+size > r.max {
			return nil, ErrTooLarge
		}
		chunk := make([]byte, size)
		if _, err := io.ReadFull(r.br, chunk); err != nil {
			return nil, unexpectedEOF(err)
		}
		msg = append(msg, chunk...)
		chunks++
	}
}

func (r *Reader) expect(want byte) error {
	b, err := r.br.ReadByte()
	if err != nil {
		return err
	}
	if b != want {
		return fmt.Errorf("%w: expected %q", ErrInvalidFrame, want)
	}
	return nil
}

func unexpectedEOF(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}
