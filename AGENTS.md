# moistchat — P2P Terminal Chat with ASCII Video

## Project Overview

Cross-platform P2P terminal chat application with audio/video calling capabilities.
Built with Bubble Tea (TUI), tsnet (Tailscale in-process node), and Pion WebRTC.

**Go version:** 1.26.4  
**Module:** `moistchat`

## Build & Run

```bash
make dev              # Run without auth key (UI-only mode)
make run              # Run with TAILSCALE_AUTH_KEY env var set
make build            # Build binary with auth key
make lobby            # Build lobby broker binary
make lobby-run        # Run standalone lobby broker
make clean-state      # Remove tsnet state dir (~/.config/tsnet-moistchat/)
make cross-compile    # Cross-compile for Linux/Darwin/Windows
```

The `TAILSCALE_AUTH_KEY` is injected at build time via `-ldflags`:  
`-X moistchat/internal/network.AuthKey=$(TAILSCALE_AUTH_KEY)`

## Directory Structure

```
moistchat/
├── cmd/moistchat/main.go           # Entry point: auto-provision lobby + tea.NewProgram + Close()
├── cmd/lobby/main.go             # Standalone lobby broker (thin wrapper)
├── internal/
│   ├── tui/                      # Bubble Tea UI
│   │   ├── model.go              # Root Model, Init, formatMessages, event handlers
│   │   ├── update.go             # Update loop — message routing
│   │   ├── view.go               # Layout — vertical split (video/chat/input/status)
│   │   ├── commands.go           # /create, /user, /settings, /help
│   │   └── components/
│   │       ├── video.go          # Tiled avatar grid (placeholder for ASCII video)
│   │       ├── chat.go           # Scrolling message stream + SetLines() + SetSize()
│   │       └── input.go          # Text input + Tab cycling completion + SetPrompt()
│   ├── state/                    # Mutable app state (mutex-protected)
│   │   ├── types.go             # Peer, Message, Room, signaling structs
│   │   └── state.go             # State store with Get/Set methods
│   ├── network/                  # Tailscale networking layer
│   │   ├── auth.go               # AuthKey var (injected via -ldflags)
│   │   ├── node.go               # tsnet.Server wrapper + lobby client + relay
│   │   ├── transport.go          # TCP connection handler + WireMessage protocol + Broadcast relay
│   │   ├── status.go             # StatusMsg type + Node.Status() using LocalClient + RoomInfo
│   │   ├── lobby.go              # LobbyClient: register, unregister, list rooms, heartbeat
│   │   └── lobby_server.go       # LobbyServer: extracted room registry + cleanup
│   └── media/                    # Future: WebRTC + ASCII video rendering
├── Makefile
├── go.mod
└── AGENTS.md
```

## Architecture

### Layout (vertical split, top to bottom)
```
┌──────────────────────────┐
│   Video / Avatars (55%)  │
├──────────────────────────┤
│   Chat stream            │
├──────────────────────────┤
│   > /command input...    │
│   ● Connected  moistchat   │  ← status bar at bottom
└──────────────────────────┘
```

### Bubble Tea Event Flow
```
User Input → tea.Msg → Update() → Model (state) → View() → Terminal
                            ↓
                     tea.Cmd (async goroutine)
                            ↓
                     tea.Msg → Update()
```

### Async Event Bridge (Network → TUI)
The tsnet `Node.Events` channel feeds into the Update loop via a recursive tea.Cmd:

```
Init() → waitForNetworkCmd() → blocks on m.node.Events channel
  → event arrives → Update handles it → re-queues waitForNetworkCmd()
```

### Status Polling
A `tea.Tick(5*time.Second)` fires from Init(), calls `Node.Status()` via LocalClient,
and updates `Model.statusDisplay` with the result. The ticker is re-queued
every time a `StatusMsg` is processed.

### Host Relay Architecture
The host node acts as a central relay for all connected peers:

```
  Alice ──join──► Bob (host)
  Tom   ──join──► Bob (host)
                 conns: "alice" → net.Conn, "tom" → net.Conn
  Alice types → SendChat → Bob receives → Broadcast(msg, "alice") → Tom receives
  Tom   types → SendChat → Bob receives → Broadcast(msg, "tom")   → Alice receives
```

- The host maintains a `conns map[string]net.Conn` of all connected peers
- Each chat message is broadcast to ALL peers except the sender
- When a new peer joins, the host sends `peer_info` about all existing peers to the new peer,
  AND sends `peer_info` about the new peer to all existing peers
