package commands

import (
	"fmt"

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

// renderField writes one line in the standard `<label>  <value>  <marker>`
// layout used by the per-subsystem show commands. value can be any Go value
// whose %v representation is suitable.
func renderField(label, field string, p *profile.Profile, value any) string {
	return fmt.Sprintf("%s  %v  %s", tui.CLIDim(label), value, inheritanceMarker(p, field))
}
