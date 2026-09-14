package profile

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/display"
	"github.com/jgabor/spela/internal/env"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/logging"
	"github.com/jgabor/spela/internal/privilege"
	"github.com/jgabor/spela/internal/xdg"
)

// Cleanup describes a restorable mutation that should be reverted after launch.
type Cleanup struct {
	Area string
	Run  func() error
}

type profileHardwareOperations struct {
	currentGovernor func() (cpu.Governor, error)
	smtStatus       func() (bool, error)
	powerLimit      func() (int, error)
	execSelf        func(...string) (*privilege.ExecResult, error)
}

// ApplyEnv applies only environment variable settings from the profile,
// without touching hardware or creating cleanup closures. Used by dry-run.
func (p *Profile) ApplyEnv(e *env.Environment) {
	p.applyProton(e)
	p.applyDLSS(e)
	p.applyGPU(e)
}

func (p *Profile) Apply(e *env.Environment) []Cleanup {
	var cleanup []Cleanup

	cleanup = append(cleanup, p.applyProton(e)...)
	cleanup = append(cleanup, p.applyDLSS(e)...)
	cleanup = append(cleanup, p.applyGPU(e)...)

	// Display policy belongs to the ordinary desktop session, not pkexec.
	restoreDisplay, err := display.ApplyVRR(p.GPU.VRR)
	if err != nil {
		logging.Warn("failed to apply KDE VRR policy", "error", err)
	}
	if restoreDisplay != nil {
		cleanup = append(cleanup, Cleanup{Area: "KDE VRR", Run: restoreDisplay})
	}

	if hwCleanup, err := p.applyHardware(); err != nil {
		logging.Warn("failed to apply hardware settings", "error", err)
	} else if hwCleanup.Run != nil {
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
func (p *Profile) applyHardware() (Cleanup, error) {
	return p.applyHardwareWithOperations(profileHardwareOperations{
		currentGovernor: cpu.GetCurrentGovernor,
		smtStatus:       cpu.GetSMTStatus,
		powerLimit:      gpu.GetCurrentPowerLimit,
		execSelf:        privilege.ExecSelf,
	})
}

func (p *Profile) applyHardwareWithOperations(operations profileHardwareOperations) (Cleanup, error) {
	if !p.needsHardwareApply() {
		return Cleanup{}, nil
	}

	// Capture current state for restoration.
	prevGovernor, _ := operations.currentGovernor()
	prevSMT, _ := operations.smtStatus()
	prevPowerLimit, _ := operations.powerLimit()

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

	if _, err := operations.execSelf(args...); err != nil {
		return Cleanup{}, fmt.Errorf("apply hardware settings: %w", err)
	}

	cleanup := Cleanup{Area: "hardware", Run: func() error {
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
		if _, err := operations.execSelf(resetArgs...); err != nil {
			return fmt.Errorf("restore hardware settings: %w", err)
		}
		return nil
	}}

	return cleanup, nil
}

func (p *Profile) applyProton(e *env.Environment) []Cleanup {
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
		e.EnableVKD3DHeap(false)
	}
	return nil
}

func (p *Profile) applyDLSS(e *env.Environment) []Cleanup {
	if p.DLSS.SROverride {
		e.Set("DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE", "on")
		if p.DLSS.SRMode != "" {
			e.Set("DXVK_NVAPI_DRS_NGX_DLSS_SR_MODE", string(p.DLSS.SRMode))
		}
		if p.DLSS.SRPreset != "" {
			e.Set("DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION", dlssSRPresetToEnv(p.DLSS.SRPreset, p.DLSS.SRMode))
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

func (p *Profile) applyGPU(e *env.Environment) []Cleanup {
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

func dlssSRPresetToEnv(preset DLSSPreset, mode DLSSMode) string {
	if preset == DLSSPresetAuto {
		preset = resolveAutoSRPreset(mode)
	}
	return dlssPresetToEnv(preset)
}

func resolveAutoSRPreset(mode DLSSMode) DLSSPreset {
	switch mode {
	case DLSSModeUltraPerformance:
		return DLSSPresetL
	case DLSSModePerformance:
		return DLSSPresetM
	default:
		return DLSSPresetK
	}
}

func dlssPresetToEnv(preset DLSSPreset) string {
	switch preset {
	case DLSSPresetA, DLSSPresetB, DLSSPresetC, DLSSPresetD, DLSSPresetE, DLSSPresetF, DLSSPresetJ, DLSSPresetK, DLSSPresetL, DLSSPresetM:
		return "render_preset_" + strings.ToLower(string(preset))
	default:
		return "render_preset_default"
	}
}
