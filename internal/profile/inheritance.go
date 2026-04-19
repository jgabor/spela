package profile

import (
	"fmt"
	"reflect"
	"sort"
)

// Field keys for per-field inheritance tracking. Keys are dot-paths shaped
// `<subsystem>.<yaml_field>` so they match the YAML keys users see in their
// profile files. Every settable leaf on Profile has a key here; if a new leaf
// is added anywhere in Profile, `fieldsBySection` below must also be updated.
const (
	FieldProtonEnableWayland    = "proton.enable_wayland"
	FieldProtonEnableHDR        = "proton.enable_hdr"
	FieldProtonEnableNGXUpdater = "proton.enable_ngx_updater"
	FieldProtonVKD3DHeap        = "proton.vkd3d_heap"

	FieldDLSSSRMode        = "dlss.sr_mode"
	FieldDLSSSRPreset      = "dlss.sr_preset"
	FieldDLSSSRModelPreset = "dlss.sr_model_preset"
	FieldDLSSSROverride    = "dlss.sr_override"
	FieldDLSSRRMode        = "dlss.rr_mode"
	FieldDLSSRRPreset      = "dlss.rr_preset"
	FieldDLSSRROverride    = "dlss.rr_override"
	FieldDLSSFGEnabled     = "dlss.fg_enabled"
	FieldDLSSFGOverride    = "dlss.fg_override"
	FieldDLSSMultiFrame    = "dlss.multi_frame"
	FieldDLSSIndicator     = "dlss.indicator"
	FieldDLSSFGIndicator   = "dlss.fg_indicator"

	FieldGPUShaderCache          = "gpu.shader_cache"
	FieldGPUShaderCachePath      = "gpu.shader_cache_path"
	FieldGPUThreadedOptimization = "gpu.threaded_optimization"
	FieldGPUClockOffset          = "gpu.clock_offset"
	FieldGPUMemoryOffset         = "gpu.memory_offset"
	FieldGPUPowerLimit           = "gpu.power_limit"
	FieldGPUPowerMizer           = "gpu.power_mizer"
	FieldGPUFanSpeed             = "gpu.fan_speed"

	FieldCPUGovernor = "cpu.governor"
	FieldCPUSMT      = "cpu.smt"
	FieldCPUAffinity = "cpu.affinity"

	FieldOverlayEnabled       = "overlay.enabled"
	FieldOverlayPosition      = "overlay.position"
	FieldOverlayShowFPS       = "overlay.show_fps"
	FieldOverlayShowFrametime = "overlay.show_frametime"
	FieldOverlayShowCPU       = "overlay.show_cpu"
	FieldOverlayShowGPU       = "overlay.show_gpu"
	FieldOverlayShowVRAM      = "overlay.show_vram"
	FieldOverlayToggleKey     = "overlay.toggle_key"
)

// fieldsBySection lists every inheritance-tracked field key, grouped by
// subsystem. The ordering inside each section is the canonical rendering
// order used by `spela <subsystem> show`.
var fieldsBySection = map[string][]string{
	"proton": {
		FieldProtonEnableHDR,
		FieldProtonEnableWayland,
		FieldProtonEnableNGXUpdater,
		FieldProtonVKD3DHeap,
	},
	"dlss": {
		FieldDLSSSRMode,
		FieldDLSSSRPreset,
		FieldDLSSSRModelPreset,
		FieldDLSSSROverride,
		FieldDLSSRRMode,
		FieldDLSSRRPreset,
		FieldDLSSRROverride,
		FieldDLSSFGEnabled,
		FieldDLSSFGOverride,
		FieldDLSSMultiFrame,
		FieldDLSSIndicator,
		FieldDLSSFGIndicator,
	},
	"gpu": {
		FieldGPUClockOffset,
		FieldGPUMemoryOffset,
		FieldGPUPowerLimit,
		FieldGPUFanSpeed,
		FieldGPUPowerMizer,
		FieldGPUShaderCache,
		FieldGPUShaderCachePath,
		FieldGPUThreadedOptimization,
	},
	"cpu": {
		FieldCPUGovernor,
		FieldCPUSMT,
		FieldCPUAffinity,
	},
	"overlay": {
		FieldOverlayEnabled,
		FieldOverlayPosition,
		FieldOverlayShowFPS,
		FieldOverlayShowFrametime,
		FieldOverlayShowCPU,
		FieldOverlayShowGPU,
		FieldOverlayShowVRAM,
		FieldOverlayToggleKey,
	},
}

