package commands

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
)

func executeSupportedCommand(t *testing.T, command *cobra.Command, args ...string) string {
	t.Helper()
	if args == nil {
		args = []string{}
	}
	return captureStdout(t, func() {
		command.SetArgs(args)
		if err := command.Execute(); err != nil {
			t.Fatalf("%s %v: %v", command.Name(), args, err)
		}
	})
}

func saveCommandDatabase(t *testing.T, database *game.Database) {
	t.Helper()
	if _, err := game.Transaction(func(current *game.Database) (bool, error) {
		*current = *database
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestListSupportedTextFlow(t *testing.T) {
	state := withTempXDG(t)
	t.Setenv("HOME", state+"/home")
	t.Setenv("XDG_CACHE_HOME", state+"/cache")
	t.Setenv("XDG_RUNTIME_DIR", state+"/runtime")
	configuration := config.Default()
	configuration.RescanOnStartup = false
	steamRoot := filepath.Join(state, "steam")
	libraryRoot := filepath.Join(state, "library")
	if err := os.MkdirAll(filepath.Join(steamRoot, "steamapps"), 0o755); err != nil {
		t.Fatal(err)
	}
	vdf := `"libraryfolders"
{
  "0"
  {
    "path" "` + libraryRoot + `"
  }
}`
	if err := os.WriteFile(filepath.Join(steamRoot, "steamapps", "libraryfolders.vdf"), []byte(vdf), 0o644); err != nil {
		t.Fatal(err)
	}
	configuration.SteamPath = steamRoot
	if err := configuration.Save(); err != nil {
		t.Fatal(err)
	}
	saveCommandDatabase(t, &game.Database{Games: map[uint64]*game.Game{}})
	if output := executeSupportedCommand(t, ListCmd); !strings.Contains(output, "No games found") {
		t.Fatalf("empty list output:\n%s", output)
	}
	seedGame(t, "Cyberpunk 2077", 1091500)
	if output := executeSupportedCommand(t, ListCmd); !strings.Contains(output, "Cyberpunk 2077") {
		t.Fatalf("text list output:\n%s", output)
	}
}

func TestTUIReportsMalformedConfigWithoutUsage(t *testing.T) {
	state := withTempXDG(t)
	t.Setenv("HOME", state+"/home")
	t.Setenv("XDG_RUNTIME_DIR", state+"/runtime")
	configDir := filepath.Join(state, "config", "spela")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("show_hints: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command := &cobra.Command{}
	command.SetErr(&stderr)
	if err := runTUI(command, nil); err != nil {
		t.Fatal(err)
	}
	output := stderr.String()
	if !strings.Contains(output, configPath) || !strings.Contains(output, "Fix the YAML or remove the file") || strings.Contains(output, "Usage:") {
		t.Fatalf("malformed config recovery:\n%s", output)
	}
}

func TestProfileSettingCommandsExecuteSupportedMutationAndProjectionFlows(t *testing.T) {
	state := withTempXDG(t)
	t.Setenv("HOME", state+"/home")
	t.Setenv("XDG_CACHE_HOME", state+"/cache")
	seedGame(t, "Cyberpunk 2077", 1091500)

	if output := executeSupportedCommand(t, ProfileCmd, "create", "Cyberpunk 2077"); !strings.Contains(output, "Created profile") {
		t.Fatalf("profile create output:\n%s", output)
	}
	if output := executeSupportedCommand(t, ProfileCmd, "list"); !strings.Contains(output, "Cyberpunk 2077") {
		t.Fatalf("profile list output:\n%s", output)
	}

	dlssOutput := executeSupportedCommand(
		t, DLSSCmd,
		"set", "Cyberpunk 2077",
		"--sr-mode", "quality", "--sr-preset", "K",
		"--rr-mode", "dlaa", "--rr-preset", "L", "--rr-override", "true",
		"--fg", "true", "--fg-override", "true", "--fg-indicator",
		"--multi-frame", "3", "--indicator",
	)
	if !strings.Contains(dlssOutput, "Updated DLSS configuration") {
		t.Fatalf("DLSS set output:\n%s", dlssOutput)
	}
	if output := executeSupportedCommand(t, DLSSCmd, "show", "Cyberpunk 2077"); !strings.Contains(output, "Super Resolution") || !strings.Contains(output, "quality") {
		t.Fatalf("DLSS show output:\n%s", output)
	}
	if output := executeSupportedCommand(t, DLSSCmd, "show", "Cyberpunk 2077", "--json"); !strings.Contains(output, `"SRMode": "quality"`) {
		t.Fatalf("DLSS JSON output:\n%s", output)
	}
	if output := executeSupportedCommand(t, DLSSCmd, "reset", "Cyberpunk 2077", "sr-mode"); !strings.Contains(output, "inherited") {
		t.Fatalf("DLSS reset output:\n%s", output)
	}

	overlayOutput := executeSupportedCommand(
		t, OverlayCmd,
		"set", "Cyberpunk 2077", "--enabled", "true", "--position", "bottom-right",
		"--show-fps", "true", "--show-frametime", "true", "--show-cpu", "true",
		"--show-gpu", "true", "--show-vram", "true", "--toggle-key", "F12",
	)
	if !strings.Contains(overlayOutput, "Updated overlay profile") {
		t.Fatalf("overlay set output:\n%s", overlayOutput)
	}
	if output := executeSupportedCommand(t, OverlayCmd, "show", "Cyberpunk 2077"); !strings.Contains(output, "bottom-right") || !strings.Contains(output, "Show VRAM") {
		t.Fatalf("overlay show output:\n%s", output)
	}
	if output := executeSupportedCommand(t, OverlayCmd, "show", "Cyberpunk 2077", "--json"); !strings.Contains(output, `"Position": "bottom-right"`) {
		t.Fatalf("overlay JSON output:\n%s", output)
	}
	if output := executeSupportedCommand(t, OverlayCmd, "reset", "Cyberpunk 2077", "show-fps"); !strings.Contains(output, "inherited") {
		t.Fatalf("overlay reset output:\n%s", output)
	}

	gpuOutput := executeSupportedCommand(
		t, GPUCmd,
		"set", "Cyberpunk 2077", "--clock-offset", "150", "--memory-offset", "500",
		"--power-limit", "300", "--fan-speed", "70", "--power-mizer", "max",
		"--shader-cache", "true", "--shader-cache-path", "/shader", "--threaded-opt", "true",
	)
	if !strings.Contains(gpuOutput, "Updated GPU profile") {
		t.Fatalf("GPU set output:\n%s", gpuOutput)
	}
	if output := executeSupportedCommand(t, GPUCmd, "show", "Cyberpunk 2077"); !strings.Contains(output, "150") || !strings.Contains(output, "70%") {
		t.Fatalf("GPU show output:\n%s", output)
	}
	if output := executeSupportedCommand(t, GPUCmd, "show", "Cyberpunk 2077", "--json"); !strings.Contains(output, `"ClockOffset": 150`) {
		t.Fatalf("GPU JSON output:\n%s", output)
	}
	if output := executeSupportedCommand(t, GPUCmd, "profile-reset", "Cyberpunk 2077", "clock-offset"); !strings.Contains(output, "inherited") {
		t.Fatalf("GPU reset output:\n%s", output)
	}

	cpuOutput := executeSupportedCommand(t, CPUCmd, "set", "Cyberpunk 2077", "--governor", "performance", "--smt", "off")
	if !strings.Contains(cpuOutput, "Updated CPU profile") {
		t.Fatalf("CPU set output:\n%s", cpuOutput)
	}
	if output := executeSupportedCommand(t, CPUCmd, "show", "Cyberpunk 2077"); !strings.Contains(output, "performance") || !strings.Contains(output, "off") {
		t.Fatalf("CPU show output:\n%s", output)
	}
	if output := executeSupportedCommand(t, CPUCmd, "show", "Cyberpunk 2077", "--json"); !strings.Contains(output, `"Governor": "performance"`) {
		t.Fatalf("CPU JSON output:\n%s", output)
	}
	if output := executeSupportedCommand(t, CPUCmd, "profile-reset", "Cyberpunk 2077", "smt"); !strings.Contains(output, "inherited") {
		t.Fatalf("CPU reset output:\n%s", output)
	}

	if output := executeSupportedCommand(t, ProtonCmd, "set", "Cyberpunk 2077", "--hdr", "true", "--wayland", "true", "--ngx-updater", "true", "--vkd3d-heap", "true"); !strings.Contains(output, "Updated Proton profile") {
		t.Fatalf("Proton set output:\n%s", output)
	}
	if output := executeSupportedCommand(t, ProtonCmd, "show", "Cyberpunk 2077"); !strings.Contains(output, "HDR") {
		t.Fatalf("Proton show output:\n%s", output)
	}
	if output := executeSupportedCommand(t, ProtonCmd, "show", "Cyberpunk 2077", "--json"); !strings.Contains(output, `"EnableHDR": true`) {
		t.Fatalf("Proton JSON output:\n%s", output)
	}
	if output := executeSupportedCommand(t, ProtonCmd, "reset", "Cyberpunk 2077", "hdr"); !strings.Contains(output, "inherited") {
		t.Fatalf("Proton reset output:\n%s", output)
	}

	p, err := profile.Load(1091500)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{
		profile.FieldDLSSFGEnabled, profile.FieldOverlayEnabled,
		profile.FieldGPUMemoryOffset, profile.FieldCPUGovernor,
	} {
		if !p.IsOverridden(field) {
			t.Errorf("supported command did not persist override %q", field)
		}
	}
}

func TestGameDLLAndDenylistCommandsExecuteSupportedReadWriteFlows(t *testing.T) {
	state := withTempXDG(t)
	t.Setenv("HOME", state+"/home")
	t.Setenv("XDG_CACHE_HOME", state+"/cache")
	entry := &game.Game{
		AppID: 1091500, Name: "Cyberpunk 2077", InstallDir: state + "/game", ScannedAt: time.Now(),
		DLLs: []game.DetectedDLL{{Name: "nvngx_dlss.dll", Path: state + "/game/nvngx_dlss.dll", Version: "3.8.10", Type: game.DLLTypeDLSS}},
	}
	db := &game.Database{Games: map[uint64]*game.Game{entry.AppID: entry}, UpdatedAt: time.Now()}
	saveCommandDatabase(t, db)
	configuration := config.Default()
	configuration.RescanOnStartup = false
	if err := configuration.Save(); err != nil {
		t.Fatal(err)
	}

	if output := executeSupportedCommand(t, ListCmd); !strings.Contains(output, entry.Name) {
		t.Fatalf("game list output:\n%s", output)
	}
	if output := executeSupportedCommand(t, ShowCmd, "Cyberpunk 2077"); !strings.Contains(output, "nvngx_dlss.dll") || !strings.Contains(output, "3.8.10") {
		t.Fatalf("game show output:\n%s", output)
	}
	if output := executeSupportedCommand(t, DLLCmd, "list", "Cyberpunk 2077"); !strings.Contains(output, "nvngx_dlss.dll") {
		t.Fatalf("DLL list output:\n%s", output)
	}
	if err := os.MkdirAll(entry.InstallDir, 0o755); err != nil {
		t.Fatal(err)
	}
	targetPath := state + "/game/nvngx_dlss.dll"
	if err := os.WriteFile(targetPath, []byte("original DLL"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload := []byte("updated DLL")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(payload)
	}))
	defer server.Close()
	manifest := &dll.Manifest{
		Version: "1", UpdatedAt: time.Now(),
		DLLs: map[string][]dll.DLL{"dlss": {{Version: "3.9.0", Filename: "nvngx_dlss.dll", URL: server.URL, SHA256: fmt.Sprintf("%x", sha256.Sum256(payload))}}},
	}
	if err := dll.SaveManifest(manifest); err != nil {
		t.Fatal(err)
	}
	if output := executeSupportedCommand(t, DLLCmd, "check-updates"); !strings.Contains(output, "3.8.10 -> 3.9.0") {
		t.Fatalf("DLL update check output:\n%s", output)
	}
	if output := executeSupportedCommand(t, DLLCmd, "update", "Cyberpunk 2077", "dlss"); !strings.Contains(output, "Updated dlss DLL to version 3.9.0") {
		t.Fatalf("DLL update output:\n%s", output)
	}
	if data, err := os.ReadFile(targetPath); err != nil || string(data) != string(payload) {
		t.Fatalf("updated DLL payload = %q, %v", data, err)
	}
	if output := executeSupportedCommand(t, DLLCmd, "restore", "Cyberpunk 2077"); !strings.Contains(output, "Restored original DLLs") {
		t.Fatalf("DLL restore output:\n%s", output)
	}
	if data, err := os.ReadFile(targetPath); err != nil || string(data) != "original DLL" {
		t.Fatalf("restored DLL payload = %q, %v", data, err)
	}

	if output := executeSupportedCommand(t, DenylistCmd, "deny", "Cyberpunk 2077", "--reason", "contract test"); !strings.Contains(output, "contract test") {
		t.Fatalf("deny output:\n%s", output)
	}
	if output := executeSupportedCommand(t, DenylistCmd, "check", "1091500"); !strings.Contains(output, "DENIED") {
		t.Fatalf("deny check output:\n%s", output)
	}
	if output := executeSupportedCommand(t, DenylistCmd, "show"); !strings.Contains(output, "Cyberpunk 2077") {
		t.Fatalf("denylist show output:\n%s", output)
	}
	if output := executeSupportedCommand(t, DenylistCmd, "allow", "Cyberpunk 2077"); !strings.Contains(output, "Force-allowed") {
		t.Fatalf("allow output:\n%s", output)
	}
	if output := executeSupportedCommand(t, DenylistCmd, "check", "Cyberpunk 2077"); !strings.Contains(output, "is allowed") {
		t.Fatalf("allowed check output:\n%s", output)
	}
}
