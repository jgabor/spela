// Package gpu exposes NVIDIA GPU metric and control primitives, backed
// by NVML with an nvidia-smi fallback. Callers get structured metrics,
// clock/power control, and the driver version lookup used by preflight
// compatibility checks.
package gpu

// Driver version lookup lives in its own file because it is the cheapest
// NVIDIA probe spela performs — no device handle, no metric sweep — and
// is consumed by preflight checks that may run before the wider GPU
// subsystem has been initialized. Isolating it makes the API surface,
// contract, and error model obvious to callers.

import (
	"strings"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

// DriverVersionString returns the raw NVIDIA driver version string (e.g.
// "580.94.16"), obtained via NVML when available and falling back to
// nvidia-smi otherwise.
//
// Contract:
//
//   - Returns (version, nil) on success. The returned string is
//     whitespace-trimmed and preserves NVIDIA's native formatting,
//     which may be two- or three-component depending on the driver
//     build and query path. Pass it through proton.ParseDriverVersion
//     for structured comparison.
//   - Returns ("", nil) when no NVIDIA driver is detected (no NVML and
//     nvidia-smi missing or failing). Callers that must distinguish
//     "non-NVIDIA system" from "probe error" should treat empty-string
//     as the former; no error is returned because the absence of an
//     NVIDIA driver is an expected state on many Linux installs.
//   - Returns ("", err) is reserved for future use; today the function
//     does not surface NVML or nvidia-smi errors because none of the
//     callers would act differently on a probe failure vs. a missing
//     driver. The signature leaves room to tighten this later without
//     breaking the proton.NoticeDeps contract.
//
// The function is safe to call before gpu.Init(); when NVML has not been
// initialized it transparently falls back to nvidia-smi. It is also
// safe on non-NVIDIA systems and on machines where nvidia-smi has been
// uninstalled.
func DriverVersionString() (string, error) {
	if nvmlAvailable {
		if driver, ret := nvml.SystemGetDriverVersion(); ret == nvml.SUCCESS {
			return strings.TrimSpace(driver), nil
		}
		// NVML is up but the driver query failed — fall through to
		// nvidia-smi. This matches the behaviour of GetGPUInfo and
		// keeps a single code path for "NVML present but flaky".
	}
	out, err := runNvidiaSMI("--query-gpu=driver_version", "--format=csv,noheader,nounits")
	if err != nil {
		// nvidia-smi absent or failed — treat as "no NVIDIA driver".
		// See the contract notes above for why this is (nil) rather
		// than a typed error.
		return "", nil
	}
	return strings.TrimSpace(out), nil
}
