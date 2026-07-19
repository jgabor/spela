package commands

import (
	"fmt"
	"strings"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/tui"
)

// inheritanceMarker returns a short annotation indicating whether `field` is
// overridden on the game profile or inherited from defaults. The marker is
// styled via the existing CLI helpers so it reads the same regardless of
// which subsystem calls it.
//
// Passing a nil profile means "no per-game profile exists"; the field is
// implicitly inherited. Passing a profile without overrides likewise yields
// the inherited marker.
func inheritanceMarker(p *profile.Profile, field string) string {
	if p != nil && p.IsOverridden(field) {
		return tui.CLIAccent("[override]")
	}
	return tui.CLIDim("[inherited]")
}

func resolveProfileField(section, input string) (string, bool) {
	leaf := strings.ReplaceAll(input, "-", "_")
	switch section + "." + leaf {
	case "proton.hdr":
		leaf = "enable_hdr"
	case "proton.wayland":
		leaf = "enable_wayland"
	case "proton.ngx_updater":
		leaf = "enable_ngx_updater"
	case "gpu.threaded_opt":
		leaf = "threaded_optimization"
	case "dlss.fg":
		leaf = "fg_enabled"
	}
	key := section + "." + leaf
	descriptor, ok := profile.Field(key)
	return descriptor.Key, ok && descriptor.Subsystem == section
}

type profileChanges struct {
	profile *profile.Profile
	changed bool
	err     error
}

func (changes *profileChanges) set(field string, value any) {
	changes.changed = true
	if changes.err == nil {
		changes.err = changes.profile.Set(field, value)
	}
}

func resetProfileField(args []string, section, fieldName, errorSuffix string) error {
	database, err := game.LoadDatabase()
	if err != nil {
		return err
	}
	selectedGame := database.FindGame(args[0])
	if selectedGame == nil {
		return fmt.Errorf("game not found: %s", args[0])
	}
	field, ok := resolveProfileField(section, args[1])
	if !ok {
		return fmt.Errorf("unknown %s field %q%s", fieldName, args[1], errorSuffix)
	}
	stored, err := profile.Load(selectedGame.AppID)
	if err != nil {
		return err
	}
	if stored == nil {
		fmt.Printf("No profile for %s; field is already inherited.\n", selectedGame.Name)
		return nil
	}
	if !stored.IsOverridden(field) {
		fmt.Printf("%s: %s is already inherited.\n", selectedGame.Name, args[1])
		return nil
	}
	if err := stored.Reset(field); err != nil {
		return err
	}
	if err := profile.Save(selectedGame.AppID, stored); err != nil {
		return err
	}
	fmt.Printf("Reset %s on %s to inherited.\n", args[1], selectedGame.Name)
	return nil
}

// renderField writes one line in the standard `<label>  <value>  <marker>`
// layout used by the per-subsystem show commands. value can be any Go value
// whose %v representation is suitable.
func renderField(label, field string, p *profile.Profile, value any) string {
	return fmt.Sprintf("%s  %v  %s", tui.CLIDim(label), value, inheritanceMarker(p, field))
}
