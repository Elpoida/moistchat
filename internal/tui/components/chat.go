package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type ChatModel struct {
	lines        []string
	maxLines     int
	scrollOffset int
}

func NewChatModel() ChatModel {
	return ChatModel{maxLines: 100}
}

func (m ChatModel) Init() tea.Cmd { return nil }

func (m ChatModel) Update(msg tea.Msg) (ChatModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "pgup":
			m.scrollOffset += m.maxLines / 2
		case "pgdown":
			m.scrollOffset -= m.maxLines / 2
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
		case "home":
			off := len(m.lines) - m.maxLines
			if off < 0 {
				off = 0
			}
			m.scrollOffset = off
		case "end":
			m.scrollOffset = 0
		}
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.scrollOffset += 3
		case tea.MouseButtonWheelDown:
			m.scrollOffset -= 3
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
		}
	}
	return m, nil
}

func (m ChatModel) SetLines(lines []string) ChatModel {
	if len(lines) > len(m.lines) {
		m.scrollOffset = 0
	}
	m.lines = lines
	return m
}

func (m ChatModel) SetSize(width, height int) ChatModel {
	if height < 1 {
		height = 1
	}
	m.maxLines = height
	return m
}

func (m ChatModel) View() string {
	n := len(m.lines)
	if n == 0 {
		mf := m.maxLines
		if mf > 1 {
			return strings.Repeat("\n", mf-1)
		}
		return ""
	}

	maxOff := n - m.maxLines
	if maxOff < 0 {
		maxOff = 0
	}
	if m.scrollOffset > maxOff {
		m.scrollOffset = maxOff
	}

	start := n - m.maxLines - m.scrollOffset
	if start < 0 {
		start = 0
	}
	end := start + m.maxLines
	if end > n {
		end = n
	}

	visible := m.lines[start:end]
	result := strings.Join(visible, "\n")
	needed := m.maxLines - len(visible)
	if needed > 0 {
		result += strings.Repeat("\n", needed)
	}
	return result
}
