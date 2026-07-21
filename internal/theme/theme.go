package theme

import "github.com/charmbracelet/lipgloss"

type Palette struct {
	Text       string
	DimText    string
	Muted      string
	Subtle     string
	Gray       string
	Divider    string
	Teal       string
	Green      string
	Orange     string
	Orange208  string
	Avatars   []string
}

var Themes = map[string]Palette{
	"Solarized": {
		Text: "#1E8CB0", DimText: "#88A8BF", Muted: "#657B83",
		Subtle: "#839496", Gray: "#EEE8D5", Divider: "#EEE8D5",
		Teal: "#2AA198", Green: "#859900", Orange: "#CB4B16", Orange208: "166",
		Avatars: []string{"#268BD2", "#2AA198", "#859900", "#B58900",
			"#CB4B16", "#DC322F", "#6C71C4", "#D33682"},
	},
	"eDEX": {
		Text: "#EAEAEA", DimText: "#A0A8B8", Muted: "#6E7681",
		Subtle: "#8B949E", Gray: "#6E7681", Divider: "#317682",
		Teal: "#58A6FF", Green: "#7EE787", Orange: "#FF7B72", Orange208: "208",
		Avatars: []string{"#58A6FF", "#39D2C0", "#7EE787", "#D29922",
			"#FF7B72", "#DB61A2", "#A371F7", "#F78166"},
	},
	"Amber": {
		Text: "#FFB04D", DimText: "#CC8533", Muted: "#A67C53",
		Subtle: "#C4A484", Gray: "#EEDC5B", Divider: "#F2E3C6",
		Teal: "#E67300", Green: "#2E6930", Orange: "#C82333", Orange208: "208",
		Avatars: []string{"#FFB04D", "#CC8533", "#E6993D", "#B3732A",
			"#FFD369", "#D94F04", "#8B4513", "#FFC87C"},
	},
	"Neon": {
		Text: "#EAD6FF", DimText: "#B38FFF", Muted: "#7A629B",
		Subtle: "#9E8BB8", Gray: "#E6DCFF", Divider: "#E6DCFF",
		Teal: "#8A2BE2", Green: "#008080", Orange: "#D9381E", Orange208: "166",
		Avatars: []string{"#EAD6FF", "#B38FFF", "#D300FF", "#8A2BE2",
			"#00FFCC", "#FF5E00", "#6C3AA0", "#FF6EC7"},
	},
}

var (
	Active       string
	ColorText    lipgloss.Color
	ColorDimText lipgloss.Color
	ColorMuted   lipgloss.Color
	ColorSubtle  lipgloss.Color
	ColorGray    lipgloss.Color
	ColorDivider lipgloss.Color
	ColorTeal    lipgloss.Color
	ColorGreen   lipgloss.Color
	ColorOrange  lipgloss.Color
	ColorOrange208 lipgloss.Color
	AvatarColors []lipgloss.Color
)

var onThemeChange []func()

func OnThemeChange(fn func()) {
	onThemeChange = append(onThemeChange, fn)
}

func SetTheme(name string) {
	p, ok := Themes[name]
	if !ok {
		name = "eDEX"
		p = Themes["eDEX"]
	}
	Active = name
	ColorText = lipgloss.Color(p.Text)
	ColorDimText = lipgloss.Color(p.DimText)
	ColorMuted = lipgloss.Color(p.Muted)
	ColorSubtle = lipgloss.Color(p.Subtle)
	ColorGray = lipgloss.Color(p.Gray)
	ColorDivider = lipgloss.Color(p.Divider)
	ColorTeal = lipgloss.Color(p.Teal)
	ColorGreen = lipgloss.Color(p.Green)
	ColorOrange = lipgloss.Color(p.Orange)
	ColorOrange208 = lipgloss.Color(p.Orange208)
	AvatarColors = nil
	for _, a := range p.Avatars {
		AvatarColors = append(AvatarColors, lipgloss.Color(a))
	}
	for _, cb := range onThemeChange {
		cb()
	}
}
