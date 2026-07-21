package components

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"moistchat/internal/config"
	"moistchat/internal/media"
	"moistchat/internal/theme"
)

type DevicesLoadedMsg struct {
	Mics     []media.DeviceInfo
	Speakers []media.DeviceInfo
	Webcams  []media.DeviceInfo
}

type SettingsUsernameChanged struct{ Value string }
type SettingsAuthKeyChanged struct{ Value string }
type ThemeChanged struct{}

type SettingsPanel struct {
	open            bool
	focus           int
	username        string
	authKey         string
	editingUsername bool
	editingAuthKey  bool
	usernameInput   textinput.Model
	authKeyInput    textinput.Model
	mics            []media.DeviceInfo
	speakers        []media.DeviceInfo
	webcams         []media.DeviceInfo
	selectedMic     int
	selectedSpeaker int
	selectedWebcam  int
	micLevel        int
	speakerLevel    int
	contrast        int
	themeName       string
	keyChanged      bool
	loading         bool
	storedMicID     string
	storedSpeakerID string
	storedWebcamID  string
}

var (
	settingsBorderStyle lipgloss.Style
	settingsLabelStyle  lipgloss.Style
	settingsValueStyle  lipgloss.Style
	settingsFocusStyle  lipgloss.Style
	settingsDimStyle    lipgloss.Style
	settingsBarStyle    lipgloss.Style
	settingsEmptyStyle  lipgloss.Style
	settingsAlertStyle  lipgloss.Style
)

func init() {
	theme.OnThemeChange(func() {
		settingsBorderStyle = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(theme.ColorSubtle).
					Padding(1, 2).
					Width(0).
					Height(0)
		settingsLabelStyle = lipgloss.NewStyle().
					Foreground(theme.ColorDimText).
					Width(16).
					Align(lipgloss.Left)
		settingsValueStyle = lipgloss.NewStyle().
					Foreground(theme.ColorText)
		settingsFocusStyle = lipgloss.NewStyle().
					Foreground(theme.ColorTeal).
					Bold(true)
		settingsDimStyle = lipgloss.NewStyle().
					Foreground(theme.ColorMuted)
		settingsBarStyle = lipgloss.NewStyle().
					Foreground(theme.ColorTeal)
		settingsEmptyStyle = lipgloss.NewStyle().
					Foreground(theme.ColorMuted).
					Italic(true)
		settingsAlertStyle = lipgloss.NewStyle().
					Foreground(theme.ColorOrange).
					Bold(true)
	})
}

func NewSettingsPanel() SettingsPanel {
	ui := textinput.New()
	ui.Placeholder = "Enter username..."
	ak := textinput.New()
	ak.Placeholder = "Enter auth key..."
	ak.EchoMode = textinput.EchoPassword
	ak.EchoCharacter = '•'

	cfg, _ := config.Load()
	return SettingsPanel{
		micLevel:        80,
		speakerLevel:    100,
		contrast:       30,
		loading:         true,
		usernameInput:   ui,
		authKeyInput:    ak,
		storedMicID:     cfg.MicID,
		storedSpeakerID: cfg.SpeakerID,
		storedWebcamID:  cfg.WebcamID,
	}
}

func (s SettingsPanel) IsOpen() bool {
	return s.open
}

func (s SettingsPanel) Toggle() SettingsPanel {
	s.open = !s.open
	s.focus = 0
	if s.open {
		s.loading = true
		s.contrast = int(media.LumaThreshold) * 100 / 255
		s.themeName = theme.Active
	}
	return s
}

func (s SettingsPanel) SetUsername(name string) SettingsPanel {
	s.username = name
	return s
}

func (s SettingsPanel) SelectedMic() *media.DeviceInfo {
	if len(s.mics) == 0 || s.selectedMic >= len(s.mics) {
		return nil
	}
	return &s.mics[s.selectedMic]
}

func (s SettingsPanel) SelectedSpeaker() *media.DeviceInfo {
	if len(s.speakers) == 0 || s.selectedSpeaker >= len(s.speakers) {
		return nil
	}
	return &s.speakers[s.selectedSpeaker]
}

func (s SettingsPanel) SetAuthKey(key string) SettingsPanel {
	s.authKey = key
	return s
}

func loadDevicesCmd() tea.Cmd {
	return func() tea.Msg {
		var result DevicesLoadedMsg
		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()
			mics, err := media.ListMicrophones()
			if err == nil {
				result.Mics = mics
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			speakers, err := media.ListSpeakers()
			if err == nil {
				result.Speakers = speakers
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			cams, err := media.ListWebcams()
			if err == nil {
				result.Webcams = cams
			}
		}()

		wg.Wait()
		return result
	}
}

func findDeviceIndex(devs []media.DeviceInfo, id string) int {
	for i, d := range devs {
		if d.ID == id {
			return i
		}
	}
	return 0
}

func maskedKey(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:4] + strings.Repeat("*", len(key)-7) + key[len(key)-3:]
}

