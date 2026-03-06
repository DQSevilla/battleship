package game

// ShipConfig defines a type of ship with a name and length.
type ShipConfig struct {
	Name   string `json:"name"`
	Length int    `json:"length"`
}

// GameConfig holds all configurable parameters for a game.
type GameConfig struct {
	BoardSize int          `json:"board_size"`
	Ships     []ShipConfig `json:"ships"`
}

// DefaultConfig returns the classic Battleship configuration:
// 10x10 board with Carrier(5), Battleship(4), Cruiser(3), Submarine(3), Destroyer(2).
func DefaultConfig() GameConfig {
	return GameConfig{
		BoardSize: 10,
		Ships: []ShipConfig{
			{Name: "Carrier", Length: 5},
			{Name: "Battleship", Length: 4},
			{Name: "Cruiser", Length: 3},
			{Name: "Submarine", Length: 3},
			{Name: "Destroyer", Length: 2},
		},
	}
}

// Validate checks that the config is playable.
func (c GameConfig) Validate() error {
	if c.BoardSize < 2 {
		return ErrBoardTooSmall
	}
	if len(c.Ships) == 0 {
		return ErrNoShips
	}
	// Check that all ships fit on the board.
	for _, s := range c.Ships {
		if s.Length < 1 {
			return ErrInvalidShipLength
		}
		if s.Length > c.BoardSize {
			return ErrShipTooLong
		}
	}
	// Check that total ship cells don't exceed board area.
	total := 0
	for _, s := range c.Ships {
		total += s.Length
	}
	if total > c.BoardSize*c.BoardSize {
		return ErrTooManyShipCells
	}
	return nil
}
