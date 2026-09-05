package ncserver

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeHostname(t *testing.T) {
	ns := map[string]string{"ietf-system": ietfSystemNS}
	tree := hostnameInstance("lab-rtr-a")
	raw := encodeTree(tree, ns)
	if !bytes.Contains(raw, []byte("lab-rtr-a")) {
		t.Fatalf("encode = %s", raw)
	}
	if !bytes.Contains(raw, []byte(ietfSystemNS)) {
		t.Fatalf("missing ns: %s", raw)
	}
	got, err := decodeConfig(raw, ns)
	if err != nil {
		t.Fatal(err)
	}
	sys, _ := got["ietf-system"].(map[string]any)
	system, _ := sys["system"].(map[string]any)
	if system["hostname"] != "lab-rtr-a" {
		t.Fatalf("decoded = %#v", got)
	}
}

func TestFilterPathHostname(t *testing.T) {
	ns := map[string]string{"ietf-system": ietfSystemNS}
	p, err := filterPath([]byte(`<system xmlns="urn:ietf:params:xml:ns:yang:ietf-system"><hostname/></system>`), ns)
	if err != nil {
		t.Fatal(err)
	}
	if p != "ietf-system:system/hostname" {
		t.Fatalf("path = %q", p)
	}
}
