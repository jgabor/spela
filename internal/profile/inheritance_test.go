package profile_test

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/jgabor/spela/internal/profile"
)

// withTempXDG redirects XDG_CONFIG_HOME/XDG_DATA_HOME into a temp dir and
// restores the original env on cleanup. Returns the temp dir root.
func withTempXDG(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	return dir
}

// --- IsOverridden --------------------------------------------------------

// Pass path: a MarkOverride'd field reports true.
func TestIsOverridden_Pass(t *testing.T) {
	p := &profile.Profile{Proton: profile.ProtonSettings{VKD3DHeap: true}}
	p.MarkOverride(profile.FieldProtonVKD3DHeap)

	if !p.IsOverridden(profile.FieldProtonVKD3DHeap) {
		t.Fatal("IsOverridden(proton.vkd3d_heap): expected true, got false")
	}
}

// Fail path: an unmarked field reports false even when the backing value is
// non-zero (zero vs non-zero alone never implies override).
func TestIsOverridden_FailsForUnmarkedField(t *testing.T) {
	p := &profile.Profile{Proton: profile.ProtonSettings{VKD3DHeap: true}}
	// Deliberately do NOT MarkOverride — the override map is empty.
	if p.IsOverridden(profile.FieldProtonVKD3DHeap) {
		t.Fatal("IsOverridden(proton.vkd3d_heap): expected false for unmarked field, got true")
	}
}

// --- Reset ---------------------------------------------------------------

// Pass path: Reset clears the override flag AND zeros the backing value so
// ResolveForApply falls back to the default.
func TestReset_Pass(t *testing.T) {
	p := &profile.Profile{Proton: profile.ProtonSettings{VKD3DHeap: true}}
	p.MarkOverride(profile.FieldProtonVKD3DHeap)

	if err := p.Reset(profile.FieldProtonVKD3DHeap); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	if p.IsOverridden(profile.FieldProtonVKD3DHeap) {
		t.Error("IsOverridden after Reset: expected false, got true")
	}
	if p.Proton.VKD3DHeap {
		t.Error("Proton.VKD3DHeap after Reset: expected false (zero), got true")
	}

	defaults := &profile.Profile{Proton: profile.ProtonSettings{VKD3DHeap: true}}
	resolved := p.ResolveForApply(defaults)
	if !resolved.Proton.VKD3DHeap {
		t.Error("ResolveForApply after Reset: expected VKD3DHeap=true from defaults, got false")
	}
}

// Fail path: unknown field keys surface an error rather than silently no-op.
func TestReset_FailsOnUnknownField(t *testing.T) {
	p := &profile.Profile{}
	err := p.Reset("nonsense.field")
	if err == nil {
		t.Fatal("Reset(unknown): expected error, got nil")
	}
}

// --- ResetAll ------------------------------------------------------------

// Pass path: every override is cleared and every tracked field is zeroed.
func TestResetAll_Pass(t *testing.T) {
	p := &profile.Profile{
		Name:    "test",
		Proton:  profile.ProtonSettings{VKD3DHeap: true, EnableHDR: true},
		DLSS:    profile.DLSSSettings{SRMode: profile.DLSSModeQuality, SROverride: true},
		GPU:     profile.GPUSettings{PowerLimit: 350},
		Overlay: profile.OverlaySettings{Enabled: true, Position: "top-left"},
	}
	p.MarkOverride(profile.FieldProtonVKD3DHeap)
	p.MarkOverride(profile.FieldProtonEnableHDR)
	p.MarkOverride(profile.FieldDLSSSRMode)
	p.MarkOverride(profile.FieldGPUPowerLimit)
	p.MarkOverride(profile.FieldOverlayEnabled)

	p.ResetAll()

	for _, field := range profile.AllFields() {
		if p.IsOverridden(field) {
			t.Errorf("%s: expected inherited after ResetAll, got overridden", field)
		}
	}
	if p.Proton.VKD3DHeap || p.Proton.EnableHDR || p.DLSS.SRMode != "" || p.GPU.PowerLimit != 0 || p.Overlay.Enabled {
		t.Errorf("ResetAll left non-zero values: %+v", p)
	}
	if p.Name != "test" {
		t.Errorf("ResetAll clobbered Name: got %q, want %q", p.Name, "test")
	}
}

