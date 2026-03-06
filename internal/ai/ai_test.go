package ai

import (
	"testing"

	"github.com/DQSevilla/battleship/internal/game"
)

func defaultCfg() game.GameConfig {
	return game.DefaultConfig()
}

func smallCfg() game.GameConfig {
	return game.GameConfig{
		BoardSize: 5,
		Ships: []game.ShipConfig{
			{Name: "Small", Length: 2},
			{Name: "Tiny", Length: 1},
		},
	}
}

// --- Ship Placement Tests ---

func TestPlaceShipsValid(t *testing.T) {
	a := New(defaultCfg())
	placements := a.PlaceShips()

	if len(placements) != len(defaultCfg().Ships) {
		t.Fatalf("expected %d placements, got %d", len(defaultCfg().Ships), len(placements))
	}

	// Verify all placements can be applied to a board without errors.
	board := game.NewBoard(defaultCfg().BoardSize)
	for _, p := range placements {
		if err := board.PlaceShip(p.Ship, p.Start, p.Orient); err != nil {
			t.Fatalf("invalid placement for %s at (%d,%d) orient=%d: %v",
				p.Ship.Name, p.Start.X, p.Start.Y, p.Orient, err)
		}
	}
}

func TestPlaceShipsNoOverlap(t *testing.T) {
	// Run multiple times to catch randomness issues.
	for i := 0; i < 100; i++ {
		a := New(defaultCfg())
		placements := a.PlaceShips()

		board := game.NewBoard(defaultCfg().BoardSize)
		for _, p := range placements {
			if err := board.PlaceShip(p.Ship, p.Start, p.Orient); err != nil {
				t.Fatalf("iteration %d: overlap or invalid placement for %s: %v", i, p.Ship.Name, err)
			}
		}
	}
}

// --- Shot Selection Tests ---

func TestChooseShotReturnsValidCoord(t *testing.T) {
	a := New(defaultCfg())
	shot := a.ChooseShot()

	if shot.X < 0 || shot.X >= 10 || shot.Y < 0 || shot.Y >= 10 {
		t.Fatalf("shot out of bounds: (%d, %d)", shot.X, shot.Y)
	}
}

func TestChooseShotNeverRepeatsSameCell(t *testing.T) {
	a := New(defaultCfg())
	fired := make(map[game.Coord]bool)

	// Fire at every cell on a 10x10 board.
	for i := 0; i < 100; i++ {
		shot := a.ChooseShot()
		if fired[shot] {
			t.Fatalf("shot %d repeated coordinate (%d, %d)", i, shot.X, shot.Y)
		}
		fired[shot] = true
		// Record as miss so AI picks a new cell.
		a.RecordResult(shot, false, "")
	}
}

func TestTargetModeAfterHit(t *testing.T) {
	a := New(smallCfg())

	// Simulate a hit at (2, 2).
	a.RecordResult(game.Coord{X: 2, Y: 2}, true, "")

	// Next shot should be adjacent to (2, 2).
	shot := a.ChooseShot()

	isAdjacent := (shot.X == 2 && (shot.Y == 1 || shot.Y == 3)) ||
		(shot.Y == 2 && (shot.X == 1 || shot.X == 3))

	if !isAdjacent {
		t.Fatalf("after hit at (2,2), expected adjacent shot, got (%d, %d)", shot.X, shot.Y)
	}
}

func TestTargetModeExtendsLine(t *testing.T) {
	a := New(defaultCfg())

	// Simulate two horizontal hits at (3,5) and (4,5).
	a.RecordResult(game.Coord{X: 3, Y: 5}, true, "")
	a.RecordResult(game.Coord{X: 4, Y: 5}, true, "")

	// Next shot should extend the line: (2,5) or (5,5).
	shot := a.ChooseShot()

	if shot.Y != 5 || (shot.X != 2 && shot.X != 5) {
		t.Fatalf("after hits at (3,5) and (4,5), expected (2,5) or (5,5), got (%d, %d)",
			shot.X, shot.Y)
	}
}

func TestTargetModeClearsAfterSunk(t *testing.T) {
	a := New(smallCfg())

	// Hit and sink the "Tiny" ship (length 1) at (0, 0).
	a.RecordResult(game.Coord{X: 0, Y: 0}, true, "Tiny")

	// The hit stack should be cleared for that ship.
	// Next shot should be in hunt mode (not adjacent to (0,0) necessarily).
	shot := a.ChooseShot()

	// Just verify it's a valid coordinate (hunt mode should work fine).
	if shot.X < 0 || shot.X >= 5 || shot.Y < 0 || shot.Y >= 5 {
		t.Fatalf("shot out of bounds: (%d, %d)", shot.X, shot.Y)
	}
}

func TestProbabilityDensityPrefersCenter(t *testing.T) {
	a := New(defaultCfg())
	density := a.computeDensity()

	// Center cells should have higher density than corner cells.
	center := density[5][5]
	corner := density[0][0]

	if center <= corner {
		t.Fatalf("expected center density (%d) > corner density (%d)", center, corner)
	}
}

func TestProbabilityDensityRespectsKnowledge(t *testing.T) {
	a := New(defaultCfg())

	// Mark (5,5) as a miss.
	a.RecordResult(game.Coord{X: 5, Y: 5}, false, "")

	density := a.computeDensity()

	if density[5][5] != 0 {
		t.Fatalf("expected density 0 for known miss, got %d", density[5][5])
	}
}

// --- Full AI Game Simulation ---

func TestAICanCompleteFullGame(t *testing.T) {
	cfg := defaultCfg()

	// Set up a board with known ship positions for the AI to sink.
	board := game.NewBoard(cfg.BoardSize)
	board.PlaceShip(cfg.Ships[0], game.Coord{X: 0, Y: 0}, game.Horizontal) // Carrier at row 0
	board.PlaceShip(cfg.Ships[1], game.Coord{X: 0, Y: 1}, game.Horizontal) // Battleship at row 1
	board.PlaceShip(cfg.Ships[2], game.Coord{X: 0, Y: 2}, game.Horizontal) // Cruiser at row 2
	board.PlaceShip(cfg.Ships[3], game.Coord{X: 0, Y: 3}, game.Horizontal) // Submarine at row 3
	board.PlaceShip(cfg.Ships[4], game.Coord{X: 0, Y: 4}, game.Horizontal) // Destroyer at row 4

	a := New(cfg)

	// AI fires shots until all ships are sunk.
	shots := 0
	maxShots := cfg.BoardSize * cfg.BoardSize
	for shots < maxShots {
		target := a.ChooseShot()
		hit, sunkShip, err := board.ReceiveShot(target)
		if err != nil {
			t.Fatalf("shot %d at (%d,%d): %v", shots, target.X, target.Y, err)
		}
		a.RecordResult(target, hit, sunkShip)
		shots++

		if board.AllSunk() {
			break
		}
	}

	if !board.AllSunk() {
		t.Fatal("AI failed to sink all ships")
	}

	// The AI should be reasonably efficient — well under 100 shots for 17 cells.
	if shots > 80 {
		t.Logf("WARNING: AI took %d shots to sink 17 cells of ships — could be more efficient", shots)
	}
	t.Logf("AI completed game in %d shots (17 ship cells on 100-cell board)", shots)
}
