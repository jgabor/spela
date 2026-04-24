package launcher

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/overlay"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/xdg"
)

func TestCleanupReverseOrder(t *testing.T) {
	l := New(nil)

	var order []int
	l.OnCleanup(func() { order = append(order, 1) })
	l.OnCleanup(func() { order = append(order, 2) })
	l.OnCleanup(func() { order = append(order, 3) })

	if err := l.Launch([]string{"true"}); err != nil {
		t.Fatalf("Launch() error = %v", err)
	}

	if len(order) != 3 {
		t.Fatalf("cleanup count = %d, want 3", len(order))
	}
	if order[0] != 3 || order[1] != 2 || order[2] != 1 {
		t.Errorf("cleanup order = %v, want [3 2 1]", order)
	}
}

func TestLaunchEmptyArgs(t *testing.T) {
	l := New(nil)
	if err := l.Launch(nil); err != nil {
		t.Errorf("Launch(nil) error = %v, want nil", err)
	}
}

func TestLaunchProcess(t *testing.T) {
	l := New(nil)
	l.Environment.Set("SPELA_TEST", "1")

	if err := l.Launch([]string{"true"}); err != nil {
		t.Fatalf("Launch(true) error = %v", err)
	}
}

func TestSignalForwarding(t *testing.T) {
	l := New(nil)

	var cleanupRan bool
	l.OnCleanup(func() { cleanupRan = true })

	errCh := make(chan error, 1)
	go func() {
		errCh <- l.Launch([]string{"sleep", "30"})
	}()

	// Give the child process time to start
	time.Sleep(200 * time.Millisecond)

	// Send SIGTERM to our own process — Launch() intercepts it and
	// forwards to the child, causing it to exit.
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("Kill() error = %v", err)
	}

	select {
	case <-errCh:
		// Launch returned — the child was signaled
	case <-time.After(5 * time.Second):
		t.Fatal("Launch() did not return after SIGTERM")
	}

	if !cleanupRan {
		t.Error("cleanup did not run after signal-terminated launch")
	}
}

func TestPrepareOverlayCreatesIPC(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)

	g := &game.Game{AppID: 12345, Name: "TestGame", InstallDir: "/tmp"}
	p := &profile.Profile{}
	p.Overlay.Enabled = true
	p.Overlay.Position = "top-right"

	l := New(g)
	l.Profile = p
	requirePrepare(t, l)

	ipcPath := l.Environment.Get("SPELA_OVERLAY_IPC")
	if ipcPath == "" {
		t.Fatal("SPELA_OVERLAY_IPC not set after Prepare()")
	}

	if _, err := os.Stat(ipcPath); err != nil {
		t.Fatalf("IPC file does not exist: %v", err)
	}

	// Run cleanup — IPC file should be removed
	l.runCleanup()

	if _, err := os.Stat(ipcPath); !os.IsNotExist(err) {
		t.Error("IPC file still exists after cleanup")
	}
}

func TestPrepareFailureRunsCleanupOnce(t *testing.T) {
	runtimeFile := filepath.Join(t.TempDir(), "runtime")
	if err := os.WriteFile(runtimeFile, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("write runtime file: %v", err)
	}
	t.Setenv("XDG_RUNTIME_DIR", runtimeFile)

	g := &game.Game{AppID: 12345, Name: "TestGame", InstallDir: "/tmp"}
	p := &profile.Profile{}
	p.Overlay.Enabled = true

	l := New(g)
	l.Profile = p
	cleanupCount := 0
	l.OnCleanup(func() { cleanupCount++ })

	err := l.Prepare()
	if err == nil {
		t.Fatal("Prepare() error = nil, want overlay setup failure")
	}
	if cleanupCount != 1 {
		t.Fatalf("cleanup count after Prepare failure = %d, want 1", cleanupCount)
	}

	l.runCleanup()
	if cleanupCount != 1 {
		t.Fatalf("cleanup count after second cleanup = %d, want 1", cleanupCount)
	}
}

