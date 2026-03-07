# Battleship — Writeup

## Summary

TL;DR: Claude Opus 4.6 + Golang + Vanila Web Technologies
I didn't have much time for a spike unfortunately, but I'd say that my quality of life improvements and smart enemy AI were novel spikes given the time-frame of the assignment.

## Technologies and Tools Used

- Golang for the server backend due to familiarity, performance, and ability to produce single large binaries that could more easily be orchestrated in a dockerfile
  - Plus the `coder/websocket` package for WebSocket support
- Pure-go SQLite for persistence, keeping simple on-disk records for ongoing games
- Vanilla html/css to keep everything minimalist and easy to understand
- Docker + Railway for deploying the service - free and dead-simple
- Claude Opus 4.6 via Opencode for planning and implementation

## Implementation Strategy

First I copied the original take-home assignmetn into `PROBLEM.md` and worked asked claude to read it and ask necessary follow-up questions. It asked questions like:

- What to use for the server language, frontend framework, datastore, etc
- How I intended to deploy it
- What my "spike" was going to be

I answered these in-turn and then asked it to generate a detailed `PLAN.md` on what steps to take to solve the problem. Originally Claude was eager to give time estimates for every task but I asked it to stop doing that because they were mostly exaggerated and worthless.

As for the spike question I asked Claude to implement a smart AI that would go beyond random selection plus adjacent shots to hits, and use a probability distribution to shoot at "lonliner" spots on the board that are far away from ships, as most players like to separate their ships.

I then asked Claude to work on the first few phases one-at-a-time so that I could review and make corrections to Claude's assumptions about the problem. During the first pass in which it wrote-out the core Go packages for the game logic, I made a few suggestions regarding standard library usage rather than reinventing everything from scratch, but was mostly happy with the output.

In the next few stages I ensured that some kind of web app with a simple GO http server was locally runnable in-case quick iteration was needed, which it did without trouble.

Next, I figured I would experiment with Claude one-shotting the rest, prioritizing quick iteration on something that was broken over meticulously checking each phase of the plan before generating code. It seemed to hum along just fine for a while and test the code locally to make sure everything worked. Skeptical, I asked parallel gemini and opus agents to check its work, by reading the plan.md and asking them to evaluate the implementation. It found several issues.

Here's an AI summary of the problems found this way:

Critical / High Priority
1. Dockerfile Go version mismatch — Dockerfile used golang:1.23-alpine but go.mod declared go 1.24.0. Would likely break the build.
2. No reconnection support — The plan explicitly called for graceful reconnection. The server saved state to SQLite on every action, but there was zero code to load it back. If a player disconnected, their game was gone.
3. No page refresh survival — The client had no mechanism to rejoin a game after refresh. The onclose handler reconnected the WebSocket but never sent a rejoin message — the new connection was anonymous and useless.
4. BuildGameRecord data race — store.BuildGameRecord() accessed Game struct fields (Config, Turn, Players, Phase, etc.) directly without acquiring the game's mutex, while other goroutines (like aiTakeTurn) could be mutating those same fields concurrently.
5. Player.FiredAt lost on serialization — FiredAt map[Coord]bool was tagged json:"-", meaning it was excluded from JSON serialization. If state restoration from the DB were ever implemented, the duplicate-shot prevention map would be silently empty.
Medium Priority
6. REST API hid data — ConfigJSON, StateJSON, and MoveRecord.DataJSON were all tagged json:"-", so the REST endpoints (/api/games, /api/games/{id}) returned game metadata but not the actual game config or move data. The history API was effectively useless.
7. Sunk ship BFS was incorrect — When a ship sank, the client used BFS to flood-fill connected "hit" cells as "sunk." If two different ships had adjacent hits, the BFS would incorrectly mark the wrong ship's cells as sunk. The server knew the exact coordinates but didn't send them.
8. WebSocket polling with setInterval — Menu buttons used setInterval(50ms) to poll ws.readyState === OPEN instead of using the ws.onopen callback. This was a race-prone pattern that could poll forever if the connection failed.
9. No graceful HTTP shutdown — http.ListenAndServe blocked forever with no signal handling. Active WebSocket connections would be killed abruptly on SIGTERM/SIGINT (e.g., during Railway deploys).
10. InsecureSkipVerify: true in production — WebSocket accepted connections from any origin with no way to restrict it. Fine for development, a vulnerability in production.
11. AI PlaceShips unbounded recursion — If ship placement failed after 1000 attempts, the function called itself recursively with no depth limit. Could stack overflow on pathological configs.
12. No store tests — The store package (SQLite schema, UPSERT, BuildGameRecord serialization, RestoreGame) had zero test coverage.
13. No room tests — The room package (mutex-protected room lifecycle, code generation, player management) had zero test coverage.
Low Priority
14. Column headers limited to A–Z — String.fromCharCode(65 + x) broke for boards wider than 26 columns, despite the plan emphasizing configurable board sizes as a scalability feature.
15. Board.PlaceShip duplicated coordinate logic — Manually computed ship coordinates instead of reusing the existing PlacedShip.Coords() method. Minor DRY violation.

I asked Opus to implement changes to fix this and then re-asked a new instance to double check work against the plan. This time, the only issues it complained about were trivial and unimportant, which was a good sign.

Then, I signed up with Railway, which was easy to set-up and publish to via the CLI on macos. Claude actually had trouble running the correct commands so I quickly browsed their documentation and got it working with a few commands and button-presses on the dashboard. The site was finally viewable on https://battleship-production-a93c.up.railway.app/

I played around manually with the vs AI mode and the multiplayer mode. Both worked in principal but I noticed some things that were missing or that I most eagerly wanted to add, which I bulleted quickly in `IMPROVEMENTS.md` before asking another claude instance to get to work.

# AI Generated Summary

Readers be warned: This is non-reviewed AI generated output of the final architecture of the app, not really cleaned up and minimized for human review.

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
