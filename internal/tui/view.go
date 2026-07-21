package tui

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"moistchat/internal/config"
	"moistchat/internal/network"
	"moistchat/internal/theme"
)

var (
	dividerStyle lipgloss.Style
)

func init() {
	theme.OnThemeChange(func() {
		dividerStyle = lipgloss.NewStyle().
				Foreground(theme.ColorDivider)
	})
}

func padToHeight(s string, target int) string {
	lines := strings.Count(s, "\n") + 1
	if lines > target {
		parts := strings.SplitN(s, "\n", target+1)
		return strings.Join(parts[:target], "\n")
	}
	for lines < target {
		s += "\n"
		lines++
	}
	return s
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm %ds", m, s)
}

func (m Model) buildLeftSidebar(w int) string {
	tealBold := lipgloss.NewStyle().Foreground(theme.ColorTeal).Bold(true)
	dim := lipgloss.NewStyle().Foreground(theme.ColorDimText)

	var lines []string
	lines = append(lines, tealBold.Render("SYSTEM"))
	lines = append(lines, "")

	authStatus := "set"
	if network.AuthKey == "" {
		authStatus = "not set"
	}
	lines = append(lines, dim.Render("Name:   ")+tealBold.Render(m.state.Username))
	lines = append(lines, dim.Render("Auth:   ")+tealBold.Render(authStatus))
	cfgOK := "ok"
	if !config.Exists() {
		cfgOK = "not found"
	}
	lines = append(lines, dim.Render("Config: ")+tealBold.Render(cfgOK))
	lines = append(lines, dim.Render("Theme:  ")+tealBold.Render(theme.Active))
	uptime := time.Since(m.startTime)
	lines = append(lines, dim.Render("Up:     ")+tealBold.Render(formatDuration(uptime)))
	lines = append(lines, dim.Render("Time:   ")+tealBold.Render(time.Now().Format("15:04:05")))

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	lines = append(lines, "")
	lines = append(lines, dim.Render("OS:     ")+tealBold.Render(fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)))
	lines = append(lines, dim.Render("Engine: ")+tealBold.Render(runtime.Version()))
	lines = append(lines, dim.Render("Memory: ")+tealBold.Render(fmt.Sprintf("%.1f MB", float64(ms.Alloc)/1024/1024)))
	lines = append(lines, dim.Render("GoRtns: ")+tealBold.Render(fmt.Sprintf("%d", runtime.NumGoroutine())))
	lines = append(lines, dim.Render("Cores:  ")+tealBold.Render(fmt.Sprintf("%d", runtime.NumCPU())))
	lines = append(lines, dim.Render("PID:    ")+tealBold.Render(fmt.Sprintf("%d", os.Getpid())))
	lines = append(lines, dim.Render("GC Runs:")+tealBold.Render(fmt.Sprintf("%d", ms.NumGC)))

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorSubtle).
		Width(w-2).
		Padding(0, 1).
		Render(content)
}

