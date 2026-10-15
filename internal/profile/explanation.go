package profile

import (
	"fmt"
	"reflect"
)

// ExplanationSource names where an effective profile value came from.
type ExplanationSource string

const (
	ExplanationSourceDefault  ExplanationSource = "default"
	ExplanationSourceOverride ExplanationSource = "override"
	ExplanationSourceUnset    ExplanationSource = "unset"
)

// LaunchImpact classifies the launch surface a profile value can affect.
type LaunchImpact string

const (
	LaunchImpactEnvironment   LaunchImpact = "environment"
	LaunchImpactGameFile      LaunchImpact = "game_file"
	LaunchImpactSystemState   LaunchImpact = "system_state"
	LaunchImpactOverlay       LaunchImpact = "overlay"
	LaunchImpactCompatibility LaunchImpact = "compatibility"
)

// RestoreCoverage says whether a value needs cleanup after launch.
type RestoreCoverage string

const (
	RestoreCoverageNotApplicable      RestoreCoverage = "not_applicable"
	RestoreCoverageEphemeralLaunch    RestoreCoverage = "ephemeral_launch_environment"
	RestoreCoverageRestorableMutation RestoreCoverage = "restorable_mutation"
)

// ValueOrigin says, in words a user can act on, where a displayed value comes
// from. It refines ExplanationSource, which stays unchanged for existing
// consumers, by separating "All games sets this" from "nothing sets this".
type ValueOrigin string

const (
	// OriginThisGame: the game pins a value that differs from All games.
	OriginThisGame ValueOrigin = "this_game"
	// OriginThisGameSame: the game pins a value equal to All games, so it will
	// not follow later changes to All games.
	OriginThisGameSame ValueOrigin = "this_game_same"
	// OriginAllGames: the game follows a value that All games sets.
	OriginAllGames ValueOrigin = "all_games"
	// OriginSystemDefault: no profile sets the value.
	OriginSystemDefault ValueOrigin = "system_default"
	// OriginSetHere: the All games profile itself sets the value.
	OriginSetHere ValueOrigin = "set_here"
)

// FieldExplanation describes one effective profile value using shared source,
// impact, and restore semantics for CLI, TUI, and GUI presentation layers.
type FieldExplanation struct {
	Field        string
	Value        any
	DefaultValue any
	// AllGamesSets reports whether the All games profile sets this field, as
	// opposed to leaving it at the system default.
	AllGamesSets bool
	Source       ExplanationSource
	Origin       ValueOrigin
	Impact       LaunchImpact
	Restore      RestoreCoverage
}

// Explain returns one explanation for every visible profile field. Values come
// from raw overrides when pinned, from defaults when inherited and configured,
// or from the zero value when neither layer sets the field.
func (p *Profile) Explain(defaults *Profile) ([]FieldExplanation, error) {
	items := make([]FieldExplanation, 0, len(AllFields()))
	for _, field := range AllFields() {
		item, err := p.ExplainField(field, defaults)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// ExplainAllGames returns one explanation for every field of the All games
// profile itself: each is either set here or left at the system default.
func (p *Profile) ExplainAllGames() ([]FieldExplanation, error) {
	items := make([]FieldExplanation, 0, len(AllFields()))
	for _, field := range AllFields() {
		item, err := p.ExplainAllGamesField(field)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// ExplainAllGamesField describes one field of the All games profile itself.
func (p *Profile) ExplainAllGamesField(field string) (FieldExplanation, error) {
	if !IsValidField(field) {
		return FieldExplanation{}, fmt.Errorf("unknown profile field: %q", field)
	}
	descriptor, _ := Field(field)
	value, err := fieldAccessor(p, field)
	if err != nil {
		return FieldExplanation{}, err
	}
	item := FieldExplanation{
		Field: field,
		Value: explanationValue(value),
		// The All games values are the defaults themselves, so Source keeps
		// the meaning it always had here; Origin says whether one is set.
		Source:  ExplanationSourceDefault,
		Origin:  OriginSystemDefault,
		Impact:  descriptor.Impact,
		Restore: descriptor.Restore,
	}
	if p.setsField(field) {
		item.Origin = OriginSetHere
		item.AllGamesSets = true
	}
	return item, nil
}

// setsField reports whether the All games profile sets field: it is marked in
// Overrides, or, for files saved before the marker existed, holds a non-zero value.
func (p *Profile) setsField(field string) bool {
	if p == nil {
		return false
	}
	if p.IsOverridden(field) {
		return true
	}
	value, err := fieldAccessor(p, field)
	return err == nil && !value.IsZero()
}

// ExplainField describes one effective profile field.
func (p *Profile) ExplainField(field string, defaults *Profile) (FieldExplanation, error) {
	if !IsValidField(field) {
		return FieldExplanation{}, fmt.Errorf("unknown profile field: %q", field)
	}

	descriptor, _ := Field(field)

	item := FieldExplanation{
		Field:   field,
		Impact:  descriptor.Impact,
		Restore: descriptor.Restore,
		Source:  ExplanationSourceUnset,
		Origin:  OriginSystemDefault,
	}

	if defaults != nil {
		defaultValue, err := fieldAccessor(defaults, field)
		if err != nil {
			return FieldExplanation{}, err
		}
		item.DefaultValue = explanationValue(defaultValue)
		item.Value = item.DefaultValue
		item.Source = ExplanationSourceDefault
		if defaults.setsField(field) {
			item.AllGamesSets = true
			item.Origin = OriginAllGames
		}
	}

	if p != nil && p.IsOverridden(field) {
		overrideValue, err := fieldAccessor(p, field)
		if err != nil {
			return FieldExplanation{}, err
		}
		item.Value = explanationValue(overrideValue)
		item.Source = ExplanationSourceOverride
		item.Origin = OriginThisGame
		allGamesValue := item.DefaultValue
		if !defaults.setsField(field) {
			allGamesValue = nil
		}
		if reflect.DeepEqual(item.Value, allGamesValue) || (allGamesValue == nil && isZeroValue(item.Value)) {
			item.Origin = OriginThisGameSame
		}
	}

	if item.Value == nil {
		zero := &Profile{}
		zeroValue, err := fieldAccessor(zero, field)
		if err != nil {
			return FieldExplanation{}, err
		}
		item.Value = explanationValue(zeroValue)
	}

	return item, nil
}

func explanationValue(v reflect.Value) any {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		return v.Elem().Interface()
	}
	return v.Interface()
}

func isZeroValue(value any) bool {
	return value == nil || reflect.ValueOf(value).IsZero()
}
