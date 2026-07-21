package main

import (
	"log"
	"os"

	"moistchat/internal/network"
)

func main() {
	authKey := os.Getenv("TAILSCALE_AUTH_KEY")
	if authKey == "" {
		log.Fatal("TAILSCALE_AUTH_KEY not set")
	}
	if err := network.StartLobbyServer(authKey); err != nil {
		log.Fatal(err)
	}
	select {}
}