func (m Model) buildRightSidebar(w int) string {
	tealBold := lipgloss.NewStyle().Foreground(theme.ColorTeal).Bold(true)
	green := lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true)
	dim := lipgloss.NewStyle().Foreground(theme.ColorDimText)

	var lines []string
	lines = append(lines, tealBold.Render("NETWORK"))
	lines = append(lines, "")

	cleanDisplay := m.statusDisplay
	if idx := strings.Index(cleanDisplay, " [Hosting:"); idx >= 0 {
		cleanDisplay = cleanDisplay[:idx]
	}
	parts := strings.Fields(cleanDisplay)
	isConnected := len(parts) >= 2 && parts[0] == "●" && parts[1] == "Connected"
	hostname := "-"
	wanIP := "-"
	localIP := "-"
	if isConnected {
		lines = append(lines, green.Render("● Connected"))
		if len(parts) >= 3 {
			hostname = parts[2]
		}
		var ips []string
		for i := 3; i < len(parts) && !strings.HasPrefix(parts[i], "[Hosting"); i++ {
			ips = append(ips, strings.TrimRight(parts[i], ","))
		}
		if len(ips) >= 1 {
			wanIP = ips[0]
		}
		if len(ips) >= 2 {
			localIP = ips[1]
		}
	} else {
		lines = append(lines, dim.Render(m.statusDisplay))
	}

	lines = append(lines, dim.Render("Host:   ")+tealBold.Render(hostname))
	lines = append(lines, dim.Render("IP:     WAN: ")+tealBold.Render(wanIP))
	if localIP != "-" {
		lines = append(lines, dim.Render("        LAN: ")+tealBold.Render(localIP))
	}

	lines = append(lines, "")
	peers := m.state.GetPeers()
	count := len(peers)
	if m.state.RoomName != "" {
		count++
	}
	lines = append(lines, dim.Render("Peers     ")+tealBold.Render(fmt.Sprintf("%d", count)))

	if room := m.state.RoomName; room != "" {
		lines = append(lines, "")
		lines = append(lines, dim.Render("Room:   ")+tealBold.Render(room))
		if m.state.GetIsHost() {
			lines = append(lines, dim.Render("Status: ")+tealBold.Render("Hosting"))
		}
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorSubtle).
		Width(w-2).
		Padding(0, 1).
		Render(content)
}

const baseWorldMap = "" +
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣀⣄⣠⣀⡀⣀⣠⣤⣤⣤⣀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀" +
	"\n" +
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣄⢠⣠⣼⣿⣿⣿⣟⣿⣿⣿⣿⣿⣿⣿⣿⡿⠋⠀⠀⠀⢠⣤⣦⡄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠰⢦⣄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀" +
	"\n" +
	"⠀⠀⠀⠀⠀⠀⠀⠀⣼⣿⣟⣾⣿⣽⣿⣿⣅⠈⠉⠻⣿⣿⣿⣿⣿⡿⠇⠀⠀⠀⠀⠀⠉⠀⠀⠀⠀⠀⢀⡶⠒⢉⡀⢠⣤⣶⣶⣿⣷⣆⣀⡀⠀⢲⣖⠒⠀⠀⠀⠀⠀⠀⠀" +
	"\n" +
	"⢀⣤⣾⣶⣦⣤⣤⣶⣿⣿⣿⣿⣿⣿⣽⡿⠻⣷⣀⠀⢻⣿⣿⣿⡿⠟⠀⠀⠀⠀⠀⠀⣤⣶⣶⣤⣀⣀⣬⣷⣦⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣶⣦⣤⣦⣼⣀⠀" +
	"\n" +
	"⠈⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠛⠓⣿⣿⠟⠁⠘⣿⡟⠁⠀⠘⠛⠁⠀⠀⢠⣾⣿⢿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠏⠙⠁" +
	"\n" +
	"⠀⠸⠟⠋⠀⠈⠙⣿⣿⣿⣿⣿⣿⣷⣦⡄⣿⣿⣿⣆⠀⠀⠀⠀⠀⠀⠀⠀⣼⣆⢘⣿⣯⣼⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡉⠉⢱⡿⠀⠀⠀⠀⠀" +
	"\n" +
	"⠀⠀⠀⠀⠀⠀⠀⠘⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣟⡿⠦⠀⠀⠀⠀⠀⠀⠀⠙⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⡗⠀⠈⠀⠀⠀⠀⠀⠀" +
	"\n" +
	"⠀⠀⠀⠀⠀⠀⠀⠀⢻⣿⣿⣿⣿⣿⣿⣿⣿⠋⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⢿⣿⣉⣿⡿⢿⢷⣾⣾⣿⣞⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠋⣠⠟⠀⠀⠀⠀⠀⠀⠀⠀" +
	"\n" +
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠹⣿⣿⣿⠿⠿⣿⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣀⣾⣿⣿⣷⣦⣶⣦⣼⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣷⠈⠛⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀" +
	"\n" +
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠉⠻⣿⣤⡖⠛⠶⠤⡀⠀⠀⠀⠀⠀⠀⠀⢰⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠁⠙⣿⣿⠿⢻⣿⣿⡿⠋⢩⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀" +
	"\n" +
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠙⠧⣤⣦⣤⣄⡀⠀⠀⠀⠀⠀⠘⢿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⠀⠀⠀⠘⣧⠀⠈⣹⡻⠇⢀⣿⡆⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀" +
	"\n" +
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢠⣿⣿⣿⣿⣿⣤⣀⡀⠀⠀⠀⠀⠀⠀⠈⢽⣿⣿⣿⣿⣿⠋⠀⠀⠀⠀⠀⠀⠀⠀⠹⣷⣴⣿⣷⢲⣦⣤⡀⢀⡀⠀⠀⠀⠀⠀⠀" +
	"\n" +
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⢿⣿⣿⣿⣿⣿⣿⠟⠀⠀⠀⠀⠀⠀⠀⢸⣿⣿⣿⣿⣷⢀⡄⠀⠀⠀⠀⠀⠀⠀⠀⠈⠉⠂⠛⣆⣤⡜⣟⠋⠙⠂⠀⠀⠀⠀⠀" +
	"\n" +
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢹⣿⣿⣿⣿⠟⠀⠀⠀⠀⠀⠀⠀⠀⠘⣿⣿⣿⣿⠉⣿⠃⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣤⣾⣿⣿⣿⣿⣆⠀⠰⠄⠀⠉⠀⠀" +
	"\n" +
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣸⣿⣿⡿⠃⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢹⣿⡿⠃⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢻⣿⠿⠿⣿⣿⣿⠇⠀⠀⢀⠀⠀⠀" +
	"\n" +
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣿⡿⠛⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⢻⡇⠀⠀⢀⣼⠗⠀⠀" +
	"\n" +
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢸⣿⠃⣀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠙⠁⠀⠀⠀" +
	"\n" +
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠙⠒⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀"

