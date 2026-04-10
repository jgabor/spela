package tui

import (
	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/profile"
)

// Services provides injectable dependencies for TUI models.
// Production code uses DefaultServices(); tests substitute fakes.
type Services struct {
	LoadConfig         func() (*config.Config, error)
	LoadProfile        func(appID uint64) (*profile.Profile, error)
	LoadDefaultProfile func() (*profile.Profile, error)
	ProfileExists      func(appID uint64) bool
	BackupExists       func(appID uint64) bool
}

// DefaultServices returns the production implementations.
func DefaultServices() *Services {
	return &Services{
		LoadConfig:         config.Load,
		LoadProfile:        profile.Load,
		LoadDefaultProfile: profile.LoadDefault,
		ProfileExists:      profile.Exists,
		BackupExists:       dll.BackupExists,
	}
}
