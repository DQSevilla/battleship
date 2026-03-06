package room

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"

	"github.com/coder/websocket"

	"github.com/DQSevilla/battleship/internal/game"
)

var (
	ErrRoomNotFound = errors.New("room not found")
	ErrRoomFull     = errors.New("room is full")
)

// PlayerConn associates a player ID with their WebSocket connection.
type PlayerConn struct {
	PlayerID string
	Conn     *websocket.Conn
	mu       sync.Mutex
}

// Send sends a JSON message to the player's WebSocket connection.
func (pc *PlayerConn) Send(ctx context.Context, msg interface{}) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return pc.Conn.Write(ctx, websocket.MessageText, data)
}

// Room holds a game and the connected players.
type Room struct {
	Code       string
	Game       *game.Game
	Players    [2]*PlayerConn
	Mode       string // "ai" or "human"
	AIPlayerID string // set when Mode == "ai"
	mu         sync.Mutex
}

// Broadcast sends a message to all connected players in the room.
func (r *Room) Broadcast(ctx context.Context, msg interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, pc := range r.Players {
		if pc != nil {
			if err := pc.Send(ctx, msg); err != nil {
				log.Printf("broadcast error to %s: %v", pc.PlayerID, err)
			}
		}
	}
}

// SendTo sends a message to a specific player in the room.
func (r *Room) SendTo(ctx context.Context, playerID string, msg interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, pc := range r.Players {
		if pc != nil && pc.PlayerID == playerID {
			return pc.Send(ctx, msg)
		}
	}
	return fmt.Errorf("player %s not found in room", playerID)
}

// GetOpponentConn returns the connection of the other player.
func (r *Room) GetOpponentConn(playerID string) *PlayerConn {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, pc := range r.Players {
		if pc != nil && pc.PlayerID != playerID {
			return pc
		}
	}
	return nil
}

// AddPlayer adds a player connection to the room.
func (r *Room) AddPlayer(pc *PlayerConn) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Players[0] == nil {
		r.Players[0] = pc
		return nil
	}
	if r.Players[1] == nil {
		r.Players[1] = pc
		return nil
	}
	return ErrRoomFull
}

// ReplacePlayer replaces the connection for an existing player (used for reconnection).
func (r *Room) ReplacePlayer(pc *PlayerConn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, existing := range r.Players {
		if existing != nil && existing.PlayerID == pc.PlayerID {
			r.Players[i] = pc
			return
		}
	}
	// Player wasn't in room yet (shouldn't happen for rejoin), add to first empty slot.
	for i, existing := range r.Players {
		if existing == nil {
			r.Players[i] = pc
			return
		}
	}
}

// RemovePlayer removes a player from the room and returns true if the room is now empty.
func (r *Room) RemovePlayer(playerID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, pc := range r.Players {
		if pc != nil && pc.PlayerID == playerID {
			r.Players[i] = nil
			break
		}
	}
	return r.Players[0] == nil && r.Players[1] == nil
}

// Manager manages all active game rooms.
type Manager struct {
	mu    sync.RWMutex
	rooms map[string]*Room
}

// NewManager creates a new room manager.
func NewManager() *Manager {
	return &Manager{
		rooms: make(map[string]*Room),
	}
}

// CreateRoom creates a new room with a unique code and returns it.
func (m *Manager) CreateRoom(gameID string, cfg game.GameConfig, mode string) (*Room, error) {
	g, err := game.NewGame(gameID, cfg)
	if err != nil {
		return nil, err
	}

	code := m.generateCode()

	room := &Room{
		Code: code,
		Game: g,
		Mode: mode,
	}

	m.mu.Lock()
	m.rooms[code] = room
	m.mu.Unlock()

	return room, nil
}

// RestoreRoom restores a room from a previously persisted game (used on server restart).
// The room has no player connections — players must rejoin via WebSocket.
func (m *Manager) RestoreRoom(code string, g *game.Game, mode string) *Room {
	room := &Room{
		Code: code,
		Game: g,
		Mode: mode,
	}

	m.mu.Lock()
	m.rooms[code] = room
	m.mu.Unlock()

	return room
}

// GetRoom retrieves a room by its code.
func (m *Manager) GetRoom(code string) (*Room, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	code = strings.ToUpper(code)
	room, ok := m.rooms[code]
	if !ok {
		return nil, ErrRoomNotFound
	}
	return room, nil
}

// RemoveRoom removes a room from the manager.
func (m *Manager) RemoveRoom(code string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rooms, code)
}

// RoomCount returns the number of active rooms.
func (m *Manager) RoomCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.rooms)
}

// generateCode creates a random 4-character uppercase room code.
func (m *Manager) generateCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no I/O/0/1 to avoid confusion
	for {
		code := make([]byte, 4)
		for i := range code {
			n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
			code[i] = charset[n.Int64()]
		}
		c := string(code)

		m.mu.RLock()
		_, exists := m.rooms[c]
		m.mu.RUnlock()

		if !exists {
			return c
		}
	}
}
