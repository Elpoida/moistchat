package network

import (
	"context"
	"fmt"
	"strings"
)

type RoomInfo struct {
	Room string
	Host string
	Addr string
}

type StatusMsg struct {
	Connected    bool
	BackendState string
	Hostname     string
	IPs          []string
	Error        string
}

func (n *Node) Status(ctx context.Context) StatusMsg {
	if n.server == nil {
		return StatusMsg{Error: "node not started"}
	}

	client, err := n.server.LocalClient()
	if err != nil {
		return StatusMsg{Error: fmt.Sprintf("local client: %v", err)}
	}

	st, err := client.Status(ctx)
	if err != nil {
		return StatusMsg{Error: fmt.Sprintf("status: %v", err)}
	}

	sm := StatusMsg{
		Connected:    st.BackendState == "Running",
		BackendState: st.BackendState,
	}
	if st.Self != nil {
		sm.Hostname = st.Self.HostName
		for _, addr := range st.Self.Addrs {
			sm.IPs = append(sm.IPs, addr)
		}
	}
	return sm
}

func (sm StatusMsg) Summary() string {
	if sm.Connected {
		ips := strings.Join(sm.IPs, ", ")
		return fmt.Sprintf("● Connected  %s  %s", sm.Hostname, ips)
	}
	if sm.Error != "" {
		return fmt.Sprintf("○ Disconnected  %s", sm.Error)
	}
	return "○ Starting"
}
