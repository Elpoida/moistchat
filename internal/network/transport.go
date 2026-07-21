package network

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"
)

type WireMessage struct {
	Type    string `json:"type"`
	Room    string `json:"room,omitempty"`
	From    string `json:"from,omitempty"`
	Content string `json:"content,omitempty"`
	Addr    string `json:"addr,omitempty"`
}

func (n *Node) handleConn(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return
	}

	var wm WireMessage
	if err := json.Unmarshal(scanner.Bytes(), &wm); err != nil {
		return
	}

	switch wm.Type {
	case "probe":
		n.mu.Lock()
		room := n.roomName
		from := n.username
		n.mu.Unlock()
		log.Printf("[host] probe received, room=%q from=%q", room, from)
		resp := WireMessage{
			Type: "probe_response",
			Room: room,
			From: from,
		}
		data, _ := json.Marshal(resp)
		log.Printf("[host] sending probe_response: %s", string(data))
		fmt.Fprintln(conn, string(data))
		return

	case "join":
		peerID := wm.From
		if peerID == "" {
			peerID = conn.RemoteAddr().String()
		}

		n.connsMu.Lock()
		if old, ok := n.conns[peerID]; ok {
			old.Close()
		}
		n.conns[peerID] = conn
		n.connsMu.Unlock()

		n.mu.Lock()
		hostUser := n.username
		n.mu.Unlock()
		if hostUser == "" {
			hostUser = "host"
		}
		info := WireMessage{Type: "peer_info", From: hostUser, Addr: n.localAddr}
		data, _ := json.Marshal(info)
		fmt.Fprintln(conn, string(data))

		n.connsMu.Lock()
		for id, existingConn := range n.conns {
			if id == peerID {
				continue
			}
			peerAddr := existingConn.RemoteAddr().String()
			if host, _, err := net.SplitHostPort(peerAddr); err == nil {
				peerAddr = net.JoinHostPort(host, "9090")
			}
			info := WireMessage{Type: "peer_info", From: id, Addr: peerAddr}
			data, _ := json.Marshal(info)
			fmt.Fprintln(conn, string(data))
		}
		for id, existingConn := range n.conns {
			if id == peerID {
				continue
			}
			newPeerAddr := conn.RemoteAddr().String()
			if host, _, err := net.SplitHostPort(newPeerAddr); err == nil {
				newPeerAddr = net.JoinHostPort(host, "9090")
			}
			info := WireMessage{Type: "peer_info", From: peerID, Addr: newPeerAddr}
			data, _ := json.Marshal(info)
			fmt.Fprintln(existingConn, string(data))
		}
		n.connsMu.Unlock()

		n.Events <- NetworkEvent{
			Type: "peer_connected",
			From: peerID,
			Addr: conn.RemoteAddr().String(),
		}

		for scanner.Scan() {
			var msg WireMessage
			if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
				continue
			}

		switch msg.Type {
		case "chat":
			n.Events <- NetworkEvent{
				Type:    "chat_message",
				From:    msg.From,
				Content: msg.Content,
			}
			n.Broadcast(msg, msg.From)
		case "audio":
			n.Events <- NetworkEvent{
				Type:    "audio_frame",
				From:    msg.From,
				Content: msg.Content,
			}
			n.Broadcast(msg, msg.From)
		case "video_frame":
			n.Events <- NetworkEvent{
				Type:    "video_frame",
				From:    msg.From,
				Content: msg.Content,
			}
			n.Broadcast(msg, msg.From)
		case "hang":
			n.Events <- NetworkEvent{
				Type: "info",
				Content: fmt.Sprintf("%s ended the voice call.", msg.From),
			}
			n.Broadcast(msg, msg.From)
		case "call":
			n.Events <- NetworkEvent{
				Type: "info",
				Content: fmt.Sprintf("%s joined the voice call.", msg.From),
			}
			n.Broadcast(msg, msg.From)
		case "peer_renamed":
			oldID := msg.From
			newID := msg.Content

			n.connsMu.Lock()
			if conn, ok := n.conns[oldID]; ok {
				delete(n.conns, oldID)
				n.conns[newID] = conn
			}
			n.connsMu.Unlock()

			peerID = newID

			n.Events <- NetworkEvent{
				Type: "peer_renamed", From: oldID, Content: newID,
			}
			n.Broadcast(msg, newID)
		case "leave":
			n.connsMu.Lock()
			delete(n.conns, peerID)
			n.connsMu.Unlock()

			n.Events <- NetworkEvent{
				Type: "peer_disconnected",
				From: peerID,
			}
			n.Broadcast(msg, peerID)
			return
		case "pong":
		}
		}

		n.connsMu.Lock()
		delete(n.conns, peerID)
		n.connsMu.Unlock()

		n.Events <- NetworkEvent{
			Type: "peer_disconnected",
			From: peerID,
		}
	}
}

