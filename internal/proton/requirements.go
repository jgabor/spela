// Package proton provides Proton build resolution, marker detection,
// and NVIDIA driver version gating for per-game descriptor_heap support.
//
// It walks Steam's compat-tool resolution chain to identify which Proton
// build a given AppID will launch with, checks that build for the
// PROTON_VKD3D_HEAP marker, and parses NVIDIA driver version strings
// in the several shapes reported by NVML and nvidia-smi.
//
// The package is pure-stdlib plus internal/steam for VDF parsing. It
// exposes no I/O side effects beyond filesystem reads under the Steam
// root and does not depend on NVML; driver strings are passed in by
// the caller (typically the launcher's preflight).
package proton

// Minimum versions required for the PROTON_VKD3D_HEAP + VKD3D_CONFIG=descriptor_heap
// feature to actually take effect at runtime.
//
// These constants are centralized so that a single freshness audit
// (inspektera) can verify them against upstream state without grepping
// the codebase. When updating, refresh the citation comment below.
const (
	// MinDriverVersion is the first NVIDIA driver build shipping the
	// VK_EXT_descriptor_heap extension (Vulkan 1.4.340, Jan 2026).
	// Source: NVIDIA 580.94.16 beta release notes; vkd3d-proton PR #2805
	// added the code path that consumes the extension and documents
	// 580.94.16 as the first driver on which it works.
	MinDriverVersion = "580.94.16"

	// MinProtonCachyOSBuild is the first proton-cachyos release shipping
	// the PROTON_VKD3D_HEAP prototype. Format matches the upstream build
	// tag ("MAJOR.MINOR-YYYYMMDD"). Source: proton-cachyos release
	// 10.0-20260321 on GitHub (cachyos/proton-cachyos-slim) — the
	// changelog entry introduces the PROTON_VKD3D_HEAP env var.
	MinProtonCachyOSBuild = "10.0-20260321"
)
