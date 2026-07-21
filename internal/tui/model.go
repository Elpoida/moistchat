package tui

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"moistchat/internal/media"
	"moistchat/internal/network"
	"moistchat/internal/state"
	"moistchat/internal/theme"
	"moistchat/internal/tui/components"
)

var (
	systemMsgStyle  lipgloss.Style
	usernameStyle   lipgloss.Style
	timestampStyle  lipgloss.Style
	contentStyle    lipgloss.Style
)

func init() {
	theme.OnThemeChange(func() {
		systemMsgStyle = lipgloss.NewStyle().Foreground(theme.ColorSubtle).Italic(true)
		usernameStyle = lipgloss.NewStyle().Foreground(theme.ColorTeal).Bold(true)
		timestampStyle = lipgloss.NewStyle().Foreground(theme.ColorMuted)
		contentStyle = lipgloss.NewStyle().Foreground(theme.ColorText)
	})
}

const renderWidth = 200

type sizes struct {
	width  int
	height int
}

type Model struct {
	state         *state.State
	node          *network.Node
	sizes         sizes
	startTime     time.Time
	chat          components.ChatModel
	input         components.InputModel
	video         components.VideoModel
	settings      components.SettingsPanel
	statusDisplay string
	webcam        *media.WebcamCapture
}

type FrameUpdateMsg struct {
	Frame string
	W     int
	H     int
}

type globeTickMsg struct{}

func formatMessages(msgs []state.Message) []string {
	var lines []string
	for _, msg := range msgs {
		ts := timestampStyle.Render("[" + msg.Timestamp.Format("15:04:05") + "]")
		if msg.System {
			for _, subLine := range strings.Split(msg.Content, "\n") {
				lines = append(lines, systemMsgStyle.Render(fmt.Sprintf("%s * %s", ts, subLine)))
			}
		} else {
			username := usernameStyle.Render(msg.Username)
			content := contentStyle.Render(msg.Content)
			lines = append(lines, strings.Join([]string{ts, username + ":", content}, " "))
		}
	}
	return lines
}

func NewModel(username string, firstRun bool) Model {
	if username == "" {
		username = "You"
	}
	s := state.New(username)
	welcomeMsg := "Welcome to moistchat! Type /help for commands."
	if firstRun {
		welcomeMsg = "Welcome to moistchat! Set your name with /user <name> and your auth key with /auth <key> to get started. Type /help for commands."
	}
	s.AddMessage(state.Message{
		Username: "System",
		Content:  welcomeMsg,
		System:   true,
	})

	node, err := network.NewNode()
	if err != nil {
		s.AddMessage(state.Message{
			Username: "System",
			Content:  fmt.Sprintf("Network offline: %s", err),
			System:   true,
		})
	}

	m := Model{
		state:         s,
		node:          node,
		startTime:     time.Now(),
		chat:          components.NewChatModel(),
		input:         components.NewInputModel(),
		video:         components.NewVideoModel(),
		settings:      components.NewSettingsPanel(),
		statusDisplay: "○ Starting",
	}
	m.settings = m.settings.SetAuthKey(network.AuthKey)
	m.chat = m.chat.SetLines(formatMessages(s.GetMessages()))
	return m
}

func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, tea.SetWindowTitle("moistchat"))
	cmds = append(cmds, tea.Tick(1000*time.Millisecond, func(t time.Time) tea.Msg {
		return globeTickMsg{}
	}))
	if m.node != nil {
		cmds = append(cmds, m.startNodeCmd())
		cmds = append(cmds, m.statusTickCmd())
	}
	return tea.Batch(cmds...)
}

func (m Model) statusTickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		if m.node == nil {
			return nil
		}
		return m.node.Status(context.Background())
	})
}

func (m Model) startNodeCmd() tea.Cmd {
	return func() tea.Msg {
		if err := m.node.Start(); err != nil {
			return network.NetworkEvent{
				Type:    "error",
				Content: err.Error(),
			}
		}
		return network.NetworkEvent{
			Type: "listening",
			Addr: m.node.LocalAddr(),
		}
	}
}

