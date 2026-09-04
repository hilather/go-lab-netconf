package domainerr

import (
	"errors"
	"testing"
)

func TestCodes(t *testing.T) {
	got := Codes()
	if len(got) == 0 {
		t.Fatal("empty catalog")
	}
	if got[0] != CodeValidationFailed {
		t.Fatalf("first = %q", got[0])
	}
	want := []Code{
		CodeValidationFailed, CodeUnknownField, CodeReservedKey,
		CodeTLSUnsupported, CodeCallHomeUnsupported, CodeCandidateDirty,
	}
	joined := make(map[Code]bool, len(got))
	for _, c := range got {
		joined[c] = true
	}
	for _, c := range want {
		if !joined[c] {
			t.Fatalf("catalog missing %s", c)
		}
	}
	if !Retryable(CodeRevisionMismatch) {
		t.Fatal("revision_mismatch should be retryable")
	}
	if Retryable(CodeValidationFailed) {
		t.Fatal("validation_failed should not be retryable")
	}
	if Retryable(Code("nope")) {
		t.Fatal("unknown codes are not retryable")
	}
}

func TestConstructors(t *testing.T) {
	e := ValidationFailed("msg", FieldViolation{Path: "x", Code: "required", Message: "need x"})
	if e.Code != CodeValidationFailed || e.Error() == "" {
		t.Fatalf("%#v", e)
	}
	if !errors.Is(e, ValidationFailed("other")) {
		t.Fatal("Is by code")
	}
	got, ok := As(e)
	if !ok || got.Code != CodeValidationFailed {
		t.Fatal("As")
	}
	c := e.WithRemediation("hint").WithRevision("sha256:abc")
	if c.Remediation != "hint" || c.CurrentRevision != "sha256:abc" {
		t.Fatalf("%#v", c)
	}
	if New(CodeNotFound, "").Error() != string(CodeNotFound) {
		t.Fatal("empty message")
	}
	if UnknownField("u").Code != CodeUnknownField {
		t.Fatal("UnknownField")
	}
	if ReservedKey("r").Code != CodeReservedKey {
		t.Fatal("ReservedKey")
	}
	if TLSUnsupported("t").Code != CodeTLSUnsupported {
		t.Fatal("TLSUnsupported")
	}
	if CallHomeUnsupported("c").Code != CodeCallHomeUnsupported {
		t.Fatal("CallHomeUnsupported")
	}
	if CandidateDirty("d").Code != CodeCandidateDirty {
		t.Fatal("CandidateDirty")
	}
}
