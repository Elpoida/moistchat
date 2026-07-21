package state

import (
	"sync"
	"time"
)

type State struct {
	mu              sync.RWMutex
	RoomName        string
	Peers           []Peer
	Messages        []Message
	Username        string
	ConnectedPeer   string
	IsHost          bool
	DiscoveredRooms map[string]string
	PeerVideoFrames map[string]string
}

func New(username string) *State {
	return &State{
		Username: username,
		Peers:    make([]Peer, 0),
		Messages: make([]Message, 0),
	}
}

func (s *State) AddMessage(msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg.Timestamp = time.Now()
	s.Messages = append(s.Messages, msg)
}

func (s *State) GetMessages() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]Message, len(s.Messages))
	copy(cp, s.Messages)
	return cp
}

func (s *State) SetPeers(peers []Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Peers = peers
}

func (s *State) GetPeers() []Peer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]Peer, len(s.Peers))
	copy(cp, s.Peers)
	return cp
}

func (s *State) SetConnectedPeer(addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ConnectedPeer = addr
}

func (s *State) GetConnectedPeer() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ConnectedPeer
}

func (s *State) AddPeer(peer Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, p := range s.Peers {
		if p.ID == peer.ID {
			s.Peers = append(s.Peers[:i], s.Peers[i+1:]...)
			break
		}
	}
	s.Peers = append(s.Peers, peer)
}

func (s *State) RemovePeer(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, p := range s.Peers {
		if p.ID == id {
			s.Peers = append(s.Peers[:i], s.Peers[i+1:]...)
			return
		}
	}
}

func (s *State) SetIsHost(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.IsHost = v
}

func (s *State) GetIsHost() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.IsHost
}

func (s *State) SetDiscoveredRooms(rooms map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DiscoveredRooms = rooms
}

func (s *State) GetDiscoveredRooms() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make(map[string]string, len(s.DiscoveredRooms))
	for k, v := range s.DiscoveredRooms {
		cp[k] = v
	}
	return cp
}

func (s *State) SetRoom(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RoomName = name
}

func (s *State) GetRoom() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.RoomName
}

func (s *State) SetPeerVideoFrame(peerID, frame string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.PeerVideoFrames == nil {
		s.PeerVideoFrames = make(map[string]string)
	}
	s.PeerVideoFrames[peerID] = frame
}

func (s *State) GetPeerVideoFrame(peerID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.PeerVideoFrames == nil {
		return ""
	}
	return s.PeerVideoFrames[peerID]
}

func (s *State) RemovePeerVideoFrame(peerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.PeerVideoFrames, peerID)
}
