package launcher

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/logging"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/proton"
)

// captureLogs redirects the logging package to a buffer for the duration
// of the current test. The returned pointer receives all log output as
// structured text (one key=value stream per record).
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	handler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	restore := logging.SetHandler(handler)
	t.Cleanup(restore)
	return buf
}

// preflightLauncher builds a minimal Launcher whose Prepare() will exercise
// the vkd3d preflight path. An overlay is intentionally not enabled so the
// test doesn't allocate an IPC file.
func preflightLauncher(t *testing.T, vkd3dHeap bool, result proton.CompatibilityResult) *Launcher {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	g := &game.Game{AppID: 1091500, Name: "Cyberpunk 2077", InstallDir: "/tmp"}
	p := &profile.Profile{}
	p.Proton.VKD3DHeap = vkd3dHeap

	l := New(g)
	l.Profile = p
	l.VKD3DCompatibilityCheck = func(appID uint64) proton.CompatibilityResult {
		if appID != g.AppID {
			t.Errorf("preflight received appID=%d, want %d", appID, g.AppID)
		}
		return result
	}
	return l
}

// ----------------------------------------------------------------------
// Branch 1: Proton incompatibility → slog.Warn with detected + minimum.
// ----------------------------------------------------------------------

func TestPrepare_VKD3DHeap_ProtonIncompat_LogsWarn(t *testing.T) {
	logs := captureLogs(t)
	l := preflightLauncher(t, true, proton.CompatibilityResult{
		ProtonOK:       false,
		ProtonDetected: "GE-Proton10-34",
		DriverOK:       true,
		DriverDetected: "580.94.16",
	})
	l.Prepare()

	out := logs.String()
	if !strings.Contains(out, "level=WARN") {
		t.Fatalf("expected a WARN log line, got:\n%s", out)
	}
	if !strings.Contains(out, "vkd3d_heap: Proton build does not support descriptor_heap") {
		t.Errorf("missing expected proton-warn message, got:\n%s", out)
	}
	if !strings.Contains(out, `detected=GE-Proton10-34`) {
		t.Errorf("missing detected=GE-Proton10-34, got:\n%s", out)
	}
	if !strings.Contains(out, "minimum="+proton.MinProtonCachyOSBuild) {
		t.Errorf("missing minimum=%s, got:\n%s", proton.MinProtonCachyOSBuild, out)
	}
	if strings.Contains(out, "NVIDIA driver below minimum") {
		t.Errorf("unexpected driver warn when driver was OK, got:\n%s", out)
	}
}

// Negative case: same incompat result, but vkd3d_heap=false → no warn.
func TestPrepare_VKD3DHeap_ProtonIncompat_NoWarnWhenDisabled(t *testing.T) {
	logs := captureLogs(t)
	l := preflightLauncher(t, false, proton.CompatibilityResult{
		ProtonOK:       false,
		ProtonDetected: "GE-Proton10-34",
		DriverOK:       true,
	})
	// Override the injected checker to a failing assertion — preflight
	// must not call it when vkd3d_heap=false.
	l.VKD3DCompatibilityCheck = func(uint64) proton.CompatibilityResult {
		t.Fatal("preflight invoked compatibility check with vkd3d_heap=false")
		return proton.CompatibilityResult{}
	}
	l.Prepare()

	if s := logs.String(); strings.Contains(s, "vkd3d_heap") {
		t.Errorf("unexpected vkd3d log with toggle off, got:\n%s", s)
	}
}

// ----------------------------------------------------------------------
// Branch 2: Driver incompatibility → slog.Warn with detected + minimum.
// ----------------------------------------------------------------------

func TestPrepare_VKD3DHeap_DriverIncompat_LogsWarn(t *testing.T) {
	logs := captureLogs(t)
	l := preflightLauncher(t, true, proton.CompatibilityResult{
		ProtonOK:       true,
		ProtonDetected: "cachyos-10.0-20260410-slr",
		DriverOK:       false,
		DriverDetected: "570.86.0",
	})
	l.Prepare()

	out := logs.String()
	if !strings.Contains(out, "level=WARN") {
		t.Fatalf("expected a WARN log line, got:\n%s", out)
	}
	if !strings.Contains(out, "vkd3d_heap: NVIDIA driver below minimum") {
		t.Errorf("missing expected driver-warn message, got:\n%s", out)
	}
	if !strings.Contains(out, "detected=570.86.0") {
		t.Errorf("missing detected=570.86.0, got:\n%s", out)
	}
	if !strings.Contains(out, "minimum="+proton.MinDriverVersion) {
		t.Errorf("missing minimum=%s, got:\n%s", proton.MinDriverVersion, out)
	}
	if strings.Contains(out, "Proton build does not support descriptor_heap") {
		t.Errorf("unexpected proton warn when proton was OK, got:\n%s", out)
	}
}

// Negative case: vkd3d_heap=true but the driver axis reports OK → no driver warn.
func TestPrepare_VKD3DHeap_DriverOK_NoDriverWarn(t *testing.T) {
	logs := captureLogs(t)
	l := preflightLauncher(t, true, proton.CompatibilityResult{
		ProtonOK:       true,
		ProtonDetected: "cachyos-10.0-20260410-slr",
		DriverOK:       true,
		DriverDetected: "580.94.16",
	})
	l.Prepare()

	if s := logs.String(); strings.Contains(s, "NVIDIA driver below minimum") {
		t.Errorf("unexpected driver warn when driver OK, got:\n%s", s)
	}
}

