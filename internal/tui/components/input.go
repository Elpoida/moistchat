package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"moistchat/internal/theme"
)

var commands = []string{"/create", "/join", "/list", "/user", "/leave", "/call", "/hang", "/mute", "/video", "/contrast", "/auth", "/settings", "/help", "/quit"}

type InputModel struct {
	textInput   textinput.Model
	lastMatches []string
	matchIndex  int
	baseValue   string
	roomActive  bool
}

func NewInputModel() InputModel {
	ti := textinput.New()
	ti.Placeholder = "Join a room to start chatting (/list, /join)"
	ti.Prompt = "> "
	ti.CharLimit = 500
	ti.Focus()
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(theme.ColorMuted)
	ti.PromptStyle = lipgloss.NewStyle().Foreground(theme.ColorText)

	return InputModel{textInput: ti}
}

func (m InputModel) SetThemeColors() InputModel {
	m.textInput.PlaceholderStyle = lipgloss.NewStyle().Foreground(theme.ColorMuted)
	m.textInput.PromptStyle = lipgloss.NewStyle().Foreground(theme.ColorText)
	return m
}

func (m InputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyTab {
		return m.handleTabCompletion()
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			input := strings.TrimSpace(m.textInput.Value())
			if input == "" {
				return m, nil
			}
			m.textInput.Reset()

			if strings.HasPrefix(input, "/") {
				return m, tea.Batch(cmd, func() tea.Msg {
					return CommandMsg{Text: input}
				})
			}

			return m, tea.Batch(cmd, func() tea.Msg {
				return ChatSendMsg{Text: input}
			})
		}
	}

	return m, cmd
}

func (m InputModel) SetRoomActive(active bool) InputModel {
	m.roomActive = active
	if active {
		m.textInput.Placeholder = "Type a message or /command..."
	} else {
		m.textInput.Placeholder = "Join a room to start chatting (/list, /join)"
	}
	return m
}

func (m InputModel) SetPrompt(prompt string) InputModel {
	m.textInput.Prompt = prompt
	return m
}

func (m InputModel) View() string {
	return m.textInput.View()
}

func (m InputModel) handleTabCompletion() (InputModel, tea.Cmd) {
	value := m.textInput.Value()
	if value == "" || !strings.HasPrefix(value, "/") {
		m.lastMatches = nil
		m.baseValue = ""
		return m, nil
	}

	cycling := false
	for _, prev := range m.lastMatches {
		if value == prev {
			value = m.baseValue
			cycling = true
			break
		}
	}

	var matches []string
	for _, cmd := range commands {
		if strings.HasPrefix(cmd, value) {
			matches = append(matches, cmd)
		}
	}

	switch len(matches) {
	case 1:
		m.textInput.SetValue(matches[0] + " ")
		m.textInput.CursorEnd()
		m.lastMatches = nil
		m.baseValue = ""
		return m, nil

	case 0:
		m.lastMatches = nil
		m.baseValue = ""
		return m, nil

	default:
		if cycling {
			idx := m.matchIndex % len(matches)
			m.textInput.SetValue(matches[idx])
			m.textInput.CursorEnd()
			m.matchIndex = (idx + 1) % len(matches)
			return m, nil
		}

		m.baseValue = value
		prefix := commonPrefix(matches)
		if prefix != value {
			m.textInput.SetValue(prefix)
			m.textInput.CursorEnd()
		} else {
			m.textInput.SetValue(matches[0])
			m.textInput.CursorEnd()
		}
		m.lastMatches = matches
		m.matchIndex = 1
		return m, nil
	}
}

func commonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs[1:] {
		for !strings.HasPrefix(s, prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}
	return prefix
}

type CommandMsg struct {
	Text string
}

type ChatSendMsg struct {
	Text string
}

type ShowMatchesMsg struct {
	Matches []string
}
