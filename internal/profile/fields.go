package profile

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// PrimitiveKind is the storage primitive used by a profile leaf.
type PrimitiveKind string

const (
	PrimitiveBool         PrimitiveKind = "bool"
	PrimitiveInt          PrimitiveKind = "int"
	PrimitiveString       PrimitiveKind = "string"
	PrimitiveOptionalBool PrimitiveKind = "optional_bool"
)

// EditorKind identifies the control shape a presentation layer should use.
// It deliberately describes values, not a particular UI toolkit.
type EditorKind string

const (
	EditorToggle  EditorKind = "toggle"
	EditorChoice  EditorKind = "choice"
	EditorInteger EditorKind = "integer"
	EditorText    EditorKind = "text"
)

// FieldDescriptor is the domain-owned contract for one persisted profile leaf.
// UI layout, formatting, and command aliases deliberately remain with their
// presentation layers.
type FieldDescriptor struct {
	Key           string
	Subsystem     string
	Kind          PrimitiveKind
	Label         string
	AllowedValues []string
	Impact        LaunchImpact
	Restore       RestoreCoverage
	Description   string
	Editor        EditorKind
}

var fieldDescriptors = []FieldDescriptor{
	{FieldProtonEnableHDR, "proton", PrimitiveBool, "HDR", nil, LaunchImpactCompatibility, RestoreCoverageEphemeralLaunch, "", ""},
	{FieldProtonEnableWayland, "proton", PrimitiveBool, "Wayland", nil, LaunchImpactCompatibility, RestoreCoverageEphemeralLaunch, "", ""},
	{FieldProtonEnableNGXUpdater, "proton", PrimitiveBool, "NGX updater", nil, LaunchImpactCompatibility, RestoreCoverageEphemeralLaunch, "", ""},
	{FieldProtonVKD3DHeap, "proton", PrimitiveBool, "VKD3D heap", nil, LaunchImpactCompatibility, RestoreCoverageEphemeralLaunch, "", ""},

	{FieldDLSSSRMode, "dlss", PrimitiveString, "SR mode", []string{"", "off", "ultra_performance", "performance", "balanced", "quality", "dlaa"}, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "", ""},
	{FieldDLSSSRPreset, "dlss", PrimitiveString, "SR preset", []string{"", "default", "auto", "A", "B", "C", "D", "E", "F", "J", "K", "L", "M"}, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "", ""},
	{FieldDLSSSROverride, "dlss", PrimitiveBool, "SR override", nil, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "", ""},
	{FieldDLSSRRMode, "dlss", PrimitiveString, "RR mode", []string{"", "off", "ultra_performance", "performance", "balanced", "quality", "dlaa"}, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "", ""},
	{FieldDLSSRRPreset, "dlss", PrimitiveString, "RR preset", []string{"", "default", "A", "B", "C", "D", "E", "F", "J", "K", "L", "M"}, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "", ""},
	{FieldDLSSRROverride, "dlss", PrimitiveBool, "RR override", nil, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "", ""},
	{FieldDLSSFGEnabled, "dlss", PrimitiveBool, "FG enabled", nil, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "", ""},
	{FieldDLSSFGOverride, "dlss", PrimitiveBool, "FG override", nil, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "", ""},
	{FieldDLSSMultiFrame, "dlss", PrimitiveInt, "Multi-frame", []string{"0", "1", "2", "3", "4"}, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "", ""},
	{FieldDLSSIndicator, "dlss", PrimitiveBool, "SR indicator", nil, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "", ""},
	{FieldDLSSFGIndicator, "dlss", PrimitiveBool, "FG indicator", nil, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "", ""},

	{FieldGPUClockOffset, "gpu", PrimitiveInt, "Clock offset", nil, LaunchImpactSystemState, RestoreCoverageRestorableMutation, "", ""},
	{FieldGPUMemoryOffset, "gpu", PrimitiveInt, "Memory offset", nil, LaunchImpactSystemState, RestoreCoverageRestorableMutation, "", ""},
	{FieldGPUPowerLimit, "gpu", PrimitiveInt, "Power limit", nil, LaunchImpactSystemState, RestoreCoverageRestorableMutation, "", ""},
	{FieldGPUFanSpeed, "gpu", PrimitiveInt, "Fan speed", nil, LaunchImpactSystemState, RestoreCoverageRestorableMutation, "", ""},
	{FieldGPUPowerMizer, "gpu", PrimitiveString, "Power mode", []string{"", "adaptive", "max", "prefer_max_performance", "prefer_maximum_performance"}, LaunchImpactSystemState, RestoreCoverageNotApplicable, "", ""},
	{FieldGPUShaderCache, "gpu", PrimitiveBool, "Shader cache", nil, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "", ""},
	{FieldGPUShaderCachePath, "gpu", PrimitiveString, "Shader cache path", nil, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "", ""},
	{FieldGPUThreadedOptimization, "gpu", PrimitiveBool, "Threaded opt", nil, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "", ""},

	{FieldCPUGovernor, "cpu", PrimitiveString, "Governor", []string{"", "performance", "powersave", "schedutil", "ondemand"}, LaunchImpactSystemState, RestoreCoverageRestorableMutation, "", ""},
	{FieldCPUSMT, "cpu", PrimitiveOptionalBool, "SMT", []string{"", "true", "false"}, LaunchImpactSystemState, RestoreCoverageRestorableMutation, "", ""},
	{FieldCPUAffinity, "cpu", PrimitiveString, "Affinity", nil, LaunchImpactSystemState, RestoreCoverageNotApplicable, "", ""},

	{FieldOverlayEnabled, "overlay", PrimitiveBool, "Enabled", nil, LaunchImpactOverlay, RestoreCoverageNotApplicable, "", ""},
	{FieldOverlayPosition, "overlay", PrimitiveString, "Position", []string{"", "top-left", "top-right", "bottom-left", "bottom-right"}, LaunchImpactOverlay, RestoreCoverageNotApplicable, "", ""},
	{FieldOverlayShowFPS, "overlay", PrimitiveBool, "Show FPS", nil, LaunchImpactOverlay, RestoreCoverageNotApplicable, "", ""},
	{FieldOverlayShowFrametime, "overlay", PrimitiveBool, "Show frametime", nil, LaunchImpactOverlay, RestoreCoverageNotApplicable, "", ""},
	{FieldOverlayShowCPU, "overlay", PrimitiveBool, "Show CPU", nil, LaunchImpactOverlay, RestoreCoverageNotApplicable, "", ""},
	{FieldOverlayShowGPU, "overlay", PrimitiveBool, "Show GPU", nil, LaunchImpactOverlay, RestoreCoverageNotApplicable, "", ""},
	{FieldOverlayShowVRAM, "overlay", PrimitiveBool, "Show VRAM", nil, LaunchImpactOverlay, RestoreCoverageNotApplicable, "", ""},
	{FieldOverlayToggleKey, "overlay", PrimitiveString, "Toggle key", nil, LaunchImpactOverlay, RestoreCoverageNotApplicable, "", ""},
}

