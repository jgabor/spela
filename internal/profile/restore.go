package profile

import (
	"os"
	"sync"
)

type RestorePoint struct {
	mu       sync.Mutex
	envVars  map[string]string
	cleanups []func()
}

func NewRestorePoint() *RestorePoint {
	return &RestorePoint{
		envVars: make(map[string]string),
	}
}

func (r *RestorePoint) SaveEnv(keys ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, key := range keys {
		if _, exists := r.envVars[key]; !exists {
			r.envVars[key] = os.Getenv(key)
		}
	}
}

func (r *RestorePoint) AddCleanup(fn func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cleanups = append(r.cleanups, fn)
}

func (r *RestorePoint) Restore() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, value := range r.envVars {
		if value == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, value)
		}
	}

	for i := len(r.cleanups) - 1; i >= 0; i-- {
		r.cleanups[i]()
	}

	r.envVars = make(map[string]string)
	r.cleanups = nil
}

var dlssEnvVars = []string{
	"DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE",
	"DXVK_NVAPI_DRS_NGX_DLSS_SR_MODE",
	"DXVK_NVAPI_DRS_NGX_DLSS_SR_OVERRIDE_RENDER_PRESET_SELECTION",
	"DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE",
	"DXVK_NVAPI_DRS_NGX_DLSS_RR_MODE",
	"DXVK_NVAPI_DRS_NGX_DLSS_RR_OVERRIDE_RENDER_PRESET_SELECTION",
	"DXVK_NVAPI_DRS_NGX_DLSS_FG_OVERRIDE",
	"DXVK_NVAPI_DRS_NGX_DLSSG_MULTI_FRAME_COUNT",
	"DXVK_NVAPI_SET_NGX_DEBUG_OPTIONS",
}

var protonEnvVars = []string{
	"PROTON_ENABLE_WAYLAND",
	"PROTON_ENABLE_HDR",
	"PROTON_ENABLE_NGX_UPDATER",
}

var gpuEnvVars = []string{
	"__GL_SHADER_DISK_CACHE",
	"__GL_SHADER_DISK_CACHE_PATH",
	"__GL_THREADED_OPTIMIZATION",
	"DXVK_STATE_CACHE_PATH",
}

func (r *RestorePoint) SaveAllProfileEnvVars() {
	r.SaveEnv(dlssEnvVars...)
	r.SaveEnv(protonEnvVars...)
	r.SaveEnv(gpuEnvVars...)
}