func (n *Node) Broadcast(msg WireMessage, senderID string) {
	n.connsMu.Lock()
	defer n.connsMu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	line := string(data) + "\n"

	for id, conn := range n.conns {
		if id == senderID {
			continue
		}
		if _, err := fmt.Fprint(conn, line); err != nil {
			conn.Close()
			delete(n.conns, id)
			n.Events <- NetworkEvent{
				Type: "peer_disconnected",
				From: id,
			}
		}
	}
}

func (n *Node) ConnectToHost(addr, room, username string) error {
	conn, err := n.server.Dial(n.ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dial host %s: %w", addr, err)
	}

	join := WireMessage{
		Type: "join",
		Room: room,
		From: username,
	}
	data, _ := json.Marshal(join)
	if _, err := fmt.Fprintln(conn, string(data)); err != nil {
		conn.Close()
		return fmt.Errorf("send join: %w", err)
	}

	n.connsMu.Lock()
	n.leaving = false
	if n.hostConn != nil {
		log.Printf("[hostconn] ConnectToHost closing old hostConn")
		n.hostConn.Close()
	}
	n.hostConn = conn
	n.connsMu.Unlock()

	log.Printf("[hostconn] ConnectToHost established new connection to %s", addr)

	go n.readHostMessages(conn)
	return nil
}

func (n *Node) readHostMessages(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	conn.SetReadDeadline(time.Now().Add(120 * time.Second))
	for scanner.Scan() {
		conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		var wm WireMessage
		if err := json.Unmarshal(scanner.Bytes(), &wm); err != nil {
			continue
		}

		switch wm.Type {
		case "chat":
			n.Events <- NetworkEvent{
				Type:    "chat_message",
				From:    wm.From,
				Content: wm.Content,
			}
		case "audio":
			n.Events <- NetworkEvent{
				Type:    "audio_frame",
				From:    wm.From,
				Content: wm.Content,
			}
		case "video_frame":
			n.Events <- NetworkEvent{
				Type:    "video_frame",
				From:    wm.From,
				Content: wm.Content,
			}
		case "peer_info":
			n.Events <- NetworkEvent{
				Type: "peer_connected",
				From: wm.From,
				Addr: wm.Addr,
			}
		case "peer_renamed":
			n.Events <- NetworkEvent{
				Type: "peer_renamed",
				From: wm.From,
				Content: wm.Content,
			}
		case "hang":
			n.Events <- NetworkEvent{
				Type:    "info",
				Content: fmt.Sprintf("%s ended the voice call.", wm.From),
			}
		case "call":
			n.Events <- NetworkEvent{
				Type:    "info",
				Content: fmt.Sprintf("%s joined the voice call.", wm.From),
			}
		case "ping":
			fmt.Fprintln(conn, `{"type":"pong"}`)
		case "leave":
			log.Printf("[hostconn] received leave message from %s", wm.From)
			n.Events <- NetworkEvent{
				Type: "peer_disconnected",
				From: wm.From,
			}
		}
	}

	log.Printf("[hostconn] readHostMessages scanner loop exited (scanner error: %v)", scanner.Err())

	n.connsMu.Lock()
	wasActive := n.hostConn == conn
	if wasActive {
		n.hostConn = nil
	}
	leaving := n.leaving
	n.connsMu.Unlock()

	log.Printf("[hostconn] readHostMessages cleanup: wasActive=%v leaving=%v hostConn_nil=%v", wasActive, leaving, n.hostConn == nil)

	if !leaving && wasActive {
		log.Printf("[hostconn] sending Disconnected from host event")
		n.Events <- NetworkEvent{
			Type:    "error",
			Content: "Disconnected from host",
		}
	}
}