var descriptorsByKey = func() map[string]FieldDescriptor {
	result := make(map[string]FieldDescriptor, len(fieldDescriptors))
	for _, descriptor := range fieldDescriptors {
		result[descriptor.Key] = descriptor
	}
	return result
}()

// Fields returns the complete descriptor table in domain order.
func Fields() []FieldDescriptor {
	result := make([]FieldDescriptor, len(fieldDescriptors))
	for index, descriptor := range fieldDescriptors {
		result[index] = cloneDescriptor(descriptor)
	}
	return result
}

// Field returns the descriptor for key.
func Field(key string) (FieldDescriptor, bool) {
	descriptor, ok := descriptorsByKey[key]
	return cloneDescriptor(descriptor), ok
}

func cloneDescriptor(descriptor FieldDescriptor) FieldDescriptor {
	descriptor = completeDescriptor(descriptor)
	descriptor.AllowedValues = append([]string(nil), descriptor.AllowedValues...)
	return descriptor
}

func completeDescriptor(descriptor FieldDescriptor) FieldDescriptor {
	if descriptor.Description == "" {
		descriptor.Description = "Configure " + descriptor.Label + "."
	}
	if descriptor.Editor == "" {
		switch {
		case descriptor.Kind == PrimitiveBool:
			descriptor.Editor = EditorToggle
		case descriptor.Kind == PrimitiveInt:
			descriptor.Editor = EditorInteger
		case len(descriptor.AllowedValues) > 0:
			descriptor.Editor = EditorChoice
		default:
			descriptor.Editor = EditorText
		}
	}
	return descriptor
}

