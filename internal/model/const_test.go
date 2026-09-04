package model

import "testing"

func TestAPIConstants(t *testing.T) {
	if APIVersionV1Alpha1 != "labnetconf.dev/v1alpha1" {
		t.Fatalf("apiVersion = %q", APIVersionV1Alpha1)
	}
	if KindLabNETCONF != "LabNETCONF" {
		t.Fatalf("kind = %q", KindLabNETCONF)
	}
	if RevisionPrefix != "sha256:" {
		t.Fatalf("prefix = %q", RevisionPrefix)
	}
	if !KnownRole(RoleAdministrator) || KnownRole("operator") {
		t.Fatal("KnownRole")
	}
	if !KnownSchemaType("string") || KnownSchemaType("binary") {
		t.Fatal("KnownSchemaType")
	}
}
