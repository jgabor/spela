package profile

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// The words every surface uses for profile values. The terminal, graphical, and
// command-line interfaces print these and nothing else, so a value reads the
// same everywhere. Layout and key bindings stay with the surfaces.

// AllGamesName is the name of the profile that games inherit from.
const AllGamesName = "All games"

// Action words name what a reset-like action does.
const (
	ActionFollowAllGames                 = "Follow All games"
	ActionUseSystemDefault               = "Use system default"
	ActionFollowAllGamesForAllSettings   = "Follow All games for all settings"
	ActionUseSystemDefaultForAllSettings = "Use system default for all settings"
	ActionApplyToGamesWithOwnValue       = "Apply to games with their own value"
	ActionSetForThisGame                 = "Set for this game"
	ActionSetHere                        = "Set here"
)

// Value words.
const (
	WordOn   = "On"
	WordOff  = "Off"
	WordNone = "none"
)

// RetiredWords lists wording that no surface may print any more. Surface tests
// assert that none of them appears.
func RetiredWords() []string {
	return []string{
		"(default)",
		"Override for this game",
		"Inherited from defaults",
		"Default for all games",
		"defaults profile",
		"root profile",
		"This change does not reach",
		"Effect:",
	}
}

// DisplayValue renders a stored value for people. Booleans read On and Off, an
// empty string or an unset optional value reads none, and numbers carry their
// unit. A nil value renders as the zero value of the field's kind.
func DisplayValue(descriptor FieldDescriptor, value any) string {
	switch descriptor.Kind {
	case PrimitiveBool:
		if flag, ok := value.(bool); ok && flag {
			return WordOn
		}
		return WordOff
	case PrimitiveOptionalBool:
		flag, ok := value.(bool)
		switch {
		case !ok:
			return WordNone
		case flag:
			return WordOn
		default:
			return WordOff
		}
	case PrimitiveInt:
		number := 0
		switch typed := value.(type) {
		case int:
			number = typed
		case int64:
			number = int(typed)
		}
		text := strconv.Itoa(number)
		switch descriptor.Unit {
		case "":
			return text
		case "%":
			return text + "%"
		default:
			return text + " " + descriptor.Unit
		}
	default:
		if value == nil {
			return WordNone
		}
		text := fmt.Sprint(value)
		if text == "" {
			return WordNone
		}
		return text
	}
}

// ValueText renders the effective value of an explained field. A number that
// no profile sets reads none, not 0, because zero means "leave it alone".
func ValueText(descriptor FieldDescriptor, explanation FieldExplanation) string {
	if explanation.Origin == OriginSystemDefault && descriptor.Kind == PrimitiveInt {
		return WordNone
	}
	return DisplayValue(descriptor, explanation.Value)
}

// SourceText says where a value comes from. A game's own value also shows the
// All games value it replaced, so the choice can be judged on one line.
func SourceText(descriptor FieldDescriptor, explanation FieldExplanation) string {
	switch explanation.Origin {
	case OriginThisGame:
		return "this game · " + AllGamesName + ": " + allGamesValueText(descriptor, explanation)
	case OriginThisGameSame:
		return "this game · same as " + AllGamesName
	case OriginAllGames:
		return "from " + AllGamesName
	case OriginSetHere:
		return "set here"
	default:
		return "system default"
	}
}

func allGamesValueText(descriptor FieldDescriptor, explanation FieldExplanation) string {
	if !explanation.AllGamesSets && descriptor.Kind == PrimitiveInt {
		return WordNone
	}
	return DisplayValue(descriptor, explanation.DefaultValue)
}

// ResetActionText names the action that clears a value in the given scope.
func ResetActionText(allGamesScope bool) string {
	if allGamesScope {
		return ActionUseSystemDefault
	}
	return ActionFollowAllGames
}

// ClearAllAction names the action that clears every setting a page holds itself.
func ClearAllAction(allGamesScope bool) string {
	if allGamesScope {
		return ActionUseSystemDefaultForAllSettings
	}
	return ActionFollowAllGamesForAllSettings
}

// ClearAllBody says how many settings the whole-page action clears and where
// they go afterwards.
func ClearAllBody(allGamesScope bool, count int) string {
	owner, target := "this game sets", AllGamesName
	if allGamesScope {
		owner, target = AllGamesName+" sets", "the system default"
	}
	switch count {
	case 0:
		return "Nothing to clear: every value already follows " + target + "."
	case 1:
		return fmt.Sprintf("This clears the 1 setting that %s itself. It follows %s afterwards. It is saved at once.", owner, target)
	default:
		return fmt.Sprintf("This clears the %d settings that %s itself. They follow %s afterwards. It is saved at once.", count, owner, target)
	}
}