func (m Model) waitForNetworkCmd() tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-m.node.Events
		if !ok {
			return nil
		}
		return evt
	}
}

func (m Model) broadcastChatCmd(text string) tea.Cmd {
	return func() tea.Msg {
		m.node.Broadcast(network.WireMessage{
			Type:    "chat",
			Room:    m.state.RoomName,
			From:    m.state.Username,
			Content: text,
		}, m.state.Username)
		return nil
	}
}

func (m Model) sendToHostCmd(text string) tea.Cmd {
	return func() tea.Msg {
		if err := m.node.SendChat(text); err != nil {
			return network.NetworkEvent{
				Type:    "error",
				Content: fmt.Sprintf("send: %v", err),
			}
		}
		return nil
	}
}

func (m Model) connectToHostCmd(addr string) tea.Cmd {
	return func() tea.Msg {
		err := m.node.ConnectToHost(addr, m.state.RoomName, m.state.Username)
		if err != nil {
			return network.NetworkEvent{
				Type:    "error",
				Content: fmt.Sprintf("join: %v", err),
			}
		}
		return nil
	}
}

func (m Model) videoFrameCmd() tea.Cmd {
	return func() tea.Msg {
		if m.webcam == nil {
			log.Printf("[video] cmd: webcam nil")
			return nil
		}
		frame, ok := <-m.webcam.Frames()
		if !ok {
			log.Printf("[video] cmd: frames channel closed")
			m.video = m.video.SetVideoOn(false)
			return nil
		}
		h := m.video.GetMaxHeight()
		rendered := media.RenderFrame(frame, renderWidth, h)
		return FrameUpdateMsg{
			Frame: rendered,
			W:     frame.Width,
			H:     frame.Height,
		}
	}
}

func extractBrailleBytes(frame string) []byte {
	var raw []byte
	for _, r := range frame {
		if r >= 0x2800 && r <= 0x28FF {
			raw = append(raw, byte(r-0x2800))
		}
	}
	return raw
}

func reconstructBrailleString(dotBytes []byte, outW int) string {
	var sb strings.Builder
	sb.Grow(len(dotBytes) * 3 + len(dotBytes)/outW)
	for i, b := range dotBytes {
		if i > 0 && i%outW == 0 {
			sb.WriteByte('\n')
		}
		sb.WriteRune(rune(0x2800) + rune(b))
	}
	return sb.String()
}

func (m Model) videoBroadcastCmd() tea.Cmd {
	return func() tea.Msg {
		if m.node == nil || !m.video.VideoOn() || m.video.Frame() == "" {
			return nil
		}
		dotBytes := extractBrailleBytes(m.video.Frame())
		if len(dotBytes) == 0 {
			return nil
		}
		outW := renderWidth
		payload := make([]byte, 2+len(dotBytes))
		payload[0] = byte(outW >> 8)
		payload[1] = byte(outW)
		copy(payload[2:], dotBytes)
		if err := m.node.SendVideoFrame(payload); err != nil {
			log.Printf("[video] broadcast: %v", err)
		}
		return nil
	}
}

func (m Model) audioSendLoopCmd() tea.Cmd {
	return func() tea.Msg {
		pipeline := m.node.AudioPipeline()
		if pipeline == nil {
			return nil
		}
		for frame := range pipeline.Outgoing() {
			if err := m.node.SendAudio(frame.Data); err != nil {
				log.Printf("[audio] send error: %v", err)
				return nil
			}
		}
		return nil
	}
}

func (m Model) listRoomsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.node == nil {
			return ListRoomsMsg{}
		}
		rooms := m.node.DiscoverRooms(context.Background())
		return ListRoomsMsg{Rooms: rooms}
	}
}

type ListRoomsMsg struct {
	Rooms []network.RoomInfo
}

type JoinRoomMsg struct {
	Room  string
	Addr  string
	Host  string
	Error string
}