// AllFields returns every inheritance-tracked field key on Profile in a
// deterministic order (alphabetical by subsystem, canonical order within).
func AllFields() []string {
	sections := make([]string, 0, len(fieldsBySection))
	for section := range fieldsBySection {
		sections = append(sections, section)
	}
	sort.Strings(sections)

	var out []string
	for _, section := range sections {
		out = append(out, fieldsBySection[section]...)
	}
	return out
}

// SectionFields returns the ordered field keys for a named subsystem
// (proton, dlss, gpu, cpu, overlay). Returns nil for unknown sections.
func SectionFields(section string) []string {
	keys := fieldsBySection[section]
	if len(keys) == 0 {
		return nil
	}
	out := make([]string, len(keys))
	copy(out, keys)
	return out
}

// IsValidField reports whether a field key is recognized by the inheritance
// layer. Consumers of Reset/MarkOverride should validate input with this.
func IsValidField(field string) bool {
	for _, f := range AllFields() {
		if f == field {
			return true
		}
	}
	return false
}

// fieldAccessor locates a field on Profile via reflection. Returns the
// addressable reflect.Value for the leaf field, or an error if the key is
// not a known field.
func fieldAccessor(p *Profile, field string) (reflect.Value, error) {
	v := reflect.ValueOf(p).Elem()
	for i := 0; i < v.NumField(); i++ {
		sectionValue := v.Field(i)
		if sectionValue.Kind() != reflect.Struct {
			continue
		}
		sectionType := v.Type().Field(i)
		sectionYAML := yamlTagName(sectionType.Tag.Get("yaml"))
		if sectionYAML == "" {
			continue
		}
		for j := 0; j < sectionValue.NumField(); j++ {
			leafType := sectionValue.Type().Field(j)
			leafYAML := yamlTagName(leafType.Tag.Get("yaml"))
			if leafYAML == "" {
				continue
			}
			if sectionYAML+"."+leafYAML == field {
				return sectionValue.Field(j), nil
			}
		}
	}
	return reflect.Value{}, fmt.Errorf("unknown profile field: %q", field)
}

// yamlTagName extracts the primary name from a yaml struct tag (`foo,omitempty`
// → `foo`). Returns "" for `-` or empty tags.
func yamlTagName(tag string) string {
	if tag == "" || tag == "-" {
		return ""
	}
	for i := 0; i < len(tag); i++ {
		if tag[i] == ',' {
			return tag[:i]
		}
	}
	return tag
}

// IsOverridden reports whether the field is explicitly pinned on this profile
// (an override) as opposed to inheriting from the defaults.
func (p *Profile) IsOverridden(field string) bool {
	if p == nil {
		return false
	}
	return p.Overrides[field]
}

// MarkOverride records that the caller just set `field` on this profile as an
// explicit override. CLI `set` handlers and TUI pin actions call this after
// mutating the corresponding struct field so the Overrides map stays in sync
// with the data.
func (p *Profile) MarkOverride(field string) {
	if p == nil {
		return
	}
	if !IsValidField(field) {
		return
	}
	if p.Overrides == nil {
		p.Overrides = make(map[string]bool)
	}
	p.Overrides[field] = true
}

// PinField copies the currently-resolved value of `field` onto this profile
// and marks it as an override. The "resolved value" is the value that would
// be used at apply time given the supplied defaults: if `p` already has the
// field set (non-zero), the existing value is kept; otherwise the value is
// copied from `defaults`. After pinning, the field survives subsequent
// changes to `defaults`.
//
// Semantics:
//   - If the field is already overridden, PinField is a no-op (idempotent).
//   - If `defaults` is nil and the field on `p` is zero, the resulting
//     override pins the zero value explicitly.
//   - Returns an error for unknown field keys.
//
// Task 5 consumes this from the TUI `p` binding so the user can lock in
// the value they are currently looking at on an inherited field.
func (p *Profile) PinField(field string, defaults *Profile) error {
	if p == nil {
		return fmt.Errorf("pin %s: nil profile", field)
	}
	if !IsValidField(field) {
		return fmt.Errorf("unknown profile field: %q", field)
	}
	if p.IsOverridden(field) {
		return nil
	}
	dst, err := fieldAccessor(p, field)
	if err != nil {
		return err
	}
	// If the field on p is already set (non-zero), the resolved value is p's
	// own value — nothing to copy. Otherwise pull the value from defaults.
	if dst.IsZero() && defaults != nil {
		srcVal, err := fieldAccessor(defaults, field)
		if err == nil {
			dst.Set(deepCopyValue(srcVal))
		}
	}
	p.MarkOverride(field)
	return nil
}

