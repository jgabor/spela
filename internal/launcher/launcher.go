package launcher

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/jgabor/spela/internal/env"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/ludusavi"
	"github.com/jgabor/spela/internal/profile"
)

type Launcher struct {
	Game        *game.Game
	Profile     *profile.Profile
	Environment *env.Environment
	Command     []string
	cleanup     []func()
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

func (l *Launcher) Launch(args []string) error {
	if len(args) == 0 {
		return nil
	}

	l.runPreLaunchHooks()

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

func (l *Launcher) runPreLaunchHooks() {
	if l.Profile == nil || l.Game == nil {
		return
	}

	if l.Profile.Ludusavi.BackupOnLaunch && ludusavi.IsInstalled() {
		log.Printf("Backing up saves for %s...", l.Game.Name)
		if _, err := ludusavi.BackupGame(l.Game.Name); err != nil {
			log.Printf("Warning: failed to backup saves: %v", err)
		}
	}
}

func (l *Launcher) runCleanup() {
	for i := len(l.cleanup) - 1; i >= 0; i-- {
		l.cleanup[i]()
	}
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
		if strings.HasPrefix(arg, "SteamAppId=") {
			idStr := strings.TrimPrefix(arg, "SteamAppId=")
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
