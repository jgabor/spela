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
	if value, ok := BoolFieldValue(nil, FieldProtonEnableHDR); value || ok {
		t.Fatalf("nil bool field = %v, %v", value, ok)
	}
	if IsBoolField("missing") || SetBoolField(nil, FieldProtonEnableHDR, true) || SetBoolField(&Profile{}, "missing", true) {
		t.Fatal("invalid bool field operation succeeded")
	}
	if err := CopyField(nil, &Profile{}, FieldDLSSSRMode); err == nil || !strings.Contains(err.Error(), "nil profile") {
		t.Fatalf("nil copy error = %v", err)
	}
	if err := CopyField(&Profile{}, &Profile{}, "missing"); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("unknown copy error = %v", err)
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
