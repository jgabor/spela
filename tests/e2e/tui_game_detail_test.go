//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestTUIGameDetail_ProfileMutations(t *testing.T) {
	te, cleanup := SetupTestEnvironment(t)
	defer cleanup()

	session, err := NewSession("spela-mutation-test", 120, 40, testBinaryPath, []string{"tui"}, te.Env)
	if err != nil {
		t.Fatalf("failed to start TUI session: %v", err)
	}
	defer func() { _ = session.Close() }()

	if err := session.WaitForText("Cyberpunk 2077", 3*time.Second); err != nil {
		t.Fatalf("TUI did not initialize: %v", err)
	}

	if err := session.SelectGame("Cyberpunk"); err != nil {
		t.Fatalf("failed to select game: %v", err)
	}

	if err := session.WaitForText("HDR", 2*time.Second); err != nil {
		t.Fatalf("Cyberpunk 2077 profile page did not load: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	profilePath := filepath.Join(te.ConfigHome, "spela", "profiles", "1091500.yaml")

	initialData, err := readProfileYaml(profilePath)
	if err != nil {
		t.Fatalf("failed to read profile yaml: %v", err)
	}
	overridesMap, ok := initialData["overrides"].(map[string]interface{})
	if !ok || overridesMap["proton.enable_hdr"] != true {
		t.Fatalf("expected initial profile to override proton.enable_hdr, got %v", initialData)
	}

	if err := session.SendKeys("r"); err != nil {
		t.Fatalf("failed to press 'r': %v", err)
	}

	err = pollForCondition(2*time.Second, func() bool {
		data, err := readProfileYaml(profilePath)
		if err != nil {
			return false
		}
		overrides, ok := data["overrides"].(map[string]interface{})
		if !ok {
			return true
		}
		val, exists := overrides["proton.enable_hdr"]
		return !exists || val == nil
	})
	if err != nil {
		screen, _ := session.Capture()
		t.Fatalf("timed out waiting for proton.enable_hdr override to be cleared on disk: %v\nScreen:\n%s", err, screen)
	}

	if err := session.SendKeys("j", "r"); err != nil {
		t.Fatalf("failed to press 'j' and 'r': %v", err)
	}

	err = pollForCondition(2*time.Second, func() bool {
		data, err := readProfileYaml(profilePath)
		if err != nil {
			return false
		}
		overrides, ok := data["overrides"].(map[string]interface{})
		if !ok {
			return true
		}
		val, exists := overrides["proton.enable_wayland"]
		return !exists || val == nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for proton.enable_wayland override to be cleared on disk: %v", err)
	}

	if err := session.SendKeys("R"); err != nil {
		t.Fatalf("failed to press 'R': %v", err)
	}

	err = pollForCondition(2*time.Second, func() bool {
		data, err := readProfileYaml(profilePath)
		if err != nil {
			return false
		}
		overrides, ok := data["overrides"].(map[string]interface{})
		return !ok || len(overrides) == 0
	})
	if err != nil {
		t.Fatalf("timed out waiting for all overrides to be cleared on disk: %v", err)
	}

	if err := session.SendKeys("k", "p"); err != nil {
		t.Fatalf("failed to press 'k' and 'p': %v", err)
	}

	err = pollForCondition(2*time.Second, func() bool {
		data, err := readProfileYaml(profilePath)
		if err != nil {
			return false
		}
		overrides, ok := data["overrides"].(map[string]interface{})
		if !ok {
			return false
		}
		return overrides["proton.enable_hdr"] == true || overrides["proton.enable_hdr"] == false
	})
	if err != nil {
		t.Fatalf("timed out waiting for proton.enable_hdr to be pinned on disk: %v", err)
	}
}

func readProfileYaml(path string) (map[string]interface{}, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data map[string]interface{}
	if err := yaml.Unmarshal(bytes, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func pollForCondition(timeout time.Duration, cond func() bool) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("condition not met within %v", timeout)
}
