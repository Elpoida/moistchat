package network

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

type LobbyClient struct {
	addr           string
	conn           net.Conn
	node           *Node
	ctx            context.Context
	cancel         context.CancelFunc
	mu             sync.Mutex
	room           string
	host           string
	registerFailed bool
}

func NewLobbyClient(node *Node) *LobbyClient {
	addr := os.Getenv("LOBBY_ADDR")
	if addr == "" {
		addr = "lobby:9091"
	}
	return &LobbyClient{addr: addr, node: node}
}

func (lc *LobbyClient) RegisterRoom(room, host, addr string) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if err := lc.connect(); err != nil {
		return err
	}

	msg := fmt.Sprintf(`{"type":"register","room":"%s","from":"%s","addr":"%s"}`,
		room, host, addr)
	if _, err := fmt.Fprintln(lc.conn, msg); err != nil {
		return err
	}

	scanner := bufio.NewScanner(lc.conn)
	if !scanner.Scan() {
		return fmt.Errorf("no response from lobby")
	}
	var resp struct {
		Type   string `json:"type"`
		Reason string `json:"reason,omitempty"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return fmt.Errorf("invalid lobby response: %w", err)
	}
	if resp.Type != "registered" {
		return fmt.Errorf("lobby: %s", resp.Reason)
	}

	lc.room = room
	lc.host = host
	lc.ctx, lc.cancel = context.WithCancel(context.Background())
	go lc.heartbeat()
	return nil
}

func (lc *LobbyClient) UnregisterRoom(room string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if lc.cancel != nil {
		lc.cancel()
	}
	if lc.conn == nil {
		return
	}
	fmt.Fprintf(lc.conn, `{"type":"unregister","room":"%s"}`+"\n", room)
	lc.conn.Close()
	lc.conn = nil
	lc.room = ""
}

func (lc *LobbyClient) ListRooms() ([]RoomInfo, error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if err := lc.connect(); err != nil {
		return nil, err
	}

	if _, err := fmt.Fprintln(lc.conn, `{"type":"list"}`); err != nil {
		return nil, fmt.Errorf("write list: %w", err)
	}

	scanner := bufio.NewScanner(lc.conn)
	if !scanner.Scan() {
		return nil, fmt.Errorf("no response from lobby")
	}
	var resp struct {
		Type  string `json:"type"`
		Rooms []struct {
			Room string `json:"room"`
			Host string `json:"host"`
			Addr string `json:"addr"`
		} `json:"rooms"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("invalid lobby response: %w", err)
	}

	rooms := make([]RoomInfo, 0, len(resp.Rooms))
	for _, r := range resp.Rooms {
		rooms = append(rooms, RoomInfo{
			Room: r.Room,
			Host: r.Host,
			Addr: r.Addr,
		})
	}
	return rooms, nil
}

func (lc *LobbyClient) connect() error {
	if lc.conn != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(lc.node.ctx, 5*time.Second)
	defer cancel()
	conn, err := lc.node.server.Dial(ctx, "tcp", lc.addr)
	if err != nil {
		return fmt.Errorf("dial lobby %s: %w", lc.addr, err)
	}
	lc.conn = conn
	return nil
}

func (lc *LobbyClient) heartbeat() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			lc.mu.Lock()
			if lc.conn != nil {
				fmt.Fprintf(lc.conn, `{"type":"ping","room":"%s"}`+"\n", lc.room)
			}
			lc.mu.Unlock()
		case <-lc.ctx.Done():
			log.Printf("[lobby] heartbeat stopped for room %s", lc.room)
			return
		}
	}
}