func (s SettingsPanel) Init() tea.Cmd {
	return loadDevicesCmd()
}

func (s SettingsPanel) Update(msg tea.Msg) (SettingsPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case DevicesLoadedMsg:
		s.mics = msg.Mics
		s.speakers = msg.Speakers
		s.webcams = msg.Webcams
		s.selectedMic = findDeviceIndex(s.mics, s.storedMicID)
		s.selectedSpeaker = findDeviceIndex(s.speakers, s.storedSpeakerID)
		s.selectedWebcam = findDeviceIndex(s.webcams, s.storedWebcamID)
		s.loading = false

	case tea.KeyMsg:
		key := msg.String()

		if s.editingUsername {
			switch key {
			case "enter":
				newVal := s.usernameInput.Value()
				s.editingUsername = false
				s.usernameInput.Blur()
				s.username = newVal
				return s, func() tea.Msg { return SettingsUsernameChanged{Value: newVal} }
			case "esc":
				s.editingUsername = false
				s.usernameInput.Blur()
				return s, nil
			default:
				var cmd tea.Cmd
				s.usernameInput, cmd = s.usernameInput.Update(msg)
				return s, cmd
			}
		}

		if s.editingAuthKey {
			switch key {
			case "enter":
				newVal := s.authKeyInput.Value()
				s.editingAuthKey = false
				s.authKeyInput.Blur()
				s.authKey = newVal
				s.keyChanged = true
				return s, func() tea.Msg { return SettingsAuthKeyChanged{Value: newVal} }
			case "esc":
				s.editingAuthKey = false
				s.authKeyInput.Blur()
				return s, nil
			default:
				var cmd tea.Cmd
				s.authKeyInput, cmd = s.authKeyInput.Update(msg)
				return s, cmd
			}
		}

		switch key {
		case "esc":
			s.open = false

		case "up":
			if s.focus > 0 {
				s.focus--
			}

		case "down":
			if s.focus < 8 {
				s.focus++
			}

		case "left", "right", "enter":
			k := msg.String()
			switch s.focus {
			case 0: // Username (inline edit)
				if k == "enter" {
					s.editingUsername = true
					s.usernameInput.SetValue(s.username)
					s.usernameInput.Focus()
					s.usernameInput.CursorEnd()
					return s, textinput.Blink
				}
			case 1: // Auth Key (inline edit)
				if k == "enter" {
					s.editingAuthKey = true
					s.authKeyInput.SetValue(s.authKey)
					s.authKeyInput.Focus()
					s.authKeyInput.CursorEnd()
					return s, textinput.Blink
				}
			case 2: // Input mic
				if len(s.mics) == 0 {
					break
				}
				if k == "right" || k == "enter" {
					s.selectedMic = (s.selectedMic + 1) % len(s.mics)
				} else if k == "left" {
					s.selectedMic--
					if s.selectedMic < 0 {
						s.selectedMic = len(s.mics) - 1
					}
				}
			s.storedMicID = s.mics[s.selectedMic].ID
			config.SaveMicID(s.storedMicID)
		case 3: // Mic Level
				if k == "right" && s.micLevel < 100 {
					s.micLevel += 5
				}
				if k == "left" && s.micLevel > 0 {
					s.micLevel -= 5
				}
			case 4: // Output speaker
				if len(s.speakers) == 0 {
					break
				}
				if k == "right" || k == "enter" {
					s.selectedSpeaker = (s.selectedSpeaker + 1) % len(s.speakers)
				} else if k == "left" {
					s.selectedSpeaker--
					if s.selectedSpeaker < 0 {
						s.selectedSpeaker = len(s.speakers) - 1
					}
				}
			s.storedSpeakerID = s.speakers[s.selectedSpeaker].ID
			config.SaveSpeakerID(s.storedSpeakerID)
		case 5: // Spk Level
				if k == "right" && s.speakerLevel < 100 {
					s.speakerLevel += 5
				}
				if k == "left" && s.speakerLevel > 0 {
					s.speakerLevel -= 5
				}
			case 6: // Webcam
				if len(s.webcams) == 0 {
					break
				}
				if k == "right" || k == "enter" {
					s.selectedWebcam = (s.selectedWebcam + 1) % len(s.webcams)
				} else if k == "left" {
					s.selectedWebcam--
					if s.selectedWebcam < 0 {
						s.selectedWebcam = len(s.webcams) - 1
					}
				}
			s.storedWebcamID = s.webcams[s.selectedWebcam].ID
			config.SaveWebcamID(s.storedWebcamID)
		case 7: // Contrast
				if k == "right" && s.contrast < 100 {
					s.contrast += 5
				}
				if k == "left" && s.contrast > 0 {
					s.contrast -= 5
				}
				media.LumaThreshold = byte(s.contrast * 255 / 100)
			case 8: // Theme
				if k == "left" || k == "right" {
					themes := []string{"Solarized", "eDEX", "Amber", "Neon"}
					idx := 0
					for i, t := range themes {
						if t == s.themeName {
							idx = i
							break
						}
					}
					if k == "right" {
						idx = (idx + 1) % len(themes)
					} else {
						idx = (idx - 1 + len(themes)) % len(themes)
					}
					s.themeName = themes[idx]
					config.SaveThemeName(s.themeName)
					theme.SetTheme(s.themeName)
					return s, func() tea.Msg { return ThemeChanged{} }
				}
			}
		}
	}

	return s, nil
}

