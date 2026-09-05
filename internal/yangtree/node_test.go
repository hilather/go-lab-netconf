package yangtree

import (
	"testing"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/model"
)

const hostnamePath = "ietf-system:system/hostname"

func hostnameSchema() []model.SchemaLeaf {
	return []model.SchemaLeaf{
		{Path: "ietf-system:system", Type: "container"},
		{Path: hostnamePath, Type: "string", Access: "write"},
	}
}

func hostnameInstance(name string) map[string]any {
	return map[string]any{
		"ietf-system": map[string]any{
			"system": map[string]any{
				"hostname": name,
			},
		},
	}
}

func mustCompile(t *testing.T, instance map[string]any, schema []model.SchemaLeaf) Node {
	t.Helper()
	n, err := Compile(instance, schema)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCompileLookupAndSubtree(t *testing.T) {
	n := mustCompile(t, hostnameInstance("lab-rtr-a"), hostnameSchema())
	got, ok := n.Lookup(hostnamePath)
	if !ok || got != "lab-rtr-a" {
		t.Fatalf("Lookup = %v, %v", got, ok)
	}
	p, err := ParsePath(hostnamePath)
	if err != nil {
		t.Fatal(err)
	}
	sub := n.Subtree(p)
	v, ok := sub.Lookup(hostnamePath)
	if !ok || v != "lab-rtr-a" {
		t.Fatalf("subtree Lookup = %v, %v map=%v", v, ok, sub.Map())
	}
	if _, ok := sub.Lookup("ietf-interfaces:interfaces"); ok {
		t.Fatal("subtree leaked unselected module")
	}
}

func TestUnknownPathEmptyOnGet(t *testing.T) {
	n := mustCompile(t, hostnameInstance("lab-rtr-a"), hostnameSchema())
	if _, ok := n.Lookup("ietf-system:system/nope"); ok {
		t.Fatal("unknown path must be empty")
	}
	p, err := ParsePath("ietf-system:system/nope")
	if err != nil {
		t.Fatal(err)
	}
	sub := n.Subtree(p)
	if !sub.IsEmpty() {
		t.Fatalf("unknown subtree = %#v", sub.Map())
	}
}

func TestUnknownPathErrorOnEdit(t *testing.T) {
	n := mustCompile(t, hostnameInstance("lab-rtr-a"), hostnameSchema())
	err := n.Apply(OpMerge, "ietf-system:system/nope", "x")
	if err == nil {
		t.Fatal("expected unknown path error")
	}
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeValidationFailed {
		t.Fatalf("got %v", err)
	}
	got, _ := n.Lookup(hostnamePath)
	if got != "lab-rtr-a" {
		t.Fatalf("edit must not mutate on unknown path, got %v", got)
	}
}

func TestMergeReplaceDelete(t *testing.T) {
	n := mustCompile(t, hostnameInstance("lab-rtr-a"), hostnameSchema())
	if err := n.Apply(OpMerge, hostnamePath, "merged"); err != nil {
		t.Fatal(err)
	}
	got, _ := n.Lookup(hostnamePath)
	if got != "merged" {
		t.Fatalf("merge = %v", got)
	}
	if err := n.Apply(OpReplace, hostnamePath, "replaced"); err != nil {
		t.Fatal(err)
	}
	got, _ = n.Lookup(hostnamePath)
	if got != "replaced" {
		t.Fatalf("replace = %v", got)
	}
	if err := n.Apply(OpDelete, hostnamePath, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := n.Lookup(hostnamePath); ok {
		t.Fatal("delete left hostname")
	}
}

func TestCloneDoesNotAlias(t *testing.T) {
	n := mustCompile(t, hostnameInstance("lab-rtr-a"), hostnameSchema())
	c := n.Clone()
	if err := n.Apply(OpMerge, hostnamePath, "changed"); err != nil {
		t.Fatal(err)
	}
	got, _ := c.Lookup(hostnamePath)
	if got != "lab-rtr-a" {
		t.Fatalf("clone aliased, got %v", got)
	}
}

func TestListKeyMergeReplaceDelete(t *testing.T) {
	schema := []model.SchemaLeaf{
		{Path: "ietf-interfaces:interfaces", Type: "container"},
		{Path: "ietf-interfaces:interfaces/interface", Type: "list"},
		{Path: "ietf-interfaces:interfaces/interface/name", Type: "string"},
		{Path: "ietf-interfaces:interfaces/interface/enabled", Type: "boolean", Access: "write"},
	}
	instance := map[string]any{
		"ietf-interfaces": map[string]any{
			"interfaces": map[string]any{
				"interface": []any{
					map[string]any{"name": "eth0", "enabled": true},
				},
			},
		},
	}
	n := mustCompile(t, instance, schema)
	got, ok := n.Lookup("ietf-interfaces:interfaces/interface[name=eth0]/enabled")
	if !ok || got != true {
		t.Fatalf("eth0 enabled = %v, %v", got, ok)
	}
	if err := n.Apply(OpMerge, "ietf-interfaces:interfaces/interface[name=eth1]", map[string]any{
		"name": "eth1", "enabled": false,
	}); err != nil {
		t.Fatal(err)
	}
	got, ok = n.Lookup("ietf-interfaces:interfaces/interface[name=eth1]/enabled")
	if !ok || got != false {
		t.Fatalf("eth1 enabled = %v, %v map=%v", got, ok, n.Map())
	}
	if err := n.Apply(OpReplace, "ietf-interfaces:interfaces/interface[name=eth0]/enabled", false); err != nil {
		t.Fatal(err)
	}
	got, _ = n.Lookup("ietf-interfaces:interfaces/interface[name=eth0]/enabled")
	if got != false {
		t.Fatalf("eth0 after replace = %v", got)
	}
	if err := n.Apply(OpDelete, "ietf-interfaces:interfaces/interface[name=eth0]", nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := n.Lookup("ietf-interfaces:interfaces/interface[name=eth0]/enabled"); ok {
		t.Fatal("eth0 still present")
	}
	if _, ok := n.Lookup("ietf-interfaces:interfaces/interface[name=eth1]/enabled"); !ok {
		t.Fatal("eth1 was deleted")
	}
}

func TestCompactTypeCheck(t *testing.T) {
	n := mustCompile(t, hostnameInstance("lab-rtr-a"), hostnameSchema())
	err := n.Apply(OpMerge, hostnamePath, 12)
	if err == nil {
		t.Fatal("expected type error")
	}
	got, _ := n.Lookup(hostnamePath)
	if got != "lab-rtr-a" {
		t.Fatalf("failed type check mutated tree: %v", got)
	}
}

func TestRangeAndPattern(t *testing.T) {
	schema := []model.SchemaLeaf{
		{Path: "ex:n", Type: "int32", Range: "1..10", Access: "write"},
		{Path: "ex:s", Type: "string", Pattern: `^lab-`, Access: "write"},
	}
	n := mustCompile(t, map[string]any{"ex": map[string]any{"n": 3, "s": "lab-a"}}, schema)
	if err := n.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := n.Apply(OpMerge, "ex:n", 99); err == nil {
		t.Fatal("expected range error")
	}
	if err := n.Apply(OpMerge, "ex:s", "other"); err == nil {
		t.Fatal("expected pattern error")
	}
}

func TestProcessUptimeNotWritable(t *testing.T) {
	schema := []model.SchemaLeaf{
		{Path: "ex:uptime", Type: "uint32", Access: "read", ValueFrom: model.ValueFromProcessUptime},
	}
	n := mustCompile(t, map[string]any{}, schema)
	got, ok := n.Lookup("ex:uptime")
	if !ok {
		t.Fatal("uptime missing")
	}
	switch got.(type) {
	case uint64, uint32, int64, int:
	default:
		t.Fatalf("uptime type %T %v", got, got)
	}
	err := n.Apply(OpMerge, "ex:uptime", uint64(1))
	if err == nil {
		t.Fatal("expected not writable")
	}
	de, ok := domainerr.As(err)
	if !ok || de.Code != domainerr.CodeNotWritable {
		t.Fatalf("got %v", err)
	}
}

func TestFailedValidateLeavesTree(t *testing.T) {
	schema := []model.SchemaLeaf{
		{Path: hostnamePath, Type: "int32", Access: "write"},
	}
	n := mustCompile(t, hostnameInstance("lab-rtr-a"), schema)
	if err := n.Validate(); err == nil {
		t.Fatal("expected validate error")
	}
	got, ok := n.Lookup(hostnamePath)
	if !ok || got != "lab-rtr-a" {
		t.Fatalf("validate mutated tree: %v, %v", got, ok)
	}
}

func TestSubtreeWholeTree(t *testing.T) {
	n := mustCompile(t, hostnameInstance("lab-rtr-a"), hostnameSchema())
	sub := n.Subtree(Path{})
	got, ok := sub.Lookup(hostnamePath)
	if !ok || got != "lab-rtr-a" {
		t.Fatalf("full subtree = %v, %v", got, ok)
	}
}