- Only `IsHost` nodes advertise room names in probe responses — clients never respond

### Lobby Architecture (Phase 4)
The lobby broker is a centralized room registry on the tailnet. It provides deterministic room discovery:

```
┌─────────────────────────────┐
│  Lobby (tsnet, hostname     │
│  "lobby", port :9091)       │
│                             │
│  rooms map:                 │
│    "test" → {host, addr}    │  ← one room per name (rejects duplicates)
│    "chat" → {host, addr}    │
│                             │
│  Heartbeat: 10s ping        │
│  Timeout: 20s → remove room │
└──────────┬──────────────────┘
           │ TCP :9091
     ┌─────┴────┬──────────┐
     │          │          │
   Host       Alice      Tom
```

**Auto-provisioning** (cmd/moistchat/main.go):
1. On startup, call `network.StartLobbyServer(AuthKey)` unconditionally
2. If port :9091 is free → lobby starts in background goroutine
3. If port :9091 is in use → another instance already hosts the lobby → skip
4. The `LobbyClient` connects via tsnet Dill to the lobby address
5. `LOBBY_ADDR` env var defaults to `"lobby:9091"` (tsnet MagicDNS)

### Message Relay Flow
```
Alice types → ChatSendMsg → update.go catches it
  local copy: state.AddMessage("Alice: hello")
  Alice.IsHost()=false → sendToHostCmd → node.SendChat("hello")
    → writes {"type":"chat","room":"test","from":"alice","content":"hello"} to hostConn
Bob's handleConn receives:
  → emits chat_message(From:"alice") to Events (Bob sees "alice: hello")
  → Broadcast(msg, "alice") → writes to Tom's conn (Tom sees "alice: hello")
```

### Peer Discovery (Lobby-first, Probe fallback)
```
/list → listRoomsCmd → node.DiscoverRooms()
  → If lobby available:
       lobbyClient.ListRooms() → returns []RoomInfo (registered rooms)
  → If lobby unavailable:
       Fall back to probe discovery:
       LocalClient.Status() → iterate st.Peer (skip self)
       Dial each peer on :9090 (5s timeout)
       Send {"type":"probe"}
       Host responds {"type":"probe_response","room":"test","from":"bob"}
       Deduplicate by room name (keep first found)
```

### Join Flow
```
/tom types /join test
  → update.go looks up DiscoveredRooms["test"] → addr "100.x.x.x:9090"
  → state.SetRoom("test"), SetIsHost(false), SetConnectedPeer(addr)
  → connectToHostCmd fires as tea.Cmd
  → node.ConnectToHost(addr, "test", "tom")
    → Dial addr:9090 via tsnet
    → Send {"type":"join","room":"test","from":"tom"}
    → Read responses:
      → {"type":"peer_info","from":"bob"}     → video shows bob
      → {"type":"peer_info","from":"alice"}   → video shows alice
      → Enters persistent readHostMessages loop

Bob's handleConn receives "join":
  → Register tom in conns map
  → Send {"type":"peer_info","from":"bob"} to tom
  → Broadcast peer_info to all existing peers: {"type":"peer_info","from":"tom"} to alice
  → Enter persistent read loop (for tom's messages)
```

### Name-Change Propagation
```
/user bob2 → refreshChat → node.SetUsername("bob2") → old="bob", true
  → node.RenameSelf("bob", "bob2")
  → Broadcasts {"type":"peer_renamed","from":"bob","content":"bob2"} to all conns
  → Also sends to hostConn if client

Each peer's readHostMessages receives "peer_renamed":
  → Emits NetworkEvent{Type:"peer_renamed", From:"bob", Content:"bob2"}
  → handleNetworkEvent: RemovePeer("bob"), AddPeer("bob2"), refreshVideo
  → Chat message: "bob renamed to bob2"
```

### /leave Flow
```
/leave → update.go → m.leaveRoom()
  → node.UnregisterFromLobby(roomName)
  → node.LeaveRoom():
    → Set leaving=true
    → Broadcast {"type":"leave","from":"bob"} to all conns
    → Close all connections
    → If client: also send to hostConn
  → state: clear room, IsHost, ConnectedPeer, clear peers
  → refreshVideo → empty video panel

Host handleConn receives "leave":
  → Delete peer from conns map
  → Emit peer_disconnected to host's TUI
  → Broadcast leave to remaining peers
  → Return (exit handleConn)

Client readHostMessages receives "leave":
  → Emit peer_disconnected to client's TUI
```