// Fail path: ResetAll called on a nil profile must not panic. Failing this
// test means the contract of "safe on any profile value" is broken.
func TestResetAll_NilSafe(t *testing.T) {
	var p *profile.Profile
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ResetAll on nil panicked: %v", r)
		}
	}()
	p.ResetAll() // must be a no-op, not panic
}

// --- ResolveForApply -----------------------------------------------------

// Pass path: inherited fields take their value from defaults; overrides keep
// the pinned value even when the default changes.
func TestResolveForApply_Pass(t *testing.T) {
	defaults := &profile.Profile{
		Proton: profile.ProtonSettings{
			EnableHDR:     true,  // inherited on game
			VKD3DHeap:     false, // inherited on game, default false
			EnableWayland: true,
		},
		GPU: profile.GPUSettings{PowerLimit: 300},
	}

	game := &profile.Profile{
		Proton: profile.ProtonSettings{VKD3DHeap: true}, // pinned override
	}
	game.MarkOverride(profile.FieldProtonVKD3DHeap)

	resolved := game.ResolveForApply(defaults)

	if !resolved.Proton.EnableHDR {
		t.Error("inherited EnableHDR: expected true from defaults, got false")
	}
	if !resolved.Proton.EnableWayland {
		t.Error("inherited EnableWayland: expected true from defaults, got false")
	}
	if !resolved.Proton.VKD3DHeap {
		t.Error("overridden VKD3DHeap: expected true (pinned), got false")
	}
	if resolved.GPU.PowerLimit != 300 {
		t.Errorf("inherited PowerLimit: expected 300 from defaults, got %d", resolved.GPU.PowerLimit)
	}

	// Acceptance: override persists through default edits.
	defaults.Proton.VKD3DHeap = true
	defaults.Proton.EnableHDR = false
	resolved2 := game.ResolveForApply(defaults)
	if !resolved2.Proton.VKD3DHeap {
		t.Error("override persistence: VKD3DHeap lost pin after default edit")
	}
	if resolved2.Proton.EnableHDR {
		t.Error("inherited EnableHDR: expected false after default edit, got true")
	}
}

// Fail path: ResolveForApply on nil+nil must return a zero profile, not panic.
func TestResolveForApply_NilSafe(t *testing.T) {
	var p *profile.Profile
	resolved := p.ResolveForApply(nil)
	if resolved == nil {
		t.Fatal("ResolveForApply(nil, nil): expected non-nil zero profile, got nil")
	}
	if resolved.Proton.VKD3DHeap || resolved.GPU.PowerLimit != 0 {
		t.Errorf("ResolveForApply(nil, nil): expected zero profile, got %+v", resolved)
	}
}

// --- Zero-vs-non-zero override correctness -------------------------------

// Acceptance criterion: a zero value that differs from a non-zero default is
// still an override, not inheritance. Pinning `vkd3d_heap: false` when the
// default is `true` must persist through ResolveForApply as `false`.
func TestResolveForApply_ZeroValueOverride(t *testing.T) {
	defaults := &profile.Profile{Proton: profile.ProtonSettings{VKD3DHeap: true}}

	game := &profile.Profile{Proton: profile.ProtonSettings{VKD3DHeap: false}}
	game.MarkOverride(profile.FieldProtonVKD3DHeap)

	resolved := game.ResolveForApply(defaults)
	if resolved.Proton.VKD3DHeap {
		t.Error("zero-valued override lost: expected VKD3DHeap=false, got true")
	}
}

