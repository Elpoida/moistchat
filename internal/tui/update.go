package tui

import (
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"moistchat/internal/config"
	"moistchat/internal/media"
	"moistchat/internal/network"
	"moistchat/internal/state"
	"moistchat/internal/theme"
	"moistchat/internal/tui/components"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	netEventHandled := false

	if frameMsg, ok := msg.(FrameUpdateMsg); ok {
		m.video = m.video.SetFrame(frameMsg.Frame)
		if m.video.VideoOn() {
			cmds = append(cmds, m.videoFrameCmd())
			cmds = append(cmds, m.videoBroadcastCmd())
		}
	}

	if change, ok := msg.(components.SettingsUsernameChanged); ok {
		config.SaveUsername(change.Value)
		m.state.Username = change.Value
		m = m.refreshChat()
	}
	if change, ok := msg.(components.SettingsAuthKeyChanged); ok {
		config.SaveAuthKey(change.Value)
		m.settings = m.settings.SetAuthKey(change.Value)
		m.state.AddMessage(state.Message{
			Username: "System", Content: "Auth key saved. Restart the app to use it.", System: true,
		})
	}

	if _, ok := msg.(components.ThemeChanged); ok {
		m.input = m.input.SetThemeColors()
		m = m.refreshChat()
	}

	if _, ok := msg.(globeTickMsg); ok {
		cmds = append(cmds, tea.Tick(1000*time.Millisecond, func(t time.Time) tea.Msg {
			return globeTickMsg{}
		}))
	}

	if evt, ok := msg.(network.NetworkEvent); ok {
		netEventHandled = true
		if evt.Type == "audio_frame" && m.node.AudioActive() {
			decoded, err := base64.StdEncoding.DecodeString(evt.Content)
			if err == nil {
				m.node.AudioPipeline().EnqueueIncoming(media.AudioFrame{
					From: evt.From, Data: decoded,
				})
			}
		} else {
			hostDisconnected := evt.Type == "error" && evt.Content == "Disconnected from host"
			if evt.Type == "peer_disconnected" && !m.state.GetIsHost() {
				connectedAddr := m.state.GetConnectedPeer()
				for _, p := range m.state.GetPeers() {
					if p.ID == evt.From && p.Addr == connectedAddr {
						hostDisconnected = true
						break
					}
				}
			}
			m = m.handleNetworkEvent(evt)
			if hostDisconnected && !m.state.GetIsHost() {
				var electCmd tea.Cmd
				m, electCmd = m.electNewHost()
				m = m.refreshChat()
				cmds = append(cmds, electCmd)
			}
		}
		cmds = append(cmds, m.waitForNetworkCmd())
	}

	if m.settings.IsOpen() {
		var cmd tea.Cmd
		m.settings, cmd = m.settings.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.sizes.width = msg.Width
		m.sizes.height = msg.Height

	case components.ChatSendMsg:
		if m.state.RoomName == "" {
			m.state.AddMessage(state.Message{
				Username: "System",
				Content:  "You are not in a room. Use /list, /create, or /join first.",
				System:   true,
			})
			m = m.refreshChat()
			break
		}
		m.state.AddMessage(state.Message{
			Username: m.state.Username,
			Content:  msg.Text,
		})
		if m.state.GetIsHost() {
			cmds = append(cmds, m.broadcastChatCmd(msg.Text))
		} else if m.state.GetConnectedPeer() != "" {
			cmds = append(cmds, m.sendToHostCmd(msg.Text))
		}
		m = m.refreshChat()

	case components.CommandMsg:
		if msg.Text == "/quit" || msg.Text == "/exit" {
			return m, tea.Quit
		}
		if msg.Text == "/leave" {
			m = m.leaveRoom()
			return m, m.statusTickCmd()
		}
		if msg.Text == "/call" {
			if m.state.RoomName == "" {
				m.state.AddMessage(state.Message{
					Username: "System", Content: "You are not in a room.", System: true,
				})
				m = m.refreshChat()
				return m, nil
			}
			if m.node.AudioActive() {
				m.state.AddMessage(state.Message{
					Username: "System", Content: "Already in a voice call.", System: true,
				})
			} else {
				micID := ""
				spkID := ""
				if mic := m.settings.SelectedMic(); mic != nil {
					micID = mic.ID
				}
				if spk := m.settings.SelectedSpeaker(); spk != nil {
					spkID = spk.ID
				}
				if err := m.node.StartAudio(micID, spkID); err != nil {
					m.state.AddMessage(state.Message{
						Username: "System", Content: fmt.Sprintf("Voice call failed: %v", err), System: true,
					})
				} else {
					m.node.SendCallMessage(m.state.Username)
					m.state.AddMessage(state.Message{
						Username: "System", Content: "Joined voice call.", System: true,
					})
					cmds = append(cmds, m.audioSendLoopCmd())
				}
			}
			m = m.refreshChat()
			return m, tea.Batch(cmds...)
		}
		if msg.Text == "/hang" {
			if !m.node.AudioActive() {
				m.state.AddMessage(state.Message{
					Username: "System", Content: "Not in a voice call.", System: true,
				})
			} else {
				m.node.SendHangMessage(m.state.Username)
				if m.video.VideoOn() {
					if m.webcam != nil {
						m.webcam.Close()
						m.webcam = nil
					}
					m.video = m.video.SetVideoOn(false)
				}
				m.node.StopAudio()
				m.state.AddMessage(state.Message{
					Username: "System", Content: "Left voice call.", System: true,
				})
			}
			m = m.refreshChat()
			return m, nil
		}
		if msg.Text == "/mute" {
			if !m.node.AudioActive() {
				m.state.AddMessage(state.Message{
					Username: "System", Content: "Not in a voice call.", System: true,
				})
			} else {
				muted := m.node.ToggleMute()
				if muted {
					m.state.AddMessage(state.Message{
						Username: "System", Content: "Mic muted.", System: true,
					})
				} else {
					m.state.AddMessage(state.Message{
						Username: "System", Content: "Mic unmuted.", System: true,
					})
				}
			}
			m = m.refreshChat()
			return m, nil
		}
		if msg.Text == "/video" {
			if m.video.VideoOn() {
				if m.webcam != nil {
					m.webcam.Close()
					m.webcam = nil
				}
				m.video = m.video.SetVideoOn(false)
				m.state.AddMessage(state.Message{
					Username: "System", Content: "Video feed stopped.", System: true,
				})
				m = m.refreshChat()
				return m, nil
			}
			if !m.node.AudioActive() {
				m.state.AddMessage(state.Message{
					Username: "System", Content: "Join the voice call first (/call).", System: true,
				})
				m = m.refreshChat()
				return m, nil
			}
			device := "/dev/video0"
			wc, err := media.StartWebcam(device)
			if err != nil {
				m.state.AddMessage(state.Message{
					Username: "System",
					Content:  fmt.Sprintf("Camera error: %v", err),
					System:   true,
				})
				m = m.refreshChat()
				return m, nil
			}
			m.webcam = wc
			m.video = m.video.SetVideoOn(true)
			cmds = append(cmds, m.videoFrameCmd())
			cmds = append(cmds, m.videoBroadcastCmd())
			m.state.AddMessage(state.Message{
				Username: "System", Content: "Video feed started.", System: true,
			})
			m = m.refreshChat()
			return m, tea.Batch(cmds...)
		}
		if strings.HasPrefix(msg.Text, "/auth") {
			parts := strings.Fields(msg.Text)
			if len(parts) < 2 {
				m.state.AddMessage(state.Message{
					Username: "System", Content: "Usage: /auth <tailscale-auth-key>", System: true,
				})
			} else {
				config.SaveAuthKey(parts[1])
				m.settings = m.settings.SetAuthKey(parts[1])
				m.state.AddMessage(state.Message{
					Username: "System", Content: "Auth key saved. Restart the app to use it.", System: true,
				})
			}
			m = m.refreshChat()
			return m, nil
		}
		if msg.Text == "/contrast" || strings.HasPrefix(msg.Text, "/contrast ") {
			val := int(media.LumaThreshold) * 100 / 255
			parts := strings.SplitN(msg.Text, " ", 2)
			if len(parts) == 2 {
				fmt.Sscanf(parts[1], "%d", &val)
			}
			val = max(0, min(100, val))
			media.LumaThreshold = byte(val * 255 / 100)
			m.state.AddMessage(state.Message{
				Username: "System",
				Content:  fmt.Sprintf("Contrast: %d%% (threshold %d). Adjust in /settings.", val, media.LumaThreshold),
				System:   true,
			})
			m = m.refreshChat()
			return m, nil
		}
		if msg.Text == "/settings" {
			m.settings = m.settings.Toggle()
			if m.settings.IsOpen() {
				cmds = append(cmds, m.settings.Init())
			}
			return m, tea.Batch(cmds...)
		}

		switch {
		case strings.HasPrefix(msg.Text, "/list"):
			cmds = append(cmds, m.listRoomsCmd())
			m.state.AddMessage(state.Message{
				Username: "System",
				Content:  "Scanning tailnet for active rooms...",
				System:   true,
			})
			m = m.refreshChat()

		case strings.HasPrefix(msg.Text, "/join "):
			parts := strings.Fields(msg.Text)
			if len(parts) < 2 {
				m.state.AddMessage(state.Message{
					Username: "System",
					Content:  "Usage: /join <room-name>",
					System:   true,
				})
				m = m.refreshChat()
				break
			}
			cmds = append(cmds, m.joinRoomCmd(parts[1]))
			m.state.AddMessage(state.Message{
				Username: "System",
				Content:  "Looking up room...",
				System:   true,
			})
			m = m.refreshChat()
			m = m.refreshChat()

		default:
			ParseCommand(msg.Text, m.state)
			m = m.refreshChat()
		}

	case components.ShowMatchesMsg:
		m.state.AddMessage(state.Message{
			Username: "System",
			Content:  "Commands: " + strings.Join(msg.Matches, ", "),
			System:   true,
		})
		m = m.refreshChat()

	case network.NetworkEvent:
		if netEventHandled {
			break
		}
		netEventHandled = true
		if msg.Type == "audio_frame" && m.node.AudioActive() {
			decoded, err := base64.StdEncoding.DecodeString(msg.Content)
			if err == nil {
				m.node.AudioPipeline().EnqueueIncoming(media.AudioFrame{
					From: msg.From, Data: decoded,
				})
			}
			break
		}
		hostDisconnected := msg.Type == "error" && msg.Content == "Disconnected from host"
		if msg.Type == "peer_disconnected" && !m.state.GetIsHost() {
			connectedAddr := m.state.GetConnectedPeer()
			for _, p := range m.state.GetPeers() {
				if p.ID == msg.From && p.Addr == connectedAddr {
					hostDisconnected = true
					log.Printf("[failover] hostDisconnected via peer_disconnected: %s", msg.From)
					break
				}
			}
		}
		if hostDisconnected {
			log.Printf("[failover] hostDisconnected detected")
		}
		m = m.handleNetworkEvent(msg)
		if hostDisconnected && !m.state.GetIsHost() {
			log.Printf("[failover] triggering election, isHost=%v", m.state.GetIsHost())
			var electCmd tea.Cmd
			m, electCmd = m.electNewHost()
			m = m.refreshChat()
			log.Printf("[failover] election complete, isHost=%v", m.state.GetIsHost())
			cmds = append(cmds, electCmd)
		}

	case network.StatusMsg:
		m.statusDisplay = msg.Summary()
		if m.state.GetIsHost() {
			m.statusDisplay += " [Hosting: " + m.state.RoomName + "]"
		}
		m = m.refreshChat()
		return m, m.statusTickCmd()

	case ListRoomsMsg:
		rooms := make(map[string]string)
		var lines []string
		for _, r := range msg.Rooms {
			rooms[r.Room] = r.Addr
			lines = append(lines, fmt.Sprintf("  %s — host: %s", lipgloss.NewStyle().Foreground(theme.ColorOrange208).Render(r.Room), r.Host))
		}
		m.state.SetDiscoveredRooms(rooms)
		if len(lines) > 0 {
			m.state.AddMessage(state.Message{
				Username: "System",
				Content:  "Active rooms:\n" + strings.Join(lines, "\n"),
				System:   true,
			})
		} else {
			m.state.AddMessage(state.Message{
				Username: "System",
				Content:  "No active rooms found on tailnet.",
				System:   true,
			})
		}
		m = m.refreshChat()

	case JoinRoomMsg:
		if msg.Error != "" {
			m.state.AddMessage(state.Message{
				Username: "System",
				Content:  fmt.Sprintf("Room \"%s\" not found. Run /list first.", msg.Room),
				System:   true,
			})
			m = m.refreshChat()
			break
		}
		m.state.AddMessage(state.Message{
			Username: "System",
			Content:  fmt.Sprintf("Joining room \"%s\" at %s...", msg.Room, msg.Addr),
			System:   true,
		})
		m.state.SetRoom(msg.Room)
		m.state.SetIsHost(false)
		m.state.SetConnectedPeer(msg.Addr)
		cmds = append(cmds, m.connectToHostCmd(msg.Addr))
		m = m.refreshChat()

	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	m.chat, cmd = m.chat.Update(msg)
	cmds = append(cmds, cmd)

	m.video, cmd = m.video.Update(msg)
	cmds = append(cmds, cmd)

	if netEventHandled && m.node != nil {
		cmds = append(cmds, m.waitForNetworkCmd())
	}

	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "ctrl+c" {
		return m, tea.Quit
	}

	return m, tea.Batch(cmds...)
}
