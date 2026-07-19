package profile

import (
	"reflect"
	"slices"
	"testing"
)

func TestFieldDescriptorsExhaustProfileLeavesExactlyOnce(t *testing.T) {
	if got := len(Fields()); got != 34 {
		t.Fatalf("descriptor count = %d, want 34", got)
	}
	seen := map[string]bool{}
	for _, descriptor := range Fields() {
		if seen[descriptor.Key] {
			t.Fatalf("duplicate descriptor %q", descriptor.Key)
		}
		seen[descriptor.Key] = true
		if descriptor.Subsystem == "" || descriptor.Label == "" || descriptor.Kind == "" || descriptor.Impact == "" || descriptor.Restore == "" {
			t.Errorf("incomplete descriptor: %+v", descriptor)
		}
		value, err := fieldAccessor(&Profile{}, descriptor.Key)
		if err != nil {
			t.Fatal(err)
		}
		wantKind := map[reflect.Kind]PrimitiveKind{reflect.Bool: PrimitiveBool, reflect.Int: PrimitiveInt, reflect.String: PrimitiveString, reflect.Pointer: PrimitiveOptionalBool}[value.Kind()]
		if descriptor.Kind != wantKind {
			t.Errorf("%s kind = %s, want %s", descriptor.Key, descriptor.Kind, wantKind)
		}
	}
	assertDescriptorLeavesMatchProfile(t, seen)
}

func assertDescriptorLeavesMatchProfile(t *testing.T, seen map[string]bool) {
	t.Helper()
	typeOfProfile := reflect.TypeOf(Profile{})
	count := 0
	for sectionIndex := 0; sectionIndex < typeOfProfile.NumField(); sectionIndex++ {
		section := typeOfProfile.Field(sectionIndex)
		if section.Type.Kind() != reflect.Struct {
			continue
		}
		sectionName := yamlTagName(section.Tag.Get("yaml"))
		for leafIndex := 0; leafIndex < section.Type.NumField(); leafIndex++ {
			key := sectionName + "." + yamlTagName(section.Type.Field(leafIndex).Tag.Get("yaml"))
			count++
			if !seen[key] {
				t.Errorf("profile leaf %q has no descriptor", key)
			}
		}
	}
	if count != len(seen) {
		t.Fatalf("profile has %d leaves, descriptors have %d", count, len(seen))
	}
}

func TestFieldDescriptorReadWriteSetPinResetEveryPrimitive(t *testing.T) {
	cases := []struct {
		field       string
		value, zero any
	}{
		{FieldProtonEnableHDR, true, false},
		{FieldGPUClockOffset, 42, 0},
		{FieldGPUShaderCachePath, "/cache", ""},
		{FieldCPUSMT, false, nil},
	}
	for _, test := range cases {
		t.Run(test.field, func(t *testing.T) {
			p := &Profile{}
			if err := p.Set(test.field, test.value); err != nil {
				t.Fatal(err)
			}
			if got, err := ReadField(p, test.field); err != nil || !reflect.DeepEqual(got, test.value) {
				t.Fatalf("read = %#v, %v", got, err)
			}
			if !p.IsOverridden(test.field) {
				t.Fatal("set did not mark override")
			}
			if err := p.Reset(test.field); err != nil {
				t.Fatal(err)
			}
			if got, _ := ReadField(p, test.field); !reflect.DeepEqual(got, test.zero) {
				t.Fatalf("reset = %#v, want %#v", got, test.zero)
			}

			defaults := &Profile{}
			if err := WriteField(defaults, test.field, test.value); err != nil {
				t.Fatal(err)
			}
			if err := p.PinField(test.field, defaults); err != nil {
				t.Fatal(err)
			}
			if got, _ := ReadField(p, test.field); !reflect.DeepEqual(got, test.value) {
				t.Fatalf("pin = %#v", got)
			}
		})
	}
}

