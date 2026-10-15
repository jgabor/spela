package commands

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jgabor/spela/internal/game"
	"github.com/jgabor/spela/internal/profile"
	"github.com/jgabor/spela/internal/tui"
)

// profileSnapshot is a game's stored profile with the defaults it inherits
// from, loaded once for a show command. raw is never nil: a game without a
// profile file reads as an empty profile that follows All games.
type profileSnapshot struct {
	game     *game.Game
	raw      *profile.Profile
	defaults *profile.Profile
	resolved *profile.Profile
	exists   bool
}

func loadProfileSnapshot(query string) (profileSnapshot, error) {
	database, err := game.LoadDatabase()
	if err != nil {
		return profileSnapshot{}, err
	}
	selected := database.FindGame(query)
	if selected == nil {
		return profileSnapshot{}, fmt.Errorf("game not found: %s", query)
	}
	raw, err := profile.Load(selected.AppID)
	if err != nil {
		return profileSnapshot{}, err
	}
	defaults, err := profile.LoadDefault()
	if err != nil {
		return profileSnapshot{}, fmt.Errorf("load default profile: %w", err)
	}
	snapshot := profileSnapshot{game: selected, defaults: defaults, exists: raw != nil}
	if raw == nil {
		raw = &profile.Profile{}
	}
	snapshot.raw = raw
	snapshot.resolved = raw.ResolveForApply(defaults)
	return snapshot, nil
}

// printNoProfileNote tells the reader that every value below comes from
// All games or the system default.
func (snapshot profileSnapshot) printNoProfileNote() {
	if !snapshot.exists {
		fmt.Printf("No profile for %s; every value follows %s or the system default.\n\n", snapshot.game.Name, profile.AllGamesName)
	}
}

var subsystemTitles = map[string]string{
	"proton":  "Proton",
	"dlss":    "DLSS",
	"gpu":     "GPU",
	"cpu":     "CPU",
	"overlay": "Overlay",
}

// subsystemOrder is the order the terminal interface lists subsystems in.
var subsystemOrder = []string{"proton", "dlss", "gpu", "cpu", "overlay"}

// subsystemFieldLines renders every field of one subsystem, in descriptor
// order, as `<label>  <value>  <source>` with aligned columns. All show
// commands use it, so a field reads the same wherever it is printed.
func subsystemFieldLines(subsystem string, snapshot profileSnapshot) ([]string, error) {
	type row struct {
		label, value, source string
		own                  bool
	}
	var rows []row
	labelWidth, valueWidth := 0, 0
	for _, descriptor := range profile.Fields() {
		if descriptor.Subsystem != subsystem {
			continue
		}
		explanation, err := snapshot.raw.ExplainField(descriptor.Key, snapshot.defaults)
		if err != nil {
			return nil, err
		}
		entry := row{
			label:  descriptor.Label,
			value:  profile.ValueText(descriptor, explanation),
			source: profile.SourceText(descriptor, explanation),
			own:    explanation.Origin == profile.OriginThisGame || explanation.Origin == profile.OriginThisGameSame,
		}
		labelWidth = max(labelWidth, utf8.RuneCountInString(entry.label))
		valueWidth = max(valueWidth, utf8.RuneCountInString(entry.value))
		rows = append(rows, entry)
	}
	lines := make([]string, 0, len(rows))
	for _, entry := range rows {
		source := tui.CLIDim(entry.source)
		if entry.own {
			source = tui.CLIAccent(entry.source)
		}
		lines = append(lines, fmt.Sprintf("%s  %s  %s",
			tui.CLIDim(padRight(entry.label, labelWidth)), padRight(entry.value, valueWidth), source))
	}
	return lines, nil
}

func padRight(text string, width int) string {
	if gap := width - utf8.RuneCountInString(text); gap > 0 {
		return text + strings.Repeat(" ", gap)
	}
	return text
}

func printSubsystemFields(subsystem string, snapshot profileSnapshot, indent string) error {
	lines, err := subsystemFieldLines(subsystem, snapshot)
	if err != nil {
		return err
	}
	for _, line := range lines {
		fmt.Println(indent + line)
	}
	return nil
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
	name      string
	mutations []func(*profile.Profile) error
}

func (changes *profileChanges) set(field string, value any) {
	changes.mutations = append(changes.mutations, func(current *profile.Profile) error {
		return current.Set(field, value)
	})
}

func (changes *profileChanges) reset(field string) {
	changes.mutations = append(changes.mutations, func(current *profile.Profile) error {
		return current.Reset(field)
	})
}

func (changes profileChanges) save(appID uint64) error {
	return profile.Mutate(appID, func(current, _ *profile.Profile) error {
		if current.Name == "" {
			current.Name = changes.name
		}
		for _, mutate := range changes.mutations {
			if err := mutate(current); err != nil {
				return err
			}
		}
		return nil
	})
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
	descriptor, _ := profile.Field(field)
	reset := false
	exists, err := profile.MutateExisting(selectedGame.AppID, func(current, _ *profile.Profile) error {
		if !current.IsOverridden(field) {
			return nil
		}
		reset = true
		return current.Reset(field)
	})
	if err != nil {
		return err
	}
	if !exists {
		fmt.Printf("No profile for %s; %s already follows %s.\n", selectedGame.Name, descriptor.Label, profile.AllGamesName)
		return nil
	}
	if !reset {
		fmt.Printf("%s: %s already follows %s.\n", selectedGame.Name, descriptor.Label, profile.AllGamesName)
		return nil
	}
	fmt.Printf("%s: %s now follows %s.\n", selectedGame.Name, descriptor.Label, profile.AllGamesName)
	return nil
}
