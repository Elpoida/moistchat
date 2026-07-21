package network

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"tailscale.com/tsnet"
)

const lobbyHostname = "lobby"

type RoomEntry struct {
	Room          string
	Host          string
	Addr          string
	LastHeartbeat time.Time
}

type LobbyServer struct {
	rooms map[string]RoomEntry
	mu    sync.RWMutex
}

func StartLobbyServer(authKey string) error {
	s := &tsnet.Server{
		AuthKey:   authKey,
		Hostname:  lobbyHostname,
		Ephemeral: true,
	}

	ln, err := s.Listen("tcp", ":9091")
	if err != nil {
		return fmt.Errorf("lobby listen: %w", err)
	}
	log.Printf("[lobby] started on %s", ln.Addr())

	lobby := &LobbyServer{rooms: make(map[string]RoomEntry)}
	go lobby.cleanupExpired()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				continue
			}
			go lobby.handleConn(conn)
		}
	}()

	return nil
}

func (l *LobbyServer) handleConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var msg struct {
			Type string `json:"type"`
			Room string `json:"room"`
			From string `json:"from,omitempty"`
			Addr string `json:"addr,omitempty"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "register":
			l.mu.Lock()
			l.rooms[msg.Room] = RoomEntry{
				Room:          msg.Room,
				Host:          msg.From,
				Addr:          msg.Addr,
				LastHeartbeat: time.Now(),
			}
			resp := fmt.Sprintf(`{"type":"registered","room":"%s"}`, msg.Room)
			fmt.Fprintln(conn, resp)
			l.mu.Unlock()

		case "unregister":
			l.mu.Lock()
			delete(l.rooms, msg.Room)
			l.mu.Unlock()
			fmt.Fprintln(conn, `{"type":"unregistered"}`)

		case "list":
			l.mu.RLock()
			type roomJSON struct {
				Room string `json:"room"`
				Host string `json:"host"`
				Addr string `json:"addr"`
			}
			rooms := make([]roomJSON, 0, len(l.rooms))
			for _, r := range l.rooms {
				rooms = append(rooms, roomJSON{Room: r.Room, Host: r.Host, Addr: r.Addr})
			}
			l.mu.RUnlock()
			resp, _ := json.Marshal(map[string]any{
				"type": "room_list", "rooms": rooms,
			})
			fmt.Fprintln(conn, string(resp))

		case "ping":
			l.mu.Lock()
			if entry, ok := l.rooms[msg.Room]; ok {
				entry.LastHeartbeat = time.Now()
				l.rooms[msg.Room] = entry
			}
			l.mu.Unlock()
			fmt.Fprintln(conn, `{"type":"pong"}`)
		}
	}
}

func (l *LobbyServer) cleanupExpired() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		<-ticker.C
		l.mu.Lock()
		for name, entry := range l.rooms {
			if time.Since(entry.LastHeartbeat) > 20*time.Second {
				delete(l.rooms, name)
				log.Printf("[lobby] removed expired room %s (host: %s)", name, entry.Host)
			}
		}
		l.mu.Unlock()
	}
}