func TestPrepareNoOverlayWithoutProfile(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)

	l := New(nil)
	requirePrepare(t, l)

	if ipc := l.Environment.Get("SPELA_OVERLAY_IPC"); ipc != "" {
		t.Errorf("SPELA_OVERLAY_IPC = %q, want empty (no profile)", ipc)
	}
}

func TestParseWrapperArguments(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantCmd []string
		wantEnv map[string]string
	}{
		{
			name:    "plain command",
			args:    []string{"echo", "hello"},
			wantCmd: []string{"echo", "hello"},
			wantEnv: map[string]string{},
		},
		{
			name:    "env vars before command",
			args:    []string{"FOO=bar", "BAZ=qux", "echo"},
			wantCmd: []string{"echo"},
			wantEnv: map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:    "double-dash separator",
			args:    []string{"FOO=bar", "--", "echo", "hello"},
			wantCmd: []string{"echo", "hello"},
			wantEnv: map[string]string{"FOO": "bar"},
		},
		{
			name:    "SteamAppId assignment",
			args:    []string{"SteamAppId=1091500", "/path/to/game.exe"},
			wantCmd: []string{"/path/to/game.exe"},
			wantEnv: map[string]string{"SteamAppId": "1091500"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := ParseWrapperArguments(tt.args)
			if len(inv.Command) != len(tt.wantCmd) {
				t.Fatalf("Command = %v, want %v", inv.Command, tt.wantCmd)
			}
			for i, c := range inv.Command {
				if c != tt.wantCmd[i] {
					t.Errorf("Command[%d] = %q, want %q", i, c, tt.wantCmd[i])
				}
			}
			for k, v := range tt.wantEnv {
				if inv.Environment[k] != v {
					t.Errorf("Environment[%q] = %q, want %q", k, inv.Environment[k], v)
				}
			}
		})
	}
}

func TestDetectGameFromCommand(t *testing.T) {
	db := &game.Database{
		Games: map[uint64]*game.Game{
			1091500: {AppID: 1091500, Name: "Cyberpunk 2077", InstallDir: "/steam/common/Cyberpunk 2077"},
			292030:  {AppID: 292030, Name: "The Witcher 3", InstallDir: "/steam/common/The Witcher 3"},
		},
	}

	tests := []struct {
		name    string
		args    []string
		wantApp uint64
	}{
		{
			name:    "detect by SteamAppId env",
			args:    []string{"SteamAppId=1091500", "/path/to/game.exe"},
			wantApp: 1091500,
		},
		{
			name:    "detect by install dir match",
			args:    []string{"/steam/common/Cyberpunk 2077/bin/game.exe"},
			wantApp: 1091500,
		},
		{
			name:    "no match",
			args:    []string{"/opt/unknown/game.exe"},
			wantApp: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear any SteamAppId env for isolation (except when testing env detection)
			if tt.name != "detect by SteamAppId env" {
				t.Setenv("SteamAppId", "")
				t.Setenv("SteamGameId", "")
			} else {
				t.Setenv("SteamAppId", "1091500")
			}

			g := DetectGameFromCommand(db, tt.args)
			if tt.wantApp == 0 {
				if g != nil {
					t.Errorf("DetectGameFromCommand() = %v, want nil", g)
				}
			} else {
				if g == nil {
					t.Fatalf("DetectGameFromCommand() = nil, want AppID %d", tt.wantApp)
				}
				if g.AppID != tt.wantApp {
					t.Errorf("AppID = %d, want %d", g.AppID, tt.wantApp)
				}
			}
		})
	}
}

// Verify overlay.Setup and xdg are accessible from tests (compile check).
var (
	_ = overlay.Setup
	_ = xdg.RuntimeDir
)

func requirePrepare(t *testing.T, l *Launcher) {
	t.Helper()
	if err := l.Prepare(); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
}
