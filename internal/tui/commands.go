package tui

import (
	"fmt"
	"strings"

	"moistchat/internal/config"
	"moistchat/internal/state"
)

func ParseCommand(input string, st *state.State) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}
	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "/create":
		if len(args) < 1 {
			st.AddMessage(state.Message{
				Username: "System",
				Content:  "Usage: /create <room-name>",
				System:   true,
			})
			return
		}
		st.SetRoom(args[0])
		st.SetIsHost(true)
		st.AddMessage(state.Message{
			Username: "System",
			Content:  fmt.Sprintf("Room \"%s\" created. You are the host.", args[0]),
			System:   true,
		})

	case "/user":
		if len(args) < 1 {
			st.AddMessage(state.Message{
				Username: "System",
				Content:  "Usage: /user <display-name>",
				System:   true,
			})
			return
		}
		st.Username = args[0]
		config.SaveUsername(args[0])
		st.AddMessage(state.Message{
			Username: "System",
			Content:  fmt.Sprintf("Username set to \"%s\".", args[0]),
			System:   true,
		})

	case "/help":
		st.AddMessage(state.Message{
			Username: "System",
			Content:  "Commands: /create <room>, /list, /join <room>, /user <name>, /leave, /call, /hang, /mute, /video, /contrast <0-100>, /auth <key>, /settings, /help, /quit",
			System:   true,
		})

	default:
		if strings.HasPrefix(cmd, "/") {
			st.AddMessage(state.Message{
				Username: "System",
				Content:  fmt.Sprintf("Unknown command: %s. Type /help for available commands.", cmd),
				System:   true,
			})
		}
	}
}
