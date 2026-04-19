package proton

import (
	"errors"
	"strings"
	"testing"
)

// stubDeps builds a NoticeDeps populated with controllable fakes.
//
// protonName is the build name returned by the resolver (or "" to simulate
// ErrProtonNotResolved). supports toggles the VKD3DHeap marker result.
// driver is the raw driver string the driver-version probe returns; an
// explicit error overrides it.
func stubDeps(protonName string, supports bool, driver string, resolveErr, driverErr error) NoticeDeps {
	return NoticeDeps{
		SteamRoot: "/fake/steam",
		ResolveForAppID: func(root string, appID uint64) (Build, error) {
			if resolveErr != nil {
				return Build{}, resolveErr
			}
			if protonName == "" {
				return Build{}, ErrProtonNotResolved
			}
			return Build{Name: protonName, Path: "/fake/build"}, nil
		},
		SupportsVKD3DHeap: func(b Build) (bool, error) {
			return supports, nil
		},
		DriverVersion: func() (string, error) {
			return driver, driverErr
		},
	}
}

func TestCompatibilityNotice_AllCompatible(t *testing.T) {
	deps := stubDeps("cachyos-10.0-20260410-slr", true, "580.94.16", nil, nil)
	got := CompatibilityNotice(1091500, deps)
	if got != "" {
		t.Errorf("expected empty notice when compatible, got %q", got)
	}
}

func TestCompatibilityNotice_ProtonIncompatible(t *testing.T) {
	deps := stubDeps("GE-Proton10-34", false, "580.94.16", nil, nil)
	got := CompatibilityNotice(1091500, deps)
	want := "⚠ descriptor_heap requires Proton-CachyOS 10.0-20260321+ (detected: GE-Proton10-34)"
	if got != want {
		t.Errorf("notice mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestCompatibilityNotice_DriverIncompatible(t *testing.T) {
	deps := stubDeps("cachyos-10.0-20260410-slr", true, "570.86", nil, nil)
	got := CompatibilityNotice(1091500, deps)
	want := "⚠ descriptor_heap requires NVIDIA driver 580.94.16+ (detected: 570.86.0)"
	if got != want {
		t.Errorf("notice mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestCompatibilityNotice_BothIncompatible(t *testing.T) {
	deps := stubDeps("GE-Proton10-34", false, "570.86", nil, nil)
	got := CompatibilityNotice(1091500, deps)
	want := "⚠ descriptor_heap requires Proton-CachyOS 10.0-20260321+ and NVIDIA driver 580.94.16+ (detected: GE-Proton10-34, driver 570.86.0)"
	if got != want {
		t.Errorf("notice mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestCompatibilityNotice_ResolverErrorAndDriverOK(t *testing.T) {
	deps := stubDeps("", true, "580.94.16", ErrProtonNotResolved, nil)
	got := CompatibilityNotice(1091500, deps)
	if !strings.HasPrefix(got, "ⓘ ") {
		t.Errorf("expected info-level notice, got %q", got)
	}
	if !strings.Contains(got, "could not resolve active Proton") {
		t.Errorf("expected resolver-skip message, got %q", got)
	}
}

func TestCompatibilityNotice_DriverUnavailableAndProtonOK(t *testing.T) {
	deps := stubDeps("cachyos-10.0-20260410-slr", true, "", nil, nil)
	got := CompatibilityNotice(1091500, deps)
	if !strings.HasPrefix(got, "ⓘ ") {
		t.Errorf("expected info-level notice, got %q", got)
	}
	if !strings.Contains(got, "NVIDIA driver not detected") {
		t.Errorf("expected driver-skip message, got %q", got)
	}
}

func TestCompatibilityNotice_ResolverErrorButDriverTooOld(t *testing.T) {
	// When both checks have something to say, hard incompatibilities
	// take precedence over info-level skips.
	deps := stubDeps("", true, "570.86", ErrProtonNotResolved, nil)
	got := CompatibilityNotice(1091500, deps)
	if strings.HasPrefix(got, "ⓘ ") {
		t.Errorf("expected hard ⚠ notice (driver too old), got info %q", got)
	}
	if !strings.Contains(got, "580.94.16") {
		t.Errorf("expected driver minimum in notice, got %q", got)
	}
}

func TestCompatibilityNotice_ProtonProbeError_InfoSkip(t *testing.T) {
	// Simulate a non-sentinel resolver error (e.g. permission denied).
	deps := stubDeps("", true, "580.94.16", errors.New("boom"), nil)
	got := CompatibilityNotice(1091500, deps)
	if !strings.HasPrefix(got, "ⓘ ") {
		t.Errorf("expected info-level notice for non-sentinel resolver error, got %q", got)
	}
}

func TestCompatibilityNotice_NilDeps_ReturnsEmpty(t *testing.T) {
	got := CompatibilityNotice(1091500, NoticeDeps{})
	if got != "" {
		t.Errorf("expected empty notice with zero-value deps, got %q", got)
	}
}
