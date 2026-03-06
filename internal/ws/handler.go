package ws

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/DQSevilla/battleship/internal/ai"
	"github.com/DQSevilla/battleship/internal/game"
	"github.com/DQSevilla/battleship/internal/room"
)

// Handler manages WebSocket connections and routes messages.
type Handler struct {
	rooms *room.Manager
	aiMu  sync.Mutex
	ais   map[string]*ai.AI // room code -> AI instance
}

// NewHandler creates a new WebSocket handler.
func NewHandler(rooms *room.Manager) *Handler {
	return &Handler{
		rooms: rooms,
		ais:   make(map[string]*ai.AI),
	}
}

// getAI retrieves the AI for a room.
func (h *Handler) getAI(roomCode string) *ai.AI {
	h.aiMu.Lock()
	defer h.aiMu.Unlock()
	return h.ais[roomCode]
}

// setAI stores the AI for a room.
func (h *Handler) setAI(roomCode string, a *ai.AI) {
	h.aiMu.Lock()
	defer h.aiMu.Unlock()
	h.ais[roomCode] = a
}

// removeAI removes the AI for a room.
func (h *Handler) removeAI(roomCode string) {
	h.aiMu.Lock()
	defer h.aiMu.Unlock()
	delete(h.ais, roomCode)
}

// ServeHTTP upgrades the connection to WebSocket and starts handling messages.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Allow all origins for development
	})
	if err != nil {
		log.Printf("websocket accept error: %v", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "internal error")

	ctx := r.Context()
	h.handleConnection(ctx, conn)
}

// handleConnection reads messages from a WebSocket connection and routes them.
func (h *Handler) handleConnection(ctx context.Context, conn *websocket.Conn) {
	// Each connection needs to create or join a game first.
	// We track which room/player this connection belongs to.
	var currentRoom *room.Room
	var playerID string

	defer func() {
		if currentRoom != nil && playerID != "" {
			empty := currentRoom.RemovePlayer(playerID)
			// Notify opponent that player left.
			opponent := currentRoom.GetOpponentConn(playerID)
			if opponent != nil {
				_ = opponent.Send(ctx, ServerMessage{
					Type:    MsgOpponentLeft,
					Message: "Opponent has disconnected",
				})
			}
			if empty {
				h.rooms.RemoveRoom(currentRoom.Code)
			}
		}
	}()

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			// Connection closed or errored.
			if websocket.CloseStatus(err) != -1 {
				log.Printf("websocket closed: %v", err)
			}
			return
		}

		var msg ClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			sendError(ctx, conn, "invalid message format")
			continue
		}

		switch msg.Type {
		case MsgCreateGame:
			currentRoom, playerID = h.handleCreateGame(ctx, conn, msg)

		case MsgJoinGame:
			currentRoom, playerID = h.handleJoinGame(ctx, conn, msg)

		case MsgPlaceShip:
			if currentRoom == nil {
				sendError(ctx, conn, "not in a game")
				continue
			}
			h.handlePlaceShip(ctx, currentRoom, playerID, msg)

		case MsgFire:
			if currentRoom == nil {
				sendError(ctx, conn, "not in a game")
				continue
			}
			h.handleFire(ctx, currentRoom, playerID, msg)

		default:
			sendError(ctx, conn, fmt.Sprintf("unknown message type: %s", msg.Type))
		}
	}
}

// handleCreateGame creates a new room and adds the player to it.
func (h *Handler) handleCreateGame(ctx context.Context, conn *websocket.Conn, msg ClientMessage) (*room.Room, string) {
	playerID := generatePlayerID()
	cfg := game.DefaultConfig()

	mode := msg.Mode
	if mode == "" {
		mode = "human"
	}

	gameID := fmt.Sprintf("game-%s", generateShortID())
	r, err := h.rooms.CreateRoom(gameID, cfg, mode)
	if err != nil {
		sendError(ctx, conn, fmt.Sprintf("failed to create game: %v", err))
		return nil, ""
	}

	// Add player to game engine.
	if err := r.Game.AddPlayer(playerID); err != nil {
		sendError(ctx, conn, fmt.Sprintf("failed to add player: %v", err))
		return nil, ""
	}

	// Add player connection to room.
	pc := &room.PlayerConn{PlayerID: playerID, Conn: conn}
	if err := r.AddPlayer(pc); err != nil {
		sendError(ctx, conn, fmt.Sprintf("failed to join room: %v", err))
		return nil, ""
	}

	ships := makeShipInfo(cfg.Ships)

	_ = pc.Send(ctx, ServerMessage{
		Type:     MsgGameCreated,
		RoomCode: r.Code,
		PlayerID: playerID,
		GameID:   gameID,
		Config:   &cfg,
		Ships:    ships,
	})

	log.Printf("game created: room=%s player=%s mode=%s", r.Code, playerID, mode)

	// If AI mode, add AI player, place AI ships, and start game immediately.
	if mode == "ai" {
		h.setupAIPlayer(ctx, r, playerID)
	}

	return r, playerID
}

