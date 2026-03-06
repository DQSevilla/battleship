package ws

import "github.com/DQSevilla/battleship/internal/game"

// Client-to-server message types.
const (
	MsgCreateGame = "create_game"
	MsgJoinGame   = "join_game"
	MsgPlaceShip  = "place_ship"
	MsgFire       = "fire"
)

// Server-to-client message types.
const (
	MsgError         = "error"
	MsgGameCreated   = "game_created"
	MsgGameJoined    = "game_joined"
	MsgGameStart     = "game_start"
	MsgPlaceResult   = "place_result"
	MsgAllPlaced     = "all_placed"
	MsgFireResult    = "fire_result"
	MsgOpponentFired = "opponent_fired"
	MsgGameOver      = "game_over"
	MsgTurnUpdate    = "turn_update"
	MsgOpponentReady = "opponent_ready"
	MsgOpponentLeft  = "opponent_left"
)

// ClientMessage is the envelope for all client-to-server messages.
type ClientMessage struct {
	Type string `json:"type"`

	// create_game fields
	Mode string `json:"mode,omitempty"` // "ai" or "human"

	// join_game fields
	RoomCode string `json:"room_code,omitempty"`

	// place_ship fields
	ShipName    string           `json:"ship_name,omitempty"`
	Start       *game.Coord      `json:"start,omitempty"`
	Orientation game.Orientation `json:"orientation,omitempty"`

	// fire fields
	Target *game.Coord `json:"target,omitempty"`
}

// ServerMessage is the envelope for all server-to-client messages.
type ServerMessage struct {
	Type string `json:"type"`

	// Error info
	Message string `json:"message,omitempty"`

	// game_created / game_joined
	RoomCode string `json:"room_code,omitempty"`
	PlayerID string `json:"player_id,omitempty"`
	GameID   string `json:"game_id,omitempty"`

	// game_start / all_placed
	Config   *game.GameConfig `json:"config,omitempty"`
	YourTurn *bool            `json:"your_turn,omitempty"`
	Ships    []ShipInfo       `json:"ships,omitempty"`

	// place_result
	ShipName    string            `json:"ship_name,omitempty"`
	Start       *game.Coord       `json:"start,omitempty"`
	Orientation *game.Orientation `json:"orientation,omitempty"`
	Remaining   []string          `json:"remaining,omitempty"`

	// fire_result / opponent_fired
	Coord    *game.Coord `json:"coord,omitempty"`
	Hit      *bool       `json:"hit,omitempty"`
	SunkShip string      `json:"sunk_ship,omitempty"`

	// game_over
	Winner string `json:"winner,omitempty"`
	YouWin *bool  `json:"you_win,omitempty"`
}

// ShipInfo is a summary of a ship for the client (used in game state sync).
type ShipInfo struct {
	Name   string `json:"name"`
	Length int    `json:"length"`
}
