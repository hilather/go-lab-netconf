package ncframing

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestWrite10RejectsEOMInPayload(t *testing.T) {
	err := Write1_0(io.Discard, []byte("foo]]>]]>bar"))
	if !errors.Is(err, ErrEOMInMessage) {
		t.Fatalf("error = %v, want EOM in message", err)
	}
}

func TestWrite11RejectsEmpty(t *testing.T) {
	err := Write1_1(io.Discard, nil)
	if !errors.Is(err, ErrEmptyMessage) {
		t.Fatalf("error = %v, want empty message", err)
	}
}

func TestReadTwoMessages10(t *testing.T) {
	var buf bytes.Buffer
	if err := Write1_0(&buf, []byte("<a/>")); err != nil {
		t.Fatal(err)
	}
	if err := Write1_0(&buf, []byte("<b/>")); err != nil {
		t.Fatal(err)
	}
	r := NewReader(&buf, Version10)
	a, err := r.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != "<a/>" || string(b) != "<b/>" {
		t.Fatalf("got %q %q", a, b)
	}
}

func TestReadTwoMessages11(t *testing.T) {
	var buf bytes.Buffer
	if err := Write1_1(&buf, []byte("<a/>")); err != nil {
		t.Fatal(err)
	}
	if err := Write1_1(&buf, []byte("<b/>")); err != nil {
		t.Fatal(err)
	}
	r := NewReader(&buf, Version11)
	a, err := r.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != "<a/>" || string(b) != "<b/>" {
		t.Fatalf("got %q %q", a, b)
	}
}

func TestRead11MultiChunk(t *testing.T) {
	frame := []byte("\n#3\n<he\n#5\nllo/>\n##\n")
	r := NewReader(bytes.NewReader(frame), Version11)
	got, err := r.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "<hello/>" {
		t.Fatalf("got %q, want <hello/>", got)
	}
}

func TestSetVersionAfterHello(t *testing.T) {
	hello := []byte("<hello/>")
	rpc := []byte(`<rpc message-id="1"/>`)
	var buf bytes.Buffer
	if err := Write1_0(&buf, hello); err != nil {
		t.Fatal(err)
	}
	if err := Write1_1(&buf, rpc); err != nil {
		t.Fatal(err)
	}
	r := NewReader(&buf, Version10)
	got, err := r.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, hello) {
		t.Fatalf("hello = %q", got)
	}
	r.SetVersion(Version11)
	if r.Version() != Version11 {
		t.Fatalf("version = %d", r.Version())
	}
	got, err = r.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, rpc) {
		t.Fatalf("rpc = %q", got)
	}
}

func TestRead11RejectsLeadingZeroSize(t *testing.T) {
	r := NewReader(bytes.NewReader([]byte("\n#01\nX\n##\n")), Version11)
	_, err := r.ReadMessage()
	if !errors.Is(err, ErrInvalidFrame) {
		t.Fatalf("error = %v, want invalid frame", err)
	}
}

func TestRead11RejectsEndWithoutChunk(t *testing.T) {
	r := NewReader(bytes.NewReader([]byte("\n##\n")), Version11)
	_, err := r.ReadMessage()
	if !errors.Is(err, ErrInvalidFrame) {
		t.Fatalf("error = %v, want invalid frame", err)
	}
}

func TestRead10TooLarge(t *testing.T) {
	r := NewReader(bytes.NewReader([]byte("abcdefgh]]>]]>")), Version10)
	r.max = 4
	_, err := r.ReadMessage()
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("error = %v, want too large", err)
	}
}

func TestRead10ExactMax(t *testing.T) {
	msg := []byte("abcdefgh")
	frame := append(append([]byte{}, msg...), []byte(EOM10)...)
	r := NewReader(bytes.NewReader(frame), Version10)
	r.max = len(msg)
	got, err := r.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, msg) {
		t.Fatalf("got %q, want %q", got, msg)
	}
}

func TestRead11TooLargeChunk(t *testing.T) {
	r := NewReader(bytes.NewReader([]byte("\n#20\n01234567890123456789\n##\n")), Version11)
	r.max = 8
	_, err := r.ReadMessage()
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("error = %v, want too large", err)
	}
}

func TestRead10PartialEOF(t *testing.T) {
	r := NewReader(bytes.NewReader([]byte("<hello/>]]")), Version10)
	_, err := r.ReadMessage()
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("error = %v, want unexpected EOF", err)
	}
}

func TestReadEmptyEOF(t *testing.T) {
	r := NewReader(bytes.NewReader(nil), Version10)
	_, err := r.ReadMessage()
	if err != io.EOF {
		t.Fatalf("1.0 error = %v, want EOF", err)
	}
	r = NewReader(bytes.NewReader(nil), Version11)
	_, err = r.ReadMessage()
	if err != io.EOF {
		t.Fatalf("1.1 error = %v, want EOF", err)
	}
}

func TestWriteUnsupportedVersion(t *testing.T) {
	if err := Write(io.Discard, Version(0), []byte("x")); err == nil {
		t.Fatal("expected error")
	}
}

func TestFramingGoldens(t *testing.T) {
	hello := bytes.TrimRight(sessionGolden(t, "hello.xml"), "\r\n")
	var buf bytes.Buffer
	if err := Write1_0(&buf, hello); err != nil {
		t.Fatal(err)
	}
	want10 := sessionGolden(t, "hello-1.0.frame")
	if !bytes.Equal(buf.Bytes(), want10) {
		t.Fatalf("1.0 golden mismatch:\n got %q\nwant %q", buf.Bytes(), want10)
	}
	buf.Reset()
	if err := Write1_1(&buf, hello); err != nil {
		t.Fatal(err)
	}
	want11 := sessionGolden(t, "hello-1.1.frame")
	if !bytes.Equal(buf.Bytes(), want11) {
		t.Fatalf("1.1 golden mismatch:\n got %q\nwant %q", buf.Bytes(), want11)
	}

	r := NewReader(bytes.NewReader(want10), Version10)
	got, err := r.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, hello) {
		t.Fatalf("decoded 1.0 golden = %q", got)
	}
	r = NewReader(bytes.NewReader(want11), Version11)
	got, err = r.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, hello) {
		t.Fatalf("decoded 1.1 golden = %q", got)
	}
}

func TestWriteTooLarge(t *testing.T) {
	msg := bytes.Repeat([]byte("a"), MaxMessageSize+1)
	if err := Write1_0(io.Discard, msg); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("1.0 error = %v", err)
	}
	if err := Write1_1(io.Discard, msg); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("1.1 error = %v", err)
	}
}
