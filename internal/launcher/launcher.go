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
	if len(args) == 0 {
		return false
	}

	first := args[0]
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

func DetectGameFromCommand(db *game.Database, args []string) *game.Game {
	if len(args) == 0 {
		return nil
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