func cursor(focused bool) string {
	if focused {
		return settingsFocusStyle.Render("> ")
	}
	return "  "
}

func (s SettingsPanel) line(label string, val string, focused bool) string {
	labelRendered := settingsLabelStyle.Render(label)
	valRendered := cursor(focused)
	if val == "" {
		valRendered += settingsEmptyStyle.Render("(none)")
	} else {
		valRendered += val
	}
	return labelRendered + "  " + valRendered
}

func volBar(level int) string {
	bars := level / 10
	fill := settingsBarStyle.Render(strings.Repeat("█", bars))
	empty := settingsDimStyle.Render(strings.Repeat("░", 10-bars))
	return fmt.Sprintf("%s%s %d%%", fill, empty, level)
}

func (s SettingsPanel) View() string {
	if s.loading {
		return settingsBorderStyle.Render("Loading devices...")
	}

	var lines []string

	lines = append(lines, settingsFocusStyle.Render("Settings"))
	lines = append(lines, "")

	// Username (inline editable)
	if s.editingUsername {
		lines = append(lines, fmt.Sprintf(
			"%s  %s%s",
			settingsLabelStyle.Render("Username:"),
			cursor(true),
			s.usernameInput.View(),
		))
	} else {
		lines = append(lines, s.line("Username:", s.username, s.focus == 0))
	}

	// Auth Key (inline editable)
	if s.editingAuthKey {
		lines = append(lines, fmt.Sprintf(
			"%s  %s%s",
			settingsLabelStyle.Render("Auth Key:"),
			cursor(true),
			s.authKeyInput.View(),
		))
	} else {
		masked := ""
		if s.authKey != "" {
			masked = maskedKey(s.authKey)
		}
		lines = append(lines, s.line("Auth Key:", masked, s.focus == 1))
	}
	if s.keyChanged {
		lines = append(lines, "  "+settingsAlertStyle.Render("Please restart the app to apply your new authentication key."))
	}

	lines = append(lines, "")
	lines = append(lines, settingsDimStyle.Render("── Audio ──"))
	lines = append(lines, "")

	// Audio input
	devName := ""
	if s.selectedMic < len(s.mics) {
		devName = s.mics[s.selectedMic].Name
	}
	lines = append(lines, s.line("Input:", devName, s.focus == 2))

	// Audio input volume
	lines = append(lines, fmt.Sprintf(
		"%s  %s%s",
		settingsLabelStyle.Render("Mic Level:"),
		cursor(s.focus == 3),
		volBar(s.micLevel),
	))

	// Audio output
	devName = ""
	if s.selectedSpeaker < len(s.speakers) {
		devName = s.speakers[s.selectedSpeaker].Name
	}
	lines = append(lines, s.line("Output:", devName, s.focus == 4))

	// Audio output volume
	lines = append(lines, fmt.Sprintf(
		"%s  %s%s",
		settingsLabelStyle.Render("Spk Level:"),
		cursor(s.focus == 5),
		volBar(s.speakerLevel),
	))

	// Webcam
	lines = append(lines, "")
	lines = append(lines, settingsDimStyle.Render("── Video ──"))
	lines = append(lines, "")
	devName = ""
	if s.selectedWebcam < len(s.webcams) {
		devName = s.webcams[s.selectedWebcam].Name
	}
	lines = append(lines, s.line("Webcam:", devName, s.focus == 6))

	lines = append(lines, fmt.Sprintf(
		"%s  %s%s",
		settingsLabelStyle.Render("Contrast:"),
		cursor(s.focus == 7),
		volBar(s.contrast),
	))

	lines = append(lines, "")
	lines = append(lines, settingsDimStyle.Render("── Display ──"))
	lines = append(lines, "")
	lines = append(lines, s.line("Theme:", s.themeName, s.focus == 8))

	lines = append(lines, "")
	lines = append(lines, settingsDimStyle.Render("↑↓ navigate  →← cycle/volume  Enter edit  Esc close"))

	content := strings.Join(lines, "\n")
	return settingsBorderStyle.Render(content)
}
