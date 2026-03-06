package game

import (
	"sync"
	"time"
)

// GamePhase represents the current phase of the game.
type GamePhase int

const (
	PhaseWaiting  GamePhase = iota // Waiting for second player
	PhasePlacing                   // Both players placing ships
	PhaseFiring                    // Players taking turns firing
	PhaseFinished                  // Game over
)

// FireResult is returned from a Fire call with all relevant info.
type FireResult struct {
	Coord    Coord  `json:"coord"`
	Hit      bool   `json:"hit"`
	SunkShip string `json:"sunk_ship,omitempty"`
	GameOver bool   `json:"game_over"`
	Winner   string `json:"winner,omitempty"`
}

// Game is the central game state machine.
type Game struct {
	mu sync.Mutex

	ID        string     `json:"id"`
	Config    GameConfig `json:"config"`
	Phase     GamePhase  `json:"phase"`
	Players   [2]*Player `json:"players"`
	Turn      int        `json:"turn"` // 0 or 1, index into Players
	Winner    string     `json:"winner,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// NewGame creates a new game in the waiting phase.
func NewGame(id string, cfg GameConfig) (*Game, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	now := time.Now()
	return &Game{
		ID:        id,
		Config:    cfg,
		Phase:     PhaseWaiting,
		Turn:      0,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// AddPlayer adds a player to the game. The first player triggers
// the waiting state; the second moves the game to placement.
func (g *Game) AddPlayer(playerID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Players[0] == nil {
		g.Players[0] = NewPlayer(playerID, g.Config)
		g.UpdatedAt = time.Now()
		return nil
	}
	if g.Players[1] == nil {
		g.Players[1] = NewPlayer(playerID, g.Config)
		g.Phase = PhasePlacing
		g.UpdatedAt = time.Now()
		return nil
	}
	return ErrGameFull
}

// PlayerIndex returns the index (0 or 1) for a given player ID, or an error.
func (g *Game) PlayerIndex(playerID string) (int, error) {
	for i, p := range g.Players {
		if p != nil && p.ID == playerID {
			return i, nil
		}
	}
	return -1, ErrPlayerNotFound
}

// PlaceShip places a ship for the given player.
func (g *Game) PlaceShip(playerID string, shipName string, start Coord, orient Orientation) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Phase != PhasePlacing {
		return ErrGameNotPlacing
	}

	idx, err := g.PlayerIndex(playerID)
	if err != nil {
		return err
	}

	player := g.Players[idx]
	if player.AllPlaced() {
		return ErrPlacementDone
	}

	if err := player.PlaceShip(shipName, start, orient); err != nil {
		return err
	}

	// If both players have placed all ships, move to firing phase.
	if g.Players[0].AllPlaced() && g.Players[1].AllPlaced() {
		g.Phase = PhaseFiring
		g.Turn = 0
	}

	g.UpdatedAt = time.Now()
	return nil
}

// PlayerReady returns true if the given player has placed all ships.
func (g *Game) PlayerReady(playerID string) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	idx, err := g.PlayerIndex(playerID)
	if err != nil {
		return false, err
	}
	return g.Players[idx].AllPlaced(), nil
}

// BothReady returns true if both players have placed all ships.
func (g *Game) BothReady() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.Players[0] != nil && g.Players[1] != nil &&
		g.Players[0].AllPlaced() && g.Players[1].AllPlaced()
}

// Fire processes a shot from the given player at the given coordinate.
func (g *Game) Fire(playerID string, target Coord) (FireResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Phase == PhaseFinished {
		return FireResult{}, ErrGameFinished
	}
	if g.Phase != PhaseFiring {
		return FireResult{}, ErrGameNotFiring
	}

	idx, err := g.PlayerIndex(playerID)
	if err != nil {
		return FireResult{}, err
	}

	if idx != g.Turn {
		return FireResult{}, ErrNotYourTurn
	}

	attacker := g.Players[idx]
	defender := g.Players[1-idx]

	// Check if already fired here.
	if attacker.HasFiredAt(target) {
		return FireResult{}, ErrAlreadyFired
	}

	hit, sunkShip, err := defender.Board.ReceiveShot(target)
	if err != nil {
		return FireResult{}, err
	}

	attacker.RecordShot(target, hit, sunkShip)

	result := FireResult{
		Coord:    target,
		Hit:      hit,
		SunkShip: sunkShip,
	}

	// Check for win condition.
	if defender.Board.AllSunk() {
		g.Phase = PhaseFinished
		g.Winner = playerID
		result.GameOver = true
		result.Winner = playerID
	} else {
		// Switch turns.
		g.Turn = 1 - g.Turn
	}

	g.UpdatedAt = time.Now()
	return result, nil
}

// GetPhase returns the current game phase (thread-safe).
func (g *Game) GetPhase() GamePhase {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.Phase
}

// GetTurnPlayerID returns the ID of the player whose turn it is.
func (g *Game) GetTurnPlayerID() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Phase != PhaseFiring {
		return ""
	}
	return g.Players[g.Turn].ID
}

// GetPlayer returns the player with the given ID, or nil.
func (g *Game) GetPlayer(playerID string) *Player {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, p := range g.Players {
		if p != nil && p.ID == playerID {
			return p
		}
	}
	return nil
}

// GetOpponent returns the opponent of the given player ID, or nil.
func (g *Game) GetOpponent(playerID string) *Player {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i, p := range g.Players {
		if p != nil && p.ID == playerID {
			return g.Players[1-i]
		}
	}
	return nil
}
