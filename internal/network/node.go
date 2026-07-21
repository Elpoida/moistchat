package network

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"tailscale.com/tsnet"
	"moistchat/internal/media"
)

const hostname = "moistchat"

type Node struct {
	server    *tsnet.Server
	ln        net.Listener
	ctx       context.Context
	cancel    context.CancelFunc
	Events    chan NetworkEvent
	localAddr string
	logFile   *os.File
	stateDir  string
	conns     map[string]net.Conn
	connsMu   sync.Mutex
	hostConn  net.Conn
	mu        sync.Mutex
	roomName    string
	username    string
	leaving       bool
	lobbyClient   *LobbyClient
	audioPipeline *media.Pipeline
}

type NetworkEvent struct {
	Type    string
	From    string
	Content string
	Addr    string
}

func NewNode() (*Node, error) {
	if AuthKey == "" {
		return nil, fmt.Errorf("no auth key; set TAILSCALE_AUTH_KEY env var")
	}

	uniqueDir, err := os.MkdirTemp("", "tsnet-moistchat-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	logPath := filepath.Join(uniqueDir, "moistchat.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file %s: %w", logPath, err)
	}
	log.SetOutput(logFile)
	logger := log.New(logFile, "", log.LstdFlags)
	logger.Printf("moistchat node initializing")

	events := make(chan NetworkEvent, 128)

	os.Setenv("TSNET_FORCE_LOGIN", "1")

	s := &tsnet.Server{
		AuthKey:   AuthKey,
		Hostname:  hostname,
		Ephemeral: true,
		Dir:       uniqueDir,
		Logf: func(format string, args ...interface{}) {
			logger.Printf("[backend] "+format, args...)
		},
		UserLogf: func(format string, args ...interface{}) {
			logger.Printf("[tsnet] "+format, args...)
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	n := &Node{
		server:   s,
		ctx:      ctx,
		cancel:   cancel,
		Events:   events,
		logFile:  logFile,
		stateDir: uniqueDir,
		conns:    make(map[string]net.Conn),
	}
	n.lobbyClient = NewLobbyClient(n)
	return n, nil
}

func (n *Node) Start() error {
	ln, err := n.server.Listen("tcp", "0.0.0.0:9090")
	if err != nil {
		return fmt.Errorf("tsnet listen: %w", err)
	}
	n.ln = ln
	n.localAddr = ln.Addr().String()
	go n.acceptLoop()
	go n.startHeartbeat()
	return nil
}

func (n *Node) acceptLoop() {
	for {
		conn, err := n.ln.Accept()
		if err != nil {
			select {
			case <-n.ctx.Done():
				return
			default:
			}
			continue
		}
		log.Printf("[host] connection accepted from %s", conn.RemoteAddr().String())
		go n.handleConn(conn)
	}
}

func (n *Node) Close() error {
	n.cancel()
	if n.ln != nil {
		n.ln.Close()
	}
	err := n.server.Close()
	if n.logFile != nil {
		n.logFile.Close()
	}
	if n.stateDir != "" {
		os.RemoveAll(n.stateDir)
	}
	return err
}

func (n *Node) LocalAddr() string {
	return n.localAddr
}

func (n *Node) IsRunning() bool {
	return n.ln != nil
}

func (n *Node) SetRoom(name string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.roomName = name
}

func (n *Node) SetUsername(name string) (string, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.username == name {
		return "", false
	}
	old := n.username
	n.username = name
	return old, true
}

func (n *Node) RenameSelf(oldName, newName string) {
	if oldName == "" || newName == "" || oldName == newName {
		return
	}

	msg := WireMessage{Type: "peer_renamed", From: oldName, Content: newName}
	data, _ := json.Marshal(msg)
	line := string(data) + "\n"

	n.connsMu.Lock()
	for _, conn := range n.conns {
		fmt.Fprint(conn, line)
	}
	if n.hostConn != nil {
		fmt.Fprint(n.hostConn, line)
	}
	n.connsMu.Unlock()
}

func (n *Node) LeaveRoom() {
	n.mu.Lock()
	username := n.username
	n.mu.Unlock()

	log.Printf("[hostconn] LeaveRoom called for %s (conns=%d hostConn=%v)", username, len(n.conns), n.hostConn != nil)

	msg := WireMessage{Type: "leave", From: username}
	data, _ := json.Marshal(msg)
	line := string(data) + "\n"

	n.connsMu.Lock()
	n.leaving = true

	for id, conn := range n.conns {
		log.Printf("[hostconn] LeaveRoom sending leave to peer %s", id)
		fmt.Fprint(conn, line)
		conn.Close()
		delete(n.conns, id)
	}

	if n.hostConn != nil {
		log.Printf("[hostconn] LeaveRoom sending leave via hostConn")
		fmt.Fprint(n.hostConn, line)
		n.hostConn.Close()
		n.hostConn = nil
	}
	n.connsMu.Unlock()
}

func (n *Node) IsLobbyRegistered() bool {
	return n.lobbyClient != nil && n.lobbyClient.room != ""
}

func (n *Node) ClearLobbyRegisterFailed() {
	if n.lobbyClient != nil {
		n.lobbyClient.registerFailed = false
	}
}

func (n *Node) TryRegisterToLobby(room, host string) {
	if n.lobbyClient == nil || n.lobbyClient.registerFailed || n.IsLobbyRegistered() {
		return
	}
	if err := n.RegisterToLobby(room, host); err != nil {
		log.Printf("[lobby] register failed: %v", err)
		n.lobbyClient.registerFailed = true
	}
}

func (n *Node) RegisterToLobby(room, host string) error {
	if n.lobbyClient == nil {
		return nil
	}
	client, err := n.server.LocalClient()
	if err != nil {
		return fmt.Errorf("local client: %w", err)
	}
	st, err := client.Status(n.ctx)
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}
	if st.Self == nil {
		return fmt.Errorf("no self status")
	}
	var addr string
	for _, ip := range st.Self.TailscaleIPs {
		addr = net.JoinHostPort(ip.String(), "9090")
		break
	}
	return n.lobbyClient.RegisterRoom(room, host, addr)
}

func (n *Node) LobbyListRooms() ([]RoomInfo, error) {
	if n.lobbyClient == nil {
		return nil, fmt.Errorf("no lobby client")
	}
	return n.lobbyClient.ListRooms()
}

func (n *Node) UnregisterFromLobby(room string) {
	if n.lobbyClient != nil {
		n.lobbyClient.registerFailed = false
		n.lobbyClient.UnregisterRoom(room)
	}
}

func (n *Node) StartAudio(micID, spkID string) error {
	if n.audioPipeline == nil {
		n.audioPipeline = media.NewPipeline()
	}
	return n.audioPipeline.Start(micID, spkID)
}

func (n *Node) StopAudio() {
	if n.audioPipeline != nil {
		n.audioPipeline.Stop()
	}
}

func (n *Node) AudioActive() bool {
	return n.audioPipeline != nil && n.audioPipeline.IsRunning()
}

func (n *Node) ToggleMute() bool {
	if n.audioPipeline != nil && n.audioPipeline.IsRunning() {
		return n.audioPipeline.Mute()
	}
	return false
}

func (n *Node) AudioPipeline() *media.Pipeline {
	return n.audioPipeline
}

func (n *Node) startHeartbeat() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			n.connsMu.Lock()
			for id, conn := range n.conns {
				if _, err := fmt.Fprintln(conn, `{"type":"ping"}`); err != nil {
					conn.Close()
					delete(n.conns, id)
					n.Events <- NetworkEvent{
						Type: "peer_disconnected",
						From: id,
					}
				}
			}
			n.connsMu.Unlock()
		case <-n.ctx.Done():
			return
		}
	}
}

