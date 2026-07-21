package state

import "time"

type Peer struct {
	ID       string
	Username string
	HasVideo bool
	Addr     string
}

type Message struct {
	Username  string
	Content   string
	System    bool
	Timestamp time.Time
}

type Room struct {
	Name      string
	Creator   string
	Peers     []Peer
	CreatedAt int64
}

type SignalingMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type JoinRequest struct {
	Room     string `json:"room"`
	Username string `json:"username"`
}

type LeaveRequest struct {
	Room     string `json:"room"`
	Username string `json:"username"`
}

type RoomListResponse struct {
	Rooms []Room `json:"rooms"`
}

type SDPMessage struct {
	Room     string `json:"room"`
	From     string `json:"from"`
	To       string `json:"to"`
	SDP      string `json:"sdp"`
	IsAnswer bool   `json:"is_answer"`
}

type ICECandidate struct {
	Room     string `json:"room"`
	From     string `json:"from"`
	To       string `json:"to"`
	Candidate string `json:"candidate"`
}