func (m Model) joinRoomCmd(roomName string) tea.Cmd {
	return func() tea.Msg {
		if m.node == nil {
			return JoinRoomMsg{Room: roomName, Error: "network not available"}
		}
		// Try lobby first for fresh room-to-host mapping
		rooms, err := m.node.LobbyListRooms()
		if err == nil {
			for _, r := range rooms {
				if r.Room == roomName {
					return JoinRoomMsg{Room: roomName, Addr: r.Addr, Host: r.Host}
				}
			}
		}
		// Fall back to cached DiscoveredRooms
		cached := m.state.GetDiscoveredRooms()
		if addr, ok := cached[roomName]; ok {
			return JoinRoomMsg{Room: roomName, Addr: addr}
		}
		return JoinRoomMsg{Room: roomName, Error: "room not found"}
	}
}

func (m Model) handleNetworkEvent(evt network.NetworkEvent) Model {
	switch evt.Type {
	case "listening":
		if m.node != nil {
			m.statusDisplay = m.node.Status(context.Background()).Summary()
		}
	case "peer_connected":
		m.state.AddPeer(state.Peer{ID: evt.From, Username: evt.From, Addr: evt.Addr})
		m = m.refreshVideo()
		m.state.AddMessage(state.Message{
			Username: "System",
			Content:  fmt.Sprintf("Peer connected: %s", evt.From),
			System:   true,
		})
	case "peer_disconnected":
		m.state.RemovePeer(evt.From)
		m.state.RemovePeerVideoFrame(evt.From)
		m = m.refreshVideo()
		m.state.AddMessage(state.Message{
			Username: "System",
			Content:  fmt.Sprintf("Peer disconnected: %s", evt.From),
			System:   true,
		})
	case "chat_message":
		m.state.AddMessage(state.Message{
			Username: evt.From,
			Content:  evt.Content,
		})
	case "video_frame":
		raw, err := base64.StdEncoding.DecodeString(evt.Content)
		if err != nil || len(raw) < 3 {
			return m.refreshChat()
		}
		outW := int(raw[0])<<8 | int(raw[1])
		m.state.SetPeerVideoFrame(evt.From, reconstructBrailleString(raw[2:], outW))
	case "peer_renamed":
		// Preserve the Addr from the old peer entry
		oldPeers := m.state.GetPeers()
		oldAddr := ""
		for _, p := range oldPeers {
			if p.ID == evt.From {
				oldAddr = p.Addr
				break
			}
		}
		m.state.RemovePeer(evt.From)
		m.state.AddPeer(state.Peer{ID: evt.Content, Username: evt.Content, Addr: oldAddr})
		m = m.refreshVideo()
		m.state.AddMessage(state.Message{
			Username: "System",
			Content:  fmt.Sprintf("%s renamed to %s", evt.From, evt.Content),
			System:   true,
		})
	case "info":
		m.state.AddMessage(state.Message{
			Username: "System",
			Content:  evt.Content,
			System:   true,
		})
	case "error":
		m.state.AddMessage(state.Message{
			Username: "System",
			Content:  fmt.Sprintf("Network error: %s", evt.Content),
			System:   true,
		})
	}
	return m.refreshChat()
}

func peerNames(peers []state.Peer) []string {
	names := make([]string, len(peers))
	for i, p := range peers {
		names[i] = p.Username
	}
	return names
}

func (m Model) Close() {
	if m.node != nil {
		m.node.LeaveRoom()
		m.node.Close()
	}
}

func (m Model) refreshVideo() Model {
	peers := m.state.GetPeers()
	names := make([]string, 0, len(peers)+1)
	if m.state.RoomName != "" {
		names = append(names, m.state.Username)
	}
	for _, p := range peers {
		names = append(names, p.ID)
	}
	m.video = m.video.SetPeers(names)
	peerFrames := make(map[string]string, len(peers))
	for _, p := range peers {
		if f := m.state.GetPeerVideoFrame(p.ID); f != "" {
			peerFrames[p.ID] = f
		}
	}
	m.video = m.video.SetPeerFrames(peerFrames)
	videoHeight := max(1, m.sizes.height*55/100)
	m.video = m.video.SetSize(m.sizes.width, videoHeight)
	return m
}

