# Battleship

A server-authoritative Battleship game with AI and multiplayer modes. Built with Go, vanilla JS, and SQLite.

## For Sentience Reviewers

Please read [WRITEUP.md](WRITEUP.md) for full commentary.

## Quick Start

```bash
go run .
# Open http://localhost:8080
```

## Game Modes

- **vs AI** — Play against a probability-density AI opponent
- **vs Human** — Create a room, share the 4-letter code, play in real time

## Architecture

See [WRITEUP.md](WRITEUP.md) for full details on architecture, AI design, anti-cheat, and scalability.

## Running Tests

```bash
go test ./...
```

## Deployment

```bash
docker build -t battleship .
docker run -p 8080:8080 -v battleship-data:/data battleship
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `DB_PATH` | `battleship.db` | SQLite database file path |
| `ALLOWED_ORIGIN` | _(empty)_ | WebSocket origin restriction (e.g., `battleship.up.railway.app`) |
