package env

import (
	"os"
	"os/exec"
)

type Environment struct {
	vars map[string]string
}

func New() *Environment {
	return &Environment{
		vars: make(map[string]string),
	}
}

func (e *Environment) Set(key, value string) {
	e.vars[key] = value
}

func (e *Environment) SetIf(key, value string, condition bool) {
	if condition {
		e.vars[key] = value
	}
}

func (e *Environment) Get(key string) string {
	return e.vars[key]
}

func (e *Environment) All() map[string]string {
	result := make(map[string]string, len(e.vars))
	for k, v := range e.vars {
		result[k] = v
	}
	return result
}

func (e *Environment) Apply() {
	for k, v := range e.vars {
		os.Setenv(k, v)
	}
}

func (e *Environment) BuildEnv() []string {
	env := os.Environ()
	for k, v := range e.vars {
		env = append(env, k+"="+v)
	}
	return env
}

func (e *Environment) ApplyToCmd(cmd *exec.Cmd) {
	cmd.Env = e.BuildEnv()
}

func (e *Environment) EnableWayland() {
	e.Set("PROTON_ENABLE_WAYLAND", "1")
}

func (e *Environment) EnableHDR() {
	e.Set("PROTON_ENABLE_HDR", "1")
}

func (e *Environment) EnableNGXUpdater() {
	e.Set("PROTON_ENABLE_NGX_UPDATER", "1")
}

func (e *Environment) SetShaderCache(path string) {
	e.Set("__GL_SHADER_DISK_CACHE", "1")
	e.Set("__GL_SHADER_DISK_CACHE_PATH", path)
}

func (e *Environment) SetDXVKCache(path string) {
	e.Set("DXVK_STATE_CACHE_PATH", path)
}

func (e *Environment) SetThreadedOptimization(enabled bool) {
	if enabled {
		e.Set("__GL_THREADED_OPTIMIZATION", "1")
	} else {
		e.Set("__GL_THREADED_OPTIMIZATION", "0")
	}
}

func (e *Environment) SetDXVKConfigFile(path string) {
	e.Set("DXVK_CONFIG_FILE", path)
}