// LoadEffective: missing default profile is safe; inherited fields resolve to
// zero values rather than failing or inventing defaults.
func TestLoadEffective_MissingDefaultFallsBackSafely(t *testing.T) {
	withTempXDG(t)

	p := &profile.Profile{Name: "Cyberpunk 2077"}
	p.MarkOverride(profile.FieldProtonEnableHDR)
	p.Proton.EnableHDR = true
	if err := profile.Save(1091500, p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	resolved, err := profile.LoadEffective(1091500)
	if err != nil {
		t.Fatalf("LoadEffective: %v", err)
	}
	if resolved == nil {
		t.Fatal("LoadEffective: expected profile, got nil")
	}
	if !resolved.Proton.EnableHDR {
		t.Error("overridden HDR should survive missing defaults")
	}
	if resolved.Proton.EnableWayland {
		t.Error("inherited Wayland should fall back to zero with missing defaults")
	}
}

// LoadEffective: invalid default profile is surfaced rather than resolving
// inherited fields against a silent zero-value profile.
func TestLoadEffective_InvalidDefaultSurfacesError(t *testing.T) {
	withTempXDG(t)
	cfgDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "spela", "profiles")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "default.yaml"), []byte("proton: ["), 0o644); err != nil {
		t.Fatalf("write default: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "1091500.yaml"), []byte("name: Cyberpunk 2077\noverrides: {}\n"), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	_, err := profile.LoadEffective(1091500)
	if err == nil {
		t.Fatal("LoadEffective: expected invalid default error, got nil")
	}
}

// --- Load-time migration -------------------------------------------------

// Pass path: a legacy profile YAML whose fields match the current defaults
// is migrated so each matching field is stripped (inherited), and fields
// that differ become explicit overrides. Crucially, Apply behavior is
// preserved: the resolved profile matches the legacy profile exactly.
func TestMigration_LegacyProfileRoundTrip(t *testing.T) {
	dir := withTempXDG(t)
	_ = dir

	// Write a default profile that enables HDR and sets power limit 300.
	def := &profile.Profile{
		Proton: profile.ProtonSettings{EnableHDR: true, EnableWayland: false},
		GPU:    profile.GPUSettings{PowerLimit: 300},
	}
	if err := profile.SaveDefault(def); err != nil {
		t.Fatalf("SaveDefault: %v", err)
	}

	// Write a legacy-format game profile YAML (no `overrides:` key) where
	// EnableHDR matches the default, PowerLimit differs, and VKD3DHeap is
	// explicitly false (matching the default's zero — inherited).
	legacyYAML := []byte(`name: Cyberpunk 2077
proton:
  enable_hdr: true
  enable_wayland: true
gpu:
  power_limit: 400
`)
	cfgDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "spela", "profiles")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "1091500.yaml"), legacyYAML, 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	p, err := profile.Load(1091500)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p == nil {
		t.Fatal("Load: expected non-nil profile")
	}

	// EnableHDR equals default → should be stripped to inherited.
	if p.IsOverridden(profile.FieldProtonEnableHDR) {
		t.Error("EnableHDR: expected inherited (matches default), got overridden")
	}
	if p.Proton.EnableHDR {
		t.Error("EnableHDR: expected zero after migration (inherited), got true")
	}

	// EnableWayland differs from default → overridden.
	if !p.IsOverridden(profile.FieldProtonEnableWayland) {
		t.Error("EnableWayland: expected overridden (differs from default), got inherited")
	}
	if !p.Proton.EnableWayland {
		t.Error("EnableWayland: expected true (pinned), got false")
	}

	// PowerLimit differs from default → overridden.
	if !p.IsOverridden(profile.FieldGPUPowerLimit) {
		t.Error("PowerLimit: expected overridden, got inherited")
	}
	if p.GPU.PowerLimit != 400 {
		t.Errorf("PowerLimit: expected 400, got %d", p.GPU.PowerLimit)
	}

	// Apply behavior is preserved: resolved profile matches the legacy intent.
	resolved := p.ResolveForApply(def)
	if !resolved.Proton.EnableHDR {
		t.Error("resolved EnableHDR: expected true (from default), got false")
	}
	if !resolved.Proton.EnableWayland {
		t.Error("resolved EnableWayland: expected true (pinned), got false")
	}
	if resolved.GPU.PowerLimit != 400 {
		t.Errorf("resolved PowerLimit: expected 400 (pinned), got %d", resolved.GPU.PowerLimit)
	}
}

// Fail path: a profile YAML that already carries an `overrides:` key must
// NOT be re-migrated. Migration is one-way and idempotent; re-running it
// would clobber explicit user intent.
func TestMigration_SkipsAlreadyMigrated(t *testing.T) {
	withTempXDG(t)

	def := &profile.Profile{Proton: profile.ProtonSettings{VKD3DHeap: true}}
	if err := profile.SaveDefault(def); err != nil {
		t.Fatalf("SaveDefault: %v", err)
	}

	// Post-migration YAML: the user has explicitly pinned VKD3DHeap=false
	// even though the default is true. The overrides map is present.
	migratedYAML := []byte(`name: test
proton:
  vkd3d_heap: false
overrides:
  proton.vkd3d_heap: true
`)
	cfgDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "spela", "profiles")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "1091500.yaml"), migratedYAML, 0o644); err != nil {
		t.Fatalf("write migrated: %v", err)
	}

	p, err := profile.Load(1091500)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Override must survive (not be stripped as "matches a non-default").
	if !p.IsOverridden(profile.FieldProtonVKD3DHeap) {
		t.Error("overrides map ignored: expected VKD3DHeap pinned, got inherited")
	}
	if p.Proton.VKD3DHeap {
		t.Error("stored false value clobbered: expected VKD3DHeap=false, got true")
	}

	// Resolve against the default: override wins.
	resolved := p.ResolveForApply(def)
	if resolved.Proton.VKD3DHeap {
		t.Error("resolve: zero-valued override dropped, expected VKD3DHeap=false")
	}
}

