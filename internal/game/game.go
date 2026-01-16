package game

import (
	"time"

	"github.com/jgabor/spela/internal/dll"
)

type Game struct {
	AppID       uint64        `yaml:"app_id"`
	Name        string        `yaml:"name"`
	InstallDir  string        `yaml:"install_dir"`
	LibraryPath string        `yaml:"library_path"`
	PrefixPath  string        `yaml:"prefix_path,omitempty"`
	DLLs        []DetectedDLL `yaml:"dlls,omitempty"`
	ScannedAt   time.Time     `yaml:"scanned_at"`
}

type DetectedDLL struct {
	Path    string      `yaml:"path"`
	Name    string      `yaml:"name"`
	Type    dll.DLLType `yaml:"type"`
	Version string      `yaml:"version,omitempty"`
}

func (g *Game) HasDLSS() bool {
	for _, d := range g.DLLs {
		if d.Type == dll.DLLTypeDLSS {
			return true
		}
	}
	return false
}

func (g *Game) HasDLSSG() bool {
	for _, d := range g.DLLs {
		if d.Type == dll.DLLTypeDLSSG {
			return true
		}
	}
	return false
}

func (g *Game) HasDLSSD() bool {
	for _, d := range g.DLLs {
		if d.Type == dll.DLLTypeDLSSD {
			return true
		}
	}
	return false
}

func (g *Game) GetDLL(dllType dll.DLLType) *DetectedDLL {
	for i := range g.DLLs {
		if g.DLLs[i].Type == dllType {
			return &g.DLLs[i]
		}
	}
	return nil
}
