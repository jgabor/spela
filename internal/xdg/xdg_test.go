package xdg

import (
	"os"
	"testing"
)

func TestFallbackHomesAndAnonymousRuntimeContract(t *testing.T) {
	t.Setenv("HOME", "/home/fixture")
	t.Setenv("USER", "")
	for _, key := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_RUNTIME_DIR"} {
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	if ConfigHome() != "/home/fixture/.config/spela" || DataHome() != "/home/fixture/.local/share/spela" || CacheHome() != "/home/fixture/.cache/spela" {
		t.Fatal("fallback XDG homes changed")
	}
	if RuntimeDir() != "/tmp/runtime-unknown/spela" {
		t.Fatalf("anonymous runtime directory = %q", RuntimeDir())
	}
}