### Message Types (tea.Msg implementations)
- `components.ChatSendMsg` — user typed a chat message (Enter with no / prefix)
- `components.CommandMsg` — user typed a command (Enter with / prefix)
- `components.ShowMatchesMsg` — tab completion cycling (fallback for multi-match)
- `tea.WindowSizeMsg` — terminal resize
- `tea.KeyMsg` — keyboard input
- `network.NetworkEvent` — tsnet events: listening, peer_connected, peer_disconnected, chat_message, peer_renamed, error
- `network.StatusMsg` — periodic Tailscale backend status check
- `ListRoomsMsg` — discovery results from `/list`
- `tea.KeyMsg` with `ctrl+c` — program quit

## Key Conventions

### Code Style
- No comments in code (AGENTS.md serves as documentation)
- Go standard formatting (`go fmt`)
- Exported types for cross-package types only (Message, Peer, Room, etc.)
- Unused imports are removed; unused locals cause compile errors

### Package Dependencies
```
cmd/moistchat → tui, network
tui         → components, network, state
network     → tailscale.com/tsnet (no internal imports)
state       → (standalone, no internal imports)
components  → (standalone, no internal imports, except textinput/bubbletea)
cmd/lobby   → network
```

No circular imports allowed.

### Bubble Tea Patterns
- `Model` is a value type (not pointer). Methods use value receivers.
- `Init()` returns `tea.Cmd` — use `tea.Batch()` to compose multiple cmds
- `Update()` returns `(tea.Model, tea.Cmd)` — return `m, tea.Batch(cmds...)`
- Child components return updated copies: `m.input, cmd = m.input.Update(msg)`
- `tea.Tick(d, fn)` for periodic work (re-queue by returning it from Update)
- `tea.Quit` to exit — use `return m, tea.Quit`
- Use `tea.WithAltScreen()` for full-screen terminal rendering

### tsnet Node Lifecycle
1. `NewNode()` — validates AuthKey, creates unique temp dir, opens log file, creates tsnet.Server
2. `Node.Start()` — calls `server.Listen("tcp", "0.0.0.0:9090")`, starts acceptLoop + heartbeat goroutines
3. `Node.Close()` — cancels context, closes listener, server, log file, removes state dir
4. `Node.Status(ctx)` — queries Tailscale backend state via LocalClient
5. `Node.ConnectToHost(addr, room, username)` — Dial host, send join, start readHostMessages loop
6. `Node.LeaveRoom()` — broadcast leave to all conns, close connections, set leaving flag
7. `Node.RegisterToLobby(room, host)` — register room with lobby broker
   - `Node.TryRegisterToLobby(room, host)` — safe wrapper: skips if already registered or `registerFailed` flag is set, prevents UI-blocking retries
8. `Node.UnregisterFromLobby(room)` — deregister room from lobby, clears `registerFailed` flag

### Node Struct Fields
- `server` — tsnet.Server instance
- `ln` — TCP listener (always started on :9090)
- `ctx`/`cancel` — lifecycle control
- `Events` — channel for pushing NetworkEvents to the TUI update loop
- `conns` — map of connected peers (used by host)
- `connsMu` — mutex for conns map + hostConn
- `hostConn` — persistent connection to host (used by client)
- `mu` — mutex for roomName and username
- `roomName` — cached room name (only set by host for probe responses)
- `username` — cached username for WireMessage From fields
- `leaving` — flag set by LeaveRoom() to suppress "Disconnected from host" error
- `lobbyClient` — LobbyClient instance for room registry communication
- `registerFailed` — flag on LobbyClient: set after one registration attempt fails, prevents repeated blocking calls on the UI hot path
- `registerAttempted` — (internal) ensures registration is tried exactly once per room creation

### Wire Protocol
All messages are JSON-over-TCP, line-delimited (`\n`):

```json
// Peer messages (port :9090):
{"type":"join","room":"test","from":"alice"}
{"type":"chat","room":"test","from":"alice","content":"hello"}
{"type":"peer_info","from":"bob"}
{"type":"peer_renamed","from":"bob","content":"bob2"}
{"type":"leave","from":"alice"}
{"type":"video_frame","from":"alice","content":"<base64>"}
{"type":"ping"}
{"type":"pong"}
{"type":"probe"}
{"type":"probe_response","room":"test","from":"bob"}

// Lobby messages (port :9091):
{"type":"register","room":"test","from":"bob","addr":"100.x.x.x:9090"}
{"type":"registered","room":"test"}
{"type":"error","reason":"room already exists"}
{"type":"unregister","room":"test"}
{"type":"list"}
{"type":"room_list","rooms":[{"room":"test","host":"bob","addr":"100.x.x.x:9090"}]}
{"type":"ping","room":"test"}
{"type":"pong"}
```

