package profile

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/env"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/logging"
	"github.com/jgabor/spela/internal/privilege"
	"github.com/jgabor/spela/internal/xdg"
)

// ApplyEnv applies only environment variable settings from the profile,
// without touching hardware or creating cleanup closures. Used by dry-run.
func (p *Profile) ApplyEnv(e *env.Environment) {
	p.applyProton(e)
	p.applyDLSS(e)
	p.applyGPU(e)
}

func (p *Profile) Apply(e *env.Environment) []func() {
	var cleanup []func()

	cleanup = append(cleanup, p.applyProton(e)...)
	cleanup = append(cleanup, p.applyDLSS(e)...)
	cleanup = append(cleanup, p.applyGPU(e)...)

	if hwCleanup, err := p.applyHardware(); err != nil {
		logging.Warn("failed to apply hardware settings", "error", err)
	} else if hwCleanup != nil {
		cleanup = append(cleanup, hwCleanup)
	}

	return cleanup
}

// needsHardwareApply reports whether the profile has any privileged hardware
// settings that require elevation.
func (p *Profile) needsHardwareApply() bool {
	return p.GPU.ClockOffset != 0 ||
		p.GPU.MemoryOffset != 0 ||
		p.GPU.PowerLimit > 0 ||
		p.GPU.FanSpeed > 0 ||
		p.CPU.Governor != "" ||
		p.CPU.SMT != nil
}

// applyHardware applies privileged GPU/CPU settings via a single pkexec
// round-trip to spela apply-profile. Returns a cleanup function that restores
// the previous settings on game exit.
func (p *Profile) applyHardware() (func(), error) {
	if !p.needsHardwareApply() {
		return nil, nil
	}

	// Capture current state for restoration.
	prevGovernor, _ := cpu.GetCurrentGovernor()
	prevSMT, _ := cpu.GetSMTStatus()
	prevPowerLimit, _ := gpu.GetCurrentPowerLimit()

	args := []string{"apply-profile"}

	if p.GPU.ClockOffset != 0 {
		args = append(args, fmt.Sprintf("--gpu-clock-offset=%d", p.GPU.ClockOffset))
	}
	if p.GPU.MemoryOffset != 0 {
		args = append(args, fmt.Sprintf("--gpu-memory-offset=%d", p.GPU.MemoryOffset))
	}
	if p.GPU.PowerLimit > 0 {
		args = append(args, fmt.Sprintf("--gpu-power-limit=%d", p.GPU.PowerLimit))
	}
	if p.GPU.FanSpeed > 0 {
		args = append(args, fmt.Sprintf("--gpu-fan-speed=%d", p.GPU.FanSpeed))
	}
	if p.CPU.Governor != "" {
		args = append(args, fmt.Sprintf("--cpu-governor=%s", p.CPU.Governor))
	}
	if p.CPU.SMT != nil {
		value := "off"
		if *p.CPU.SMT {
			value = "on"
		}
		args = append(args, fmt.Sprintf("--cpu-smt=%s", value))
	}

	if _, err := privilege.ExecSelf(args...); err != nil {
		return nil, fmt.Errorf("apply hardware settings: %w", err)
	}

	cleanup := func() {
		resetArgs := []string{"apply-profile", "--reset"}
		if p.GPU.PowerLimit > 0 && prevPowerLimit > 0 {
			resetArgs = append(resetArgs, fmt.Sprintf("--gpu-power-limit=%d", prevPowerLimit))
		}
		if p.GPU.FanSpeed > 0 {
			resetArgs = append(resetArgs, "--gpu-fan-speed=0") // 0 signals reset to auto
		}
		if p.CPU.Governor != "" && prevGovernor != "" {
			resetArgs = append(resetArgs, fmt.Sprintf("--cpu-governor=%s", prevGovernor))
		}
		if p.CPU.SMT != nil {
			value := "off"
			if prevSMT {
				value = "on"
			}
			resetArgs = append(resetArgs, fmt.Sprintf("--cpu-smt=%s", value))
		}
		if _, err := privilege.ExecSelf(resetArgs...); err != nil {
			logging.Warn("failed to restore hardware settings", "error", err)
		}
	}

	return cleanup, nil
}

