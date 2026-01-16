package profile

import (
	"fmt"
	"strings"

	"github.com/jgabor/spela/internal/env"
	"github.com/jgabor/spela/internal/xdg"
)

func (p *Profile) Apply(e *env.Environment) []func() {
	var cleanup []func()

	cleanup = append(cleanup, p.applyProton(e)...)
	cleanup = append(cleanup, p.applyDLSS(e)...)
	cleanup = append(cleanup, p.applyGPU(e)...)

	return cleanup
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
	return nil
}

func (p *Profile) applyDLSS(e *env.Environment) []func() {
	if p.DLSS.SROverride {
		e.Set("DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE", "on")
		if p.DLSS.SRMode != "" {
			e.Set("DXVK_NVAPI_DRS_NGX_DLSS_SR_MODE", dlssModeToEnv(p.DLSS.SRMode))
		}
		if p.DLSS.SRPreset != "" {
			e.Set("DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION", dlssPresetToEnv(p.DLSS.SRPreset))
		}
	}

	if p.DLSS.RROverride {
		e.Set("DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE", "on")
		if p.DLSS.RRMode != "" {
			e.Set("DXVK_NVAPI_DRS_NGX_DLSS_RR_MODE", dlssModeToEnv(p.DLSS.RRMode))
		}
		if p.DLSS.RRPreset != "" {
			e.Set("DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE_RENDER_PRESET_SELECTION", dlssPresetToEnv(p.DLSS.RRPreset))
		}
	}

	if p.DLSS.FGOverride {
		e.Set("DXVK_NVAPI_DRS_NGX_DLSS_FG_OVERRIDE", "on")
		if p.DLSS.FGEnabled {
			e.Set("DXVK_NVAPI_DRS_NGX_DLSSG_MULTI_FRAME_COUNT", fmt.Sprintf("%d", p.DLSS.MultiFrame))
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

func dlssModeToEnv(mode DLSSMode) string {
	switch mode {
	case DLSSModeUltraPerformance:
		return "ultra_performance"
	case DLSSModePerformance:
		return "performance"
	case DLSSModeBalanced:
		return "balanced"
	case DLSSModeQuality:
		return "quality"
	case DLSSModeDLAA:
		return "dlaa"
	default:
		return string(mode)
	}
}

func dlssPresetToEnv(preset DLSSPreset) string {
	switch preset {
	case DLSSPresetLatest:
		return "render_preset_latest"
	case DLSSPresetA, DLSSPresetB, DLSSPresetC, DLSSPresetD:
		return "render_preset_" + strings.ToLower(string(preset))
	default:
		return "render_preset_default"
	}
}
