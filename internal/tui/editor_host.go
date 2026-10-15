package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// EditorValueKind is the shared control contract used by Profile and Settings
// catalogs. Domain catalogs retain their own metadata and translate only the
// value shape into this type.
type EditorValueKind string

const (
	EditorBool    EditorValueKind = "bool"
	EditorChoice  EditorValueKind = "choice"
	EditorInteger EditorValueKind = "integer"
	EditorText    EditorValueKind = "text"
	EditorPath    EditorValueKind = "path"
)

type EditorSpec struct {
	Key     string
	Kind    EditorValueKind
	Choices []string
	// Labels are the words shown for each choice, in the same order. Without
	// them a choice shows as its raw value.
	Labels   []string
	Validate func(string) error
}

// EditorFocus is local to one editor. Tab always visits the same three controls.
type EditorFocus int

const (
	EditorFocusInput EditorFocus = iota
	EditorFocusApply
	EditorFocusCancel
)

// EditorHost owns the value being typed or picked. It has no persistence logic;
// Profile and Settings save a committed value at once.
type EditorHost struct {
	active   bool
	spec     EditorSpec
	original string
	value    string
	err      error
	focus    EditorFocus
	cursor   int
	// replace is true until the first edit of a numeric field: typing then
	// replaces the prefilled value instead of appending to it.
	replace bool
	// search is what has been typed to jump within a list of choices.
	search string
}

func (e *EditorHost) Begin(spec EditorSpec, value string) {
	e.active = true
	e.spec = spec
	e.original = value
	e.value = value
	e.err = nil
	e.focus = EditorFocusInput
	e.cursor = len([]rune(value))
	e.replace = spec.Kind == EditorInteger
	e.search = ""
}

func (e EditorHost) Active() bool          { return e.active }
func (e EditorHost) Value() string         { return e.value }
func (e EditorHost) Error() error          { return e.err }
func (e EditorHost) Dirty() bool           { return e.active && e.value != e.original }
func (e EditorHost) InputFocused() bool    { return e.active && e.focus == EditorFocusInput }
func (e EditorHost) Kind() EditorValueKind { return e.spec.Kind }

func (e *EditorHost) Set(value string) {
	e.replace = false
	e.value, e.err = value, nil
	e.cursor = len([]rune(value))
}

func (e *EditorHost) Append(text string) {
	if e.replace {
		e.value, e.cursor = "", 0
	}
	e.replace = false
	runes := []rune(e.value)
	e.cursor = min(max(e.cursor, 0), len(runes))
	insertion := []rune(text)
	e.value = string(runes[:e.cursor]) + string(insertion) + string(runes[e.cursor:])
	e.cursor += len(insertion)
	e.err = nil
}

func (e *EditorHost) Delete() {
	if e.spec.Kind == EditorBool || e.spec.Kind == EditorChoice {
		return
	}
	e.replace = false
	runes := []rune(e.value)
	e.cursor = min(max(e.cursor, 0), len(runes))
	if e.cursor > 0 {
		e.value = string(runes[:e.cursor-1]) + string(runes[e.cursor:])
		e.cursor--
	}
	e.err = nil
}

// UpdateInput handles text editing only. Command actions use the owning model's
// UpdateAction method and cannot reach an input while a button has focus.
func (e *EditorHost) UpdateInput(key tea.KeyPressMsg) bool {
	if e.InputFocused() && e.spec.Kind == EditorChoice && key.Text != "" {
		return e.TypeAhead(key.Text)
	}
	if !e.InputFocused() || e.spec.Kind == EditorBool || e.spec.Kind == EditorChoice {
		return false
	}
	runes := []rune(e.value)
	switch key.String() {
	case "left", "right", "home", "end":
		e.replace = false
	}
	switch key.String() {
	case "left":
		e.cursor = max(e.cursor-1, 0)
	case "right":
		e.cursor = min(e.cursor+1, len(runes))
	case "home":
		e.cursor = 0
	case "end":
		e.cursor = len(runes)
	case "backspace":
		e.Delete()
	case "delete":
		e.replace = false
		if e.cursor < len(runes) {
			e.value = string(runes[:e.cursor]) + string(runes[e.cursor+1:])
			e.err = nil
		}
	default:
		if key.Text == "" {
			return false
		}
		e.Append(key.Text)
	}
	return true
}

func (e *EditorHost) FocusNext() { e.focus = (e.focus + 1) % 3 }

func (e *EditorHost) Move(direction int) {
	if !e.InputFocused() {
		// Left/right stays on the button row; Tab is the route back to Input.
		e.focus = EditorFocus(1 + (int(e.focus)-1+direction+2)%2)
		return
	}
	if e.spec.Kind == EditorBool || e.spec.Kind == EditorChoice {
		e.Cycle(direction)
		return
	}
	e.replace = false
	e.cursor = min(max(e.cursor+direction, 0), len([]rune(e.value)))
}

