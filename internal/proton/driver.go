package proton

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrDriverUnavailable is returned by ParseDriverVersion when the raw
// driver string is empty. This typically means the system has no NVIDIA
// GPU or NVML/nvidia-smi is not installed. Callers should treat this
// distinctly from "parsed but too old".
var ErrDriverUnavailable = errors.New("driver version unavailable")

// DriverVersion is a parsed NVIDIA driver version. It is comparable
// component-by-component via Compare. Zero value represents "unavailable".
//
// NVIDIA reports driver versions in several shapes:
//   - three-component "580.94.16" (typical nvidia-smi output)
//   - two-component "580.94" (some NVML return paths, older builds)
//   - beta-ish "585.0.0" (leading-zero / prototype builds)
//   - whitespace-padded "  580.94.16  " (nvidia-smi fallback paths)
//
// All four shapes must parse without panic and compare correctly.
type DriverVersion struct {
	Major int
	Minor int
	Patch int
	// Available is true when parsing succeeded from a non-empty input.
	// The zero value (Available=false) represents a non-NVIDIA system.
	Available bool
	// Raw preserves the original trimmed input for diagnostic display.
	Raw string
}

// ParseDriverVersion parses an NVIDIA driver version string into a
// comparable DriverVersion. Whitespace is trimmed. Accepts 1-3 numeric
// components; missing components default to zero. Non-numeric components
// produce a typed error rather than a panic.
//
// An empty input (or whitespace-only) returns ErrDriverUnavailable with
// a zero-value DriverVersion, signalling a non-NVIDIA or unprobed system.
func ParseDriverVersion(raw string) (DriverVersion, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return DriverVersion{}, ErrDriverUnavailable
	}

	parts := strings.Split(trimmed, ".")
	if len(parts) == 0 || len(parts) > 4 {
		return DriverVersion{Raw: trimmed}, fmt.Errorf("driver version %q has %d components, want 1-4", trimmed, len(parts))
	}

	nums := make([]int, len(parts))
	for i, p := range parts {
		p = strings.TrimSpace(p)
		n, err := strconv.Atoi(p)
		if err != nil {
			return DriverVersion{Raw: trimmed}, fmt.Errorf("driver version %q component %d %q: %w", trimmed, i, p, err)
		}
		nums[i] = n
	}

	v := DriverVersion{Available: true, Raw: trimmed}
	if len(nums) > 0 {
		v.Major = nums[0]
	}
	if len(nums) > 1 {
		v.Minor = nums[1]
	}
	if len(nums) > 2 {
		v.Patch = nums[2]
	}
	// A fourth component (rare) is ignored; it does not affect vkd3d-proton
	// compatibility ordering.
	return v, nil
}

// Compare returns -1/0/1 for v vs other using Major.Minor.Patch ordering.
// An Available=false receiver compares as less than any Available=true
// other, and equal to another unavailable.
func (v DriverVersion) Compare(other DriverVersion) int {
	if !v.Available && !other.Available {
		return 0
	}
	if !v.Available {
		return -1
	}
	if !other.Available {
		return 1
	}
	switch {
	case v.Major != other.Major:
		if v.Major < other.Major {
			return -1
		}
		return 1
	case v.Minor != other.Minor:
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	case v.Patch != other.Patch:
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}
	return 0
}

// MeetsMinimum reports whether v is at least MinDriverVersion. An
// unavailable DriverVersion never meets the minimum; callers should
// branch on Available first if they want to distinguish "no NVIDIA"
// from "too old".
func (v DriverVersion) MeetsMinimum() bool {
	if !v.Available {
		return false
	}
	mv, err := ParseDriverVersion(MinDriverVersion)
	if err != nil {
		// MinDriverVersion is a package constant we control; a parse
		// error here is a programmer error, not a runtime condition.
		return false
	}
	return v.Compare(mv) >= 0
}

// String renders the driver version in canonical "MAJOR.MINOR.PATCH"
// form, or "unavailable" for a zero-value receiver.
func (v DriverVersion) String() string {
	if !v.Available {
		return "unavailable"
	}
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}