// ----------------------------------------------------------------------
// Branch 3: Both axes satisfied → zero warnings.
// ----------------------------------------------------------------------

func TestPrepare_VKD3DHeap_BothCompatible_NoLogs(t *testing.T) {
	logs := captureLogs(t)
	l := preflightLauncher(t, true, proton.CompatibilityResult{
		ProtonOK:       true,
		ProtonDetected: "cachyos-10.0-20260410-slr",
		DriverOK:       true,
		DriverDetected: "580.94.16",
	})
	l.Prepare()

	if s := logs.String(); strings.Contains(s, "vkd3d_heap") {
		t.Errorf("expected zero vkd3d log lines on happy path, got:\n%s", s)
	}
}

// Negative case: both incompatible → two independent Warn lines.
func TestPrepare_VKD3DHeap_BothIncompat_LogsTwoWarns(t *testing.T) {
	logs := captureLogs(t)
	l := preflightLauncher(t, true, proton.CompatibilityResult{
		ProtonOK:       false,
		ProtonDetected: "GE-Proton10-34",
		DriverOK:       false,
		DriverDetected: "570.86.0",
	})
	l.Prepare()

	out := logs.String()
	if !strings.Contains(out, "vkd3d_heap: Proton build does not support descriptor_heap") {
		t.Errorf("missing proton warn, got:\n%s", out)
	}
	if !strings.Contains(out, "vkd3d_heap: NVIDIA driver below minimum") {
		t.Errorf("missing driver warn, got:\n%s", out)
	}
	if got := strings.Count(out, "level=WARN"); got != 2 {
		t.Errorf("expected 2 WARN lines, got %d in:\n%s", got, out)
	}
}

// ----------------------------------------------------------------------
// Branch 4: Resolver error → slog.Info, launch proceeds.
// ----------------------------------------------------------------------

func TestPrepare_VKD3DHeap_ResolverError_LogsInfo(t *testing.T) {
	logs := captureLogs(t)
	l := preflightLauncher(t, true, proton.CompatibilityResult{
		ProtonOK:   true,
		ProtonSkip: "could not resolve active Proton for this game; skipping compatibility check",
		DriverOK:   true,
	})
	l.Prepare()

	out := logs.String()
	if !strings.Contains(out, "level=INFO") {
		t.Fatalf("expected an INFO log line, got:\n%s", out)
	}
	if !strings.Contains(out, "could not resolve Proton for appID") {
		t.Errorf("missing resolver info message, got:\n%s", out)
	}
	if strings.Contains(out, "level=WARN") {
		t.Errorf("unexpected WARN on resolver-skip path, got:\n%s", out)
	}
}

// Negative case: the resolver skipped the check but the driver is too
// old — the hard driver warn must still fire. This also proves launch
// continues past the info log (Prepare returns normally).
func TestPrepare_VKD3DHeap_ResolverError_DriverHardStillWarns(t *testing.T) {
	logs := captureLogs(t)
	l := preflightLauncher(t, true, proton.CompatibilityResult{
		ProtonOK:       true,
		ProtonSkip:     "could not resolve active Proton for this game; skipping compatibility check",
		DriverOK:       false,
		DriverDetected: "570.86.0",
	})
	l.Prepare()

	out := logs.String()
	if !strings.Contains(out, "level=INFO") || !strings.Contains(out, "could not resolve Proton") {
		t.Errorf("missing resolver INFO, got:\n%s", out)
	}
	if !strings.Contains(out, "level=WARN") || !strings.Contains(out, "NVIDIA driver below minimum") {
		t.Errorf("driver WARN should still fire alongside info, got:\n%s", out)
	}
}

// ----------------------------------------------------------------------
// Branch 5: vkd3d_heap=false → preflight skipped entirely.
// ----------------------------------------------------------------------

func TestPrepare_VKD3DHeap_Disabled_NoLogs(t *testing.T) {
	logs := captureLogs(t)
	l := preflightLauncher(t, false, proton.CompatibilityResult{})
	// Sentinel: if preflight calls the checker with vkd3d_heap=false,
	// fail the test rather than silently passing.
	l.VKD3DCompatibilityCheck = func(uint64) proton.CompatibilityResult {
		t.Fatal("preflight invoked compatibility check with vkd3d_heap=false")
		return proton.CompatibilityResult{}
	}
	l.Prepare()

	if s := logs.String(); strings.Contains(s, "vkd3d_heap") {
		t.Errorf("preflight emitted logs with toggle disabled, got:\n%s", s)
	}
}

// Negative case: same launcher with vkd3d_heap=true (flip of the toggle)
// must invoke the checker. Proves the skip gate is the toggle, not
// anything else in the launcher wiring.
func TestPrepare_VKD3DHeap_EnabledInvokesChecker(t *testing.T) {
	_ = captureLogs(t)
	invoked := false
	l := preflightLauncher(t, true, proton.CompatibilityResult{ProtonOK: true, DriverOK: true})
	l.VKD3DCompatibilityCheck = func(appID uint64) proton.CompatibilityResult {
		invoked = true
		return proton.CompatibilityResult{ProtonOK: true, DriverOK: true}
	}
	l.Prepare()

	if !invoked {
		t.Fatal("preflight did not invoke the compatibility check when vkd3d_heap=true")
	}
}
