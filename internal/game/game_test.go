package game

import (
	"testing"
)

// --- Config Tests ---

func TestDefaultConfigIsValid(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should be valid: %v", err)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     GameConfig
		wantErr error
	}{
		{
			name:    "board too small",
			cfg:     GameConfig{BoardSize: 1, Ships: []ShipConfig{{Name: "A", Length: 1}}},
			wantErr: ErrBoardTooSmall,
		},
		{
			name:    "no ships",
			cfg:     GameConfig{BoardSize: 5, Ships: []ShipConfig{}},
			wantErr: ErrNoShips,
		},
		{
			name:    "invalid ship length",
			cfg:     GameConfig{BoardSize: 5, Ships: []ShipConfig{{Name: "A", Length: 0}}},
			wantErr: ErrInvalidShipLength,
		},
		{
			name:    "ship too long",
			cfg:     GameConfig{BoardSize: 3, Ships: []ShipConfig{{Name: "A", Length: 4}}},
			wantErr: ErrShipTooLong,
		},
		{
			name: "too many ship cells",
			cfg: GameConfig{BoardSize: 2, Ships: []ShipConfig{
				{Name: "A", Length: 2},
				{Name: "B", Length: 2},
				{Name: "C", Length: 2},
			}},
			wantErr: ErrTooManyShipCells,
		},
		{
			name:    "valid small config",
			cfg:     GameConfig{BoardSize: 5, Ships: []ShipConfig{{Name: "A", Length: 3}}},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if err != tt.wantErr {
				t.Errorf("got error %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// --- Board Tests ---

func TestBoardPlaceShipValid(t *testing.T) {
	b := NewBoard(10)
	err := b.PlaceShip(ShipConfig{Name: "Destroyer", Length: 2}, Coord{0, 0}, Horizontal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(b.Ships) != 1 {
		t.Fatalf("expected 1 ship, got %d", len(b.Ships))
	}
	// Check cells are marked.
	if b.Grid[0][0] != Ship || b.Grid[0][1] != Ship {
		t.Error("expected ship cells to be marked")
	}
}

func TestBoardPlaceShipVertical(t *testing.T) {
	b := NewBoard(10)
	err := b.PlaceShip(ShipConfig{Name: "Cruiser", Length: 3}, Coord{5, 2}, Vertical)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Grid[2][5] != Ship || b.Grid[3][5] != Ship || b.Grid[4][5] != Ship {
		t.Error("expected vertical ship cells to be marked")
	}
}

func TestBoardPlaceShipOutOfBounds(t *testing.T) {
	b := NewBoard(10)
	err := b.PlaceShip(ShipConfig{Name: "Carrier", Length: 5}, Coord{8, 0}, Horizontal)
	if err != ErrOutOfBounds {
		t.Errorf("expected ErrOutOfBounds, got %v", err)
	}
}

func TestBoardPlaceShipOutOfBoundsVertical(t *testing.T) {
	b := NewBoard(10)
	err := b.PlaceShip(ShipConfig{Name: "Carrier", Length: 5}, Coord{0, 8}, Vertical)
	if err != ErrOutOfBounds {
		t.Errorf("expected ErrOutOfBounds, got %v", err)
	}
}

func TestBoardPlaceShipOverlap(t *testing.T) {
	b := NewBoard(10)
	_ = b.PlaceShip(ShipConfig{Name: "A", Length: 3}, Coord{0, 0}, Horizontal)
	err := b.PlaceShip(ShipConfig{Name: "B", Length: 3}, Coord{1, 0}, Horizontal)
	if err != ErrOverlap {
		t.Errorf("expected ErrOverlap, got %v", err)
	}
}

func TestBoardReceiveShotMiss(t *testing.T) {
	b := NewBoard(10)
	_ = b.PlaceShip(ShipConfig{Name: "A", Length: 2}, Coord{0, 0}, Horizontal)

	hit, sunk, err := b.ReceiveShot(Coord{5, 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hit {
		t.Error("expected miss")
	}
	if sunk != "" {
		t.Error("expected no sunk ship")
	}
	if b.Grid[5][5] != Miss {
		t.Error("expected cell to be marked Miss")
	}
}

func TestBoardReceiveShotHit(t *testing.T) {
	b := NewBoard(10)
	_ = b.PlaceShip(ShipConfig{Name: "Destroyer", Length: 2}, Coord{3, 4}, Horizontal)

	hit, sunk, err := b.ReceiveShot(Coord{3, 4})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hit {
		t.Error("expected hit")
	}
	if sunk != "" {
		t.Error("should not be sunk yet")
	}
	if b.Grid[4][3] != Hit {
		t.Error("expected cell to be marked Hit")
	}
}

func TestBoardReceiveShotSunk(t *testing.T) {
	b := NewBoard(10)
	_ = b.PlaceShip(ShipConfig{Name: "Destroyer", Length: 2}, Coord{0, 0}, Horizontal)

	b.ReceiveShot(Coord{0, 0})
	hit, sunk, err := b.ReceiveShot(Coord{1, 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hit {
		t.Error("expected hit")
	}
	if sunk != "Destroyer" {
		t.Errorf("expected Destroyer to be sunk, got %q", sunk)
	}
}

func TestBoardReceiveShotDuplicate(t *testing.T) {
	b := NewBoard(10)
	b.ReceiveShot(Coord{0, 0})
	_, _, err := b.ReceiveShot(Coord{0, 0})
	if err != ErrAlreadyFired {
		t.Errorf("expected ErrAlreadyFired, got %v", err)
	}
}

func TestBoardReceiveShotOutOfBounds(t *testing.T) {
	b := NewBoard(10)
	_, _, err := b.ReceiveShot(Coord{-1, 0})
	if err != ErrInvalidCoord {
		t.Errorf("expected ErrInvalidCoord, got %v", err)
	}
	_, _, err = b.ReceiveShot(Coord{10, 0})
	if err != ErrInvalidCoord {
		t.Errorf("expected ErrInvalidCoord, got %v", err)
	}
}

func TestBoardAllSunk(t *testing.T) {
	b := NewBoard(10)
	_ = b.PlaceShip(ShipConfig{Name: "A", Length: 2}, Coord{0, 0}, Horizontal)
	_ = b.PlaceShip(ShipConfig{Name: "B", Length: 1}, Coord{5, 5}, Horizontal)

	if b.AllSunk() {
		t.Error("should not be all sunk yet")
	}

	b.ReceiveShot(Coord{0, 0})
	b.ReceiveShot(Coord{1, 0})
	if b.AllSunk() {
		t.Error("ship B not sunk yet")
	}

	b.ReceiveShot(Coord{5, 5})
	if !b.AllSunk() {
		t.Error("all ships should be sunk")
	}
}

// --- Player Tests ---

func TestPlayerPlaceShip(t *testing.T) {
	cfg := GameConfig{BoardSize: 10, Ships: []ShipConfig{{Name: "Destroyer", Length: 2}}}
	p := NewPlayer("p1", cfg)

	if p.AllPlaced() {
		t.Error("should not be all placed yet")
	}

	err := p.PlaceShip("Destroyer", Coord{0, 0}, Horizontal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !p.AllPlaced() {
		t.Error("should be all placed now")
	}
}

func TestPlayerPlaceShipDuplicate(t *testing.T) {
	cfg := GameConfig{BoardSize: 10, Ships: []ShipConfig{{Name: "Destroyer", Length: 2}}}
	p := NewPlayer("p1", cfg)

	_ = p.PlaceShip("Destroyer", Coord{0, 0}, Horizontal)
	err := p.PlaceShip("Destroyer", Coord{3, 3}, Horizontal)
	if err != ErrAlreadyPlaced {
		t.Errorf("expected ErrAlreadyPlaced, got %v", err)
	}
}

func TestPlayerPlaceUnknownShip(t *testing.T) {
	cfg := GameConfig{BoardSize: 10, Ships: []ShipConfig{{Name: "Destroyer", Length: 2}}}
	p := NewPlayer("p1", cfg)

	err := p.PlaceShip("Carrier", Coord{0, 0}, Horizontal)
	if err != ErrUnknownShip {
		t.Errorf("expected ErrUnknownShip, got %v", err)
	}
}

// --- Full Game Flow Tests ---

func smallConfig() GameConfig {
	return GameConfig{
		BoardSize: 5,
		Ships:     []ShipConfig{{Name: "Ship", Length: 2}},
	}
}

func TestGameFullFlow(t *testing.T) {
	g, err := NewGame("test-1", smallConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if g.Phase != PhaseWaiting {
		t.Fatalf("expected PhaseWaiting, got %v", g.Phase)
	}

	// Add two players.
	if err := g.AddPlayer("alice"); err != nil {
		t.Fatalf("add alice: %v", err)
	}
	if g.Phase != PhaseWaiting {
		t.Fatalf("expected PhaseWaiting after first player, got %v", g.Phase)
	}

	if err := g.AddPlayer("bob"); err != nil {
		t.Fatalf("add bob: %v", err)
	}
	if g.Phase != PhasePlacing {
		t.Fatalf("expected PhasePlacing, got %v", g.Phase)
	}

	// Third player should be rejected.
	if err := g.AddPlayer("charlie"); err != ErrGameFull {
		t.Fatalf("expected ErrGameFull, got %v", err)
	}

	// Place ships.
	if err := g.PlaceShip("alice", "Ship", Coord{0, 0}, Horizontal); err != nil {
		t.Fatalf("alice place: %v", err)
	}
	if g.Phase != PhasePlacing {
		t.Fatalf("should still be placing until both done")
	}

	if err := g.PlaceShip("bob", "Ship", Coord{0, 0}, Horizontal); err != nil {
		t.Fatalf("bob place: %v", err)
	}
	if g.Phase != PhaseFiring {
		t.Fatalf("expected PhaseFiring, got %v", g.Phase)
	}

	// Alice fires first (Turn 0).
	if g.GetTurnPlayerID() != "alice" {
		t.Fatalf("expected alice's turn, got %s", g.GetTurnPlayerID())
	}

	// Bob can't fire out of turn.
	_, err = g.Fire("bob", Coord{0, 0})
	if err != ErrNotYourTurn {
		t.Fatalf("expected ErrNotYourTurn, got %v", err)
	}

	// Alice hits.
	res, err := g.Fire("alice", Coord{0, 0})
	if err != nil {
		t.Fatalf("alice fire: %v", err)
	}
	if !res.Hit {
		t.Error("expected hit")
	}
	if res.GameOver {
		t.Error("game should not be over yet")
	}

	// Now it's bob's turn.
	if g.GetTurnPlayerID() != "bob" {
		t.Fatalf("expected bob's turn")
	}

	// Bob misses.
	res, err = g.Fire("bob", Coord{4, 4})
	if err != nil {
		t.Fatalf("bob fire: %v", err)
	}
	if res.Hit {
		t.Error("expected miss")
	}

	// Alice sinks bob's ship.
	res, err = g.Fire("alice", Coord{1, 0})
	if err != nil {
		t.Fatalf("alice fire 2: %v", err)
	}
	if !res.Hit {
		t.Error("expected hit")
	}
	if res.SunkShip != "Ship" {
		t.Errorf("expected Ship to be sunk, got %q", res.SunkShip)
	}
	if !res.GameOver {
		t.Error("game should be over")
	}
	if res.Winner != "alice" {
		t.Errorf("expected alice to win, got %q", res.Winner)
	}
	if g.GetPhase() != PhaseFinished {
		t.Error("game should be in finished phase")
	}
}

func TestFireAfterGameOver(t *testing.T) {
	g, _ := NewGame("test-2", smallConfig())
	g.AddPlayer("a")
	g.AddPlayer("b")
	g.PlaceShip("a", "Ship", Coord{0, 0}, Horizontal)
	g.PlaceShip("b", "Ship", Coord{0, 0}, Horizontal)

	// a sinks b's ship.
	g.Fire("a", Coord{0, 0})
	g.Fire("b", Coord{4, 4}) // b misses
	g.Fire("a", Coord{1, 0}) // a sinks

	_, err := g.Fire("b", Coord{0, 0})
	if err != ErrGameFinished {
		t.Errorf("expected ErrGameFinished, got %v", err)
	}
}

func TestPlaceShipWrongPhase(t *testing.T) {
	g, _ := NewGame("test-3", smallConfig())
	g.AddPlayer("a")
	g.AddPlayer("b")
	g.PlaceShip("a", "Ship", Coord{0, 0}, Horizontal)
	g.PlaceShip("b", "Ship", Coord{0, 0}, Horizontal)

	// Now in firing phase.
	err := g.PlaceShip("a", "Ship", Coord{2, 2}, Horizontal)
	if err != ErrGameNotPlacing {
		t.Errorf("expected ErrGameNotPlacing, got %v", err)
	}
}

func TestFireWrongPhase(t *testing.T) {
	g, _ := NewGame("test-4", smallConfig())
	g.AddPlayer("a")
	g.AddPlayer("b")

	// Still in placement phase.
	_, err := g.Fire("a", Coord{0, 0})
	if err != ErrGameNotFiring {
		t.Errorf("expected ErrGameNotFiring, got %v", err)
	}
}

func TestFireDuplicateShot(t *testing.T) {
	g, _ := NewGame("test-5", smallConfig())
	g.AddPlayer("a")
	g.AddPlayer("b")
	g.PlaceShip("a", "Ship", Coord{0, 0}, Horizontal)
	g.PlaceShip("b", "Ship", Coord{0, 0}, Horizontal)

	g.Fire("a", Coord{3, 3})
	g.Fire("b", Coord{3, 3})

	_, err := g.Fire("a", Coord{3, 3})
	if err != ErrAlreadyFired {
		t.Errorf("expected ErrAlreadyFired, got %v", err)
	}
}

func TestConfigurableBoardSize(t *testing.T) {
	cfg := GameConfig{
		BoardSize: 20,
		Ships: []ShipConfig{
			{Name: "Mega", Length: 10},
			{Name: "Big", Length: 7},
			{Name: "Small", Length: 3},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config: %v", err)
	}

	g, err := NewGame("big-board", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g.AddPlayer("a")
	g.AddPlayer("b")

	// Place a 10-length ship at (10, 0) horizontal — should fit on 20x20 board.
	err = g.PlaceShip("a", "Mega", Coord{10, 0}, Horizontal)
	if err != nil {
		t.Fatalf("expected valid placement on 20x20 board: %v", err)
	}

	// Place at edge — should fail.
	err = g.PlaceShip("b", "Mega", Coord{15, 0}, Horizontal)
	if err != ErrOutOfBounds {
		t.Errorf("expected ErrOutOfBounds at edge of 20x20 board, got %v", err)
	}
}
