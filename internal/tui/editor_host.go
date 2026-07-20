package tui

import (
	"fmt"
	"slices"
	"strconv"
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
	Key      string
	Kind     EditorValueKind
	Choices  []string
	Validate func(string) error
}

// EditorHost owns one recoverable field draft. It has no persistence logic;
// Profile and Settings transactions decide when a committed value is saved.
type EditorHost struct {
	active   bool
	spec     EditorSpec
	original string
	value    string
	err      error
}

func (e *EditorHost) Begin(spec EditorSpec, value string) {
	e.active = true
	e.spec = spec
	e.original = value
	e.value = value
	e.err = nil
}

func (e EditorHost) Active() bool  { return e.active }
func (e EditorHost) Value() string { return e.value }
func (e EditorHost) Error() error  { return e.err }
func (e EditorHost) Dirty() bool   { return e.active && e.value != e.original }

func (e *EditorHost) Set(value string)   { e.value, e.err = value, nil }
func (e *EditorHost) Append(text string) { e.value, e.err = e.value+text, nil }

func (e *EditorHost) Delete() {
	runes := []rune(e.value)
	if len(runes) > 0 {
		e.value = string(runes[:len(runes)-1])
	}
	e.err = nil
}

func (e *EditorHost) Cycle(direction int) {
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
			return "", e.err
		}
	}
	if len(e.spec.Choices) > 0 && !slices.Contains(e.spec.Choices, e.value) {
		e.err = fmt.Errorf("invalid value %q", e.value)
		return "", e.err
	}
	if e.spec.Validate != nil {
		if err := e.spec.Validate(e.value); err != nil {
			e.err = err
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
