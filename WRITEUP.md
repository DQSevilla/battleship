# Battleship — Writeup

## Approach

I built this as a server-authoritative real-time game using Go for the backend and vanilla HTML/CSS/JS for the frontend. The guiding principle was: **all game logic runs on the server, the client is a thin display layer**. This is the right architecture for a competitive game because it makes cheating fundamentally difficult.

**Tools used:** Claude Opus 4.6 via opencode for planning and implementation, Go 1.24 with `coder/websocket` for WebSocket support, pure-Go SQLite for persistence.

## Architecture

```
Browser (vanilla JS)  <--WebSocket/JSON-->  Go Server  <-->  SQLite
```

### Package Structure

- **`game/`** — Pure game logic (board, ships, placement, firing, state machine). Zero I/O, zero network — fully testable in isolation.
- **`ai/`** — AI opponent using probability density + hunt/target. Operates against the same `game.Game` interface as human players.
- **`room/`** — In-memory room/lobby management. Maps room codes to games and WebSocket connections.
- **`ws/`** — WebSocket message handler. Routes JSON messages to game actions, manages AI turns, handles reconnection.
- **`store/`** — SQLite persistence. Saves every game state change and every move for history/replay/reconnection.

### State Machine

The game progresses through four phases: `Waiting → Placing → Firing → Finished`. Each transition is server-enforced. The client cannot skip phases or submit out-of-turn actions.

### WebSocket Protocol

Simple JSON messages discriminated by a `type` field:

```json
// Client → Server
{"type": "fire", "target": {"x": 3, "y": 7}}

// Server → Client
{"type": "fire_result", "coord": {"x": 3, "y": 7}, "hit": true, "sunk_ship": "Cruiser", "sunk_ship_coords": [{"x":3,"y":5},{"x":3,"y":6},{"x":3,"y":7}]}
```

The server sends sunk ship coordinates explicitly so the client doesn't have to guess which cells belong to a sunk ship.

## AI Design: Hunt/Target + Probability Density

The AI uses a two-phase strategy:

1. **Hunt mode** — Computes a probability density map: for each cell, count how many ways any remaining (unsunk) ship could be placed covering that cell. Cells with higher density are more likely to contain a ship. The AI picks randomly among the highest-density cells. This naturally favors the center of open areas and avoids wasted shots near edges.

2. **Target mode** — After a hit, the AI probes adjacent cells. If multiple hits align (same row or column), it extends the line. When a ship sinks, its hits are removed from the target stack and the AI resumes hunting.

This approach is significantly smarter than random firing. In testing, the AI consistently sinks all 17 ship cells in under 60 shots on a 10x10 board (theoretical minimum is 17, random average is ~65).

## Anti-Cheat Strategy

1. **Server-authoritative design** — The client never receives opponent ship positions. The server validates every action and only sends results (hit/miss/sunk). A client inspecting WebSocket messages or JavaScript state will never see where opponent ships are.

2. **Server-side validation** — Ship placement is validated for bounds, overlap, and ship existence. Firing is validated for bounds, turn order, and duplicate shots. The game phase is enforced — you can't fire during placement, can't place during firing.

3. **Turn enforcement** — The server rejects out-of-turn fire requests with `ErrNotYourTurn`. The client disables the opponent board during the opponent's turn, but even if a user bypasses this, the server blocks it.

4. **Move history** — Every move is stored in SQLite with timestamps, enabling post-game replay and audit. If cheating is suspected, the full game can be reconstructed from the `moves` table.

## Scalability Considerations

### Configurable Game Parameters

`GameConfig` makes board size and ship roster fully configurable. The system isn't hardcoded to 10x10 with 5 ships — you could play on a 50x50 board with 20 ships. The AI's probability density computation scales as O(ships × board² × 2 × maxShipLength), which is linear in board area.

### Storage Abstraction

SQLite is the right choice for a single-node deployment — it's embedded, requires no external service, and WAL mode provides good concurrent read/write performance. The `Store` struct abstracts all database access behind methods, so swapping to PostgreSQL for a multi-node deployment would mean implementing the same interface against a different driver.

### Horizontal Scaling Path

The current architecture is single-process: games live in memory, managed by a `RoomManager`. For horizontal scaling:
- Move game state to PostgreSQL (already compatible via the `Store` interface)
- Use Redis pub/sub for cross-node WebSocket message routing
- Sticky sessions or a shared session store for WebSocket connections

The code is already structured to support this: the `room`, `game`, and `store` packages have clean separation of concerns.

### Runtime Complexity

| Operation | Complexity |
|-----------|-----------|
| Ship placement | O(ship_length) for validation |
| Fire/receive shot | O(total_ship_cells) worst case to find which ship was hit |
| AI probability density | O(remaining_ships × board² × 2 × max_ship_length) |
| AI target mode | O(hit_stack_size × 4) for adjacent probing |
| State persistence | O(1) SQLite write (UPSERT) |

For a 10x10 board, all operations complete in microseconds. For a 100x100 board, the AI density computation would take ~1ms — still negligible.

## Persistence & Reconnection

### Survive Page Refresh

1. **Server-side:** Every game state change and every move is persisted to SQLite. On server restart, active games are restored from the database.
2. **Client-side:** The player's session (playerID, roomCode, mode) is stored in `sessionStorage`. On page load, the client checks for an existing session and sends a `rejoin_game` message.
3. **Reconnection flow:** The server sends a `game_state` message with the full board state — own board (with ships, hits, misses), opponent board (hits/misses only, no ships), placed ships, remaining ships, current turn. The client reconstructs its UI from this state.

### Game History

Completed games are queryable via REST:
- `GET /api/games` — Lists completed games with config details
- `GET /api/games/{id}` — Returns a game and all its moves with parsed data

## Technology Choices

| Choice | Rationale |
|--------|-----------|
| **Go** | Fast, strongly typed, excellent concurrency primitives (goroutines, mutexes), compiles to a single binary |
| **`coder/websocket`** | Maintained WebSocket library, clean API, supports context cancellation |
| **Vanilla JS** | No build step, fast iteration, appropriate complexity for a game UI |
| **CSS Grid** | Natural fit for a grid-based game board |
| **SQLite (pure Go)** | No CGO, single-file database, WAL mode for performance, trivial deployment |
| **Multi-stage Dockerfile** | Small runtime image (~20MB), `CGO_ENABLED=0` for static binary |

## What I'd Add With More Time

1. **Game replay** — The move history is all there; a replay viewer that steps through moves would be straightforward.
2. **Spectator mode** — Add read-only WebSocket connections that receive both players' moves.
3. **ELO/ranking** — Track player performance across games.
4. **Sound effects** — Hit/miss/sunk audio feedback.
5. **Animations** — CSS transitions for shots landing, ships sinking.
6. **Rate limiting** — Protect against WebSocket message flooding.
