package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"moistchat/internal/config"
	"moistchat/internal/network"
	"moistchat/internal/theme"
	"moistchat/internal/tui"
)

func main() {
	cfg, _ := config.Load()
	firstRun := !config.Exists()

	themeName := cfg.ThemeName
	if themeName == "" {
		themeName = "eDEX"
	}
	if firstRun {
		config.SaveThemeName(themeName)
	}
	theme.SetTheme(themeName)

	if network.AuthKey == "" && cfg.AuthKey != "" {
		network.AuthKey = cfg.AuthKey
	}
	if network.AuthKey == "" {
		network.AuthKey = os.Getenv("TAILSCALE_AUTH_KEY")
	}

	if network.AuthKey != "" {
		if err := network.StartLobbyServer(network.AuthKey); err != nil {
			fmt.Fprintf(log.Writer(), "Lobby: %v (using existing)\n", err)
		} else {
			fmt.Fprintln(log.Writer(), "Lobby: auto-started locally")
		}
	}

	username := cfg.Username
	if username == "" {
		username = "You"
	}

	m := tui.NewModel(username, firstRun)
	defer m.Close()

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
