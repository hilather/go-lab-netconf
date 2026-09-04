package ncrpc

import (
	"bytes"
	"testing"
)

func TestHelloGolden(t *testing.T) {
	raw, err := EncodeHello(Hello{Capabilities: AdvertisedCapabilities(), SessionID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	want := sessionGolden(t, "hello.xml")
	if !bytes.Equal(raw, want) {
		t.Fatalf("hello encode\n got %s\nwant %s", raw, want)
	}
	msg, err := Decode(want)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Hello == nil || msg.Hello.SessionID != "1" {
		t.Fatalf("hello decode = %#v", msg.Hello)
	}
	if len(msg.Hello.Capabilities) != 6 {
		t.Fatalf("caps = %v", msg.Hello.Capabilities)
	}

	client, err := EncodeHello(Hello{Capabilities: []string{CapBase10, CapBase11}})
	if err != nil {
		t.Fatal(err)
	}
	want = sessionGolden(t, "hello-client.xml")
	if !bytes.Equal(client, want) {
		t.Fatalf("client hello\n got %s\nwant %s", client, want)
	}
}

func TestRPCGoldens(t *testing.T) {
	cases := []struct {
		file string
		rpc  RPC
	}{
		{"rpc-get.xml", RPC{MessageID: "101", Name: OpGet, Filter: &Filter{Type: FilterSubtree, Inner: []byte(hostnameFilter)}}},
		{"rpc-get-config.xml", RPC{MessageID: "102", Name: OpGetConfig, Source: StoreRunning}},
		{"rpc-edit-config.xml", RPC{MessageID: "103", Name: OpEditConfig, Target: StoreCandidate, DefaultOp: "merge", Config: []byte(hostnameConfig)}},
		{"rpc-copy-config.xml", RPC{MessageID: "104", Name: OpCopyConfig, Source: StoreRunning, Target: StoreStartup}},
		{"rpc-delete-config.xml", RPC{MessageID: "105", Name: OpDeleteConfig, Target: StoreStartup}},
		{"rpc-lock.xml", RPC{MessageID: "106", Name: OpLock, Target: StoreCandidate}},
		{"rpc-unlock.xml", RPC{MessageID: "107", Name: OpUnlock, Target: StoreCandidate}},
		{"rpc-commit.xml", RPC{MessageID: "108", Name: OpCommit}},
		{"rpc-discard-changes.xml", RPC{MessageID: "109", Name: OpDiscardChanges}},
		{"rpc-validate.xml", RPC{MessageID: "110", Name: OpValidate, Source: StoreCandidate}},
		{"rpc-close-session.xml", RPC{MessageID: "111", Name: OpCloseSession}},
		{"rpc-kill-session.xml", RPC{MessageID: "112", Name: OpKillSession, SessionID: "4"}},
		{"rpc-create-subscription.xml", RPC{MessageID: "113", Name: OpCreateSubscription, Stream: "NETCONF"}},
		{"rpc-get-xpath.xml", RPC{MessageID: "1", Name: OpGet, Filter: &Filter{Type: FilterXPath, Select: "/foo"}}},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			want := sessionGolden(t, tc.file)
			got := mustEncodeRPC(t, tc.rpc)
			if !bytes.Equal(got, want) {
				t.Fatalf("encode\n got %s\nwant %s", got, want)
			}
			decoded := mustDecodeRPC(t, want)
			equalRPC(t, decoded, tc.rpc)
		})
	}
}

func TestReplyOKGolden(t *testing.T) {
	raw, err := EncodeReply(Reply{MessageID: "101", OK: true})
	if err != nil {
		t.Fatal(err)
	}
	want := sessionGolden(t, "rpc-reply-ok.xml")
	if !bytes.Equal(raw, want) {
		t.Fatalf("reply\n got %s\nwant %s", raw, want)
	}
	msg, err := Decode(want)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Reply == nil || !msg.Reply.OK || msg.Reply.MessageID != "101" {
		t.Fatalf("reply = %#v", msg.Reply)
	}
}