// Reset clears the override flag on `field` and zeros the backing struct value
// so the resolved profile falls back to the default. Returns an error for
// unknown field keys.
func (p *Profile) Reset(field string) error {
	if p == nil {
		return fmt.Errorf("reset %s: nil profile", field)
	}
	fv, err := fieldAccessor(p, field)
	if err != nil {
		return err
	}
	fv.Set(reflect.Zero(fv.Type()))
	delete(p.Overrides, field)
	return nil
}

// ResetAll clears every override and zeros every inheritance-tracked field on
// the profile. The Name field is preserved (it is not inheritance-tracked).
func (p *Profile) ResetAll() {
	if p == nil {
		return
	}
	for _, field := range AllFields() {
		if fv, err := fieldAccessor(p, field); err == nil {
			fv.Set(reflect.Zero(fv.Type()))
		}
	}
	p.Overrides = nil
}

// ResolveForApply returns a new *Profile whose fields are taken from `p` where
// overridden and from `defaults` otherwise. The result is safe to pass into
// Apply; it reflects the effective configuration for a game launch given the
// current defaults.
//
// If `defaults` is nil, inherited fields remain at their zero value (equivalent
// to applying a profile with no overrides and no defaults).
func (p *Profile) ResolveForApply(defaults *Profile) *Profile {
	if p == nil && defaults == nil {
		return &Profile{}
	}
	out := &Profile{}
	if p != nil {
		out.Name = p.Name
	}

	for _, field := range AllFields() {
		source := defaults
		if p != nil && p.IsOverridden(field) {
			source = p
		}
		if source == nil {
			continue
		}
		srcVal, err := fieldAccessor(source, field)
		if err != nil {
			continue
		}
		dstVal, err := fieldAccessor(out, field)
		if err != nil {
			continue
		}
		dstVal.Set(deepCopyValue(srcVal))
	}
	return out
}

// deepCopyValue returns a copy of v that does not share pointer state with
// the source. Needed for `*bool` fields (CPU.SMT) where mutating the resolved
// profile would otherwise mutate the original.
func deepCopyValue(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		copied := reflect.New(v.Type().Elem())
		copied.Elem().Set(v.Elem())
		return copied
	default:
		return v
	}
}

// migrateInheritance backfills the Overrides map for a legacy profile that
// was saved before per-field inheritance existed. Each field is compared to
// the matching field on `defaults`:
//   - values that differ → marked overridden (pinned)
//   - values that equal the default → stripped to zero (treated as inherited)
//
// If `defaults` is nil, every non-zero field becomes an override (since there
// is no default to inherit from, the user's data is all they have).
//
// This function is idempotent: a profile whose Overrides map is already
// populated is returned unchanged.
func migrateInheritance(p *Profile, defaults *Profile) {
	if p == nil {
		return
	}
	if p.Overrides != nil {
		// Already migrated, or explicitly written with overrides. Nothing to do.
		return
	}

	p.Overrides = make(map[string]bool)
	for _, field := range AllFields() {
		pv, err := fieldAccessor(p, field)
		if err != nil {
			continue
		}

		if defaults == nil {
			// No default profile: any non-zero field is an override.
			if !pv.IsZero() {
				p.Overrides[field] = true
			}
			continue
		}

		dv, err := fieldAccessor(defaults, field)
		if err != nil {
			continue
		}

		if fieldValuesEqual(pv, dv) {
			// Game profile field matches the default; treat as inherited by
			// zeroing the game profile's copy and leaving it out of Overrides.
			pv.Set(reflect.Zero(pv.Type()))
		} else {
			// Values differ; mark overridden. Zero vs non-zero counts as a
			// difference, which correctly handles the case where the user
			// intentionally set a field to a zero-valued override (e.g.
			// `vkd3d_heap: false` when the default is `true`).
			p.Overrides[field] = true
		}
	}
}

// fieldValuesEqual compares two reflected field values for equality.
// Handles the `*bool` case where reflect.DeepEqual on two distinct pointers
// with the same pointed-to value correctly returns true (DeepEqual dereferences
// pointers).
func fieldValuesEqual(a, b reflect.Value) bool {
	return reflect.DeepEqual(a.Interface(), b.Interface())
}
