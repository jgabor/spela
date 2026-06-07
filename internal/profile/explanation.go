package profile

import (
	"fmt"
	"reflect"
)

// ExplanationSource names where an effective profile value came from.
type ExplanationSource string

const (
	ExplanationSourceDefault  ExplanationSource = "default"
	ExplanationSourceOverride ExplanationSource = "override"
	ExplanationSourceUnset    ExplanationSource = "unset"
)

// LaunchImpact classifies the launch surface a profile value can affect.
type LaunchImpact string

const (
	LaunchImpactEnvironment   LaunchImpact = "environment"
	LaunchImpactGameFile      LaunchImpact = "game_file"
	LaunchImpactSystemState   LaunchImpact = "system_state"
	LaunchImpactOverlay       LaunchImpact = "overlay"
	LaunchImpactCompatibility LaunchImpact = "compatibility"
)

// RestoreCoverage says whether a value needs cleanup after launch.
type RestoreCoverage string

const (
	RestoreCoverageNotApplicable      RestoreCoverage = "not_applicable"
	RestoreCoverageEphemeralLaunch    RestoreCoverage = "ephemeral_launch_environment"
	RestoreCoverageRestorableMutation RestoreCoverage = "restorable_mutation"
)

// FieldExplanation describes one effective profile value using shared source,
// impact, and restore semantics for CLI, TUI, and GUI presentation layers.
type FieldExplanation struct {
	Field        string
	Value        any
	DefaultValue any
	Source       ExplanationSource
	Impact       LaunchImpact
	Restore      RestoreCoverage
}

// Explain returns one explanation for every visible profile field. Values come
// from raw overrides when pinned, from defaults when inherited and configured,
// or from the zero value when neither layer sets the field.
func (p *Profile) Explain(defaults *Profile) ([]FieldExplanation, error) {
	items := make([]FieldExplanation, 0, len(AllFields()))
	for _, field := range AllFields() {
		item, err := p.ExplainField(field, defaults)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// ExplainField describes one effective profile field.
func (p *Profile) ExplainField(field string, defaults *Profile) (FieldExplanation, error) {
	if !IsValidField(field) {
		return FieldExplanation{}, fmt.Errorf("unknown profile field: %q", field)
	}

	impact, err := FieldLaunchImpact(field)
	if err != nil {
		return FieldExplanation{}, err
	}
	restore, err := FieldRestoreCoverage(field)
	if err != nil {
		return FieldExplanation{}, err
	}

	item := FieldExplanation{
		Field:   field,
		Impact:  impact,
		Restore: restore,
		Source:  ExplanationSourceUnset,
	}

	if defaults != nil {
		defaultValue, err := fieldAccessor(defaults, field)
		if err != nil {
			return FieldExplanation{}, err
		}
		item.DefaultValue = explanationValue(defaultValue)
		item.Value = item.DefaultValue
		item.Source = ExplanationSourceDefault
	}

	if p != nil && p.IsOverridden(field) {
		overrideValue, err := fieldAccessor(p, field)
		if err != nil {
			return FieldExplanation{}, err
		}
		item.Value = explanationValue(overrideValue)
		item.Source = ExplanationSourceOverride
	}

	if item.Value == nil {
		zero := &Profile{}
		zeroValue, err := fieldAccessor(zero, field)
		if err != nil {
			return FieldExplanation{}, err
		}
		item.Value = explanationValue(zeroValue)
	}

	return item, nil
}

// FieldLaunchImpact returns the launch impact category for a profile field.
func FieldLaunchImpact(field string) (LaunchImpact, error) {
	switch field {
	case FieldProtonEnableWayland, FieldProtonEnableHDR, FieldProtonEnableNGXUpdater, FieldProtonVKD3DHeap:
		return LaunchImpactCompatibility, nil
	case FieldDLSSSRMode, FieldDLSSSRPreset, FieldDLSSSROverride,
		FieldDLSSRRMode, FieldDLSSRRPreset, FieldDLSSRROverride, FieldDLSSFGEnabled,
		FieldDLSSFGOverride, FieldDLSSMultiFrame, FieldDLSSIndicator, FieldDLSSFGIndicator,
		FieldGPUShaderCache, FieldGPUShaderCachePath, FieldGPUThreadedOptimization:
		return LaunchImpactEnvironment, nil
	case FieldGPUClockOffset, FieldGPUMemoryOffset, FieldGPUPowerLimit, FieldGPUPowerMizer,
		FieldGPUFanSpeed, FieldCPUGovernor, FieldCPUSMT, FieldCPUAffinity:
		return LaunchImpactSystemState, nil
	case FieldOverlayEnabled, FieldOverlayPosition, FieldOverlayShowFPS, FieldOverlayShowFrametime,
		FieldOverlayShowCPU, FieldOverlayShowGPU, FieldOverlayShowVRAM, FieldOverlayToggleKey:
		return LaunchImpactOverlay, nil
	default:
		return "", fmt.Errorf("unknown profile field: %q", field)
	}
}

// FieldRestoreCoverage returns the cleanup category for a profile field.
func FieldRestoreCoverage(field string) (RestoreCoverage, error) {
	switch field {
	case FieldGPUClockOffset, FieldGPUMemoryOffset, FieldGPUPowerLimit, FieldGPUFanSpeed,
		FieldCPUGovernor, FieldCPUSMT:
		return RestoreCoverageRestorableMutation, nil
	case FieldProtonEnableWayland, FieldProtonEnableHDR, FieldProtonEnableNGXUpdater, FieldProtonVKD3DHeap,
		FieldDLSSSRMode, FieldDLSSSRPreset, FieldDLSSSROverride,
		FieldDLSSRRMode, FieldDLSSRRPreset, FieldDLSSRROverride, FieldDLSSFGEnabled,
		FieldDLSSFGOverride, FieldDLSSMultiFrame, FieldDLSSIndicator, FieldDLSSFGIndicator,
		FieldGPUShaderCache, FieldGPUShaderCachePath, FieldGPUThreadedOptimization:
		return RestoreCoverageEphemeralLaunch, nil
	case FieldGPUPowerMizer, FieldCPUAffinity, FieldOverlayEnabled, FieldOverlayPosition,
		FieldOverlayShowFPS, FieldOverlayShowFrametime, FieldOverlayShowCPU, FieldOverlayShowGPU,
		FieldOverlayShowVRAM, FieldOverlayToggleKey:
		return RestoreCoverageNotApplicable, nil
	default:
		return "", fmt.Errorf("unknown profile field: %q", field)
	}
}

func explanationValue(v reflect.Value) any {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		return v.Elem().Interface()
	}
	return v.Interface()
}
