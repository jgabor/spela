package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type MessageType int

const (
	MessageInfo MessageType = iota
	MessageSuccess
	MessageError
)

const messageClearDuration = 5 * time.Second

const flashTickInterval = 100 * time.Millisecond

type MessageBarModel struct {
	styles      *Styles
	message     string
	messageType MessageType
	timestamp   time.Time
	width       int
	flashPhase  int // 0=off, 1=bright, 2=dim, 3+=done
}

type messageClearMsg struct {
	timestamp time.Time
}

type flashTickMsg struct {
	timestamp time.Time
}

func NewMessageBar(styles *Styles) MessageBarModel {
	return MessageBarModel{styles: styles}
}

func (m *MessageBarModel) SetWidth(width int) {
	m.width = width
}

func (m *MessageBarModel) SetMessage(message string, messageType MessageType) tea.Cmd {
	m.message = message
	m.messageType = messageType
	m.timestamp = time.Now()
	m.flashPhase = 1

	ts := m.timestamp
	return tea.Batch(
		tea.Tick(messageClearDuration, func(time.Time) tea.Msg {
			return messageClearMsg{timestamp: ts}
		}),
		tea.Tick(flashTickInterval, func(time.Time) tea.Msg {
			return flashTickMsg{timestamp: ts}
		}),
	)
}

func (m *MessageBarModel) Clear() {
	m.message = ""
}

func (m MessageBarModel) Update(msg tea.Msg) (MessageBarModel, tea.Cmd) {
	switch msg := msg.(type) {
	case messageClearMsg:
		if msg.timestamp.Equal(m.timestamp) {
			m.message = ""
			m.flashPhase = 0
		}
	case flashTickMsg:
		if msg.timestamp.Equal(m.timestamp) && m.flashPhase > 0 && m.flashPhase < 3 {
			m.flashPhase++
			ts := m.timestamp
			return m, tea.Tick(flashTickInterval, func(time.Time) tea.Msg {
				return flashTickMsg{timestamp: ts}
			})
		}
	}
	return m, nil
}

func (m MessageBarModel) View() string {
	if m.message == "" {
		return ""
	}

	var style lipgloss.Style
	switch m.messageType {
	case MessageSuccess:
		style = m.styles.Success
	case MessageError:
		style = m.styles.Error
	default:
		style = m.styles.Dim
	}

	// Apply flash background for phases 1 (bright) and 2 (dim).
	t := m.styles.Theme
	switch m.flashPhase {
	case 1:
		switch m.messageType {
		case MessageSuccess:
			style = style.Background(t.Success)
		case MessageError:
			style = style.Background(t.Error)
		}
		style = style.Foreground(t.SurfaceBase)
	case 2:
		style = style.Background(t.SurfaceRaised)
	}

	return style.Width(m.width).Padding(0, 1).Render(m.message)
}

func (m MessageBarModel) HasMessage() bool {
	return m.message != ""
}