// handleJoinGame joins an existing room by code.
func (h *Handler) handleJoinGame(ctx context.Context, conn *websocket.Conn, msg ClientMessage) (*room.Room, string) {
	r, err := h.rooms.GetRoom(msg.RoomCode)
	if err != nil {
		sendError(ctx, conn, "room not found")
		return nil, ""
	}

	playerID := generatePlayerID()

	// Add player to game engine.
	if err := r.Game.AddPlayer(playerID); err != nil {
		sendError(ctx, conn, fmt.Sprintf("failed to join game: %v", err))
		return nil, ""
	}

	// Add player connection to room.
	pc := &room.PlayerConn{PlayerID: playerID, Conn: conn}
	if err := r.AddPlayer(pc); err != nil {
		sendError(ctx, conn, fmt.Sprintf("room is full: %v", err))
		return nil, ""
	}

	cfg := r.Game.Config
	ships := makeShipInfo(cfg.Ships)

	// Tell the joining player they're in.
	_ = pc.Send(ctx, ServerMessage{
		Type:     MsgGameJoined,
		RoomCode: r.Code,
		PlayerID: playerID,
		GameID:   r.Game.ID,
		Config:   &cfg,
		Ships:    ships,
	})

	// Tell both players the game is starting (placement phase).
	r.Broadcast(ctx, ServerMessage{
		Type:    MsgGameStart,
		Message: "Both players connected. Place your ships!",
	})

	log.Printf("player joined: room=%s player=%s", r.Code, playerID)
	return r, playerID
}

// handlePlaceShip processes a ship placement request.
func (h *Handler) handlePlaceShip(ctx context.Context, r *room.Room, playerID string, msg ClientMessage) {
	if msg.Start == nil {
		sendErrorTo(ctx, r, playerID, "missing start coordinate")
		return
	}

	err := r.Game.PlaceShip(playerID, msg.ShipName, *msg.Start, msg.Orientation)
	if err != nil {
		sendErrorTo(ctx, r, playerID, fmt.Sprintf("placement failed: %v", err))
		return
	}

	// Get remaining ships to place.
	player := r.Game.GetPlayer(playerID)
	remaining := make([]string, 0, len(player.Unplaced))
	for name := range player.Unplaced {
		remaining = append(remaining, name)
	}

	orient := msg.Orientation
	_ = r.SendTo(ctx, playerID, ServerMessage{
		Type:        MsgPlaceResult,
		ShipName:    msg.ShipName,
		Start:       msg.Start,
		Orientation: &orient,
		Remaining:   remaining,
	})

	// If this player is now ready, notify opponent (human games only).
	if player.AllPlaced() {
		opponent := r.GetOpponentConn(playerID)
		if opponent != nil {
			_ = opponent.Send(ctx, ServerMessage{
				Type:    MsgOpponentReady,
				Message: "Opponent has placed all ships",
			})
		}
	}

	// If both players are ready, transition to firing and notify.
	if r.Game.BothReady() {
		if r.Mode == "ai" {
			// In AI mode, only notify the human player.
			isYourTurn := true
			_ = r.SendTo(ctx, playerID, ServerMessage{
				Type:     MsgAllPlaced,
				Message:  "All ships placed! Firing phase begins. Your turn!",
				YourTurn: &isYourTurn,
			})
		} else {
			turnPlayerID := r.Game.GetTurnPlayerID()
			// Send turn updates to both players.
			for _, pc := range r.Players {
				if pc != nil {
					isYourTurn := pc.PlayerID == turnPlayerID
					_ = pc.Send(ctx, ServerMessage{
						Type:     MsgAllPlaced,
						Message:  "All ships placed! Firing phase begins.",
						YourTurn: &isYourTurn,
					})
				}
			}
		}
	}
}

