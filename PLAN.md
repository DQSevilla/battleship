# Battleship — Implementation Plan

## Stack

- **Backend:** Go (`github.com/DQSevilla/battleship`), `github.com/coder/websocket` for WebSocket support
- **Frontend:** Vanilla HTML/CSS/JS, CSS Grid, no build step
- **Storage:** SQLite via `modernc.org/sqlite` (pure Go, no CGO — easy cross-compilation and deployment)
- **Deployment:** Railway (Dockerfile-based)

## Architecture

```
┌─────────────┐       WebSocket / HTTP       ┌──────────────┐
│  Browser UI  │  ◄──────────────────────►   │   Go Server  │
│  (vanilla    │                              │  (game logic,│
│   HTML/CSS/  │                              │   AI, rooms) │
│   JS)        │                              └──────┬───────┘
└─────────────┘                                      │
                                                     ▼
                                              ┌──────────────┐
                                              │   SQLite     │
                                              └──────────────┘
```

### Server-Authoritative Design

All game logic runs on the server. The client sends coordinates and receives results. The client never sees opponent ship positions. This is the primary anti-cheat mechanism.

### WebSocket Message Protocol

Simple JSON messages:

```json
{"type": "fire", "x": 3, "y": 7}
{"type": "result", "x": 3, "y": 7, "result": "hit", "sunk": "Cruiser"}
{"type": "turn", "your_turn": true}
```

### Configurable Game Parameters

`GameConfig` struct makes board size and ship roster configurable (not hardcoded to 10x10 with standard ships). This demonstrates scalability thinking — the system supports arbitrary board sizes and ship configurations.

## Project Structure

```
battleship/
├── main.go                 # Entry point, HTTP server, routes
├── internal/
│   ├── game/
│   │   ├── board.go        # Board struct, ship placement, validation
│   │   ├── game.go         # Game state machine (placement → firing → done)
│   │   ├── player.go       # Player state, fleet tracking
│   │   └── config.go       # Configurable board size, ship definitions
│   ├── ai/
│   │   └── ai.go           # AI opponent (probability density + hunt/target)
│   ├── room/
│   │   └── room.go         # Room/lobby management, matchmaking
│   ├── ws/
│   │   └── handler.go      # WebSocket upgrade, message routing
│   └── store/
│       └── sqlite.go       # SQLite persistence (game history, active state)
├── web/
│   ├── index.html          # Main page (lobby/menu)
│   ├── game.html           # Game page
│   ├── css/
│   │   └── style.css       # Responsive CSS Grid layout
│   └── js/
│       ├── game.js         # Game logic, board rendering, WebSocket client
│       └── placement.js    # Ship placement UI (click + rotate)
├── go.mod
├── go.sum
└── Dockerfile              # For Railway deployment
```

## Implementation Phases

### Phase 1 — Core Game Engine

- `Board` type: NxN grid, ship placement with validation (bounds, overlap)
- `Game` state machine: `WaitingForPlayers → Placement → Firing → Finished`
- `Player` state: fleet, board, shot history
- `GameConfig`: configurable dimensions and ship definitions
- Unit tests for placement validation, hit/miss/sunk logic, win detection

### Phase 2 — WebSocket Server & Rooms

- HTTP server with `github.com/coder/websocket`
- `Room` struct: two player connections, game instance, message routing
- `RoomManager`: create room (returns code), join room (by code)
- JSON message protocol for all game actions
- Handle disconnection/reconnection gracefully

### Phase 3 — AI Opponent

- `AIPlayer` that implements the same interface as a human player
- Probability density calculation for shot selection
- Hunt/target state machine
- Random ship placement with retry

### Phase 4 — Frontend

- **Menu page:** New game (vs AI / vs Human), Join game (enter code)
- **Game page:** Two 10x10 CSS Grid boards (your fleet + opponent's grid)
- **Placement UI:** Click to place, button to rotate, visual validation feedback
- **Firing UI:** Click opponent grid to fire, immediate visual feedback
- **Responsive:** Side-by-side boards on desktop, stacked on mobile (`@media` queries)
- WebSocket client with reconnection logic

### Phase 5 — Persistence

- SQLite schema: `games` table (id, config, state, created_at, updated_at), `moves` table (game_id, player, x, y, result, timestamp), `game_results` table (winner, duration, etc.)
- Save game state on each action (for refresh survival)
- On WebSocket reconnect: hydrate full game state from DB
- Completed game history queryable via simple REST endpoints

### Phase 6 — Deployment

- Multi-stage Dockerfile (build stage → minimal runtime image)
- Railway config (expose port, SQLite volume mount for persistence)
- Smoke test the live URL

### Phase 7 — Polish & Writeup

- WRITEUP.md covering: approach, AI usage, architecture decisions, anti-cheat, scalability
- Final UI polish pass

## Anti-Cheat Strategy

- **Server-authoritative:** Client never receives opponent ship positions
- **Server-side validation:** Ship placement validated for bounds, overlap, and contiguity
- **Turn enforcement:** Server rejects out-of-turn fire requests
- **Move history:** All moves stored in DB for post-game verification/replay

## Scalability Story

- `GameConfig` makes board size and ship roster fully configurable
- In-memory game rooms managed by a `RoomManager` with goroutine-per-game
- SQLite is the right choice for single-node; the `Store` interface abstracts storage so swapping to Postgres is trivial
- For horizontal scaling: move to Postgres + Redis pub/sub for cross-node game events — the architecture already separates concerns cleanly

## AI: Hunt/Target + Probability Density

- **Random/Hunt phase:** Pick shots weighted by probability density — cells where remaining ships *could* fit score higher. Naturally avoids isolated cells and prefers the center of open areas.
- **Target phase:** After a hit, probe adjacent cells. Stack-based targeting: if multiple hits align, follow the line. When a ship sinks, clear the target stack and resume hunting.

## Multiplayer: Room Codes

- Player 1 creates a game, receives a short room code (e.g., `ABCD`)
- Player 1 shares the code with Player 2
- Player 2 enters the code to join
- Both players connect via WebSocket to the same game room
