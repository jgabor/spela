package commands

import (
	"strings"
	"testing"
)

func TestRunLaunchRejectsDirectSteamURI(t *testing.T) {
	withTempXDG(t)
	seedGame(t, "Cyberpunk 2077", 1091500)

	launchGameID = 0
	launchDryRun = false
	t.Cleanup(func() {
		launchGameID = 0
		launchDryRun = false
	})

	err := runLaunch(LaunchCmd, []string{"Cyberpunk 2077"})
	if err == nil {
		t.Fatal("runLaunch() error = nil, want direct Steam URI rejection")
	}
	if !strings.Contains(err.Error(), "spela %command%") {
		t.Fatalf("expected wrapper guidance in error, got %q", err.Error())
	}
}