func (m Model) refreshChat() Model {
	msgs := m.state.GetMessages()
	m.chat = m.chat.SetLines(formatMessages(msgs))

	room := m.state.GetRoom()
	username := m.state.Username
	prompt := "> "
	if room != "" {
		prompt = fmt.Sprintf("[%s] > ", room)
		if m.node != nil && m.state.GetIsHost() {
			m.node.SetRoom(room)
			m.node.TryRegisterToLobby(room, username)
		}
	}
	m.input = m.input.SetPrompt(prompt)
	m.input = m.input.SetRoomActive(room != "")
	m.settings = m.settings.SetUsername(username)
	if username != "" && m.node != nil {
		if old, changed := m.node.SetUsername(username); changed {
			m.node.RenameSelf(old, username)
			if room != "" && m.state.GetIsHost() {
				m.node.UnregisterFromLobby(room)
				m.node.ClearLobbyRegisterFailed()
				m.node.TryRegisterToLobby(room, username)
			}
		}
	}

	m = m.refreshVideo()
	return m
}

func (m Model) leaveRoom() Model {
	roomName := m.state.RoomName
	if m.node != nil {
		m.node.UnregisterFromLobby(roomName)
		m.node.LeaveRoom()
	}

	m.state.AddMessage(state.Message{
		Username: "System",
		Content:  "You have left the room.",
		System:   true,
	})

	m.state.SetRoom("")
	m.state.SetIsHost(false)
	m.state.SetConnectedPeer("")
	m.state.SetPeers(nil)
	m = m.refreshVideo()
	m.statusDisplay = ""
	m.statusDisplay = m.node.Status(context.Background()).Summary()

	return m
}

func (m Model) electNewHost() (Model, tea.Cmd) {
	peers := m.state.GetPeers()
	log.Printf("[failover] electNewHost: current peers: %v", peerNames(peers))

	selfAddr := ""
	if m.node != nil {
		selfAddr = m.node.LocalAddr()
	}

	hasSelf := false
	if m.state.Username != "" {
		for _, p := range peers {
			if p.Username == m.state.Username {
				hasSelf = true
				break
			}
		}
		if !hasSelf {
			peers = append(peers, state.Peer{
				ID: m.state.Username, Username: m.state.Username,
				Addr: selfAddr,
			})
		}
	}

	log.Printf("[failover] peers after self-add: %v", peerNames(peers))

	if len(peers) == 0 {
		log.Printf("[failover] no peers available, cannot elect")
		m.state.AddMessage(state.Message{
			Username: "System",
			Content:  "Host disconnected. No peers available.",
			System:   true,
		})
		return m, nil
	}

	sort.Slice(peers, func(i, j int) bool {
		return peers[i].Username < peers[j].Username
	})

	newHost := peers[0]
	log.Printf("[failover] sorted peers: %v, elected new host: %s (addr=%s)", peerNames(peers), newHost.Username, newHost.Addr)

	if newHost.Username == m.state.Username {
		m.state.SetIsHost(true)
		m.state.AddMessage(state.Message{
			Username: "System",
			Content:  fmt.Sprintf("You are now hosting the room \"%s\"", m.state.RoomName),
			System:   true,
		})
		m.node.ClearLobbyRegisterFailed()
		m.node.SetRoom(m.state.RoomName)
		go m.node.TryRegisterToLobby(m.state.RoomName, m.state.Username)
		return m, nil
	}

	if newHost.Addr == "" {
		m.state.AddMessage(state.Message{
			Username: "System",
			Content:  fmt.Sprintf("New host is %s but their address is unknown. Run /list and /join again.", newHost.Username),
			System:   true,
		})
		return m, nil
	}

	m.state.SetConnectedPeer(newHost.Addr)
	m.state.AddMessage(state.Message{
		Username: "System",
		Content:  fmt.Sprintf("Reconnecting to new host: %s at %s", newHost.Username, newHost.Addr),
		System:   true,
	})
	return m, m.connectToHostCmd(newHost.Addr)
}