// ReadField returns a canonical leaf value. Optional booleans are nil or bool.
func ReadField(p *Profile, key string) (any, error) {
	if p == nil {
		return nil, fmt.Errorf("read %s: nil profile", key)
	}
	descriptor, ok := descriptorsByKey[key]
	if !ok {
		return nil, fmt.Errorf("unknown profile field: %q", key)
	}
	value, err := fieldAccessor(p, key)
	if err != nil {
		return nil, err
	}
	switch descriptor.Kind {
	case PrimitiveString:
		return value.String(), nil
	case PrimitiveInt:
		return int(value.Int()), nil
	case PrimitiveBool:
		return value.Bool(), nil
	default:
		return explanationValue(value), nil
	}
}

// WriteField assigns a canonical typed primitive without changing override intent.
func WriteField(p *Profile, key string, input any) error {
	if p == nil {
		return fmt.Errorf("write %s: nil profile", key)
	}
	descriptor, ok := descriptorsByKey[key]
	if !ok {
		return fmt.Errorf("unknown profile field: %q", key)
	}
	value, err := fieldAccessor(p, key)
	if err != nil {
		return err
	}
	converted, err := convertFieldValue(descriptor, input, value.Type())
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	value.Set(converted)
	return nil
}

func convertFieldValue(descriptor FieldDescriptor, input any, target reflect.Type) (reflect.Value, error) {
	allowedInput := fmt.Sprint(input)
	if input == nil {
		allowedInput = ""
	}
	if len(descriptor.AllowedValues) > 0 && !slices.Contains(descriptor.AllowedValues, allowedInput) {
		return reflect.Value{}, fmt.Errorf("unsupported value %q (valid: %s)", input, strings.Join(descriptor.AllowedValues, ", "))
	}
	if descriptor.Kind == PrimitiveOptionalBool {
		if input == nil || input == "" {
			return reflect.Zero(target), nil
		}
		boolean, ok := input.(bool)
		if !ok {
			parsed, err := strconv.ParseBool(fmt.Sprint(input))
			if err != nil {
				return reflect.Value{}, fmt.Errorf("expected optional boolean")
			}
			boolean = parsed
		}
		result := reflect.New(target.Elem())
		result.Elem().SetBool(boolean)
		return result, nil
	}

	var result reflect.Value
	switch descriptor.Kind {
	case PrimitiveBool:
		boolean, ok := input.(bool)
		if !ok {
			return reflect.Value{}, fmt.Errorf("expected boolean")
		}
		result = reflect.ValueOf(boolean)
	case PrimitiveInt:
		var integer int64
		switch value := input.(type) {
		case int:
			integer = int64(value)
		case float64:
			integer = int64(value)
			if float64(integer) != value {
				return reflect.Value{}, fmt.Errorf("expected integer")
			}
		default:
			return reflect.Value{}, fmt.Errorf("expected integer")
		}
		result = reflect.ValueOf(int(integer))
	case PrimitiveString:
		text, ok := input.(string)
		if !ok {
			return reflect.Value{}, fmt.Errorf("expected string")
		}
		result = reflect.ValueOf(text)
	default:
		return reflect.Value{}, fmt.Errorf("unknown primitive kind %q", descriptor.Kind)
	}
	if result.Type() != target {
		if !result.Type().ConvertibleTo(target) {
			return reflect.Value{}, fmt.Errorf("value is not convertible to %s", target)
		}
		result = result.Convert(target)
	}
	return result, nil
}

// Set writes value and marks the leaf as explicitly overridden.
func (p *Profile) Set(field string, value any) error {
	if _, ok := descriptorsByKey[field]; !ok {
		return fmt.Errorf("unknown profile field: %q", field)
	}
	if err := WriteField(p, field, value); err != nil {
		return err
	}
	p.MarkOverride(field)
	return nil
}

// MergeChanges applies only fields changed between before and after to current.
// It lets a transaction preserve unrelated edits committed by another surface.
func MergeChanges(current, before, after *Profile) error {
	if before == nil {
		before = &Profile{}
	}
	if after == nil {
		after = &Profile{}
	}
	if before.Name != after.Name {
		current.Name = after.Name
	}
	for _, descriptor := range Fields() {
		oldValue, _ := ReadField(before, descriptor.Key)
		newValue, _ := ReadField(after, descriptor.Key)
		if reflect.DeepEqual(oldValue, newValue) && before.IsOverridden(descriptor.Key) == after.IsOverridden(descriptor.Key) {
			continue
		}
		if !after.IsOverridden(descriptor.Key) {
			if err := current.Reset(descriptor.Key); err != nil {
				return err
			}
			continue
		}
		if err := current.Set(descriptor.Key, newValue); err != nil {
			return err
		}
	}
	return nil
}
