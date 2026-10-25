package profile_test

import (
	"path/filepath"
	"testing"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/env"
	"github.com/jgabor/spela/internal/profile"
)

func TestShaderCachePathFallsBackToSettingsThenSpelaCache(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	shaderCachePath := func(p *profile.Profile) string {
		e := env.New()
		p.ApplyEnv(e)
		return environmentValue(e, "__GL_SHADER_DISK_CACHE_PATH")
	}
	enabled := &profile.Profile{GPU: profile.GPUSettings{ShaderCache: true}}

	if got, want := shaderCachePath(enabled), filepath.Join(root, "cache", "spela", "nvidia"); got != want {
		t.Fatalf("without config = %q, want %q", got, want)
	}

	save := func(path string) {
		t.Helper()
		configuration := config.Default()
		configuration.ShaderCache = path
		if err := configuration.Save(); err != nil {
			t.Fatal(err)
		}
	}
	save("/settings/shaders")
	if got := shaderCachePath(enabled); got != "/settings/shaders" {
		t.Fatalf("with settings path = %q, want /settings/shaders", got)
	}
	own := &profile.Profile{GPU: profile.GPUSettings{ShaderCache: true, ShaderCachePath: "/profile/shaders"}}
	if got := shaderCachePath(own); got != "/profile/shaders" {
		t.Fatalf("profile path = %q, want /profile/shaders", got)
	}
	if got := shaderCachePath(&profile.Profile{GPU: profile.GPUSettings{ShaderCachePath: "/profile/shaders"}}); got != "" {
		t.Fatalf("shader cache off still set path %q", got)
	}

	for _, unusable := range []string{"", "relative/shaders"} {
		save(unusable)
		if got, want := shaderCachePath(enabled), filepath.Join(root, "cache", "spela", "nvidia"); got != want {
			t.Fatalf("settings path %q = %q, want %q", unusable, got, want)
		}
	}
}
