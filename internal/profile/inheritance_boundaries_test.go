package profile

import (
	"reflect"
	"strings"
	"testing"
)

func TestInheritanceNilUnknownAndCopyBoundaries(t *testing.T) {
	var nilProfile *Profile
	if nilProfile.IsOverridden(FieldDLSSSRMode) {
		t.Fatal("nil profile reported an override")
	}
	nilProfile.MarkOverride(FieldDLSSSRMode)
	if _, err := ReadField(nil, FieldProtonEnableHDR); err == nil {
		t.Fatal("nil field read succeeded")
	}
	if err := nilProfile.Set(FieldProtonEnableHDR, true); err == nil || (&Profile{}).Set("missing", true) == nil {
		t.Fatal("invalid field set succeeded")
	}
	if err := nilProfile.PinField(FieldDLSSSRMode, nil); err == nil || !strings.Contains(err.Error(), "nil profile") {
		t.Fatalf("nil pin error = %v", err)
	}
	if err := (&Profile{}).PinField("missing", nil); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("unknown pin error = %v", err)
	}
	if err := nilProfile.Reset(FieldDLSSSRMode); err == nil || !strings.Contains(err.Error(), "nil profile") {
		t.Fatalf("nil reset error = %v", err)
	}
	if err := (&Profile{}).Reset("missing"); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("unknown reset error = %v", err)
	}

	nilPointer := reflect.ValueOf((*bool)(nil))
	if copied := deepCopyValue(nilPointer); !copied.IsNil() {
		t.Fatal("nil pointer deep copy became non-nil")
	}
	migrateInheritance(nil, nil)
	alreadyMigrated := &Profile{Overrides: map[string]bool{}}
	migrateInheritance(alreadyMigrated, &Profile{})
	if len(alreadyMigrated.Overrides) != 0 {
		t.Fatalf("idempotent migration = %v", alreadyMigrated.Overrides)
	}
}
