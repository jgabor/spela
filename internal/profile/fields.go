package profile

import (
	"fmt"
	"math"
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

// NoMaximum is the Max of a ValueRange that has only a lower bound.
const NoMaximum = math.MaxInt

// ValueRange is the range of a numeric field. It is shown to users and
// enforced when a value is set. A stored value outside it still loads, so a
// profile written by an older version is never lost.
type ValueRange struct {
	Min, Max int
}

// Contains reports whether value lies inside the range.
func (valueRange ValueRange) Contains(value int) bool {
	return value >= valueRange.Min && value <= valueRange.Max
}

// String names the range for people: "0 to 100" or "at least 0".
func (valueRange ValueRange) String() string {
	if valueRange.Max == NoMaximum {
		return fmt.Sprintf("at least %d", valueRange.Min)
	}
	return fmt.Sprintf("%d to %d", valueRange.Min, valueRange.Max)
}

// FieldDescriptor is the domain-owned contract for one persisted profile leaf,
// including the words that describe it. Layout, key bindings, and command
// aliases remain with the presentation layers; the words every surface prints
// for a field (label, description, unit, display range) live here so that the
// terminal, graphical, and command-line interfaces cannot drift apart.
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
	// Unit is the unit of a numeric field, such as "MHz", or empty.
	Unit string
	// Range is the range of a numeric field, or nil when no reliable range is
	// known. Hardware limits differ per GPU, so only ranges that hold for every
	// device are listed.
	Range *ValueRange
}