// OwnCount is the number of settings a page holds itself: the values a game
// sets for itself, or on All games the values set there.
func OwnCount(explanations []FieldExplanation, allGamesScope bool) int {
	count := 0
	for _, explanation := range explanations {
		switch {
		case allGamesScope && explanation.Origin == OriginSetHere,
			!allGamesScope && (explanation.Origin == OriginThisGame || explanation.Origin == OriginThisGameSame):
			count++
		}
	}
	return count
}

// OwnValueEntry is one game and its stored profile.
type OwnValueEntry struct {
	AppID   uint64
	Name    string
	Profile *Profile
}

// CollectOwnValueGames lists, per field, the names of the games that set their
// own value, sorted by name.
func CollectOwnValueGames(entries []OwnValueEntry) map[string][]string {
	games := map[string][]string{}
	for _, entry := range entries {
		if entry.Profile == nil {
			continue
		}
		for _, field := range AllFields() {
			if entry.Profile.IsOverridden(field) {
				games[field] = append(games[field], entry.Name)
			}
		}
	}
	for field := range games {
		slices.Sort(games[field])
	}
	return games
}

// OwnValueSuffix says how many games set their own value for a field on All
// games, so a change there is not mistaken for global. Empty for none.
func OwnValueSuffix(count int) string {
	switch {
	case count <= 0:
		return ""
	case count == 1:
		return "1 game has its own value"
	default:
		return fmt.Sprintf("%d games have their own value", count)
	}
}

// GamesText counts games: "1 game", "4 games".
func GamesText(count int) string {
	if count == 1 {
		return "1 game"
	}
	return fmt.Sprintf("%d games", count)
}

// namesShown is how many game names a sentence lists before it says "and N more".
const namesShown = 6

// ApplyToOwnValueBody asks before All games is applied to the games that set
// their own value for a field. It names the games and the value they will
// follow (left out when valueText is empty, as for a control that covers two
// fields), and says the change is saved at once and cannot be undone.
func ApplyToOwnValueBody(label, valueText string, names []string) string {
	list := strings.Join(names[:min(len(names), namesShown)], ", ")
	if len(names) > namesShown {
		list += fmt.Sprintf(" and %d more", len(names)-namesShown)
	}
	subject, verb := fmt.Sprintf("%d games have", len(names)), "They"
	if len(names) == 1 {
		subject, verb = "1 game has", "It"
	}
	follow := AllGamesName
	if valueText != "" {
		follow += " (" + valueText + ")"
	}
	return fmt.Sprintf("%s: %s their own value: %s. %s will follow %s. It is saved at once and cannot be undone.", label, subject, list, verb, follow)
}

// FollowFailureText names the games that could not be written, each with the
// reason, such as "Beta (disk full)". Empty when every write worked.
func FollowFailureText(outcome FollowOutcome) string {
	var failures []string
	for _, name := range outcome.Failed {
		failures = append(failures, fmt.Sprintf("%s (%v)", name, outcome.Errors[name]))
	}
	return strings.Join(failures, ", ")
}

// FollowOutcomeText says what an applied change did, for the message bar and
// the save line. A failure comes first, naming the game and the reason; the
// games that did follow are counted after it.
func FollowOutcomeText(label string, outcome FollowOutcome) string {
	followed := GamesText(len(outcome.Followed)) + " follow"
	if len(outcome.Followed) == 1 {
		followed += "s"
	}
	text := fmt.Sprintf("%s: %s %s", label, followed, AllGamesName)
	if failure := FollowFailureText(outcome); failure != "" {
		return "Not saved for " + failure + ". " + text
	}
	return text
}

// NumberHint names the unit and shown range of a numeric field. The range is
// guidance; no value is refused for it. Empty for a field that is not a free number.
func NumberHint(descriptor FieldDescriptor) string {
	if descriptor.Kind != PrimitiveInt || len(descriptor.AllowedValues) > 0 {
		return ""
	}
	hint := "Whole number"
	if descriptor.Unit != "" {
		hint += " in " + descriptor.Unit
	}
	if descriptor.Range != nil {
		hint += fmt.Sprintf(", shown range %d to %d", descriptor.Range.Min, descriptor.Range.Max)
	}
	return hint + "."
}
