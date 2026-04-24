package launcher

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/env"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/logging"
	"github.com/jgabor/spela/internal/overlay"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/proton"
	"github.com/jgabor/spela/internal/steam"
	"github.com/jgabor/spela/internal/xdg"
)

// VKD3DCompatibilityCheckFunc is the hook the launcher calls to evaluate
// vkd3d_heap environment compatibility. Indirected so tests can inject a
// deterministic result without touching Steam config or NVML.
type VKD3DCompatibilityCheckFunc func(appID uint64) proton.CompatibilityResult

type Launcher struct {
	Game        *game.Game
	Profile     *profile.Profile
	Environment *env.Environment
	Command     []string
	cleanup     []func()

	// VKD3DCompatibilityCheck is invoked during Prepare() when the
	// profile has vkd3d_heap=true, to surface preflight warnings. Nil
	// means "use the default that wires production Steam + NVML probes".
	VKD3DCompatibilityCheck VKD3DCompatibilityCheckFunc
}

type WrapperInvocation struct {
	Command     []string
	Environment map[string]string
}

func New(g *game.Game) *Launcher {
	return &Launcher{
		Game:        g,
		Environment: env.New(),
	}
}

func (l *Launcher) OnCleanup(fn func()) {
	l.cleanup = append(l.cleanup, fn)
}

// Prepare applies the profile settings, creates a restore point for
// environment variables, registers all cleanup closures, and starts the
// overlay collector if enabled. Call before Launch.
func (l *Launcher) Prepare() error {
	restore := profile.NewRestorePoint()
	restore.SaveAllProfileEnvVars()
	l.OnCleanup(restore.Restore)

	if l.Profile != nil {
		cleanups := l.Profile.Apply(l.Environment)
		for _, c := range cleanups {
			l.OnCleanup(c)
		}
		l.vkd3dPreflight()
		if err := l.setupOverlay(); err != nil {
			l.runCleanup()
			return err
		}
	}

	return nil
}

// vkd3dPreflight emits slog warnings when vkd3d_heap is enabled but the
// environment cannot honor it. Non-blocking: a failed check never returns
// an error, never panics, and never prevents Launch from proceeding. When
// vkd3d_heap is disabled on the profile, the check is skipped entirely —
// no resolver/NVML probe fires.
func (l *Launcher) vkd3dPreflight() {
	if l.Profile == nil || !l.Profile.Proton.VKD3DHeap || l.Game == nil {
		return
	}

	check := l.VKD3DCompatibilityCheck
	if check == nil {
		check = defaultVKD3DCompatibilityCheck
	}

	result := check(l.Game.AppID)

	// Skip-reason (info) logs: surface the probe failure but don't gate
	// anything. Each axis logs independently so one missing probe doesn't
	// silence the other.
	if result.ProtonSkip != "" {
		logging.Info(
			"vkd3d_heap: could not resolve Proton for appID; skipping Proton compatibility check",
			"reason", result.ProtonSkip,
			"appID", l.Game.AppID,
		)
	}
	if result.DriverSkip != "" {
		logging.Info(
			"vkd3d_heap: NVIDIA driver not detected; skipping driver compatibility check",
			"reason", result.DriverSkip,
			"appID", l.Game.AppID,
		)
	}

	// Hard incompatibility warnings: the probe ran and reported the
	// axis is below minimum. Each axis warns independently; the user
	// sees one line per real problem rather than one combined line
	// that's harder to scan in logs.
	if !result.ProtonOK && result.ProtonSkip == "" {
		logging.Warn(
			"vkd3d_heap: Proton build does not support descriptor_heap",
			"detected", result.ProtonDetected,
			"minimum", proton.MinProtonCachyOSBuild,
		)
	}
	if !result.DriverOK && result.DriverSkip == "" {
		logging.Warn(
			"vkd3d_heap: NVIDIA driver below minimum",
			"detected", result.DriverDetected,
			"minimum", proton.MinDriverVersion,
		)
	}
}

