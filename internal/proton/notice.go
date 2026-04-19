package proton

import (
	"errors"
	"fmt"
)

// NoticeDeps carries the small set of side-effecting lookups that the
// CompatibilityNotice helper needs in order to decide whether to emit
// a user-facing incompatibility notice. Exposed as a struct so both CLI
// and TUI can inject fakes in tests without touching the global filesystem
// or NVML.
type NoticeDeps struct {
	// SteamRoot is the absolute Steam install root, or "" when unavailable.
	SteamRoot string
	// ResolveForAppID walks Steam config to identify the active Proton
	// build for a given AppID.
	ResolveForAppID func(steamRoot string, appID uint64) (Build, error)
	// SupportsVKD3DHeap reports whether a resolved Proton build ships the
	// PROTON_VKD3D_HEAP marker.
	SupportsVKD3DHeap func(build Build) (bool, error)
	// DriverVersion returns the raw NVIDIA driver version string
	// (e.g. "580.94.16"). Returning "" signals a non-NVIDIA or unprobed
	// system.
	DriverVersion func() (string, error)
}

// CompatibilityResult is the structured outcome of a vkd3d_heap
// environment check. Both axes (Proton build + NVIDIA driver) are
// evaluated independently so callers can render them separately.
//
// Typical branching:
//
//   - ProtonOK=false, ProtonSkip="" → hard incompatibility on Proton axis
//   - ProtonOK=true,  ProtonSkip!="" → check couldn't run; surface as info
//   - ProtonOK=true,  ProtonSkip=""  → satisfied
//
// The same shape applies to the Driver* fields.
type CompatibilityResult struct {
	// ProtonOK is true when the resolved Proton build supports the
	// PROTON_VKD3D_HEAP marker, OR when the check could not be run
	// (ProtonSkip explains why). Callers that want to distinguish
	// "passed" from "skipped" should inspect ProtonSkip.
	ProtonOK bool
	// ProtonDetected is the human-readable build name, when resolvable.
	// Empty when the resolver returned an error before naming the build.
	ProtonDetected string
	// ProtonSkip is a human-readable reason the Proton check was skipped
	// (resolver error, unreadable marker probe). Empty when the check ran.
	ProtonSkip string

	// DriverOK is true when the parsed driver version meets or exceeds
	// MinDriverVersion, OR when the check could not be run. Inspect
	// DriverSkip to distinguish.
	DriverOK bool
	// DriverDetected is the canonical "MAJOR.MINOR.PATCH" driver string
	// when parsed successfully, or the raw input when parsing failed.
	DriverDetected string
	// DriverSkip is a human-readable reason the driver check was skipped
	// (no NVIDIA driver present, probe error). Empty when the check ran.
	DriverSkip string
}

// CheckCompatibility evaluates the vkd3d_heap environment for the given
// AppID using the injected dependencies and returns a structured result.
//
// Both axes are always evaluated (unless deps are misconfigured, in which
// case the zero-value result is returned with both OK flags false). The
// launcher preflight uses this raw shape to emit slog key/value pairs;
// CompatibilityNotice wraps it with a glyph-prefixed human string for CLI
// and TUI.
func CheckCompatibility(appID uint64, deps NoticeDeps) CompatibilityResult {
	if deps.ResolveForAppID == nil || deps.SupportsVKD3DHeap == nil || deps.DriverVersion == nil {
		// Misconfigured caller: return an "everything OK, nothing ran"
		// result so downstream logic stays silent rather than printing
		// alarming notices the user can't act on.
		return CompatibilityResult{ProtonOK: true, DriverOK: true}
	}

	protonOK, protonDetected, protonSkip := evaluateProton(appID, deps)
	driverOK, driverDetected, driverSkip := evaluateDriver(deps)

	return CompatibilityResult{
		ProtonOK:       protonOK,
		ProtonDetected: protonDetected,
		ProtonSkip:     protonSkip,
		DriverOK:       driverOK,
		DriverDetected: driverDetected,
		DriverSkip:     driverSkip,
	}
}

