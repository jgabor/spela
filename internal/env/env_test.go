package env

import (
	"os"
	"os/exec"
	"testing"
)

func TestAllReturnsIsolatedMap(t *testing.T) {
	e := New()
	e.Set("SPELA_TEST", "original")

	vars := e.All()
	vars["SPELA_TEST"] = "mutated"

	if got := e.Get("SPELA_TEST"); got != "original" {
		t.Fatalf("Environment map leaked mutation: got %q", got)
	}
}

func TestAllIsolationFailsForMissingKey(t *testing.T) {
	e := New()
	vars := e.All()
	vars["SPELA_ADDED"] = "value"

	if got := e.Get("SPELA_ADDED"); got != "" {
		t.Fatalf("Environment map leaked added key: got %q", got)
	}
}

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
	t.Setenv("SPELA_APPLY_TEST", "before")
	environment := New()
	environment.SetIf("IGNORED", "value", false)
	environment.SetIf("CONDITIONAL", "value", true)
	environment.EnableWayland()
	environment.EnableHDR()
	environment.EnableNGXUpdater()
	environment.EnableVKD3DHeap(true)
	environment.SetShaderCache("/cache")
	environment.SetDXVKCache("/dxvk")
	environment.SetThreadedOptimization(true)
	environment.SetDXVKConfigFile("/dxvk.conf")
	environment.Set("SPELA_APPLY_TEST", "after")
	environment.Unset("IGNORED")
	environment.Apply()

	for key, want := range map[string]string{
		"CONDITIONAL":                 "value",
		"PROTON_ENABLE_WAYLAND":       "1",
		"PROTON_ENABLE_HDR":           "1",
		"PROTON_ENABLE_NGX_UPDATER":   "1",
		"PROTON_VKD3D_HEAP":           "1",
		"VKD3D_CONFIG":                "descriptor_heap",
		"__GL_SHADER_DISK_CACHE":      "1",
		"__GL_SHADER_DISK_CACHE_PATH": "/cache",
		"DXVK_STATE_CACHE_PATH":       "/dxvk",
		"__GL_THREADED_OPTIMIZATION":  "1",
		"DXVK_CONFIG_FILE":            "/dxvk.conf",
	} {
		if got := environment.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if got := os.Getenv("SPELA_APPLY_TEST"); got != "after" {
		t.Fatalf("Apply value = %q", got)
	}

	withoutLegacyGate := New()
	withoutLegacyGate.EnableVKD3DHeap(false)
	withoutLegacyGate.SetThreadedOptimization(false)
	if withoutLegacyGate.Get("PROTON_VKD3D_HEAP") != "" || withoutLegacyGate.Get("__GL_THREADED_OPTIMIZATION") != "0" {
		t.Fatalf("modern Proton environment = %#v", withoutLegacyGate.All())
	}
}

func envContains(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}
	return false
}
