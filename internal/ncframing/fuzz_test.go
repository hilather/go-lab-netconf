package ncframing

import (
	"bytes"
	"testing"
)

func FuzzRead10(f *testing.F) {
	var buf bytes.Buffer
	_ = Write1_0(&buf, []byte("<hello/>"))
	f.Add(buf.Bytes())
	f.Add([]byte("]]>]]>"))
	f.Add([]byte("<hello/>]]>]]>"))
	f.Add([]byte(""))
	f.Add([]byte("]]>"))
	f.Add([]byte("<rpc message-id=\"1\"/>]]>]]>"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64*1024 {
			data = data[:64*1024]
		}
		r := NewReader(bytes.NewReader(data), Version10)
		r.max = 64 * 1024
		msg, err := r.ReadMessage()
		if err != nil {
			return
		}
		var out bytes.Buffer
		if err := Write1_0(&out, msg); err != nil {
			t.Fatal(err)
		}
		r2 := NewReader(&out, Version10)
		got, err := r2.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, msg) {
			t.Fatalf("round-trip mismatch")
		}
	})
}

func FuzzRead11(f *testing.F) {
	var buf bytes.Buffer
	_ = Write1_1(&buf, []byte("<hello/>"))
	f.Add(buf.Bytes())
	f.Add([]byte("\n#4\ndata\n##\n"))
	f.Add([]byte("\n#3\n<he\n#5\nllo/>\n##\n"))
	f.Add([]byte("\n##\n"))
	f.Add([]byte(""))
	f.Add([]byte("\n#01\nX\n##\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64*1024 {
			data = data[:64*1024]
		}
		r := NewReader(bytes.NewReader(data), Version11)
		r.max = 64 * 1024
		msg, err := r.ReadMessage()
		if err != nil {
			return
		}
		if len(msg) == 0 {
			return
		}
		var out bytes.Buffer
		if err := Write1_1(&out, msg); err != nil {
			t.Fatal(err)
		}
		r2 := NewReader(&out, Version11)
		got, err := r2.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, msg) {
			t.Fatalf("round-trip mismatch")
		}
	})
}
