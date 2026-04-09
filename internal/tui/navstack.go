package tui

import tea "charm.land/bubbletea/v2"

// StackEntry is a view that can be pushed onto the navigation stack.
// Each entry handles its own messages and renders its own content.
type StackEntry interface {
	// Name returns the display name for breadcrumb rendering.
	Name() string

	// Update handles a message and returns the updated entry and command.
	Update(msg tea.Msg) (StackEntry, tea.Cmd)

	// View renders the entry's content.
	View() string

	// SetSize updates the available dimensions for the entry.
	SetSize(width, height int)
}

// NavStack is a LIFO stack of StackEntry views.
type NavStack struct {
	entries []StackEntry
}

// NewNavStack creates a navigation stack with the given root entry.
// The root entry can never be popped.
func NewNavStack(root StackEntry) NavStack {
	return NavStack{entries: []StackEntry{root}}
}

// Push adds an entry to the top of the stack.
func (s *NavStack) Push(entry StackEntry) {
	s.entries = append(s.entries, entry)
}

// Pop removes the top entry from the stack, unless it is the root.
// Returns the popped entry and true, or nil and false if at root.
func (s *NavStack) Pop() (StackEntry, bool) {
	if len(s.entries) <= 1 {
		return nil, false // never pop the root
	}
	popped := s.entries[len(s.entries)-1]
	s.entries = s.entries[:len(s.entries)-1]
	return popped, true
}

// Top returns the entry at the top of the stack.
func (s *NavStack) Top() StackEntry {
	return s.entries[len(s.entries)-1]
}

// Depth returns the number of entries on the stack.
func (s *NavStack) Depth() int {
	return len(s.entries)
}

// Breadcrumbs returns the names of all entries in order, root to top.
func (s *NavStack) Breadcrumbs() []string {
	names := make([]string, len(s.entries))
	for i, e := range s.entries {
		names[i] = e.Name()
	}
	return names
}

// Replace swaps the top entry with a new one.
func (s *NavStack) Replace(entry StackEntry) {
	s.entries[len(s.entries)-1] = entry
}

// Navigation messages -- stack entries return these as tea.Cmd to request navigation.

type pushMsg struct {
	entry StackEntry
}

type popMsg struct {
	result tea.Msg
}

// contentEntry wraps ContentModel to implement StackEntry.
type contentEntry struct {
	model ContentModel
}

func newContentEntry(model ContentModel) *contentEntry {
	return &contentEntry{model: model}
}

func (e *contentEntry) Name() string {
	return e.model.Name()
}

func (e *contentEntry) Update(msg tea.Msg) (StackEntry, tea.Cmd) {
	updated, cmd := e.model.Update(msg)
	e.model = updated
	return e, cmd
}

func (e *contentEntry) View() string {
	return e.model.View()
}

func (e *contentEntry) SetSize(width, height int) {
	e.model.SetSize(width, height)
}