func substringWindow(s string, offset, width int) string {
	runes := []rune(s)
	n := len(runes)
	if n == 0 {
		return strings.Repeat(" ", width)
	}
	offset = offset % n
	var result []rune
	for i := 0; i < width; i++ {
		result = append(result, runes[(offset+i)%n])
	}
	return string(result)
}

func (m Model) buildGlobeSidebar(w int) string {
	tealBold := lipgloss.NewStyle().Foreground(theme.ColorTeal).Bold(true)
	dim := lipgloss.NewStyle().Foreground(theme.ColorDimText)

	mapLines := strings.Split(baseWorldMap, "\n")
	windowW := w - 6
	offset := int((time.Now().UnixMilli() / 1000) % 150)

	var outLines []string
	for _, line := range mapLines {
		segment := substringWindow(line, offset, windowW)
		outLines = append(outLines, segment)
	}
	frame := strings.Join(outLines, "\n")

	var lines []string
	lines = append(lines, tealBold.Render("ORBITAL VECTOR"))
	lines = append(lines, "")
	lines = append(lines, frame)
	lines = append(lines, "")
	lines = append(lines, dim.Render("Tracking: ")+tealBold.Render("SYS_LOC_ACTIVE"))

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorSubtle).
		Width(w-2).
		Padding(0, 1).
		Render(content)
}

