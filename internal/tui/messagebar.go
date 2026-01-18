package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type MessageType int

const (
	MessageInfo MessageType = iota
	MessageSuccess
	MessageError
)

const messageClearDuration = 5 * time.Second

type MessageBarModel struct {
	message     string
	messageType MessageType
	timestamp   time.Time
	width       int
}

type messageClearMsg struct {
	timestamp time.Time
}

func NewMessageBar() MessageBarModel {
	return MessageBarModel{}
}

func (m *MessageBarModel) SetWidth(width int) {
	m.width = width
}

func (m *MessageBarModel) SetMessage(message string, messageType MessageType) tea.Cmd {
	m.message = message
	m.messageType = messageType
	m.timestamp = time.Now()

	ts := m.timestamp
	return tea.Tick(messageClearDuration, func(time.Time) tea.Msg {
		return messageClearMsg{timestamp: ts}
	})
}

func (m *MessageBarModel) Clear() {
	m.message = ""
}

func (m MessageBarModel) Update(msg tea.Msg) (MessageBarModel, tea.Cmd) {
	switch msg := msg.(type) {
	case messageClearMsg:
		if msg.timestamp.Equal(m.timestamp) {
			m.message = ""
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
		style = successStyle
	case MessageError:
		style = errorStyle
	default:
		style = dimStyle
	}

	return style.Width(m.width).Padding(0, 1).Render(m.message)
}

func (m MessageBarModel) HasMessage() bool {
	return m.message != ""
}
