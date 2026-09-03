package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHeaderFetchMetricsSupportedNvidiaSMIFallback(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\nprintf '55, 250.5, 450.0, 98, 4096, 32768, 2800, 14000\\n'\n"
	if err := os.WriteFile(filepath.Join(bin, "nvidia-smi"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	message, ok := NewHeader(NewStyles(DefaultTheme, true)).Init()().(metricsMsg)
	if !ok || message.gpuMetrics == nil || message.gpuMetrics.Temperature != 55 || message.cpuMetrics == nil {
		t.Fatalf("header initial command = %#v", message)
	}
	header := NewHeader(NewStyles(DefaultTheme, true))
	header, command := header.Update(message)
	if command == nil || header.gpuMetrics == nil || header.tempBuffer.Len() != 1 || header.cpuBuffer.Len() != 1 {
		t.Fatalf("header update = metrics %+v, command %v", header.gpuMetrics, command)
	}
}
