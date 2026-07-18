//go:build dev || production || bindings

package gui

import (
	"errors"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options"
)

func TestRunBuildsSupportedDesktopApplicationContract(t *testing.T) {
	wantError := errors.New("fixture stop")
	var got *options.App
	runApplication := func(application *options.App) error {
		got = application
		return wantError
	}
	if err := run(runApplication); !errors.Is(err, wantError) {
		t.Fatalf("Run error = %v", err)
	}
	if got == nil || got.Title != "Spela" || got.Width != 1024 || got.Height != 768 || got.AssetServer == nil || got.Menu == nil || len(got.Bind) != 1 || got.OnStartup == nil || got.OnShutdown == nil {
		t.Fatalf("desktop options = %+v", got)
	}
}