func TestFieldSetPreservesExplicitZeroFalseEmptyAndSMTNil(t *testing.T) {
	p := &Profile{}
	for field, value := range map[string]any{
		FieldProtonEnableHDR:    false,
		FieldGPUClockOffset:     0,
		FieldGPUShaderCachePath: "",
		FieldCPUSMT:             nil,
	} {
		if err := p.Set(field, value); err != nil {
			t.Fatal(err)
		}
		if !p.IsOverridden(field) {
			t.Errorf("%s lost explicit override", field)
		}
	}
}

func TestFieldDescriptorRejectsInvalidProfilesKindsAndValues(t *testing.T) {
	for name, operation := range map[string]func() error{
		"read nil":          func() error { _, err := ReadField(nil, FieldProtonEnableHDR); return err },
		"read unknown":      func() error { _, err := ReadField(&Profile{}, "unknown.field"); return err },
		"write nil":         func() error { return WriteField(nil, FieldProtonEnableHDR, true) },
		"write unknown":     func() error { return WriteField(&Profile{}, "unknown.field", "x") },
		"bad optional bool": func() error { return WriteField(&Profile{}, FieldCPUSMT, "maybe") },
		"bad bool":          func() error { return WriteField(&Profile{}, FieldProtonEnableHDR, "true") },
		"fractional int":    func() error { return WriteField(&Profile{}, FieldGPUClockOffset, 1.5) },
		"wrong int type":    func() error { return WriteField(&Profile{}, FieldGPUClockOffset, "1") },
		"bad string":        func() error { return WriteField(&Profile{}, FieldGPUShaderCachePath, 1) },
		"invalid mode":      func() error { return WriteField(&Profile{}, FieldDLSSSRMode, "invalid") },
		"invalid RR preset": func() error { return WriteField(&Profile{}, FieldDLSSRRPreset, "auto") },
		"set unknown":       func() error { return (&Profile{}).Set("unknown.field", true) },
		"set wrong value":   func() error { return (&Profile{}).Set(FieldProtonEnableHDR, "true") },
	} {
		t.Run(name, func(t *testing.T) {
			if err := operation(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	if err := WriteField(&Profile{}, FieldCPUSMT, "true"); err != nil {
		t.Fatalf("string optional bool: %v", err)
	}
}

func TestFieldDescriptorsReturnIndependentAllowedValues(t *testing.T) {
	descriptor, _ := Field(FieldDLSSSRMode)
	descriptor.AllowedValues[0] = "corrupt"
	if current, _ := Field(FieldDLSSSRMode); current.AllowedValues[0] != "" {
		t.Fatal("Field exposed descriptor slice")
	}
	fields := Fields()
	fields[4].AllowedValues[0] = "corrupt"
	if current, _ := Field(FieldDLSSSRMode); current.AllowedValues[0] != "" {
		t.Fatal("Fields exposed descriptor slice")
	}
}

func TestPresetDescriptorsAcceptEveryFrontendOption(t *testing.T) {
	modelPresets := []string{"A", "B", "C", "D", "E", "F", "J", "K", "L", "M"}
	tests := []struct {
		field  string
		values []string
	}{
		{FieldDLSSSRPreset, append([]string{"", "default", "auto"}, modelPresets...)},
		{FieldDLSSRRPreset, append([]string{"", "default"}, modelPresets...)},
	}
	for _, test := range tests {
		descriptor, ok := Field(test.field)
		if !ok {
			t.Fatalf("missing descriptor %q", test.field)
		}
		if !slices.Equal(descriptor.AllowedValues, test.values) {
			t.Errorf("%s allowed values = %v, want frontend options %v", test.field, descriptor.AllowedValues, test.values)
		}
		for _, value := range test.values {
			if err := WriteField(&Profile{}, test.field, value); err != nil {
				t.Errorf("%s rejected selectable value %q: %v", test.field, value, err)
			}
		}
	}
	if descriptor, _ := Field(FieldDLSSRRPreset); slices.Contains(descriptor.AllowedValues, "auto") {
		t.Fatal("RR descriptor accepts invalid auto preset")
	}
}
