package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"moistchat/internal/theme"
)

type VideoModel struct {
	peers      []string
	width      int
	maxHeight  int
	frame      string
	videoOn    bool
	peerFrames map[string]string
}

func subsampleBrailleFrame(frame string, tileW, tileH int) string {
	lines := strings.Split(frame, "\n")
	if len(lines) == 0 {
		return frame
	}
	outW := len([]rune(lines[0]))
	outH := len(lines)
	if outW <= tileW || outH <= tileH {
		return frame
	}
	stepX := outW / tileW
	stepY := outH / tileH
	var sb strings.Builder
	for ty := 0; ty < tileH; ty++ {
		line := []rune(lines[ty*stepY])
		for tx := 0; tx < tileW; tx++ {
			sb.WriteRune(line[tx*stepX])
		}
		if ty < tileH-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func NewVideoModel() VideoModel {
	return VideoModel{width: 48}
}

func (m VideoModel) SetPeers(names []string) VideoModel {
	m.peers = names
	return m
}

func (m VideoModel) SetSize(w, h int) VideoModel {
	m.width = max(24, w)
	m.maxHeight = max(1, h)
	return m
}

func (m VideoModel) GetMaxHeight() int {
	return m.maxHeight
}

func (m VideoModel) SetFrame(f string) VideoModel {
	m.frame = f
	return m
}

func (m VideoModel) Frame() string {
	return m.frame
}

func (m VideoModel) SetVideoOn(on bool) VideoModel {
	m.videoOn = on
	return m
}

func (m VideoModel) SetPeerFrames(frames map[string]string) VideoModel {
	m.peerFrames = frames
	return m
}

func (m VideoModel) VideoOn() bool {
	return m.videoOn
}

func (m VideoModel) Init() tea.Cmd { return nil }

func (m VideoModel) Update(msg tea.Msg) (VideoModel, tea.Cmd) {
	return m, nil
}

func avatar(username string, color lipgloss.Color, width, height int) string {
	initial := strings.ToUpper(username[:1])

	letter := lipgloss.NewStyle().
		Foreground(color).
		Bold(true).
		Render(initial)

	nameStyle := lipgloss.NewStyle().
		Foreground(theme.ColorGray)

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center)

	content := lipgloss.JoinVertical(lipgloss.Center,
		"",
		"  "+letter+"  ",
		nameStyle.Render(username),
	)

	return border.Render(content)
}

func aspectClamp(boxW, boxH int) (int, int) {
	h := max(1, boxH)
	w := h * 8 / 3
	if w <= boxW {
		return w, h
	}
	w = max(1, boxW)
	h = w * 3 / 8
	return max(1, w), max(1, h)
}

func tileBorder(width, height int, color lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center)
}

func (m VideoModel) View() string {
	if len(m.peers) == 0 {
		return lipgloss.NewStyle().
			Align(lipgloss.Center, lipgloss.Center).
			Render("Waiting for participants...")
	}

	n := len(m.peers)
	hasVideo := m.videoOn && m.frame != ""
	if !hasVideo {
		for _, peer := range m.peers[1:] {
			if m.peerFrames[peer] != "" {
				hasVideo = true
				break
			}
		}
	}

	var tileW, tileH, cols int
	if hasVideo {
		targetRows := 1
		for {
			h := max(3, m.maxHeight/targetRows-2)
			w := max(8, h*8/3)
			maxCols := max(1, m.width/(w+2))
			if maxCols*targetRows >= n {
				break
			}
			targetRows++
		}
		tileH = max(3, m.maxHeight/targetRows-2)
		tileW = max(8, tileH*8/3)
		cols = (n + targetRows - 1) / targetRows
	} else {
		tileW = 24
		tileH = 7
		cols = max(1, m.width/tileW)
		for {
			rows := (n + cols - 1) / cols
			if rows*tileH <= m.maxHeight || tileW <= 8 {
				break
			}
			tileW = max(8, tileW-2)
			tileH = max(3, m.maxHeight/rows)
		}
	}

	var tiles []string
	for i, peer := range m.peers {
		color := theme.AvatarColors[i%len(theme.AvatarColors)]
		var tile string
		switch {
		case i == 0 && m.videoOn && m.frame != "":
			subW, subH := aspectClamp(tileW-2, tileH-2)
			small := subsampleBrailleFrame(m.frame, subW, subH)
			tile = tileBorder(tileW, tileH, color).Render(small)
		case m.peerFrames[peer] != "":
			subW, subH := aspectClamp(tileW-2, tileH-2)
			small := subsampleBrailleFrame(m.peerFrames[peer], subW, subH)
			tile = tileBorder(tileW, tileH, color).Render(small)
		default:
			tile = avatar(peer, color, tileW, tileH)
		}
		tiles = append(tiles, tile)
	}

	rows := (len(tiles) + cols - 1) / cols
	gridH := rows * tileH
	var panel []string
	for i := 0; i < len(tiles); i += cols {
		end := i + cols
		if end > len(tiles) {
			end = len(tiles)
		}
		rowStr := lipgloss.JoinHorizontal(lipgloss.Top, tiles[i:end]...)
		panel = append(panel, rowStr)
	}
	result := lipgloss.JoinVertical(lipgloss.Left, panel...)
	if gridH < m.maxHeight {
		result += strings.Repeat("\n", m.maxHeight-gridH)
	}
	return result
}