func (m Model) View() string {
	if m.sizes.width == 0 {
		return "Initializing..."
	}

	w := m.sizes.width
	h := m.sizes.height

	inputHeight := 1
	sidebarH := h - 1 - inputHeight
	videoHeight := max(1, sidebarH*55/100)
	chatHeight := max(0, sidebarH - videoHeight - 1)
	leftWidth := 52
	mainWidth := max(10, w-leftWidth)

	divider := dividerStyle.Render(strings.Repeat("─", w))
	inputContent := m.input.View()

	if m.settings.IsOpen() {
		settingsH := h - inputHeight - 1
		settingsPanel := m.settings.View()
		settingsPanel = lipgloss.NewStyle().Width(w).Height(max(1, settingsH)).Render(settingsPanel)
		fullInput := lipgloss.NewStyle().Width(w).Height(inputHeight).Padding(0, 0).Margin(0, 0).Render(inputContent)
		return lipgloss.NewStyle().Foreground(theme.ColorText).Render(fmt.Sprintf("%s\n%s", settingsPanel, fullInput))
	}

	m.video = m.video.SetSize(mainWidth, videoHeight)
	videoPanel := m.video.View()
	videoPanel = lipgloss.NewStyle().Width(mainWidth).Render(videoPanel)
	videoLines := strings.Count(videoPanel, "\n") + 1
	if videoLines > videoHeight {
		parts := strings.SplitN(videoPanel, "\n", videoHeight+1)
		videoPanel = strings.Join(parts[:videoHeight], "\n")
	} else if videoLines < videoHeight {
		videoPanel += strings.Repeat("\n", videoHeight-videoLines)
	}

	m.chat = m.chat.SetSize(mainWidth, chatHeight)
	chatPanel := m.chat.View()
	chatPanel = lipgloss.NewStyle().Width(mainWidth).MaxWidth(mainWidth).Render(chatPanel)
	chatLines := strings.Split(chatPanel, "\n")
	if len(chatLines) > chatHeight {
		chatPanel = strings.Join(chatLines[:chatHeight], "\n")
	} else if len(chatLines) < chatHeight {
		chatPanel += strings.Repeat("\n", chatHeight-len(chatLines))
	}

	center := lipgloss.JoinVertical(lipgloss.Left, videoPanel, divider, chatPanel)
	center = padToHeight(center, sidebarH)

	tealBold := lipgloss.NewStyle().Foreground(theme.ColorTeal).Bold(true)
	logo := []string{
		tealBold.Render("╔═══ c h a t ══════════════════════════════╗"),
		tealBold.Render("║ ███╗   ███╗ ██████╗ ██╗███████╗████████╗ ║"),
		tealBold.Render("║ ████╗ ████║██╔═══██╗██║██╔════╝╚══██╔══╝ ║"),
		tealBold.Render("║ ██╔████╔██║██║   ██║██║███████╗   ██║    ║"),
		tealBold.Render("║ ██║╚██╔╝██║██║   ██║██║╚════██║   ██║    ║"),
		tealBold.Render("║ ██║ ╚═╝ ██║╚██████╔╝██║███████║   ██║    ║"),
		tealBold.Render("║ ╚═╝     ╚═╝ ╚═════╝ ╚═╝╚══════╝   ╚═╝    ║"),
		tealBold.Render("╚══════════════════════════════════════════╝"),
	}
	var leftLines []string
	leftLines = append(leftLines, logo...)
	leftLines = append(leftLines, "")
	leftLines = append(leftLines, m.buildLeftSidebar(leftWidth))
	leftLines = append(leftLines, "")
	leftLines = append(leftLines, m.buildRightSidebar(leftWidth))
	leftLines = append(leftLines, "")
	leftLines = append(leftLines, m.buildGlobeSidebar(leftWidth))
	leftColumn := padToHeight(strings.Join(leftLines, "\n"), sidebarH)

	composite := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, center)
	bottomDivider := dividerStyle.Render(strings.Repeat("─", w))

	inputPanel := lipgloss.NewStyle().Width(mainWidth).Height(inputHeight).Padding(0, 0).Margin(0, 0).Render(inputContent)
	alignedInput := strings.Repeat(" ", leftWidth) + inputPanel

	return lipgloss.NewStyle().
		Foreground(theme.ColorText).
		Render(fmt.Sprintf("%s\n%s\n%s", composite, bottomDivider, alignedInput))
}
