package ncrpc

import "testing"

func FuzzDecode(f *testing.F) {
	f.Add([]byte(`<hello xmlns="urn:ietf:params:xml:ns:netconf:base:1.0"><capabilities><capability>urn:ietf:params:netconf:base:1.0</capability></capabilities></hello>`))
	f.Add([]byte(`<rpc xmlns="urn:ietf:params:xml:ns:netconf:base:1.0" message-id="1"><get/></rpc>`))
	f.Add([]byte(`<rpc-reply xmlns="urn:ietf:params:xml:ns:netconf:base:1.0" message-id="1"><ok/></rpc-reply>`))
	f.Add([]byte(""))
	f.Add([]byte("<"))
	f.Add([]byte("<rpc/>"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64*1024 {
			data = data[:64*1024]
		}
		msg, err := Decode(data)
		if err != nil {
			return
		}
		raw, err := Encode(msg)
		if err != nil {
			return
		}
		_, err = Decode(raw)
		if err != nil {
			t.Fatalf("re-decode: %v", err)
		}
	})
}
