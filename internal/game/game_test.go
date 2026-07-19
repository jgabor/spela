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
