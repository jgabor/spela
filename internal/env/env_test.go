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

func envContains(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}
	return false
}