// CompatibilityNotice returns a human-readable notice describing any
// vkd3d_heap compatibility problem for the given AppID, using the
// injected NoticeDeps. An empty string means "compatible" or
// "checks skipped cleanly".
//
// The returned string is ready to display and includes a leading glyph:
//
//   - "⚠ " for hard incompatibility (Proton too old, driver too old, or both)
//   - "ⓘ " for an information-level skip (resolver error or driver absent)
//
// Callers (CLI and TUI) are expected to surface this string inline with
// the vkd3d_heap toggle when the toggle is enabled. When the toggle is
// disabled, callers should not invoke this helper at all.
func CompatibilityNotice(appID uint64, deps NoticeDeps) string {
	if deps.ResolveForAppID == nil || deps.SupportsVKD3DHeap == nil || deps.DriverVersion == nil {
		// Misconfigured caller — silently skip rather than showing a scary
		// notice for a situation the user can't act on.
		return ""
	}

	result := CheckCompatibility(appID, deps)

	// Info-level skip takes precedence only when no hard incompatibility
	// is detected on the other axis. If the driver is too old, we surface
	// that even if the Proton check was skipped (and vice versa).
	if result.ProtonSkip != "" && result.DriverOK && result.DriverSkip == "" {
		return "ⓘ " + result.ProtonSkip
	}
	if result.DriverSkip != "" && result.ProtonOK && result.ProtonSkip == "" {
		return "ⓘ " + result.DriverSkip
	}

	protonHard := !result.ProtonOK && result.ProtonSkip == ""
	driverHard := !result.DriverOK && result.DriverSkip == ""

	switch {
	case protonHard && driverHard:
		return fmt.Sprintf(
			"⚠ descriptor_heap requires Proton-CachyOS %s+ and NVIDIA driver %s+ (detected: %s, driver %s)",
			MinProtonCachyOSBuild, MinDriverVersion, result.ProtonDetected, result.DriverDetected,
		)
	case protonHard:
		return fmt.Sprintf(
			"⚠ descriptor_heap requires Proton-CachyOS %s+ (detected: %s)",
			MinProtonCachyOSBuild, result.ProtonDetected,
		)
	case driverHard:
		return fmt.Sprintf(
			"⚠ descriptor_heap requires NVIDIA driver %s+ (detected: %s)",
			MinDriverVersion, result.DriverDetected,
		)
	}
	return ""
}

// evaluateProton resolves the active Proton build for appID and checks
// the PROTON_VKD3D_HEAP marker. Returns (ok, detectedName, skipReason).
// skipReason is non-empty when the check could not be performed at all
// (resolver error); in that case ok is true so the caller only surfaces
// the skip when no other incompatibility is present.
func evaluateProton(appID uint64, deps NoticeDeps) (ok bool, detected string, skip string) {
	build, err := deps.ResolveForAppID(deps.SteamRoot, appID)
	if err != nil {
		if errors.Is(err, ErrProtonNotResolved) {
			return true, "", "could not resolve active Proton for this game; skipping compatibility check"
		}
		return true, "", "Proton resolution failed; skipping compatibility check"
	}
	supported, err := deps.SupportsVKD3DHeap(build)
	if err != nil {
		return true, build.Name, "Proton marker probe failed; skipping compatibility check"
	}
	return supported, build.Name, ""
}

// evaluateDriver parses the NVIDIA driver version and checks it against
// the documented minimum. Returns (ok, detectedString, skipReason).
// skipReason is non-empty when no NVIDIA driver is reachable.
func evaluateDriver(deps NoticeDeps) (ok bool, detected string, skip string) {
	raw, err := deps.DriverVersion()
	if err != nil {
		return true, "", "NVIDIA driver probe failed; skipping driver compatibility check"
	}
	v, err := ParseDriverVersion(raw)
	if err != nil {
		if errors.Is(err, ErrDriverUnavailable) {
			return true, "", "NVIDIA driver not detected; skipping driver compatibility check"
		}
		return true, raw, "NVIDIA driver version unparsable; skipping driver compatibility check"
	}
	if !v.Available {
		return true, "", "NVIDIA driver not detected; skipping driver compatibility check"
	}
	return v.MeetsMinimum(), v.String(), ""
}
