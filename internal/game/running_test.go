package game

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunningGamesFindsProcessesByFolderPathOrSteamLaunch(t *testing.T) {
	root := t.TempDir()
	games := []*Game{
		{AppID: 1, InstallDir: "/games/One"},
		{AppID: 2, InstallDir: "/games/Two"},
		{AppID: 3, InstallDir: "/games/Three"},
		{AppID: 4, InstallDir: "/games/Four"},
		{AppID: 5, InstallDir: "/games/Five"},
	}
	fakeProcess(t, root, "10", []string{"wine", `Z:\games\One\one.exe`}, "/games/One/bin", "")
	fakeProcess(t, root, "11", []string{"python3", "/proton", "waitforexitandrun", "/games/Two/two.exe"}, "/", "")
	fakeProcess(t, root, "12", []string{"reaper", "SteamLaunch", "AppId=3", "--"}, "/", "")
	fakeProcess(t, root, "13", []string{"four"}, "/", "/games/Four/four")
	// A sibling folder that shares a prefix is a different game.
	fakeProcess(t, root, "14", []string{"five"}, "/games/Five 2", "")
	// The checking process itself never counts.
	fakeProcess(t, root, "15", []string{"spela"}, "/games/Five", "")
	fakeProcess(t, root, "self", []string{"ignored"}, "/games/Five", "")

	running := runningGames(root, 15, games)
	for appID, want := range map[uint64]bool{1: true, 2: true, 3: true, 4: true, 5: false} {
		if running[appID] != want {
			t.Errorf("game %d running = %v, want %v (all: %v)", appID, running[appID], want, running)
		}
	}
}

func TestRunningGamesMatchesTheRealFolderBehindALink(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(t.TempDir(), "Game")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "steam")
	if err := os.Symlink(filepath.Dir(real), link); err != nil {
		t.Fatal(err)
	}
	fakeProcess(t, root, "20", []string{"game"}, real, "")
	running := runningGames(root, 1, []*Game{{AppID: 7, InstallDir: filepath.Join(link, "Game")}})
	if !running[7] {
		t.Fatalf("a game reached through a link must match its real folder: %v", running)
	}
}

func TestRunningGamesWithoutProcessesOrGames(t *testing.T) {
	if running := runningGames(filepath.Join(t.TempDir(), "missing"), 1, []*Game{{AppID: 1, InstallDir: "/games/One"}}); len(running) != 0 {
		t.Fatalf("unreadable process root = %v", running)
	}
	if running := runningGames(t.TempDir(), 1, nil); len(running) != 0 {
		t.Fatalf("no games = %v", running)
	}
}

func fakeProcess(t *testing.T, root, pid string, arguments []string, cwd, exe string) {
	t.Helper()
	directory := filepath.Join(root, pid)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	commandLine := ""
	for _, argument := range arguments {
		commandLine += argument + "\x00"
	}
	if err := os.WriteFile(filepath.Join(directory, "cmdline"), []byte(commandLine), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"cwd": cwd, "exe": exe} {
		if target == "" {
			continue
		}
		if err := os.Symlink(target, filepath.Join(directory, name)); err != nil {
			t.Fatal(err)
		}
	}
}