func (n *Node) DiscoverRooms(ctx context.Context) []RoomInfo {
	// Try lobby first, fall back to probes if lobby is empty
	if n.lobbyClient != nil {
		rooms, err := n.lobbyClient.ListRooms()
		if err == nil && len(rooms) > 0 {
			log.Printf("[discovery] lobby returned %d rooms", len(rooms))
			return rooms
		}
		if err != nil {
			log.Printf("[discovery] lobby unavailable (%v), falling back to probes", err)
		} else {
			log.Printf("[discovery] lobby returned 0 rooms, falling back to probes")
		}
	}

	if n.server == nil {
		log.Printf("[discovery] server is nil, skipping scan")
		return nil
	}

	log.Printf("[discovery] scanning tailnet for rooms...")

	client, err := n.server.LocalClient()
	if err != nil {
		log.Printf("[discovery] LocalClient failed: %v", err)
		return nil
	}

	st, err := client.Status(ctx)
	if err != nil {
		log.Printf("[discovery] Status failed: %v", err)
		return nil
	}

	var rooms []RoomInfo

	if st.Self == nil {
		log.Printf("[discovery] no self status available")
		return nil
	}

	log.Printf("[discovery] self hostname: %s, IPs: %v", st.Self.HostName, st.Self.TailscaleIPs)

	// Build set of self IPs for skip-self check
	selfIPs := make(map[string]bool)
	for _, addr := range st.Self.TailscaleIPs {
		selfIPs[addr.String()] = true
	}

	peerCount := 0
	for _, peer := range st.Peer {
		peerCount++

		// Skip self
		skip := false
		for _, ip := range peer.TailscaleIPs {
			if selfIPs[ip.String()] {
				skip = true
				break
			}
		}
		if skip {
			log.Printf("[discovery] skipping self (peer: %s, IPs: %v)", peer.HostName, peer.TailscaleIPs)
			continue
		}

		if !peer.Online {
			log.Printf("[discovery] skipping offline peer: %s", peer.HostName)
			continue
		}

		log.Printf("[discovery] peer found: %s, IPs: %v, DNSName: %s", peer.HostName, peer.TailscaleIPs, peer.DNSName)

		// Try each IP for this peer
		for _, ip := range peer.TailscaleIPs {
			probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			addr := net.JoinHostPort(ip.String(), "9090")
			log.Printf("[discovery] probing %s at %s", peer.HostName, addr)
			conn, err := n.server.Dial(probeCtx, "tcp", addr)
			cancel()
			if err != nil {
				log.Printf("[discovery] dial %s failed: %v", addr, err)
				continue
			}
			log.Printf("[discovery] dial successful to %s, sending probe", addr)

			fmt.Fprintln(conn, `{"type":"probe"}`)
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))

			scanner := bufio.NewScanner(conn)
			if scanner.Scan() {
				log.Printf("[discovery] raw response from %s: %s", addr, scanner.Text())
				var wm WireMessage
				if err := json.Unmarshal(scanner.Bytes(), &wm); err == nil && wm.Type == "probe_response" && wm.Room != "" {
					hostName := wm.From
					if hostName == "" {
						hostName = peer.HostName
					}
					log.Printf("[discovery] room found: room=%s host=%s addr=%s", wm.Room, hostName, addr)
					rooms = append(rooms, RoomInfo{
						Room: wm.Room,
						Host: hostName,
						Addr: addr,
					})
				} else {
					if err != nil {
						log.Printf("[discovery] failed to parse probe_response from %s: %v", addr, err)
					} else {
						log.Printf("[discovery] unexpected response from %s: type=%s room=%s", addr, wm.Type, wm.Room)
					}
				}
			} else {
				log.Printf("[discovery] no response from %s (read timeout or closed)", addr)
			}
			conn.Close()
			break // found this peer's port, don't try other IPs
		}
	}
	log.Printf("[discovery] scan complete: %d peers scanned, %d rooms found", peerCount, len(rooms))
	return rooms
}
