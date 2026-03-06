package game

import "slices"

// Orientation represents horizontal or vertical ship placement.
type Orientation int

const (
	Horizontal Orientation = iota
	Vertical
)

// Coord represents a position on the board.
type Coord struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// CellState represents the state of a single cell on the board.
type CellState int

const (
	Empty CellState = iota
	Ship
	Miss
	Hit
)

// PlacedShip tracks a ship on the board along with damage state.
type PlacedShip struct {
	Config ShipConfig  `json:"config"`
	Start  Coord       `json:"start"`
	Orient Orientation `json:"orientation"`
	Hits   []bool      `json:"hits"` // true for each segment that's been hit
}

// Coords returns all coordinates occupied by this ship.
func (s *PlacedShip) Coords() []Coord {
	coords := make([]Coord, s.Config.Length)
	for i := 0; i < s.Config.Length; i++ {
		if s.Orient == Horizontal {
			coords[i] = Coord{X: s.Start.X + i, Y: s.Start.Y}
		} else {
			coords[i] = Coord{X: s.Start.X, Y: s.Start.Y + i}
		}
	}
	return coords
}

// IsSunk returns true if every segment has been hit.
func (s *PlacedShip) IsSunk() bool {
	return !slices.Contains(s.Hits, false)
}

// Board represents a player's grid.
type Board struct {
	Size  int           `json:"size"`
	Grid  [][]CellState `json:"grid"`
	Ships []*PlacedShip `json:"ships"`
}

// NewBoard creates an empty board of the given size.
func NewBoard(size int) *Board {
	grid := make([][]CellState, size)
	for i := range grid {
		grid[i] = make([]CellState, size)
	}
	return &Board{
		Size:  size,
		Grid:  grid,
		Ships: make([]*PlacedShip, 0),
	}
}

// PlaceShip places a ship on the board at the given position and orientation.
// Returns an error if the placement is invalid.
func (b *Board) PlaceShip(cfg ShipConfig, start Coord, orient Orientation) error {
	// Compute all coordinates the ship would occupy.
	coords := make([]Coord, cfg.Length)
	for i := 0; i < cfg.Length; i++ {
		if orient == Horizontal {
			coords[i] = Coord{X: start.X + i, Y: start.Y}
		} else {
			coords[i] = Coord{X: start.X, Y: start.Y + i}
		}
	}

	// Bounds check.
	for _, c := range coords {
		if c.X < 0 || c.X >= b.Size || c.Y < 0 || c.Y >= b.Size {
			return ErrOutOfBounds
		}
	}

	// Overlap check.
	for _, c := range coords {
		if b.Grid[c.Y][c.X] != Empty {
			return ErrOverlap
		}
	}

	// Place the ship.
	ship := &PlacedShip{
		Config: cfg,
		Start:  start,
		Orient: orient,
		Hits:   make([]bool, cfg.Length),
	}
	for _, c := range coords {
		b.Grid[c.Y][c.X] = Ship
	}
	b.Ships = append(b.Ships, ship)
	return nil
}

// ReceiveShot processes an incoming shot at the given coordinate.
// Returns the result and, if a ship was sunk, the ship's name.
func (b *Board) ReceiveShot(c Coord) (hit bool, sunkShip string, err error) {
	if c.X < 0 || c.X >= b.Size || c.Y < 0 || c.Y >= b.Size {
		return false, "", ErrInvalidCoord
	}

	switch b.Grid[c.Y][c.X] {
	case Empty:
		b.Grid[c.Y][c.X] = Miss
		return false, "", nil
	case Ship:
		b.Grid[c.Y][c.X] = Hit
		// Find which ship was hit and mark the segment.
		for _, ship := range b.Ships {
			for i, sc := range ship.Coords() {
				if sc.X == c.X && sc.Y == c.Y {
					ship.Hits[i] = true
					if ship.IsSunk() {
						return true, ship.Config.Name, nil
					}
					return true, "", nil
				}
			}
		}
		// Should not reach here if board state is consistent.
		return true, "", nil
	case Miss, Hit:
		return false, "", ErrAlreadyFired
	}

	return false, "", ErrInvalidCoord
}

// AllSunk returns true if every ship on the board has been sunk.
func (b *Board) AllSunk() bool {
	for _, ship := range b.Ships {
		if !ship.IsSunk() {
			return false
		}
	}
	return len(b.Ships) > 0
}
