package tui

import (
	"fmt"
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/overlay"
)

// TestRenderProbe_Task6 emits ANSI-rendered output for the DLLs and Metrics
// resources so a human verifier can inspect the Task 6 acceptance criteria
// visually. Gated by SPELA_RENDER_PROBE=1 so it does not run in normal test
// suites.
//
//	SPELA_RENDER_PROBE=1 go test ./internal/tui/ -run TestRenderProbe_Task6 -v
//
// Scenario: two games. Alpha installs a stale DLSS (3.7.0) and an
// up-to-date DLSS-G (1.0.3). Beta installs an up-to-date DLSS (3.8.10).
// No game installs XeSS or FSR, so those rows MUST be omitted from the
// deployment matrix. The cached index carries both 3.8.10 and 1.0.3 so the
// library section shows a non-zero cached count for DLSS and DLSS-G.
func TestRenderProbe_Task6(t *testing.T) {
	if os.Getenv("SPELA_RENDER_PROBE") == "" {
		t.Skip("set SPELA_RENDER_PROBE=1 to see the render output")
	}

	alpha := &game.Game{
		AppID:      1091500,
		Name:       "Alpha",
		InstallDir: "/tmp/alpha",
		DLLs: []game.DetectedDLL{
			{Path: "/tmp/alpha/nvngx_dlss.dll", Name: "nvngx_dlss.dll", Type: game.DLLTypeDLSS, Version: "3.7.0"},
			{Path: "/tmp/alpha/nvngx_dlssg.dll", Name: "nvngx_dlssg.dll", Type: game.DLLTypeDLSSG, Version: "1.0.3"},
		},
	}
	beta := &game.Game{
		AppID:      292030,
		Name:       "Beta",
		InstallDir: "/tmp/beta",
		DLLs: []game.DetectedDLL{
			{Path: "/tmp/beta/nvngx_dlss.dll", Name: "nvngx_dlss.dll", Type: game.DLLTypeDLSS, Version: "3.8.10"},
		},
	}

	styles := NewStyles(DefaultTheme, true)
	svc := testServices()
	db := testDatabase(alpha, beta)

	m := NewLayout(db, svc)
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	layout := sized.(LayoutModel)

	// Seed the DLLs resource with a deterministic cached-version index.
	layout.pane.dllsResource.cached = map[string][]string{
		"dlss":  {"3.8.10", "3.7.0"},
		"dlssg": {"1.0.3"},
	}
	// And a minimal manifest so the library column "latest" is non-empty.
	manifest := &dll.Manifest{
		DLLs: map[string][]dll.DLL{
			"dlss":  {{Version: "3.8.10"}},
			"dlssg": {{Version: "1.0.3"}},
		},
	}
	layout.pane.dllsResource = layout.pane.dllsResource.SetManifest(manifest)

	_ = styles

	// --- DLLs resource ---
	result, _ := sendKey(&layout, "2")
	layout = result.(LayoutModel)
	result, _ = sendKey(&layout, "tab")
	layout = result.(LayoutModel)
	dllsView := layout.pane.View(ResourceDLLs, true)
	fmt.Fprintln(os.Stderr, "═════════════════════ DLLS RESOURCE ═════════════════════")
	fmt.Fprintln(os.Stderr, dllsView)
	fmt.Fprintln(os.Stderr, "══════════════════════════════════════════════════════════")

	// --- Metrics resource ---
	// Seed the header with synthetic metrics so the sparklines have data.
	tempBuf := NewMetricsBuffer(20)
	utilBuf := NewMetricsBuffer(20)
	powerBuf := NewMetricsBuffer(20)
	cpuBuf := NewMetricsBuffer(20)
	for i := range 20 {
		tempBuf.Push(float64(50 + i))
		utilBuf.Push(float64(20 + i*3))
		powerBuf.Push(float64(100 + i*5))
		cpuBuf.Push(float64(10 + i*4))
	}
	layout.header.gpuMetrics = &gpu.GPUMetrics{
		Temperature: 72, PowerDraw: 250, PowerLimit: 350,
		Utilization: 78, MemoryUsed: 8192, MemoryTotal: 16384,
		GraphicsClock: 2400, MemoryClock: 10500, FanSpeed: 65,
	}
	layout.header.cpuMetrics = &cpu.CPUMetrics{
		AverageFrequency: 4200, Utilization: 55, Governor: cpu.Governor("performance"),
		RAMUsedMB: 12288, RAMTotalMB: 32768,
	}
	layout.header.alerts = []overlay.Alert{
		{Type: overlay.AlertThermalThrottle, Severity: overlay.AlertWarning},
	}
	layout.header.tempBuffer = tempBuf
	layout.header.utilBuffer = utilBuf
	layout.header.powerBuffer = powerBuf
	layout.header.cpuBuffer = cpuBuf
	layout.pane.SetMetricsData(layout.header)

	result, _ = sendKey(&layout, "4")
	layout = result.(LayoutModel)
	result, _ = sendKey(&layout, "tab")
	layout = result.(LayoutModel)
	metricsView := layout.pane.View(ResourceMetrics, true)
	fmt.Fprintln(os.Stderr, "═════════════════════ METRICS RESOURCE ═════════════════════")
	fmt.Fprintln(os.Stderr, metricsView)
	fmt.Fprintln(os.Stderr, "═════════════════════════════════════════════════════════════")
}
