package ncframing

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"testing"
)

func TestFraming10EOM(t *testing.T) {
	msg := []byte(`<hello xmlns="urn:ietf:params:xml:ns:netconf:base:1.0"/>`)
	var buf bytes.Buffer
	if err := Write1_0(&buf, msg); err != nil {
		t.Fatal(err)
	}
	got := buf.Bytes()
	if !bytes.HasSuffix(got, []byte(EOM10)) {
		t.Fatalf("1.0 frame missing %q: %q", EOM10, got)
	}
	if bytes.Contains(got[:len(got)-len(EOM10)], []byte(EOM10)) {
		t.Fatalf("1.0 payload contains EOM: %q", got)
	}
	if !bytes.Equal(got[:len(got)-len(EOM10)], msg) {
		t.Fatalf("1.0 payload = %q, want %q", got[:len(got)-len(EOM10)], msg)
	}
	if !bytes.Equal(got, append(append([]byte{}, msg...), []byte(EOM10)...)) {
		t.Fatalf("1.0 frame = %q", got)
	}

	r := NewReader(&byteReader{b: append([]byte{}, got...)}, Version10)
	out, err := r.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, msg) {
		t.Fatalf("decoded 1.0 = %q, want %q", out, msg)
	}
	if _, err := r.ReadMessage(); err != io.EOF {
		t.Fatalf("second read error = %v, want EOF", err)
	}
}

func TestFraming11Chunked(t *testing.T) {
	msg := []byte(`<rpc xmlns="urn:ietf:params:xml:ns:netconf:base:1.0" message-id="101"><get/></rpc>`)
	var buf bytes.Buffer
	if err := Write1_1(&buf, msg); err != nil {
		t.Fatal(err)
	}
	got := buf.Bytes()
	wantHead := []byte("\n#" + strconv.Itoa(len(msg)) + "\n")
	if !bytes.HasPrefix(got, wantHead) {
		t.Fatalf("1.1 missing chunk header %q: %q", wantHead, got)
	}
	if !bytes.HasSuffix(got, []byte("\n##\n")) {
		t.Fatalf("1.1 missing end-of-chunks: %q", got)
	}
	mid := got[len(wantHead) : len(got)-len("\n##\n")]
	if !bytes.Equal(mid, msg) {
		t.Fatalf("1.1 chunk data = %q, want %q", mid, msg)
	}
	want := fmt.Sprintf("\n#%d\n%s\n##\n", len(msg), msg)
	if string(got) != want {
		t.Fatalf("1.1 frame = %q, want %q", got, want)
	}

	r := NewReader(&byteReader{b: append([]byte{}, got...)}, Version11)
	out, err := r.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, msg) {
		t.Fatalf("decoded 1.1 = %q, want %q", out, msg)
	}
	if _, err := r.ReadMessage(); err != io.EOF {
		t.Fatalf("second read error = %v, want EOF", err)
	}
}