// handleFire processes a fire request.
func (h *Handler) handleFire(ctx context.Context, r *room.Room, playerID string, msg ClientMessage) {
	if msg.Target == nil {
		sendErrorTo(ctx, r, playerID, "missing target coordinate")
		return
	}

	result, err := r.Game.Fire(playerID, *msg.Target)
	if err != nil {
		sendErrorTo(ctx, r, playerID, fmt.Sprintf("fire failed: %v", err))
		return
	}

	// Send result to the attacker.
	_ = r.SendTo(ctx, playerID, ServerMessage{
		Type:     MsgFireResult,
		Coord:    &result.Coord,
		Hit:      &result.Hit,
		SunkShip: result.SunkShip,
	})

	// Notify the defender.
	opponent := r.GetOpponentConn(playerID)
	if opponent != nil {
		_ = opponent.Send(ctx, ServerMessage{
			Type:     MsgOpponentFired,
			Coord:    &result.Coord,
			Hit:      &result.Hit,
			SunkShip: result.SunkShip,
		})
	}

	if result.GameOver {
		if r.Mode == "ai" {
			// AI game: only notify the human.
			youWin := true
			_ = r.SendTo(ctx, playerID, ServerMessage{
				Type:   MsgGameOver,
				Winner: result.Winner,
				YouWin: &youWin,
			})
			h.removeAI(r.Code)
		} else {
			// Human game: notify both players.
			for _, pc := range r.Players {
				if pc != nil {
					youWin := pc.PlayerID == result.Winner
					_ = pc.Send(ctx, ServerMessage{
						Type:   MsgGameOver,
						Winner: result.Winner,
						YouWin: &youWin,
					})
				}
			}
		}
		log.Printf("game over: room=%s winner=%s", r.Code, result.Winner)
	} else if r.Mode == "ai" {
		// AI game: it's now the AI's turn. Fire back.
		isYourTurn := false
		_ = r.SendTo(ctx, playerID, ServerMessage{
			Type:     MsgTurnUpdate,
			YourTurn: &isYourTurn,
		})
		go h.aiTakeTurn(ctx, r, playerID)
	} else {
		// Human game: send turn update to both players.
		turnPlayerID := r.Game.GetTurnPlayerID()
		for _, pc := range r.Players {
			if pc != nil {
				isYourTurn := pc.PlayerID == turnPlayerID
				_ = pc.Send(ctx, ServerMessage{
					Type:     MsgTurnUpdate,
					YourTurn: &isYourTurn,
				})
			}
		}
	}
}

// --- AI Support ---

// setupAIPlayer adds an AI player to the game, places its ships, and transitions
// the game to the placement phase (the human still needs to place their ships).
func (h *Handler) setupAIPlayer(ctx context.Context, r *room.Room, humanPlayerID string) {
	cfg := r.Game.Config
	aiPlayerID := "ai-opponent"

	// Add AI as second player (transitions game to PhasePlacing).
	if err := r.Game.AddPlayer(aiPlayerID); err != nil {
		log.Printf("failed to add AI player: %v", err)
		return
	}

	r.AIPlayerID = aiPlayerID

	// Create AI and place its ships.
	aiInstance := ai.New(cfg)
	h.setAI(r.Code, aiInstance)

	placements := aiInstance.PlaceShips()
	for _, p := range placements {
		if err := r.Game.PlaceShip(aiPlayerID, p.Ship.Name, p.Start, p.Orient); err != nil {
			log.Printf("AI failed to place ship %s: %v", p.Ship.Name, err)
			return
		}
	}

	// Tell the human player the game has started (AI is ready).
	_ = r.SendTo(ctx, humanPlayerID, ServerMessage{
		Type:    MsgGameStart,
		Message: "AI opponent is ready. Place your ships!",
	})

	log.Printf("AI setup complete: room=%s", r.Code)
}

// aiTakeTurn executes the AI's turn: choose a shot, fire, notify the human,
// then check for game over.
func (h *Handler) aiTakeTurn(ctx context.Context, r *room.Room, humanPlayerID string) {
	aiInstance := h.getAI(r.Code)
	if aiInstance == nil {
		return
	}

	// Small delay so the human can see the turn switch.
	time.Sleep(500 * time.Millisecond)

	target := aiInstance.ChooseShot()

	result, err := r.Game.Fire(r.AIPlayerID, target)
	if err != nil {
		log.Printf("AI fire error: %v", err)
		return
	}

	// Update AI knowledge.
	aiInstance.RecordResult(target, result.Hit, result.SunkShip)

	// Notify human that AI fired at their board.
	_ = r.SendTo(ctx, humanPlayerID, ServerMessage{
		Type:     MsgOpponentFired,
		Coord:    &result.Coord,
		Hit:      &result.Hit,
		SunkShip: result.SunkShip,
	})

	if result.GameOver {
		youWin := false
		_ = r.SendTo(ctx, humanPlayerID, ServerMessage{
			Type:   MsgGameOver,
			Winner: r.AIPlayerID,
			YouWin: &youWin,
		})
		h.removeAI(r.Code)
		log.Printf("game over (AI wins): room=%s", r.Code)
	} else {
		// It's the human's turn again.
		isYourTurn := true
		_ = r.SendTo(ctx, humanPlayerID, ServerMessage{
			Type:     MsgTurnUpdate,
			YourTurn: &isYourTurn,
		})
	}
}

// --- Helpers ---

func sendError(ctx context.Context, conn *websocket.Conn, message string) {
	data, _ := json.Marshal(ServerMessage{
		Type:    MsgError,
		Message: message,
	})
	_ = conn.Write(ctx, websocket.MessageText, data)
}

func sendErrorTo(ctx context.Context, r *room.Room, playerID string, message string) {
	_ = r.SendTo(ctx, playerID, ServerMessage{
		Type:    MsgError,
		Message: message,
	})
}

func makeShipInfo(ships []game.ShipConfig) []ShipInfo {
	info := make([]ShipInfo, len(ships))
	for i, s := range ships {
		info[i] = ShipInfo{Name: s.Name, Length: s.Length}
	}
	return info
}

func generatePlayerID() string {
	return fmt.Sprintf("player-%s", generateShortID())
}

func generateShortID() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}