func (n *Node) SendAudio(opusData []byte) error {
	encoded := base64.StdEncoding.EncodeToString(opusData)

	n.mu.Lock()
	from := n.username
	n.mu.Unlock()

	msg := WireMessage{
		Type:    "audio",
		Room:    n.roomName,
		From:    from,
		Content: encoded,
	}
	data, _ := json.Marshal(msg)
	line := string(data) + "\n"

	n.connsMu.Lock()
	conn := n.hostConn
	n.connsMu.Unlock()

	if conn != nil {
		// Client mode: send to host
		_, err := fmt.Fprint(conn, line)
		return err
	}

	// Host mode: broadcast to all peers (exclude self via Broadcast senderID)
	n.Broadcast(msg, from)
	return nil
}

func (n *Node) SendVideoFrame(frameData []byte) error {
	encoded := base64.StdEncoding.EncodeToString(frameData)

	n.mu.Lock()
	from := n.username
	n.mu.Unlock()

	msg := WireMessage{
		Type:    "video_frame",
		Room:    n.roomName,
		From:    from,
		Content: encoded,
	}
	data, _ := json.Marshal(msg)
	line := string(data) + "\n"

	n.connsMu.Lock()
	conn := n.hostConn
	n.connsMu.Unlock()

	if conn != nil {
		_, err := fmt.Fprint(conn, line)
		return err
	}

	n.Broadcast(msg, from)
	return nil
}

func (n *Node) SendHangMessage(username string) error {
	n.mu.Lock()
	msg := WireMessage{Type: "hang", Room: n.roomName, From: username}
	n.mu.Unlock()
	data, _ := json.Marshal(msg)
	line := string(data) + "\n"

	n.connsMu.Lock()
	conn := n.hostConn
	n.connsMu.Unlock()

	if conn != nil {
		_, err := fmt.Fprint(conn, line)
		return err
	}

	n.Broadcast(msg, username)
	return nil
}

func (n *Node) SendCallMessage(username string) error {
	n.mu.Lock()
	msg := WireMessage{Type: "call", Room: n.roomName, From: username}
	n.mu.Unlock()
	data, _ := json.Marshal(msg)
	line := string(data) + "\n"

	n.connsMu.Lock()
	conn := n.hostConn
	n.connsMu.Unlock()

	if conn != nil {
		_, err := fmt.Fprint(conn, line)
		return err
	}

	n.Broadcast(msg, username)
	return nil
}

func (n *Node) SendChat(content string) error {
	n.connsMu.Lock()
	conn := n.hostConn
	n.connsMu.Unlock()

	if conn == nil {
		return fmt.Errorf("not connected to host")
	}

	n.mu.Lock()
	from := n.username
	n.mu.Unlock()

	msg := WireMessage{
		Type:    "chat",
		Room:    n.roomName,
		From:    from,
		Content: content,
	}
	data, _ := json.Marshal(msg)
	if _, err := fmt.Fprintln(conn, string(data)); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

func (n *Node) SendMessage(addr string, msg WireMessage) error {
	conn, err := n.server.Dial(n.ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	defer conn.Close()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if _, err := fmt.Fprintln(conn, string(data)); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}