### Lobby Protocol

| Direction | Type | Purpose |
|-----------|------|---------|
| Host → Lobby | `register` | Register a room (rejected if name exists) |
| Lobby → Host | `registered` | Confirmation |
| Lobby → Host | `error` | Rejection (e.g., duplicate room) |
| Host → Lobby | `unregister` | Remove room on /leave |
| Any → Lobby | `list` | Request active rooms |
| Lobby → Any | `room_list` | Response with room array |
| Host → Lobby | `ping` | Heartbeat (every 10s) |
| Lobby → Host | `pong` | Heartbeat confirmation |

**Lobby lifecycle:**
- Host registers room → lobby adds to registry
- Host sends heartbeat every 10s → updates `LastHeartbeat`
- Lobby cleanup goroutine runs every 10s → removes rooms with `LastHeartbeat > 20s`
- Host unregisters on `/leave` or disconnect → immediately removed
- Room name conflicts → rejected with `{"type":"error","reason":"room already exists"}`

### Style Variables
Lipgloss styles for UI components:
- `statusStyle` — gray (#888888)
- `systemMsgStyle` — gray italic (#666666)
- `usernameStyle` — teal bold (#4ECDC4)
- `timestampStyle` — muted (#555555)
- `contentStyle` — light gray (#CCCCCC)
- `dividerStyle` — dark gray (#444444)

### Status Bar States
```
○ Starting         — Default; shown until backend reaches Running
● Connected  ...   — BackendState == "Running"
○ Disconnected ... — Real error (e.g., no auth key)
[Hosting: general]  — Appended when IsHost and room is active
```

### Logger
All of the client's tsnet logs are written to `/<tempdir>/tsnet-moistchat-XXXXXXXXX/moistchat.log` (truncated each run).
The lobby broker logs to stderr via `log.Printf` (visible in the terminal that launched, or hidden behind Bubble Tea's alt screen).
View with: `tail -f $(ls -d /tmp/tsnet-moistchat-* | head -1)/moistchat.log`
Tags: `[backend]` for verbose tsnet backend logs, `[tsnet]` for UserLogf messages,
`[discovery]` for room scan logs, `[host]` for probe/connection logs, `[lobby]` for lobby lifecycle.

### Tab Completion
Command list is in `internal/tui/components/input.go`:
```
/create, /join, /list, /user, /leave, /settings, /help, /quit
```
Tab cycles through matches: `/l` → `/list` → `/leave` → `/list` ...
Single match auto-completes with trailing space: `/cr` → `/create `
Cycling detects the current input value among previously shown matches and restores
the original prefix before advancing the index.

### View Height Guarantees
Each panel is independently height-constrained before layout composition:
- **Video panel:** lipgloss `Width(w)` for width, manual `\n`-based pad/truncate to `videoHeight`
- **Chat panel:** `maxLines` clamping via SetSize, manual `\n`-based safety truncate to `chatHeight`  
- **Input panel:** lipgloss `Width(w).Height(inputHeight)`
- **Status bar:** lipgloss `Width(w)`, natural 1 line

## Phase Status

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Scaffolding + TUI Layout | ✅ Complete |
| 2 | State + Command Integration | ✅ Complete |
| 3 | Networking (tsnet) | ✅ Complete |
| 3.5 | Room Host/Relay Architecture | ✅ Complete |
| 4 | Signaling / Lobby Protocol | ✅ Complete |
| 4.5 | Host Failover (election on host disconnect) | ✅ Complete |
| 5 | Media (Audio/Video Device I/O over TCP relay) | ✅ Complete |
| 6 | ASCII Video Rendering | ✅ Complete |
| 6b | P2P Video Transport | ✅ Complete |
| 7a | Local Config Persistence | ✅ Complete |
| 7b | UI Integration, Themes & Polish | ✅ Complete |
| 8 | Cross-Platform Compilation & Distribution | ✅ Complete |

## Phase 6b — P2P Video Transport

Each peer broadcasts their rendered Braille frame over the existing TCP relay at the camera's frame rate. Received frames are stored per-peer and displayed in the avatar grid tiles, replacing the static initial-letter avatar with a live subsampled Braille preview.

### Data Flow

```
Local camera → RenderFrame → videoBroadcastCmd → SendVideoFrame()
  → {"type":"video_frame","from":"alice","content":"<base64>"}
  → Host relay → Broadcast to all peers
  → Receiver handleConn/readHostMessages
  → NetworkEvent{Type:"video_frame"}
  → handleNetworkEvent → SetPeerVideoFrame
  → refreshVideo → SetPeerFrames
  → VideoModel.View → subsampleBrailleFrame for each tile
```

### UI Rendering

There is no full-screen local video takeover. The avatar grid is always shown. Each tile renders one of:

| Condition | Content |
|-----------|---------|
| Self tile + `videoOn` | Subsampled local Braille frame from `m.frame` |
| Peer tile + peer has stored frame | Subsampled received Braille frame from `m.peerFrames[peer]` |
| Default | Initial-letter avatar (`avatar()`) |

**Dynamic tile sizing (height-constrained):** Tile dimensions are determined by the available panel height. The algorithm iterates from 1 row upward until all peers fit. For each row count, `tileH = panelHeight/rows - 2` (content area), `tileW = tileH × 8/3` (aspect-corrected width from 4:3 source × 1:2 terminal cells). Columns are computed from panel width: `cols = width / (tileW+2)`. If `cols × rows < peers`, another row is added. Solo streaming gets one tile filling the full height.

**Uniform grid layout:** All tiles share the same outer `tileW × tileH` border. Aspect-corrected Braille content is centered inside each border via `Align(Center, Center)`. Non-video peers show avatars at the same dimensions. Empty rows below the grid are padded to `maxHeight`.

**Height enforcement:** Both `VideoModel.View()` and the outer `View()` in `view.go` independently enforce the video panel to exactly `videoHeight` lines — padding below with `\n` when content is shorter, truncating when taller. The chat panel below never shifts regardless of video state.

### Wire Format

`SendVideoFrame(frameData []byte)` encodes the serialized frame as:

| Offset | Size | Content |
|--------|------|---------|
| 0–1 | 2 bytes | `uint16` outW (sender's character width) |
| 2+ | N bytes | Braille dot-bits (1 byte per char, value 0–255) |

The payload is base64-encoded into the `content` field of a `video_frame` WireMessage. On receipt, the receiver reconstructs the Braille string by:
1. Decoding base64 → bytes
2. Reading outW from bytes[0:2]
3. Iterating remaining bytes, inserting `\n` every outW chars
4. Each byte → `rune(0x2800 + byte)`

### Broadcast Chain

A single unified `FrameUpdateMsg` pre-handler (runs before the settings-modal check) re-queues both `videoFrameCmd()` and `videoBroadcastCmd()` on every frame. There is no duplicate `case FrameUpdateMsg:` in the type-switch — one handler, no goroutine leak.

### Host Failover — Dual Trigger

When the host disconnects, the election (`electNewHost()`) is triggered by either:
- `"error: Disconnected from host"` (TCP connection loss detected by `readHostMessages`)
- `"peer_disconnected"` where the departed peer's `Addr` matches `GetConnectedPeer()` (graceful leave notification)

Whichever arrives first fires the election. The surviving peer alphabetically first (excluding the departed host) becomes the new host and re-registers the room with the lobby.

### Controls

- `/video` toggles camera + broadcast on/off
- `/contrast <0-100>` sets luminance threshold (live; slider in `/settings` also syncs)
- `/hang` stops camera + broadcast + audio call
- Received frames appear in the avatar grid automatically; the self tile shows local camera when `videoOn`; peers without frames show their avatar letter

### Files changed

| File | Change |
|------|--------|
| `internal/state/state.go` | `PeerVideoFrames map[string]string` — per-peer frame storage + Get/Set/Remove methods |
| `internal/network/transport.go` | `SendVideoFrame()` method; `"video_frame"` handling in `handleConn` and `readHostMessages` |
| `internal/tui/model.go` | `videoBroadcastCmd()`, `extractBrailleBytes()`, `reconstructBrailleString()`; `"video_frame"` case in `handleNetworkEvent`; peer frames wired in `refreshVideo()` |
| `internal/tui/components/video.go` | `peerFrames map[string]string`, `SetPeerFrames()`, `Frame()` getter; `subsampleBrailleFrame()` for tile rendering; self-tile with local frame when `videoOn`; dynamic grid scaling; extracted `tileBorder()` |
| `internal/tui/update.go` | Unified FrameUpdateMsg pre-handler with `videoBroadcastCmd()`; dual-trigger failover for `peer_disconnected`; `/contrast` command handler |
| `internal/tui/components/input.go` | `/video`, `/contrast` added to tab-completion commands |
| `internal/tui/commands.go` | `/help` text includes `/video`, `/contrast <0-100>` |
| `internal/tui/components/settings.go` | `Toggle()` syncs `s.contrast` from `media.LumaThreshold` on open |

### WireMessage types

| Type | Purpose |
|------|---------|
| `"video_frame"` | P2P Braille frame broadcast (host relay) |

## Phase 7a — Local Config Persistence

Persist username and auth key across restarts. On first launch, the onboarding message prompts for `/user <name>` and `/auth <key>`.

### New file: `internal/config/config.go`

```go
type Config struct {
    Username string `json:"username"`
    AuthKey  string `json:"auth_key,omitempty"`
}

func Exists() bool                          // os.Stat check for ~/.config/moistchat/config.json
func Load() (*Config, error)                // returns empty Config if missing/corrupt
func (c *Config) Save() error               // MkdirAll 0700, OpenFile O_CREATE|W|TRUNC 0600
func SaveUsername(name string) error        // read-modify-write, preserves auth key
func SaveAuthKey(key string) error          // read-modify-write, preserves username
```

Auth key priority: ldflags `-X` (build-time) → env var `TAILSCALE_AUTH_KEY` → config file.

### Integration

| File | Change |
|------|--------|
| `cmd/moistchat/main.go` | `config.Load()` at startup; `config.Exists()` detects first-run; `NewModel(username, firstRun)`; `network.AuthKey` falls back to env → config |
| `internal/tui/model.go` | `NewModel(username string, firstRun bool)` — shows onboarding prompt on first run |
| `internal/tui/commands.go` | `/user` calls `config.SaveUsername()` to persist name |
| `internal/tui/update.go` | `/auth <key>` command saves key to config; `SettingsUsernameChanged`/`SettingsAuthKeyChanged` pre-handlers |
| `internal/tui/components/settings.go` | Inline textinput editing for Username (focus 0) and Auth Key (focus 1); masked input for auth key; `keyChanged` boolean → orange restart banner |
| `internal/tui/components/input.go` | `/auth` added to tab-completion commands |
| `Makefile` | Removed `check-env` target — no longer requires `TAILSCALE_AUTH_KEY` to be pre-set |

### First-run flow

Config file doesn't exist → `config.Exists()` returns false → welcome message: "Set your name with /user <name> and your auth key with /auth <key> to get started."

`/user alice` → `config.SaveUsername("peter")` → file created at `~/.config/moistchat/config.json` with 0600 permissions. Next launch: file exists → `Exists()` returns true → standard welcome message.

### Inline editing in settings

Settings panel focus map:

| Focus | Row | Interaction |
|-------|-----|------------|
| 0 | Username | Enter → textinput, Enter to submit, Esc to cancel |
| 1 | Auth Key | Enter → masked textinput, Enter to submit, Esc to cancel |
| 2–7 | Audio/Video controls | ← → cycle/adjust as before |

Auth key submit → `keyChanged = true` → orange banner: "Please restart the app to apply your new authentication key."

### Status bar colors

Connected state (`● Connected ...`) rendered in bright green `#00FF00`. Starting/disconnected states remain neutral gray (`#888888`).

## Phase 7b — UI Integration, Themes & Polish

Refined the visual layouts, color engine, and interactive elements for a polished terminal experience.

### 4-Theme Color Engine

Replaced the original AdaptiveColor-based 2-theme system with a single-mode 4-theme palette system:

| Theme | Style | Colors Used |
|-------|-------|-------------|
| Solarized | Light-toned teal/gold | `#1E8CB0`, `#88A8BF`, `#2AA198`, `#859900` |
| eDEX | Dark slate with cyan accents | `#EAEAEA`, `#A0A8B8`, `#58A6FF`, `#7EE787` |
| Amber | Warm amber/gold palette | `#FFB04D`, `#CC8533`, `#E67300`, `#2E6930` |
| Neon | Cool purple/cyan neon | `#EAD6FF`, `#B38FFF`, `#8A2BE2`, `#008080` |

**Architecture:**
- `internal/theme/theme.go` — `Palette` struct with 10 color fields + 8 avatar colors; 4 theme entries in a map
- `SetTheme(name)` — reassigns all mutable `lipgloss.Color` globals and fires registered `OnThemeChange` callbacks
- Each UI file registers a rebuild callback that recreates all `var` style blocks with the new theme colors
- `config.ThemeName` replaces `config.DarkMode` — persists across restarts

### eDEX-UI 2-Column Layout (`internal/tui/view.go`)

Complete structural layout overhaul:

```
Left sidebar (52 fixed cols)    Center workspace (w - 52 flexible)
╔═══ c h a t ═══╗              ┌───────────────────────────────┐
║ MOIST ASCII  ║              │  VIDEO / AVATAR GRID          │
╚══════════════╝              │                               │
                               └───────────────────────────────┘
┌─ SYSTEM ────┐                ────────────────────────────────
│ Name: alice │               ┌───────────────────────────────┐
│ Auth: set   │               │  CHAT STREAM                   │
│ Config: ok  │               │  (scroll with PgUp/PgDn/      │
│ Theme: eDEX │               │   Home/End + mouse wheel)     │
│ Up:   2h 15m│               └───────────────────────────────┘
│ Time: 16:22 │
│ OS:   linux │
│ Engine:1.26 │
│ Memory:14.5M│
│ GoRtn:  42  │
│ Cores:  16  │
│ PID: 12345  │
│ GC Runs: 12 │
└─────────────┘
┌─ NETWORK ───┐
│ ● Connected │
│ Host: moist │
│ IP: WAN: ip │
│     LAN: ip │
│ Peers    1  │  ← includes self/host
│ Room:   hi  │
│ Status: Host│
└─────────────┘
┌─ ORBITAL V ─┐
│  ⢀⣠⣴⣶⣿⣿ │  ← scrolling Braille world map
│ ⣠⣾⣿⣿⣿⣿ │    (1s tick, smooth orbit)
│ ...           │
│ Tracking: SYS │
└─────────────┘
```

- **Status bar removed** — connection info moved to network panel
- **Input bar** — rendered at `mainWidth`, aligned under center column, no overflow; set to 1 line height
- **Chat scrollback** — PageUp/PageDown (half-page), Home/End (jump), mouse wheel (3 lines), auto-scroll on new messages
- **Height math**: `sidebarH = h - 1 - inputHeight` (minus bottom divider and input)
- Both columns `padToHeight`'d to `sidebarH`, joined with `JoinHorizontal(Top)`
- Panels use natural heights with `""` separators, only overall column is padded

### Key files changed

| File | Change |
|------|--------|
| `internal/theme/theme.go` | Full rewrite: `Palette` struct, 4 theme maps, `SetTheme()`, `OnThemeChange()` |
| `internal/config/config.go` | `DarkMode bool` → `ThemeName string`; `SaveThemeName()` |
| `cmd/moistchat/main.go` | `tea.WithMouseCellMotion()` for mouse wheel; `theme.SetTheme(cfg.ThemeName)` |
| `internal/tui/view.go` | 2-column eDEX layout, left sidebar with logo + 3 panels, no status bar |
| `internal/tui/model.go` | 4 mutable styles + `init()` rebuild; `startTime` for uptime; `renderWidth = 200` forces Braille to fixed 200 columns |
| `internal/tui/update.go` | Synchronous audio start with error reporting; goroutine removed from `/call` path |
| `internal/tui/components/chat.go` | `scrollOffset` for scrollback; PgUp/PgDn/Home/End keybindings; mouse wheel support; auto-scroll on new messages |
| `internal/tui/components/settings.go` | 8 mutable styles + `init()` rebuild; Theme cycles 4 names |
| `internal/tui/components/video.go` | `AdaptiveColor` → `Color` types |
| `internal/tui/components/input.go` | `SetThemeColors()` method |
| `internal/tui/update.go` | `ThemeChanged` + `globeTickMsg` handlers; call `SetThemeColors()` + `refreshChat()` |
| `internal/network/node.go` | Hostname constant: `"moistchat"` → `"moistchat"` |


## Gotchas

- `NetworkEvent` types are matched by string in `handleNetworkEvent` — keep values in sync
- The `UserLogf` field on `tsnet.Server` has different behavior than `Logf` — it falls back to `log.Printf` if nil (must be explicitly set)
- `TSNET_FORCE_LOGIN=1` is set via `os.Setenv` before `tsnet.Server` creation to force re-auth
- The Events channel has a 128-slot buffer; non-blocking sends prevent deadlocks
- `make dev` skips the auth key check — node is nil, all network ops are no-ops
- `log.SetOutput(logFile)` must be set in NewNode() to redirect global log.Printf to file
- All panel heights are independently enforced in View() — never rely on lipgloss Height/MaxHeight alone
- `SetRoom` and `SetUsername` use `mu` mutex for thread safety — reads in handleConn/readHostMessages must also lock
- `leaving` flag on Node must be set before closing hostConn in LeaveRoom() to suppress disconnect noise
- Lobby auto-provisioning: first instance starts the lobby, subsequent instances use it. If lobby port is skipped, startup proceeds normally (uses probe-based discovery fallback)
- `LobbyClient` uses a fresh `context.WithCancel` per registration — allows repeated `/create` → `/leave` → `/create` cycles without context starvation
- `RegisterToLobby` is a blocking call (3 IO operations: LocalClient, Status, TCP dial). Never call it from `refreshChat()` directly — always use `TryRegisterToLobby` which checks the `registerFailed` flag first
- macOS builds are done natively on a Mac using `make cross-compile`. macOS CGO audio requires Apple's Xcode command-line tools (`xcode-select --install`). The Linux/Windows binaries for distribution are built via `make cross-compile` on macOS — Go cross-compilation handles the different targets without needing platform-specific toolchains.
- Windows CGO builds require MSYS2/MinGW with `mingw-w64-ucrt-x86_64-gcc` and `mingw-w64-ucrt-x86_64-opus`. Native build on Windows is required for audio support — cross-compilation from Linux was dropped.
- The `rsrc_windows_amd64.syso` file embeds `icon.png` into the Windows `.exe` via `golang/go` resource embedding. It's ignored on Linux/macOS builds.

## Phase 8 — Cross-Platform Compilation & Distribution

Generate optimized standalone executable binaries for Linux, macOS (Darwin), and Windows networks using the built-in cross-compilation toolchain (`make cross-compile`). This phase is to be executed only after all core features, TUI layouts, and theme protocols are fully verified and stabilized.

### Distribution Targets

- **Linux:** Native binary executable.
- **macOS:** Darwin-targeted binary compatible with modern terminal emulators.
- **Windows:** Self-contained `moistchat.exe` executable running natively within PowerShell / Windows Terminal.

### Build Tags (Windows CGO Integration)

On native Windows builds with CGO enabled, the camera driver and opus codec compile through updated build tags:

| File | Tag | Effect |
|------|-----|--------|
| `internal/media/camera_register_cgo.go` | `cgo` (no `!windows` exclusion) | Camera driver registered on Windows CGO builds |
| `internal/media/opus.go` | `cgo && (linux \|\| windows)` | Opus CGO compiled on Windows with MinGW headers |
| `internal/media/opus_stub.go` | `!cgo` | Stub only used when CGO is disabled |

On Windows, build natively with MSYS2/MinGW for full audio via WASAPI/malgo.

### macOS Build

1. **Go 1.26.4** — Download from:
   - [go1.26.4.darwin-amd64.pkg](https://go.dev/dl/go1.26.4.darwin-amd64.pkg) (Intel Macs)
   - [go1.26.4.darwin-arm64.pkg](https://go.dev/dl/go1.26.4.darwin-arm64.pkg) (Apple Silicon M1/M2/M3)

2. **Xcode Command Line Tools** (required for CGO — audio + camera):
   ```bash
   xcode-select --install
   ```

3. **Homebrew packages** (Opus audio codec):
   ```bash
   brew install opus pkg-config
   ```

4. **Build**:
   ```bash
   cd /path/to/moistchat
   make build
   ```

5. **Cross-compile for all platforms**:
   ```bash
   make cross-compile
   ```

- Audio uses macOS CoreAudio via `malgo` (CGO) — works natively
- Camera uses AVFoundation via `pion/mediadevices` (CGO) — works natively
- Opus links against Homebrew's `libopus`
- Config saved to `~/.config/moistchat/config.json`

### Verification Checklist

- Confirm all persistent local configuration permissions (0600) map properly across cross-platform directory targets.
- Verify Lip Gloss styles and color profiles degrade gracefully across distinct terminal environments.

## Future — P2P File Transfer

Add peer-to-peer file transfer over the existing TCP relay using a `file_transfer` wire message type. No SFTP/SSH needed — reuse the same relay connections already carrying chat, audio, and video.

### Wire Message

```json
{"type":"file_transfer","from":"alice","content":"<base64 chunks>","metadata":{"name":"cat.png","size":12345,"chunks":3,"index":0}}
```

Chunked transfer over the existing connection. The receiver reassembles chunks and writes to a configurable download directory.

### Design Decisions Needed

- `/send <path>` command to initiate a transfer
- Files appear in chat as "[alice sent: cat.png (1.2 MB)]"
- Progress shown in the status/input area
- Receive directory: `~/Downloads/moistchat/` by default, configurable
- Size limits / concurrent transfer handling
- Video broadcast pauses during active transfer (or multiplex)
