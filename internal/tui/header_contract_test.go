package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/jgabor/spela/internal/cpu"
	"github.com/jgabor/spela/internal/gpu"
	"github.com/jgabor/spela/internal/nav"
	"github.com/jgabor/spela/internal/overlay"
)

func TestHeaderAndMonitorRenderLiveUnavailableAndAlertStates(t *testing.T) {
	styles := NewStyles(DefaultTheme, true)
	header := NewHeader(styles)
	header.SetWidth(120)
	if unavailable := stripANSI(header.View()); !strings.Contains(unavailable, "GPU: N/A") || !strings.Contains(unavailable, "CPU: N/A") {
		t.Fatalf("unavailable header state changed:\n%s", unavailable)
	}

	gpuMetrics := &gpu.GPUMetrics{
		Temperature: 92, PowerDraw: 275, PowerLimit: 300, Utilization: 98,
		MemoryUsed: 8192, MemoryTotal: 12288, GraphicsClock: 2800,
		MemoryClock: 10500, FanSpeed: 100,
		ThrottleReasons: &gpu.ThrottleReasons{ThermalHardware: true, PowerCap: true},
	}
	cpuMetrics := &cpu.CPUMetrics{
		Frequencies: []int{4200, 4300}, AverageFrequency: 4250,
		Utilization: 64.5, Governor: cpu.Governor("performance"), SMTEnabled: true,
		RAMUsedMB: 16384, RAMTotalMB: 32768,
	}
	alerts := []overlay.Alert{
		{Type: overlay.AlertPowerLimit, Severity: overlay.AlertWarning},
		{Type: overlay.AlertThermalThrottle, Severity: overlay.AlertCritical},
		{Type: overlay.AlertFanMaximum, Severity: overlay.AlertWarning},
	}
	header, command := header.Update(metricsMsg{gpuMetrics: gpuMetrics, cpuMetrics: cpuMetrics, alerts: alerts})
	if command == nil || header.GPUMetrics() != gpuMetrics || header.CPUMetrics() != cpuMetrics {
		t.Fatal("metrics update did not schedule the next sample or retain the snapshot")
	}
	header.SetWidth(120)
	view := stripANSI(header.View())
	for _, fragment := range []string{"92°C", "98%", "275W", "4250MHz", "Throttling"} {
		if !strings.Contains(view, fragment) {
			t.Errorf("full header missing %q:\n%s", fragment, view)
		}
	}
	for _, width := range []int{80, 75, 50} {
		header.SetWidth(width)
		view := header.View()
		if got := lipgloss.Width(view); got > width {
			t.Errorf("header width %d rendered at %d", width, got)
		}
		if got := lipgloss.Height(view); got != headerHeight {
			t.Errorf("header width %d rendered %d lines", width, got)
		}
	}
	compact := stripANSI(header.ViewCompact())
	for _, fragment := range []string{"VRAM", "RAM", "Throttling"} {
		if !strings.Contains(compact, fragment) {
			t.Errorf("compact header missing %q:\n%s", fragment, compact)
		}
	}
	if next, command := header.Update(headerTickMsg{}); command == nil || next.GPUMetrics() != gpuMetrics {
		t.Fatal("header tick did not preserve state and request a sample")
	}

	temperature, utilization, power, cpuBuffer := NewMetricsBuffer(4), NewMetricsBuffer(4), NewMetricsBuffer(4), NewMetricsBuffer(4)
	for _, sample := range []float64{40, 55, 75, 92} {
		temperature.Push(sample)
		utilization.Push(sample)
		power.Push(sample * 3)
		cpuBuffer.Push(sample / 2)
	}
	monitor := NewMetricsResource(styles).SetData(gpuMetrics, cpuMetrics, alerts, temperature, utilization, power, cpuBuffer)
	for _, width := range []int{80, 24} {
		monitor.SetSize(width, 30)
		for section, heading := range []string{"GPU", "CPU", "Alerts"} {
			view := stripANSI(monitor.View(true, nav.MonitorSection(section)))
			if !strings.Contains(view, heading) {
				t.Errorf("monitor section %d missing %q:\n%s", section, heading, view)
			}
		}
	}
	empty := NewMetricsResource(styles)
	for section, fragment := range []string{"N/A", "N/A", "No active alerts"} {
		if view := stripANSI(empty.View(false, nav.MonitorSection(section))); !strings.Contains(view, fragment) {
			t.Errorf("empty monitor section %d missing %q:\n%s", section, fragment, view)
		}
	}
}

func TestAlertLabelsCoverSupportedAlertKinds(t *testing.T) {
	tests := []struct {
		alert overlay.Alert
		want  string
	}{
		{overlay.Alert{Type: overlay.AlertThermalThrottle, Severity: overlay.AlertWarning}, "High temp"},
		{overlay.Alert{Type: overlay.AlertThermalThrottle, Severity: overlay.AlertCritical}, "Throttling"},
		{overlay.Alert{Type: overlay.AlertPowerLimit}, "Power limited"},
		{overlay.Alert{Type: overlay.AlertFanMaximum}, "Fan max"},
		{overlay.Alert{Type: overlay.AlertType(99)}, ""},
	}
	for _, test := range tests {
		if got := alertLabel(&test.alert); got != test.want {
			t.Errorf("alertLabel(%+v) = %q, want %q", test.alert, got, test.want)
		}
	}
	if highestSeverityAlert(nil) != nil {
		t.Fatal("empty alerts should not have a highest severity")
	}
}
