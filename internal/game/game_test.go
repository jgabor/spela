package game

import "testing"

func TestHasDLSS(t *testing.T) {
	g := &Game{
		DLLs: []DetectedDLL{
			{Name: "nvngx_dlss.dll", Type: DLLTypeDLSS},
			{Name: "nvngx_dlssg.dll", Type: DLLTypeDLSSG},
		},
	}

	if !g.HasDLSS() {
		t.Error("HasDLSS() = false, want true")
	}
	if !g.HasDLSSG() {
		t.Error("HasDLSSG() = false, want true")
	}
	if g.HasDLSSD() {
		t.Error("HasDLSSD() = true, want false")
	}
}

func TestHasDLSSEmpty(t *testing.T) {
	g := &Game{}
	if g.HasDLSS() {
		t.Error("HasDLSS() on empty game = true, want false")
	}
}

func TestGetDLL(t *testing.T) {
	g := &Game{
		DLLs: []DetectedDLL{
			{Name: "nvngx_dlss.dll", Type: DLLTypeDLSS, Version: "3.7.20"},
			{Name: "libxess.dll", Type: DLLTypeXeSS, Version: "1.3.0"},
		},
	}

	dlss := g.GetDLL(DLLTypeDLSS)
	if dlss == nil {
		t.Fatal("GetDLL(DLSS) = nil")
	}
	if dlss.Version != "3.7.20" {
		t.Errorf("GetDLL(DLSS).Version = %q, want %q", dlss.Version, "3.7.20")
	}

	if got := g.GetDLL(DLLTypeFSR); got != nil {
		t.Errorf("GetDLL(FSR) = %v, want nil", got)
	}
}

func TestIsToolName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"Proton Experimental", true},
		{"Proton 9.0", true},
		{"Steam Linux Runtime - Sniper", true},
		{"Steamworks Common Redistributables", true},
		{"Steam Controller Driver", true},
		{"Cyberpunk 2077", false},
		{"The Witcher 3", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsToolName(tt.name); got != tt.want {
				t.Errorf("IsToolName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