var fieldDescriptors = []FieldDescriptor{
	{FieldProtonEnableHDR, "proton", PrimitiveBool, "HDR", nil, LaunchImpactCompatibility, RestoreCoverageEphemeralLaunch, "Enables HDR output in Proton (PROTON_ENABLE_HDR=1) when the game starts. Off leaves the variable unset.", "", "", nil},
	{FieldProtonEnableWayland, "proton", PrimitiveBool, "Wayland", nil, LaunchImpactCompatibility, RestoreCoverageEphemeralLaunch, "Runs the game on native Wayland in Proton (PROTON_ENABLE_WAYLAND=1) when the game starts. Off leaves the variable unset.", "", "", nil},
	{FieldProtonEnableNGXUpdater, "proton", PrimitiveBool, "NGX updater", nil, LaunchImpactCompatibility, RestoreCoverageEphemeralLaunch, "Lets Proton update the DLSS DLLs (PROTON_ENABLE_NGX_UPDATER=1) when the game starts. Off leaves the variable unset.", "", "", nil},
	{FieldProtonVKD3DHeap, "proton", PrimitiveBool, "VKD3D heap", nil, LaunchImpactCompatibility, RestoreCoverageEphemeralLaunch, "Enables the VKD3D descriptor heap path (VKD3D_CONFIG=descriptor_heap) when the game starts. Needs Proton-CachyOS 10.0-20260321+ or 11.0+ and NVIDIA driver 580.94.16+.", "", "", nil},

	{FieldDLSSSRMode, "dlss", PrimitiveString, "Super resolution", []string{"", "off", "ultra_performance", "performance", "balanced", "quality", "dlaa"}, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "DLSS super resolution quality mode. Applied only when Resolution override is On.", "", "", nil},
	{FieldDLSSSRPreset, "dlss", PrimitiveString, "Resolution preset", []string{"", "default", "auto", "A", "B", "C", "D", "E", "F", "J", "K", "L", "M"}, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "DLSS super resolution render preset: auto follows the mode, A to F are the CNN presets (DLSS 2/3), J to M are the transformer presets (DLSS 4/4.5). Applied only when Resolution override is On.", "", "", nil},
	{FieldDLSSSROverride, "dlss", PrimitiveBool, "Resolution override", nil, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "Turns on the DLSS super resolution override. Mode and preset take effect only when this is On.", "", "", nil},
	{FieldDLSSRRMode, "dlss", PrimitiveString, "Ray reconstruction", []string{"", "off", "ultra_performance", "performance", "balanced", "quality", "dlaa"}, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "DLSS ray reconstruction quality mode. Applied only when Ray override is On.", "", "", nil},
	{FieldDLSSRRPreset, "dlss", PrimitiveString, "Ray preset", []string{"", "default", "A", "B", "C", "D", "E", "F", "J", "K", "L", "M"}, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "DLSS ray reconstruction render preset: A to F are the CNN presets, J to M are the transformer presets. Applied only when Ray override is On.", "", "", nil},
	{FieldDLSSRROverride, "dlss", PrimitiveBool, "Ray override", nil, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "Turns on the DLSS ray reconstruction override. Mode and preset take effect only when this is On.", "", "", nil},
	{FieldDLSSFGEnabled, "dlss", PrimitiveBool, "Frame generation", nil, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "Requests DLSS frame generation. Applied only when Generation override is On.", "", "", nil},
	{FieldDLSSFGOverride, "dlss", PrimitiveBool, "Generation override", nil, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "Turns on the DLSS frame generation override. Frame generation and multi-frame take effect only when this is On.", "", "", nil},
	{FieldDLSSMultiFrame, "dlss", PrimitiveInt, "Multi-frame", []string{"0", "1", "2", "3", "4"}, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "Extra frames to generate, 0 to 4. Applied only when Generation override and Frame generation are both On.", "", "", nil},
	{FieldDLSSIndicator, "dlss", PrimitiveBool, "SR indicator", nil, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "Shows the DLSS status indicator in the game.", "", "", nil},
	{FieldDLSSFGIndicator, "dlss", PrimitiveBool, "FG indicator", nil, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "Shows the DLSS frame generation indicator in the game.", "", "", nil},

	{FieldGPUClockOffset, "gpu", PrimitiveInt, "Clock offset", nil, LaunchImpactSystemState, RestoreCoverageRestorableMutation, "Limits the GPU clock to at most 2100 MHz plus this offset while the game runs. Clocks return to the driver default at exit. 0 leaves clocks unchanged.", "", "MHz", nil},
	{FieldGPUVRR, "gpu", PrimitiveString, "KDE VRR", []string{"", "unset", "automatic", "always", "never"}, LaunchImpactSystemState, RestoreCoverageRestorableMutation, "KDE Wayland primary display only: unset leaves KDE unchanged; automatic uses fullscreen VRR; always includes the desktop; never disables VRR. Restored after a tracked launch.", EditorChoice, "", nil},
	{FieldGPUMemoryOffset, "gpu", PrimitiveInt, "Memory offset", nil, LaunchImpactSystemState, RestoreCoverageRestorableMutation, "Limits the GPU memory clock to at most this value while the game runs. Clocks return to the driver default at exit. 0 leaves the memory clock unchanged.", "", "MHz", nil},
	{FieldGPUPowerLimit, "gpu", PrimitiveInt, "Power limit", nil, LaunchImpactSystemState, RestoreCoverageRestorableMutation, "Sets the GPU power limit while the game runs. The previous limit returns at exit when it could be read. 0 leaves the limit unchanged.", "", "W", &ValueRange{Min: 0, Max: NoMaximum}},
	{FieldGPUFanSpeed, "gpu", PrimitiveInt, "Fan speed", nil, LaunchImpactSystemState, RestoreCoverageRestorableMutation, "Sets every GPU fan to this speed while the game runs. Fans return to automatic at exit. 0 leaves fan control unchanged.", "", "%", &ValueRange{Min: 0, Max: 100}},
	{FieldGPUPowerMizer, "gpu", PrimitiveString, "Power mode", []string{"", "adaptive", "max", "prefer_max_performance", "prefer_maximum_performance"}, LaunchImpactSystemState, RestoreCoverageNotApplicable, "Not used yet: Spela does not apply this setting when a game launches.", "", "", nil},
	{FieldGPUShaderCache, "gpu", PrimitiveBool, "Shader cache", nil, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "Turns on the NVIDIA shader disk cache (__GL_SHADER_DISK_CACHE=1) when the game starts. Off leaves the driver setting alone.", "", "", nil},
	{FieldGPUShaderCachePath, "gpu", PrimitiveString, "Shader cache path", nil, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "Folder for the NVIDIA shader cache. Empty uses the default shader cache path from Settings. Used only when Shader cache is On.", "", "", nil},
	{FieldGPUThreadedOptimization, "gpu", PrimitiveBool, "Threaded optimization", nil, LaunchImpactEnvironment, RestoreCoverageEphemeralLaunch, "Sets __GL_THREADED_OPTIMIZATION to 1 (On) or 0 (Off) when the game starts. Unlike other toggles, Off is applied.", "", "", nil},

	{FieldCPUGovernor, "cpu", PrimitiveString, "Governor", []string{"", "performance", "powersave", "schedutil", "ondemand"}, LaunchImpactSystemState, RestoreCoverageRestorableMutation, "Sets the CPU frequency governor while the game runs. The previous governor returns at exit when it could be read. Empty leaves it unchanged.", "", "", nil},
	{FieldCPUSMT, "cpu", PrimitiveOptionalBool, "SMT", []string{"", "true", "false"}, LaunchImpactSystemState, RestoreCoverageRestorableMutation, "Turns simultaneous multithreading (SMT) on or off while the game runs, then restores it at exit. Empty leaves it unchanged.", "", "", nil},
	{FieldCPUAffinity, "cpu", PrimitiveString, "Affinity", nil, LaunchImpactSystemState, RestoreCoverageNotApplicable, "Not used yet: Spela does not apply this setting when a game launches.", "", "", nil},

	{FieldOverlayEnabled, "overlay", PrimitiveBool, "Enabled", nil, LaunchImpactOverlay, RestoreCoverageNotApplicable, "Starts the overlay data feed when the game starts. Nothing is drawn unless an overlay layer reads it.", "", "", nil},
	{FieldOverlayPosition, "overlay", PrimitiveString, "Position", []string{"", "top-left", "top-right", "bottom-left", "bottom-right"}, LaunchImpactOverlay, RestoreCoverageNotApplicable, "Screen corner for the overlay. Passed to the overlay layer.", "", "", nil},
	{FieldOverlayShowFPS, "overlay", PrimitiveBool, "Show FPS", nil, LaunchImpactOverlay, RestoreCoverageNotApplicable, "Not used yet: Spela does not apply this setting when a game launches.", "", "", nil},
	{FieldOverlayShowFrametime, "overlay", PrimitiveBool, "Show frametime", nil, LaunchImpactOverlay, RestoreCoverageNotApplicable, "Not used yet: Spela does not apply this setting when a game launches.", "", "", nil},
	{FieldOverlayShowCPU, "overlay", PrimitiveBool, "Show CPU", nil, LaunchImpactOverlay, RestoreCoverageNotApplicable, "Not used yet: Spela does not apply this setting when a game launches.", "", "", nil},
	{FieldOverlayShowGPU, "overlay", PrimitiveBool, "Show GPU", nil, LaunchImpactOverlay, RestoreCoverageNotApplicable, "Not used yet: Spela does not apply this setting when a game launches.", "", "", nil},
	{FieldOverlayShowVRAM, "overlay", PrimitiveBool, "Show VRAM", nil, LaunchImpactOverlay, RestoreCoverageNotApplicable, "Not used yet: Spela does not apply this setting when a game launches.", "", "", nil},
	{FieldOverlayToggleKey, "overlay", PrimitiveString, "Toggle key", nil, LaunchImpactOverlay, RestoreCoverageNotApplicable, "Not used yet: Spela does not apply this setting when a game launches.", "", "", nil},
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
	if descriptor.Range != nil {
		valueRange := *descriptor.Range
		descriptor.Range = &valueRange
	}
	return descriptor
}

func completeDescriptor(descriptor FieldDescriptor) FieldDescriptor {
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
		if descriptor.Range != nil && !descriptor.Range.Contains(int(integer)) {
			return reflect.Value{}, fmt.Errorf("must be %s%s, got %d", descriptor.Range, unitSuffix(descriptor), integer)
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

func unitSuffix(descriptor FieldDescriptor) string {
	if descriptor.Unit == "" {
		return ""
	}
	return " " + descriptor.Unit
}

// LeavesUnchanged reports whether launch skips the hardware field at value:
// clock and memory offsets at 0, power limit and fan speed at 0 or less. A game
// that saves such a value opts out of the inherited hardware setting.
func LeavesUnchanged(key string, value int) bool {
	switch key {
	case FieldGPUClockOffset, FieldGPUMemoryOffset:
		return value == 0
	case FieldGPUPowerLimit, FieldGPUFanSpeed:
		return value <= 0
	}
	return false
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
