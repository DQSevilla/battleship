package game

// ShotRecord records a single shot fired by a player.
type ShotRecord struct {
	Coord    Coord  `json:"coord"`
	Hit      bool   `json:"hit"`
	SunkShip string `json:"sunk_ship,omitempty"`
}

// Player holds all state for one player in a game.
type Player struct {
	ID    string `json:"id"`
	Board *Board `json:"board"`

	// Ships the player still needs to place (by name).
	Unplaced map[string]ShipConfig `json:"unplaced"`

	// Record of shots this player has fired at the opponent.
	Shots []ShotRecord `json:"shots"`

	// Quick lookup: coordinates this player has already fired at.
	FiredAt map[Coord]bool `json:"-"`
}

// NewPlayer creates a new player with an empty board and all ships unplaced.
func NewPlayer(id string, cfg GameConfig) *Player {
	unplaced := make(map[string]ShipConfig, len(cfg.Ships))
	for _, s := range cfg.Ships {
		unplaced[s.Name] = s
	}
	return &Player{
		ID:       id,
		Board:    NewBoard(cfg.BoardSize),
		Unplaced: unplaced,
		Shots:    make([]ShotRecord, 0),
		FiredAt:  make(map[Coord]bool),
	}
}

// PlaceShip places a ship on this player's board.
func (p *Player) PlaceShip(shipName string, start Coord, orient Orientation) error {
	cfg, ok := p.Unplaced[shipName]
	if !ok {
		// Check if it was already placed.
		for _, s := range p.Board.Ships {
			if s.Config.Name == shipName {
				return ErrAlreadyPlaced
			}
		}
		return ErrUnknownShip
	}

	if err := p.Board.PlaceShip(cfg, start, orient); err != nil {
		return err
	}

	delete(p.Unplaced, shipName)
	return nil
}

// AllPlaced returns true if the player has placed all their ships.
func (p *Player) AllPlaced() bool {
	return len(p.Unplaced) == 0
}

// RecordShot records a shot fired by this player.
func (p *Player) RecordShot(c Coord, hit bool, sunkShip string) {
	p.Shots = append(p.Shots, ShotRecord{
		Coord:    c,
		Hit:      hit,
		SunkShip: sunkShip,
	})
	p.FiredAt[c] = true
}

// HasFiredAt returns true if the player already fired at this coordinate.
func (p *Player) HasFiredAt(c Coord) bool {
	return p.FiredAt[c]
}
