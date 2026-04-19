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

	protonOK, protonDetected, protonSkip := evaluateProton(appID, deps)
	driverOK, driverDetected, driverSkip := evaluateDriver(deps)

	// Info-level skip takes precedence only when no hard incompatibility
	// is detected on the other axis. If the driver is too old, we surface
	// that even if the Proton check was skipped (and vice versa).
	if protonSkip != "" && driverOK {
		return "ⓘ " + protonSkip
	}
	if driverSkip != "" && protonOK {
		return "ⓘ " + driverSkip
	}

	switch {
	case !protonOK && !driverOK:
		return fmt.Sprintf(
			"⚠ descriptor_heap requires Proton-CachyOS %s+ and NVIDIA driver %s+ (detected: %s, driver %s)",
			MinProtonCachyOSBuild, MinDriverVersion, protonDetected, driverDetected,
		)
	case !protonOK:
		return fmt.Sprintf(
			"⚠ descriptor_heap requires Proton-CachyOS %s+ (detected: %s)",
			MinProtonCachyOSBuild, protonDetected,
		)
	case !driverOK:
		return fmt.Sprintf(
			"⚠ descriptor_heap requires NVIDIA driver %s+ (detected: %s)",
			MinDriverVersion, driverDetected,
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