func (e *EditorHost) Toggle() {
	if !e.InputFocused() {
		return
	}
	if e.spec.Kind == EditorBool || e.spec.Kind == EditorChoice {
		e.Cycle(1)
		return
	}
	e.Append(" ")
}

func (e EditorHost) EnterLabel() string {
	switch e.focus {
	case EditorFocusCancel:
		return "cancel edit"
	default:
		return "apply"
	}
}

func (e EditorHost) Controls(styles *Styles) string {
	labels := []string{"Input", "Apply", "Cancel"}
	for index, label := range labels {
		if EditorFocus(index) == e.focus {
			labels[index] = styles.FocusStyle().Render("> " + label)
		} else {
			labels[index] = styles.Dim.Render(label)
		}
	}
	return strings.Join(labels, "  ")
}

// InputView keeps a text cursor and the surrounding content within the field.
func (e EditorHost) InputView(width int) string {
	if e.spec.Kind == EditorBool || e.spec.Kind == EditorChoice {
		return e.value
	}
	runes := []rune(e.value)
	cursor := min(max(e.cursor, 0), len(runes))
	marker := ""
	if e.InputFocused() {
		marker = "▏"
	}
	start := 0
	if width > 0 {
		for start < cursor && ansi.StringWidth(string(runes[start:cursor])+marker) >= width {
			start++
		}
	}
	value := string(runes[start:cursor]) + marker + string(runes[cursor:])
	if width > 0 {
		return ansi.Truncate(value, width, "…")
	}
	return value
}

// TypeAhead jumps to the first choice whose label starts with what has been
// typed. A letter that continues no match starts a new search.
func (e *EditorHost) TypeAhead(text string) bool {
	if e.spec.Kind != EditorChoice || len(e.spec.Choices) == 0 {
		return false
	}
	e.err = nil
	if typed := strings.ToLower(text); e.search == typed {
		// The same letter again moves to the next choice that starts with it.
		var matches []int
		current := -1
		for index := range e.spec.Choices {
			if strings.HasPrefix(strings.ToLower(e.ChoiceLabel(index)), typed) {
				if e.spec.Choices[index] == e.value {
					current = len(matches)
				}
				matches = append(matches, index)
			}
		}
		if len(matches) > 1 && current >= 0 {
			e.value = e.spec.Choices[matches[(current+1)%len(matches)]]
			return true
		}
	}
	for _, typed := range []string{e.search + strings.ToLower(text), strings.ToLower(text)} {
		for index := range e.spec.Choices {
			if strings.HasPrefix(strings.ToLower(e.ChoiceLabel(index)), typed) {
				e.search, e.value = typed, e.spec.Choices[index]
				return true
			}
		}
	}
	e.search = ""
	return true
}

// ChoiceLabel is the word shown for one choice.
func (e EditorHost) ChoiceLabel(index int) string {
	if index >= 0 && index < len(e.spec.Labels) {
		return e.spec.Labels[index]
	}
	if index >= 0 && index < len(e.spec.Choices) {
		return e.spec.Choices[index]
	}
	return ""
}

// ChoiceRows returns the label of every choice and the index of the current one.
func (e EditorHost) ChoiceRows() ([]string, int) {
	labels := make([]string, len(e.spec.Choices))
	for index := range labels {
		labels[index] = e.ChoiceLabel(index)
	}
	return labels, max(slices.Index(e.spec.Choices, e.value), 0)
}

func (e *EditorHost) Cycle(direction int) {
	e.search = ""
	choices := e.spec.Choices
	if len(choices) == 0 && e.spec.Kind == EditorBool {
		choices = []string{"false", "true"}
	}
	if len(choices) == 0 {
		return
	}
	index := slices.Index(choices, e.value)
	if index < 0 {
		index = 0
	}
	e.value = choices[(index+direction+len(choices))%len(choices)]
	e.err = nil
}

func (e *EditorHost) Commit() (string, error) {
	if !e.active {
		return "", fmt.Errorf("no active editor")
	}
	if e.spec.Kind == EditorInteger {
		if _, err := strconv.Atoi(e.value); err != nil {
			e.err = fmt.Errorf("invalid integer: %s", e.value)
			e.focus = EditorFocusInput
			return "", e.err
		}
	}
	if len(e.spec.Choices) > 0 && !slices.Contains(e.spec.Choices, e.value) {
		e.err = fmt.Errorf("invalid value %q", e.value)
		e.focus = EditorFocusInput
		return "", e.err
	}
	if e.spec.Validate != nil {
		if err := e.spec.Validate(e.value); err != nil {
			e.err = err
			e.focus = EditorFocusInput
			return "", err
		}
	}
	value := e.value
	e.active = false
	e.err = nil
	return value, nil
}

func (e *EditorHost) Cancel() {
	e.value = e.original
	e.active = false
	e.err = nil
}
