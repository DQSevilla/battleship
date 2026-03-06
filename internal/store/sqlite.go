package store

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"github.com/DQSevilla/battleship/internal/game"
)

// Store handles persistence of game state and history.
type Store struct {
	db *sqlx.DB
}

// GameRecord represents a completed or active game in the database.
type GameRecord struct {
	ID          string     `db:"id" json:"id"`
	RoomCode    string     `db:"room_code" json:"room_code"`
	Mode        string     `db:"mode" json:"mode"`
	ConfigJSON  string     `db:"config_json" json:"-"`
	StateJSON   string     `db:"state_json" json:"-"`
	Phase       string     `db:"phase" json:"phase"`
	Player1ID   string     `db:"player1_id" json:"player1_id"`
	Player2ID   string     `db:"player2_id" json:"player2_id"`
	Winner      string     `db:"winner" json:"winner"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
	CompletedAt *time.Time `db:"completed_at" json:"completed_at,omitempty"`
}

// MoveRecord represents a single move in the database.
type MoveRecord struct {
	ID        int64     `db:"id" json:"id"`
	GameID    string    `db:"game_id" json:"game_id"`
	PlayerID  string    `db:"player_id" json:"player_id"`
	MoveType  string    `db:"move_type" json:"move_type"` // "place" or "fire"
	DataJSON  string    `db:"data_json" json:"-"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// PlaceMoveData is the JSON payload for a placement move.
type PlaceMoveData struct {
	ShipName    string           `json:"ship_name"`
	Start       game.Coord       `json:"start"`
	Orientation game.Orientation `json:"orientation"`
}

// FireMoveData is the JSON payload for a fire move.
type FireMoveData struct {
	Target   game.Coord `json:"target"`
	Hit      bool       `json:"hit"`
	SunkShip string     `json:"sunk_ship,omitempty"`
	GameOver bool       `json:"game_over"`
}

// GameDetail is a GameRecord with parsed JSON fields for API responses.
type GameDetail struct {
	GameRecord
	Config json.RawMessage `json:"config"`
}

// ToDetail converts a GameRecord to a GameDetail with parsed JSON.
func (r *GameRecord) ToDetail() GameDetail {
	cfg := json.RawMessage(r.ConfigJSON)
	if len(cfg) == 0 {
		cfg = json.RawMessage("{}")
	}
	return GameDetail{
		GameRecord: *r,
		Config:     cfg,
	}
}

// MoveDetail is a MoveRecord with parsed JSON data for API responses.
type MoveDetail struct {
	MoveRecord
	Data json.RawMessage `json:"data"`
}

// ToDetail converts a MoveRecord to a MoveDetail with parsed JSON.
func (m *MoveRecord) ToDetail() MoveDetail {
	data := json.RawMessage(m.DataJSON)
	if len(data) == 0 {
		data = json.RawMessage("{}")
	}
	return MoveDetail{
		MoveRecord: *m,
		Data:       data,
	}
}

// New opens or creates a SQLite database and initializes the schema.
func New(dbPath string) (*Store, error) {
	db, err := sqlx.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// SQLite pragmas for performance.
	db.MustExec(`
		PRAGMA journal_mode=WAL;
		PRAGMA synchronous=NORMAL;
		PRAGMA foreign_keys=ON;
	`)

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func migrate(db *sqlx.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS games (
		id TEXT PRIMARY KEY,
		room_code TEXT NOT NULL,
		mode TEXT NOT NULL DEFAULT 'human',
		config_json TEXT NOT NULL DEFAULT '{}',
		state_json TEXT NOT NULL DEFAULT '{}',
		phase TEXT NOT NULL DEFAULT 'waiting',
		player1_id TEXT NOT NULL DEFAULT '',
		player2_id TEXT NOT NULL DEFAULT '',
		winner TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS moves (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		game_id TEXT NOT NULL REFERENCES games(id),
		player_id TEXT NOT NULL,
		move_type TEXT NOT NULL,
		data_json TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_moves_game_id ON moves(game_id);
	CREATE INDEX IF NOT EXISTS idx_games_room_code ON games(room_code);
	`
	_, err := db.Exec(schema)
	return err
}

// SaveGame saves or updates a game record.
func (s *Store) SaveGame(rec GameRecord) error {
	_, err := s.db.NamedExec(`
		INSERT INTO games (id, room_code, mode, config_json, state_json, phase, player1_id, player2_id, winner, created_at, updated_at, completed_at)
		VALUES (:id, :room_code, :mode, :config_json, :state_json, :phase, :player1_id, :player2_id, :winner, :created_at, :updated_at, :completed_at)
		ON CONFLICT(id) DO UPDATE SET
			state_json = :state_json,
			phase = :phase,
			player1_id = :player1_id,
			player2_id = :player2_id,
			winner = :winner,
			updated_at = :updated_at,
			completed_at = :completed_at
	`, rec)
	return err
}

// GetGame retrieves a game record by ID.
func (s *Store) GetGame(gameID string) (*GameRecord, error) {
	var rec GameRecord
	err := s.db.Get(&rec, "SELECT * FROM games WHERE id = ?", gameID)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// GetGameByRoom retrieves a game record by room code.
func (s *Store) GetGameByRoom(roomCode string) (*GameRecord, error) {
	var rec GameRecord
	err := s.db.Get(&rec, "SELECT * FROM games WHERE room_code = ? ORDER BY created_at DESC LIMIT 1", roomCode)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// SaveMove records a move in the database.
func (s *Store) SaveMove(move MoveRecord) error {
	_, err := s.db.NamedExec(`
		INSERT INTO moves (game_id, player_id, move_type, data_json, created_at)
		VALUES (:game_id, :player_id, :move_type, :data_json, :created_at)
	`, move)
	return err
}

// GetMoves retrieves all moves for a game, ordered by creation time.
func (s *Store) GetMoves(gameID string) ([]MoveRecord, error) {
	var moves []MoveRecord
	err := s.db.Select(&moves, "SELECT * FROM moves WHERE game_id = ? ORDER BY created_at ASC", gameID)
	return moves, err
}

// ListCompletedGames returns completed games, most recent first.
func (s *Store) ListCompletedGames(limit int) ([]GameRecord, error) {
	var games []GameRecord
	err := s.db.Select(&games, "SELECT * FROM games WHERE phase = 'finished' ORDER BY completed_at DESC LIMIT ?", limit)
	return games, err
}

// ListActiveGames returns games that are not yet finished (for server restart restoration).
func (s *Store) ListActiveGames() ([]GameRecord, error) {
	var games []GameRecord
	err := s.db.Select(&games, "SELECT * FROM games WHERE phase != 'finished' ORDER BY created_at ASC")
	return games, err
}

// RestoreGame reconstructs a live *game.Game from a GameRecord.
func RestoreGame(rec *GameRecord) (*game.Game, error) {
	var cfg game.GameConfig
	if err := json.Unmarshal([]byte(rec.ConfigJSON), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	type stateSnapshot struct {
		Turn    int             `json:"turn"`
		Players [2]*game.Player `json:"players"`
	}
	var snap stateSnapshot
	if err := json.Unmarshal([]byte(rec.StateJSON), &snap); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}

	// Rebuild FiredAt maps from Shots.
	for _, p := range snap.Players {
		if p != nil {
			p.RebuildFiredAt()
		}
	}

	phase := game.PhaseWaiting
	switch rec.Phase {
	case "placing":
		phase = game.PhasePlacing
	case "firing":
		phase = game.PhaseFiring
	case "finished":
		phase = game.PhaseFinished
	}

	g := game.RestoreGame(rec.ID, cfg, phase, snap.Players, snap.Turn, rec.Winner, rec.CreatedAt, rec.UpdatedAt)
	return g, nil
}

// --- Helper to build game records from live state ---

func PhaseToString(p game.GamePhase) string {
	switch p {
	case game.PhaseWaiting:
		return "waiting"
	case game.PhasePlacing:
		return "placing"
	case game.PhaseFiring:
		return "firing"
	case game.PhaseFinished:
		return "finished"
	default:
		return "unknown"
	}
}

// BuildGameRecord creates a GameRecord from a live game.
// Uses Game.Snapshot() for thread-safe access to game fields.
func BuildGameRecord(g *game.Game, roomCode, mode string) GameRecord {
	snap := g.Snapshot()

	cfgJSON, _ := json.Marshal(snap.Config)

	// Build a serializable state snapshot.
	type stateSnapshot struct {
		Turn    int             `json:"turn"`
		Players [2]*game.Player `json:"players"`
	}
	ss := stateSnapshot{Turn: snap.Turn, Players: snap.Players}
	stateJSON, _ := json.Marshal(ss)

	p1 := ""
	p2 := ""
	if snap.Players[0] != nil {
		p1 = snap.Players[0].ID
	}
	if snap.Players[1] != nil {
		p2 = snap.Players[1].ID
	}

	rec := GameRecord{
		ID:         snap.ID,
		RoomCode:   roomCode,
		Mode:       mode,
		ConfigJSON: string(cfgJSON),
		StateJSON:  string(stateJSON),
		Phase:      PhaseToString(snap.Phase),
		Player1ID:  p1,
		Player2ID:  p2,
		Winner:     snap.Winner,
		CreatedAt:  snap.CreatedAt,
		UpdatedAt:  snap.UpdatedAt,
	}

	if snap.Phase == game.PhaseFinished {
		now := time.Now()
		rec.CompletedAt = &now
	}

	return rec
}