// defaultVKD3DCompatibilityCheck wires the production Steam + NVML
// dependencies into proton.CheckCompatibility.
func defaultVKD3DCompatibilityCheck(appID uint64) proton.CompatibilityResult {
	cfg, _ := config.Load()
	steamRoot := ""
	if cfg != nil {
		steamRoot = cfg.SteamPath
	}
	if steamRoot == "" {
		steamRoot = steam.FindSteamPath()
	}
	return proton.CheckCompatibility(appID, proton.NoticeDeps{
		SteamRoot:         steamRoot,
		ResolveForAppID:   proton.ResolveForAppID,
		SupportsVKD3DHeap: proton.SupportsVKD3DHeap,
		DriverVersion:     gpu.DriverVersionString,
	})
}

func (l *Launcher) setupOverlay() error {
	if l.Profile == nil || !l.Profile.Overlay.Enabled || l.Game == nil {
		return nil
	}

	collect := overlay.BuildGPUCollector(l.Profile.Overlay.Position)
	ipcPath, cleanup, err := overlay.Setup(
		l.Game.AppID, xdg.RuntimeDir(),
		500*time.Millisecond, collect,
	)
	if err != nil {
		return fmt.Errorf("setup overlay collector: %w", err)
	}
	l.Environment.Set("SPELA_OVERLAY_IPC", ipcPath)
	l.OnCleanup(cleanup)
	return nil
}

func (l *Launcher) Launch(args []string) error {
	if len(args) == 0 {
		return nil
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	l.Environment.ApplyToCmd(cmd)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	var err error
	select {
	case sig := <-sigChan:
		if cmd.Process != nil {
			_ = cmd.Process.Signal(sig)
		}
		err = <-done
	case err = <-done:
	}

	l.runCleanup()
	return err
}

func (l *Launcher) runCleanup() {
	for i := len(l.cleanup) - 1; i >= 0; i-- {
		l.cleanup[i]()
	}
	l.cleanup = nil
}

func IsWrapperMode(args []string) bool {
	invocation := ParseWrapperArguments(args)
	if len(invocation.Command) == 0 {
		return false
	}

	first := invocation.Command[0]
	if strings.HasPrefix(first, "-") {
		return false
	}

	if filepath.IsAbs(first) {
		if _, err := os.Stat(first); err == nil {
			return true
		}
	}

	if strings.Contains(first, "/") {
		return true
	}

	if _, err := exec.LookPath(first); err == nil {
		ext := strings.ToLower(filepath.Ext(first))
		if ext == ".exe" || ext == ".sh" || ext == "" {
			return true
		}
	}

	return false
}

func ParseWrapperArguments(args []string) WrapperInvocation {
	variables := make(map[string]string)
	position := 0

	for position < len(args) {
		arg := args[position]
		if arg == "--" {
			position++
			break
		}
		if key, value, ok := splitEnvAssignment(arg); ok {
			variables[key] = value
			position++
			continue
		}
		break
	}

	return WrapperInvocation{
		Command:     args[position:],
		Environment: variables,
	}
}

func splitEnvAssignment(arg string) (string, string, bool) {
	parts := strings.SplitN(arg, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	if !isValidEnvKey(parts[0]) {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func isValidEnvKey(key string) bool {
	if key == "" {
		return false
	}
	if !isEnvKeyStart(key[0]) {
		return false
	}
	for i := 1; i < len(key); i++ {
		if !isEnvKeyChar(key[i]) {
			return false
		}
	}
	return true
}

func isEnvKeyStart(value byte) bool {
	return value == '_' || (value >= 'A' && value <= 'Z') || (value >= 'a' && value <= 'z')
}

func isEnvKeyChar(value byte) bool {
	return isEnvKeyStart(value) || (value >= '0' && value <= '9')
}

func DetectGameFromCommand(db *game.Database, args []string) *game.Game {
	if len(args) == 0 {
		return nil
	}

	if g := detectGameFromEnvironment(db); g != nil {
		return g
	}

	for _, arg := range args {
		if idStr, ok := strings.CutPrefix(arg, "SteamAppId="); ok {
			if id, err := strconv.ParseUint(idStr, 10, 64); err == nil {
				if g := db.GetGame(id); g != nil {
					return g
				}
			}
		}
	}

	for _, arg := range args {
		for _, g := range db.Games {
			if strings.Contains(arg, g.InstallDir) {
				return g
			}
		}
	}

	return nil
}

func detectGameFromEnvironment(db *game.Database) *game.Game {
	keys := []string{"SteamAppId", "SteamGameId"}
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			continue
		}
		id, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			continue
		}
		if g := db.GetGame(id); g != nil {
			return g
		}
	}
	return nil
}
