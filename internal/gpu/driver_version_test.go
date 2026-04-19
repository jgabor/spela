package gpu

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

// TestDriverVersionString_NVMLHappyPath exercises the fast path: NVML
// initialized, driver query succeeds, trimmed string returned. Skips on
// machines without NVML so the suite stays green in CI / non-GPU hosts.
func TestDriverVersionString_NVMLHappyPath(t *testing.T) {
	if nvml.Init() != nvml.SUCCESS {
		t.Skip("NVML not available")
	}
	_ = nvml.Shutdown()

	Init()
	defer Shutdown()

	if !nvmlAvailable {
		t.Skip("NVML initialization failed")
	}

	got, err := DriverVersionString()
	if err != nil {
		t.Fatalf("DriverVersionString() returned error: %v", err)
	}
	if got == "" {
		t.Fatal("DriverVersionString() returned empty string on NVML-capable system")
	}
	if got != strings.TrimSpace(got) {
		t.Errorf("DriverVersionString() not whitespace-trimmed: %q", got)
	}
	// Sanity: must look like a dotted numeric version, not an NVML
	// constant name or empty placeholder.
	if !strings.ContainsRune(got, '.') {
		t.Errorf("DriverVersionString() = %q, expected dotted version", got)
	}
}

// TestDriverVersionString_FallbackBranch forces the nvidia-smi fallback
// path by disabling the NVML flag. The assertion is contract-level: the
// function must not panic and must return either a version string or
// ("", nil) depending on whether nvidia-smi exists on the host.
func TestDriverVersionString_FallbackBranch(t *testing.T) {
	saved := nvmlAvailable
	nvmlAvailable = false
	defer func() { nvmlAvailable = saved }()

	got, err := DriverVersionString()
	if err != nil {
		t.Fatalf("DriverVersionString() must never return a typed error today, got: %v", err)
	}

	// If nvidia-smi is not installed the contract says ("", nil).
	// If it is installed and returns a version the value must be
	// whitespace-trimmed and non-empty.
	if _, lookErr := exec.LookPath("nvidia-smi"); lookErr != nil {
		if got != "" {
			t.Errorf("expected empty string when nvidia-smi absent, got %q", got)
		}
		return
	}
	if got != "" && got != strings.TrimSpace(got) {
		t.Errorf("DriverVersionString() not whitespace-trimmed: %q", got)
	}
}

// TestDriverVersionString_NoPanicZeroInit verifies the function is safe
// to call before Init(), which several preflight paths do. This guards
// the documented "safe to call before gpu.Init()" contract.
func TestDriverVersionString_NoPanicZeroInit(t *testing.T) {
	saved := nvmlAvailable
	nvmlAvailable = false
	defer func() { nvmlAvailable = saved }()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("DriverVersionString() panicked with pre-Init state: %v", r)
		}
	}()

	_, _ = DriverVersionString()
}