func (p *Profile) applyProton(e *env.Environment) []func() {
	if p.Proton.EnableWayland {
		e.EnableWayland()
	}
	if p.Proton.EnableHDR {
		e.EnableHDR()
	}
	if p.Proton.EnableNGXUpdater {
		e.EnableNGXUpdater()
	}
	if p.Proton.VKD3DHeap {
		e.EnableVKD3DHeap()
	}
	return nil
}

func (p *Profile) applyDLSS(e *env.Environment) []func() {
	if p.DLSS.SROverride {
		e.Set("DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE", "on")
		if p.DLSS.SRMode != "" {
			e.Set("DXVK_NVAPI_DRS_NGX_DLSS_SR_MODE", string(p.DLSS.SRMode))
		}
		if p.DLSS.SRModelPreset != "" {
			preset := resolveModelPreset(p.DLSS.SRModelPreset, p.DLSS.SRMode)
			e.Set("DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION", dlssModelPresetToEnv(preset))
		} else if p.DLSS.SRPreset != "" {
			e.Set("DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION", dlssPresetToEnv(p.DLSS.SRPreset))
		}
	}

	if p.DLSS.RROverride {
		e.Set("DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE", "on")
		if p.DLSS.RRMode != "" {
			e.Set("DXVK_NVAPI_DRS_NGX_DLSS_RR_MODE", string(p.DLSS.RRMode))
		}
		if p.DLSS.RRPreset != "" {
			e.Set("DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE_RENDER_PRESET_SELECTION", dlssPresetToEnv(p.DLSS.RRPreset))
		}
	}

	if p.DLSS.FGOverride {
		e.Set("DXVK_NVAPI_DRS_NGX_DLSS_FG_OVERRIDE", "on")
		if p.DLSS.FGEnabled {
			e.Set("DXVK_NVAPI_DRS_NGX_DLSSG_MULTI_FRAME_COUNT", strconv.Itoa(p.DLSS.MultiFrame))
		}
	}

	var debugOpts []string
	if p.DLSS.Indicator {
		debugOpts = append(debugOpts, "DLSSIndicator=1024")
	}
	if p.DLSS.FGIndicator {
		debugOpts = append(debugOpts, "DLSSGIndicator=2")
	}
	if len(debugOpts) > 0 {
		e.Set("DXVK_NVAPI_SET_NGX_DEBUG_OPTIONS", strings.Join(debugOpts, ","))
	}

	return nil
}

func (p *Profile) applyGPU(e *env.Environment) []func() {
	if p.GPU.ShaderCache {
		cachePath := p.GPU.ShaderCachePath
		if cachePath == "" {
			cachePath = xdg.CachePath("nvidia")
		}
		e.SetShaderCache(cachePath)
	}

	e.SetThreadedOptimization(p.GPU.ThreadedOptimization)

	return nil
}

func dlssPresetToEnv(preset DLSSPreset) string {
	switch preset {
	case DLSSPresetA, DLSSPresetB, DLSSPresetC, DLSSPresetD, DLSSPresetE, DLSSPresetF, DLSSPresetJ, DLSSPresetK, DLSSPresetL, DLSSPresetM:
		return "render_preset_" + strings.ToLower(string(preset))
	default:
		return "render_preset_default"
	}
}

func resolveModelPreset(modelPreset DLSSModelPreset, srMode DLSSMode) DLSSModelPreset {
	if modelPreset != DLSSModelPresetAuto {
		return modelPreset
	}
	switch srMode {
	case DLSSModeUltraPerformance:
		return DLSSModelPresetL
	case DLSSModePerformance:
		return DLSSModelPresetM
	default:
		return DLSSModelPresetK
	}
}

func dlssModelPresetToEnv(preset DLSSModelPreset) string {
	switch preset {
	case DLSSModelPresetK:
		return "render_preset_k"
	case DLSSModelPresetL:
		return "render_preset_l"
	case DLSSModelPresetM:
		return "render_preset_m"
	default:
		return "render_preset_k"
	}
}
