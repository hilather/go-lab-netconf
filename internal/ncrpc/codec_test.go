package ncrpc

import (
	"bytes"
	"strings"
	"testing"
)

const hostnameFilter = `<system xmlns="urn:ietf:params:xml:ns:yang:ietf-system"><hostname/></system>`
const hostnameConfig = `<system xmlns="urn:ietf:params:xml:ns:yang:ietf-system"><hostname>router-a</hostname></system>`

func TestAdvertisedCapabilities(t *testing.T) {
	caps := AdvertisedCapabilities()
	want := []string{CapBase10, CapBase11, CapCandidate, CapStartup, CapValidate, CapNotification}
	if len(caps) != len(want) {
		t.Fatalf("len = %d, want %d: %v", len(caps), len(want), caps)
	}
	for i, c := range want {
		if caps[i] != c {
			t.Fatalf("caps[%d] = %q, want %q", i, caps[i], c)
		}
	}
	caps[0] = "mutated"
	again := AdvertisedCapabilities()
	if again[0] != CapBase10 {
		t.Fatal("AdvertisedCapabilities returns shared backing array")
	}

	joined := strings.Join(AdvertisedCapabilities(), "\n")
	for _, bad := range []string{
		"urn:ietf:params:netconf:capability:writable-running:1.0",
		"urn:ietf:params:netconf:capability:xpath:1.0",
		"urn:ietf:params:netconf:capability:confirmed-commit:1.0",
		"urn:ietf:params:netconf:capability:url:1.0",
	} {
		if strings.Contains(joined, bad) {
			t.Fatalf("advertised contains %s", bad)
		}
	}

	raw, err := EncodeHello(Hello{Capabilities: AdvertisedCapabilities(), SessionID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, c := range want {
		if !strings.Contains(body, c) {
			t.Fatalf("hello XML missing %s: %s", c, body)
		}
	}
	for _, bad := range []string{"writable-running", "xpath", "confirmed-commit"} {
		if strings.Contains(body, bad) {
			t.Fatalf("hello XML contains %s: %s", bad, body)
		}
	}
}

func TestMessageIDPreserved(t *testing.T) {
	raw := mustEncodeRPC(t, RPC{MessageID: "101", Name: OpGet})
	if !bytes.Contains(raw, []byte(`message-id="101"`)) {
		t.Fatalf("encoded XML missing message-id: %s", raw)
	}
	got := mustDecodeRPC(t, raw)
	if got.MessageID != "101" {
		t.Fatalf("decoded message-id = %q", got.MessageID)
	}
	got = mustDecodeRPC(t, []byte(`<rpc xmlns="urn:ietf:params:xml:ns:netconf:base:1.0" message-id="abc-xyz"><commit/></rpc>`))
	if got.MessageID != "abc-xyz" {
		t.Fatalf("string message-id = %q", got.MessageID)
	}
}

func TestHelloRoundTrip(t *testing.T) {
	in := Hello{Capabilities: AdvertisedCapabilities(), SessionID: "4"}
	raw, err := EncodeHello(in)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Hello == nil {
		t.Fatal("missing hello")
	}
	if msg.Hello.SessionID != "4" {
		t.Fatalf("session-id = %q", msg.Hello.SessionID)
	}
	if len(msg.Hello.Capabilities) != len(in.Capabilities) {
		t.Fatalf("caps = %v", msg.Hello.Capabilities)
	}
	for i, c := range in.Capabilities {
		if msg.Hello.Capabilities[i] != c {
			t.Fatalf("caps[%d] = %q, want %q", i, msg.Hello.Capabilities[i], c)
		}
	}

	client := Hello{Capabilities: []string{CapBase10, CapBase11}}
	raw, err = EncodeHello(client)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("session-id")) {
		t.Fatalf("client hello has session-id: %s", raw)
	}
	msg, err = Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Hello.SessionID != "" {
		t.Fatalf("client session-id = %q", msg.Hello.SessionID)
	}
}

func TestRPCRoundTrip(t *testing.T) {
	cases := []RPC{
		{MessageID: "101", Name: OpGet, Filter: &Filter{Type: FilterSubtree, Inner: []byte(hostnameFilter)}},
		{MessageID: "102", Name: OpGetConfig, Source: StoreRunning},
		{MessageID: "103", Name: OpEditConfig, Target: StoreCandidate, DefaultOp: "merge", Config: []byte(hostnameConfig)},
		{MessageID: "104", Name: OpCopyConfig, Source: StoreRunning, Target: StoreStartup},
		{MessageID: "105", Name: OpDeleteConfig, Target: StoreStartup},
		{MessageID: "106", Name: OpLock, Target: StoreCandidate},
		{MessageID: "107", Name: OpUnlock, Target: StoreCandidate},
		{MessageID: "108", Name: OpCommit},
		{MessageID: "109", Name: OpDiscardChanges},
		{MessageID: "110", Name: OpValidate, Source: StoreCandidate},
		{MessageID: "111", Name: OpCloseSession},
		{MessageID: "112", Name: OpKillSession, SessionID: "4"},
		{MessageID: "113", Name: OpCreateSubscription, Stream: "NETCONF"},
	}
	for _, in := range cases {
		t.Run(in.Name, func(t *testing.T) {
			raw := mustEncodeRPC(t, in)
			got := mustDecodeRPC(t, raw)
			equalRPC(t, got, in)
			if in.Name == OpCreateSubscription && got.Namespace != NotificationNamespace {
				t.Fatalf("namespace = %q, want notification", got.Namespace)
			}
		})
	}
}

func TestXPathFilterDecoded(t *testing.T) {
	raw := []byte(`<rpc xmlns="urn:ietf:params:xml:ns:netconf:base:1.0" message-id="1"><get><filter type="xpath" select="/foo"/></get></rpc>`)
	got := mustDecodeRPC(t, raw)
	if got.Filter == nil || got.Filter.Type != FilterXPath || got.Filter.Select != "/foo" {
		t.Fatalf("filter = %#v", got.Filter)
	}
}

func TestReplyRoundTrip(t *testing.T) {
	okRaw, err := EncodeReply(Reply{MessageID: "101", OK: true})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(okRaw, []byte("<ok/>")) {
		t.Fatalf("ok reply = %s", okRaw)
	}
	msg, err := Decode(okRaw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Reply == nil || !msg.Reply.OK || msg.Reply.MessageID != "101" {
		t.Fatalf("reply = %#v", msg.Reply)
	}

	errRaw, err := EncodeReply(Reply{MessageID: "102", Errors: []Error{{
		Type:     "application",
		Tag:      "unknown-element",
		Severity: "error",
		Message:  "xpath is not supported",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	msg, err = Decode(errRaw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Reply == nil || len(msg.Reply.Errors) != 1 || msg.Reply.Errors[0].Tag != "unknown-element" {
		t.Fatalf("error reply = %#v", msg.Reply)
	}

	dataRaw, err := EncodeReply(Reply{MessageID: "103", Data: []byte(hostnameConfig)})
	if err != nil {
		t.Fatal(err)
	}
	msg, err = Decode(dataRaw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Reply == nil || !bytes.Equal(msg.Reply.Data, []byte(hostnameConfig)) {
		t.Fatalf("data reply = %#v", msg.Reply)
	}

	empty, err := EncodeReply(Reply{MessageID: "104", Data: []byte{}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(empty, []byte("<data>")) || bytes.Contains(empty, []byte("<ok/>")) {
		t.Fatalf("empty data reply = %s", empty)
	}
}

func TestEncodeRequiresMessageID(t *testing.T) {
	if _, err := EncodeRPC(RPC{Name: OpGet}); err == nil {
		t.Fatal("expected missing message-id")
	}
	if _, err := EncodeReply(Reply{OK: true}); err == nil {
		t.Fatal("expected missing message-id")
	}
}

func TestDecodePrefixedHello(t *testing.T) {
	raw := []byte(`<nc:hello xmlns:nc="urn:ietf:params:xml:ns:netconf:base:1.0"><nc:capabilities><nc:capability>urn:ietf:params:netconf:base:1.0</nc:capability></nc:capabilities></nc:hello>`)
	msg, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Hello == nil || len(msg.Hello.Capabilities) != 1 || msg.Hello.Capabilities[0] != CapBase10 {
		t.Fatalf("hello = %#v", msg.Hello)
	}
}

func TestEncodeEmptyRejected(t *testing.T) {
	if _, err := Encode(Message{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeXMLDeclaration(t *testing.T) {
	raw := []byte(`<?xml version="1.0" encoding="UTF-8"?><hello xmlns="urn:ietf:params:xml:ns:netconf:base:1.0"><capabilities><capability>urn:ietf:params:netconf:base:1.1</capability></capabilities></hello>`)
	msg, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Hello == nil || len(msg.Hello.Capabilities) != 1 {
		t.Fatalf("hello = %#v", msg.Hello)
	}
}