// --- YAML round-trip of overrides ---------------------------------------

// Pass path: a profile with a populated Overrides map round-trips through
// YAML without losing the map or its entries.
func TestOverrides_YAMLRoundTrip(t *testing.T) {
	p := &profile.Profile{
		Name:   "rtt",
		Proton: profile.ProtonSettings{VKD3DHeap: true},
	}
	p.MarkOverride(profile.FieldProtonVKD3DHeap)

	data, err := yaml.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var p2 profile.Profile
	if err := yaml.Unmarshal(data, &p2); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !p2.IsOverridden(profile.FieldProtonVKD3DHeap) {
		t.Errorf("Overrides lost in round-trip:\n%s", data)
	}
}

// --- PinField (Task 5) ---------------------------------------------------

// Pass path: pinning an inherited field on a game profile copies the
// currently-resolved value from defaults and marks the field as overridden.
func TestPinField_Pass_CopiesDefaultsValue(t *testing.T) {
	defaults := &profile.Profile{
		GPU: profile.GPUSettings{PowerLimit: 350},
	}
	p := &profile.Profile{} // no override, inherits PowerLimit=350

	if err := p.PinField(profile.FieldGPUPowerLimit, defaults); err != nil {
		t.Fatalf("PinField: %v", err)
	}
	if !p.IsOverridden(profile.FieldGPUPowerLimit) {
		t.Fatal("PinField: expected override flag set")
	}
	if p.GPU.PowerLimit != 350 {
		t.Errorf("PinField: expected PowerLimit=350 copied from defaults, got %d", p.GPU.PowerLimit)
	}
	// Verify persistence through a defaults change: flipping defaults to 200
	// should leave the pinned profile resolving to 350.
	defaults.GPU.PowerLimit = 200
	resolved := p.ResolveForApply(defaults)
	if resolved.GPU.PowerLimit != 350 {
		t.Errorf("PinField: expected pinned 350 after defaults change, got %d", resolved.GPU.PowerLimit)
	}
}

// Fail (negative) path: pinning an unknown field returns an error and does
// NOT mutate the profile.
func TestPinField_Fail_UnknownField(t *testing.T) {
	p := &profile.Profile{}
	err := p.PinField("proton.does_not_exist", nil)
	if err == nil {
		t.Fatal("PinField: expected error for unknown field, got nil")
	}
	if len(p.Overrides) != 0 {
		t.Errorf("PinField: expected no overrides on error, got %v", p.Overrides)
	}
}

// Idempotence: pinning a field that is already overridden is a no-op
// (returns nil, does not change value or overrides map).
func TestPinField_AlreadyOverriddenNoop(t *testing.T) {
	p := &profile.Profile{
		GPU: profile.GPUSettings{PowerLimit: 400},
	}
	p.MarkOverride(profile.FieldGPUPowerLimit)

	defaults := &profile.Profile{GPU: profile.GPUSettings{PowerLimit: 350}}

	if err := p.PinField(profile.FieldGPUPowerLimit, defaults); err != nil {
		t.Fatalf("PinField: %v", err)
	}
	if p.GPU.PowerLimit != 400 {
		t.Errorf("PinField idempotent: expected 400 preserved, got %d (defaults leaked)", p.GPU.PowerLimit)
	}
}

// Edge: nil defaults with inherited (zero) field pins the zero value.
func TestPinField_NilDefaults_PinsZero(t *testing.T) {
	p := &profile.Profile{}
	if err := p.PinField(profile.FieldProtonEnableHDR, nil); err != nil {
		t.Fatalf("PinField: %v", err)
	}
	if !p.IsOverridden(profile.FieldProtonEnableHDR) {
		t.Fatal("PinField: override flag not set on nil-defaults pin")
	}
	if p.Proton.EnableHDR {
		t.Errorf("PinField: expected zero value pinned, got true")
	}
}
