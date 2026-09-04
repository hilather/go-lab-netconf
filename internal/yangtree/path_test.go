package yangtree

import (
	"testing"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
)

func TestParsePathModuleContainerLeaf(t *testing.T) {
	p, err := ParsePath("ietf-system:system/hostname")
	if err != nil {
		t.Fatal(err)
	}
	if p.Module != "ietf-system" || len(p.Segments) != 2 {
		t.Fatalf("got %+v", p)
	}
	if p.Segments[0].Name != "system" || p.Segments[1].Name != "hostname" {
		t.Fatalf("segments %+v", p.Segments)
	}
	if p.String() != "ietf-system:system/hostname" {
		t.Fatalf("String = %q", p.String())
	}
	if p.SchemaString() != "ietf-system:system/hostname" {
		t.Fatalf("SchemaString = %q", p.SchemaString())
	}
}

func TestParsePathListKey(t *testing.T) {
	p, err := ParsePath("ietf-interfaces:interfaces/interface[name=eth0]/enabled")
	if err != nil {
		t.Fatal(err)
	}
	if p.Module != "ietf-interfaces" || len(p.Segments) != 3 {
		t.Fatalf("got %+v", p)
	}
	if p.Segments[1].Name != "interface" || len(p.Segments[1].Keys) != 1 {
		t.Fatalf("list segment %+v", p.Segments[1])
	}
	if p.Segments[1].Keys[0] != (Key{Name: "name", Value: "eth0"}) {
		t.Fatalf("key %+v", p.Segments[1].Keys)
	}
	if p.SchemaString() != "ietf-interfaces:interfaces/interface/enabled" {
		t.Fatalf("SchemaString = %q", p.SchemaString())
	}
	if p.String() != "ietf-interfaces:interfaces/interface[name=eth0]/enabled" {
		t.Fatalf("String = %q", p.String())
	}
}

func TestParsePathQuotedAndMultiKey(t *testing.T) {
	p, err := ParsePath(`ex:list[k1='a/b'][k2="c"]/leaf`)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Segments) != 2 || len(p.Segments[0].Keys) != 2 {
		t.Fatalf("got %+v", p)
	}
	if p.Segments[0].Keys[0].Value != "a/b" || p.Segments[0].Keys[1].Value != "c" {
		t.Fatalf("keys %+v", p.Segments[0].Keys)
	}
}

func TestParsePathEmptyIsRoot(t *testing.T) {
	p, err := ParsePath("")
	if err != nil {
		t.Fatal(err)
	}
	if !p.IsRoot() {
		t.Fatalf("got %+v", p)
	}
}

func TestParsePathRejectsUnqualified(t *testing.T) {
	_, err := ParsePath("system/hostname")
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := domainerr.As(err); !ok {
		t.Fatalf("error %T %v", err, err)
	}
}

func TestParsePathRejectsXPathLike(t *testing.T) {
	_, err := ParsePath("//ietf-system:system")
	if err == nil {
		t.Fatal("expected error")
	}
}
