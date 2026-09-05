package ncframing

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// Version is a NETCONF protocol version that selects a framing mechanism.
type Version int

const (
	// Version10 uses the RFC 6242 1.0 EOM delimiter.
	Version10 Version = 10
	// Version11 uses RFC 6242 1.1 chunked framing.
	Version11 Version = 11
)

// EOM10 is the NETCONF 1.0 end-of-message delimiter.
const EOM10 = "]]>]]>"

// MaxMessageSize is the largest decoded payload a Reader accepts.
const MaxMessageSize = 16 << 20

var (
	// ErrTooLarge is returned when a framed payload exceeds MaxMessageSize.
	ErrTooLarge = errors.New("ncframing: message too large")
	// ErrInvalidFrame is returned when 1.0 or 1.1 framing bytes are malformed.
	ErrInvalidFrame = errors.New("ncframing: invalid frame")
	// ErrEOMInMessage is returned when a 1.0 payload contains the EOM delimiter.
	ErrEOMInMessage = errors.New("ncframing: payload contains ]]>]]>")
	// ErrEmptyMessage is returned when writing a 1.1 frame with no payload.
	ErrEmptyMessage = errors.New("ncframing: empty 1.1 message")
)

var eom10 = []byte(EOM10)

// Write1_0 writes msg followed by the 1.0 EOM delimiter.
func Write1_0(w io.Writer, msg []byte) error {
	if bytes.Contains(msg, eom10) {
		return ErrEOMInMessage
	}
	if len(msg) > MaxMessageSize {
		return ErrTooLarge
	}
	buf := make([]byte, 0, len(msg)+len(eom10))
	buf = append(buf, msg...)
	buf = append(buf, eom10...)
	_, err := w.Write(buf)
	return err
}

// Write1_1 writes msg as a single RFC 6242 chunked frame: \n#N\n … \n##\n.
func Write1_1(w io.Writer, msg []byte) error {
	if len(msg) == 0 {
		return ErrEmptyMessage
	}
	if len(msg) > MaxMessageSize {
		return ErrTooLarge
	}
	n := strconv.Itoa(len(msg))
	buf := make([]byte, 0, 1+1+len(n)+1+len(msg)+4)
	buf = append(buf, '\n', '#')
	buf = append(buf, n...)
	buf = append(buf, '\n')
	buf = append(buf, msg...)
	buf = append(buf, '\n', '#', '#', '\n')
	_, err := w.Write(buf)
	return err
}

// Write dispatches to Write1_0 or Write1_1.
func Write(w io.Writer, ver Version, msg []byte) error {
	switch ver {
	case Version10:
		return Write1_0(w, msg)
	case Version11:
		return Write1_1(w, msg)
	default:
		return fmt.Errorf("ncframing: unsupported version %d", ver)
	}
}
