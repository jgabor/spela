package env

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestApplyToCmdAddsEnvironment(t *testing.T) {
	t.Setenv("SPELA_PARENT", "keep")
	e := New()
	e.Set("SPELA_CHILD", "set")
	cmd := exec.Command("env")

	e.ApplyToCmd(cmd)

	if !envContains(cmd.Env, "SPELA_PARENT=keep") {
		t.Fatal("command env lost parent environment")
	}
	if !envContains(cmd.Env, "SPELA_CHILD=set") {
		t.Fatal("command env missing applied variable")
	}
}

func TestApplyToCmdDoesNotMutateProcessEnvironment(t *testing.T) {
	if err := os.Unsetenv("SPELA_CHILD"); err != nil {
		t.Fatalf("unset env: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Unsetenv("SPELA_CHILD"); err != nil {
			t.Fatalf("cleanup unset env: %v", err)
		}
	})
	e := New()
	e.Set("SPELA_CHILD", "set")
	cmd := exec.Command("env")

	e.ApplyToCmd(cmd)

	if got := os.Getenv("SPELA_CHILD"); got != "" {
		t.Fatalf("ApplyToCmd mutated process environment: got %q", got)
	}
}

func TestEnvironmentSupportedLaunchOptions(t *testing.T) {
	environment := New()
	environment.EnableWayland()
	environment.EnableHDR()
	environment.EnableNGXUpdater()
	environment.EnableVKD3DHeap(true)
	environment.SetShaderCache("/cache")
	environment.SetThreadedOptimization(true)

	for key, want := range map[string]string{
		"PROTON_ENABLE_WAYLAND":       "1",
		"PROTON_ENABLE_HDR":           "1",
		"PROTON_ENABLE_NGX_UPDATER":   "1",
		"PROTON_VKD3D_HEAP":           "1",
		"VKD3D_CONFIG":                "descriptor_heap",
		"__GL_SHADER_DISK_CACHE":      "1",
		"__GL_SHADER_DISK_CACHE_PATH": "/cache",
		"__GL_THREADED_OPTIMIZATION":  "1",
	} {
		if got := envValue(environment.BuildEnv(), key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}

	withoutLegacyGate := New()
	withoutLegacyGate.EnableVKD3DHeap(false)
	withoutLegacyGate.SetThreadedOptimization(false)
	if envValue(withoutLegacyGate.BuildEnv(), "PROTON_VKD3D_HEAP") != "" || envValue(withoutLegacyGate.BuildEnv(), "__GL_THREADED_OPTIMIZATION") != "0" {
		t.Fatalf("modern Proton environment = %#v", withoutLegacyGate.BuildEnv())
	}
}

func envValue(environment []string, key string) string {
	for index := len(environment) - 1; index >= 0; index-- {
		if value, found := strings.CutPrefix(environment[index], key+"="); found {
			return value
		}
	}
	return ""
}

func envContains(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}
	return false
}
