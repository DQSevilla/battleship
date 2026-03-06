package ai

import (
	"math/rand"
	"sort"

	"github.com/DQSevilla/battleship/internal/game"
)

// AI implements a smart Battleship opponent using hunt/target mode
// with probability density for shot selection.
type AI struct {
	cfg       game.GameConfig
	boardSize int

	// What the AI knows about the opponent's board.
	// Empty = unknown, Miss = confirmed miss, Hit = confirmed hit.
	knowledge [][]game.CellState

	// Ships the AI hasn't sunk yet (tracks remaining lengths for probability).
	remainingShips []game.ShipConfig

	// Target mode state: unsunk hit coordinates to investigate.
	hitStack []game.Coord
}

// New creates a new AI for the given game config.
func New(cfg game.GameConfig) *AI {
	knowledge := make([][]game.CellState, cfg.BoardSize)
	for i := range knowledge {
		knowledge[i] = make([]game.CellState, cfg.BoardSize)
	}

	remaining := make([]game.ShipConfig, len(cfg.Ships))
	copy(remaining, cfg.Ships)

	return &AI{
		cfg:            cfg,
		boardSize:      cfg.BoardSize,
		knowledge:      knowledge,
		remainingShips: remaining,
		hitStack:       make([]game.Coord, 0),
	}
}

// PlaceShips randomly places all ships on a board and returns the placements.
// Each placement is (ShipConfig, start Coord, Orientation).
func (ai *AI) PlaceShips() []ShipPlacement {
	return ai.placeShipsWithRetry(0)
}

const maxPlacementRetries = 100

func (ai *AI) placeShipsWithRetry(depth int) []ShipPlacement {
	if depth >= maxPlacementRetries {
		// Should never happen with valid configs. Return nil to signal failure.
		return nil
	}

	placements := make([]ShipPlacement, 0, len(ai.cfg.Ships))

	// Sort ships by length descending — place largest first for fewer retries.
	ships := make([]game.ShipConfig, len(ai.cfg.Ships))
	copy(ships, ai.cfg.Ships)
	sort.Slice(ships, func(i, j int) bool {
		return ships[i].Length > ships[j].Length
	})

	board := game.NewBoard(ai.cfg.BoardSize)

	for _, ship := range ships {
		placed := false
		for attempts := 0; attempts < 1000; attempts++ {
			orient := game.Orientation(rand.Intn(2))
			var maxX, maxY int
			if orient == game.Horizontal {
				maxX = ai.boardSize - ship.Length
				maxY = ai.boardSize - 1
			} else {
				maxX = ai.boardSize - 1
				maxY = ai.boardSize - ship.Length
			}

			start := game.Coord{
				X: rand.Intn(maxX + 1),
				Y: rand.Intn(maxY + 1),
			}

			if err := board.PlaceShip(ship, start, orient); err == nil {
				placements = append(placements, ShipPlacement{
					Ship:   ship,
					Start:  start,
					Orient: orient,
				})
				placed = true
				break
			}
		}
		if !placed {
			// Extremely unlikely with valid configs, retry with depth limit.
			return ai.placeShipsWithRetry(depth + 1)
		}
	}

	return placements
}

// ChooseShot picks the best coordinate to fire at.
func (ai *AI) ChooseShot() game.Coord {
	// Target mode: if we have unsunk hits, probe adjacent cells.
	if len(ai.hitStack) > 0 {
		if target, ok := ai.targetModeShot(); ok {
			return target
		}
	}

	// Hunt mode: use probability density to pick the best shot.
	return ai.huntModeShot()
}

// RecordResult updates the AI's knowledge after a shot.
func (ai *AI) RecordResult(c game.Coord, hit bool, sunkShip string) {
	if hit {
		ai.knowledge[c.Y][c.X] = game.Hit
		ai.hitStack = append(ai.hitStack, c)

		if sunkShip != "" {
			ai.markShipSunk(sunkShip, c)
		}
	} else {
		ai.knowledge[c.Y][c.X] = game.Miss
	}
}

// --- Target Mode ---

// targetModeShot tries to find an adjacent cell to an existing hit.
func (ai *AI) targetModeShot() (game.Coord, bool) {
	// Check if we have a line of hits — if so, extend the line.
	if shot, ok := ai.extendHitLine(); ok {
		return shot, true
	}

	// Otherwise, try adjacent cells around any hit in the stack.
	for i := len(ai.hitStack) - 1; i >= 0; i-- {
		hit := ai.hitStack[i]
		adjacents := []game.Coord{
			{X: hit.X, Y: hit.Y - 1},
			{X: hit.X + 1, Y: hit.Y},
			{X: hit.X, Y: hit.Y + 1},
			{X: hit.X - 1, Y: hit.Y},
		}
		for _, adj := range adjacents {
			if ai.isValidTarget(adj) {
				return adj, true
			}
		}
	}

	// All adjacent cells explored, clear the stack and fall through to hunt.
	ai.hitStack = ai.hitStack[:0]
	return game.Coord{}, false
}

// extendHitLine looks for collinear hits and tries to extend the line.
func (ai *AI) extendHitLine() (game.Coord, bool) {
	if len(ai.hitStack) < 2 {
		return game.Coord{}, false
	}

	// Check if any two hits share an axis — find the line direction.
	// Group hits by X and Y to find lines.
	byX := make(map[int][]game.Coord)
	byY := make(map[int][]game.Coord)
	for _, h := range ai.hitStack {
		byX[h.X] = append(byX[h.X], h)
		byY[h.Y] = append(byY[h.Y], h)
	}

	// Try vertical lines (same X, different Y).
	for _, hits := range byX {
		if len(hits) < 2 {
			continue
		}
		sort.Slice(hits, func(i, j int) bool { return hits[i].Y < hits[j].Y })
		// Try extending up.
		top := game.Coord{X: hits[0].X, Y: hits[0].Y - 1}
		if ai.isValidTarget(top) {
			return top, true
		}
		// Try extending down.
		bottom := game.Coord{X: hits[len(hits)-1].X, Y: hits[len(hits)-1].Y + 1}
		if ai.isValidTarget(bottom) {
			return bottom, true
		}
	}

	// Try horizontal lines (same Y, different X).
	for _, hits := range byY {
		if len(hits) < 2 {
			continue
		}
		sort.Slice(hits, func(i, j int) bool { return hits[i].X < hits[j].X })
		// Try extending left.
		left := game.Coord{X: hits[0].X - 1, Y: hits[0].Y}
		if ai.isValidTarget(left) {
			return left, true
		}
		// Try extending right.
		right := game.Coord{X: hits[len(hits)-1].X + 1, Y: hits[len(hits)-1].Y}
		if ai.isValidTarget(right) {
			return right, true
		}
	}

	return game.Coord{}, false
}

// --- Hunt Mode (Probability Density) ---

// huntModeShot computes a probability density map and picks the highest-scoring cell.
func (ai *AI) huntModeShot() game.Coord {
	density := ai.computeDensity()

	// Find the max density value.
	maxDensity := 0
	for y := 0; y < ai.boardSize; y++ {
		for x := 0; x < ai.boardSize; x++ {
			if density[y][x] > maxDensity {
				maxDensity = density[y][x]
			}
		}
	}

	// Collect all unfired cells with the max density.
	var candidates []game.Coord
	for y := 0; y < ai.boardSize; y++ {
		for x := 0; x < ai.boardSize; x++ {
			if density[y][x] == maxDensity && ai.knowledge[y][x] == game.Empty {
				candidates = append(candidates, game.Coord{X: x, Y: y})
			}
		}
	}

	// Fallback: if no candidates (shouldn't happen with valid remaining ships),
	// pick any unfired cell.
	if len(candidates) == 0 {
		for y := 0; y < ai.boardSize; y++ {
			for x := 0; x < ai.boardSize; x++ {
				if ai.knowledge[y][x] == game.Empty {
					candidates = append(candidates, game.Coord{X: x, Y: y})
				}
			}
		}
	}

	// Pick randomly among the best candidates.
	return candidates[rand.Intn(len(candidates))]
}

// computeDensity builds a probability density map. For each cell, count
// how many ways remaining ships could be placed covering that cell.
func (ai *AI) computeDensity() [][]int {
	density := make([][]int, ai.boardSize)
	for i := range density {
		density[i] = make([]int, ai.boardSize)
	}

	for _, ship := range ai.remainingShips {
		// Try every possible placement for this ship.
		for y := 0; y < ai.boardSize; y++ {
			for x := 0; x < ai.boardSize; x++ {
				for _, orient := range []game.Orientation{game.Horizontal, game.Vertical} {
					if ai.canPlaceShip(ship.Length, x, y, orient) {
						// Increment density for each cell this placement covers.
						for i := 0; i < ship.Length; i++ {
							if orient == game.Horizontal {
								density[y][x+i]++
							} else {
								density[y+i][x]++
							}
						}
					}
				}
			}
		}
	}

	// Zero out cells we already know about (hit or miss).
	for y := 0; y < ai.boardSize; y++ {
		for x := 0; x < ai.boardSize; x++ {
			if ai.knowledge[y][x] != game.Empty {
				density[y][x] = 0
			}
		}
	}

	return density
}

// canPlaceShip checks if a ship of given length could potentially occupy
// cells starting at (x,y) with the given orientation, considering what
// the AI knows. A placement is invalid if any cell is a Miss or out of bounds.
func (ai *AI) canPlaceShip(length, x, y int, orient game.Orientation) bool {
	for i := 0; i < length; i++ {
		cx, cy := x, y
		if orient == game.Horizontal {
			cx = x + i
		} else {
			cy = y + i
		}
		if cx < 0 || cx >= ai.boardSize || cy < 0 || cy >= ai.boardSize {
			return false
		}
		if ai.knowledge[cy][cx] == game.Miss {
			return false
		}
	}
	return true
}

// --- Helpers ---

// isValidTarget returns true if the coordinate is in bounds and hasn't been shot at.
func (ai *AI) isValidTarget(c game.Coord) bool {
	if c.X < 0 || c.X >= ai.boardSize || c.Y < 0 || c.Y >= ai.boardSize {
		return false
	}
	return ai.knowledge[c.Y][c.X] == game.Empty
}

// markShipSunk removes the sunk ship from remainingShips and clears
// its hit coordinates from the hitStack.
func (ai *AI) markShipSunk(shipName string, lastHit game.Coord) {
	// Remove from remaining ships (remove first match only).
	for i, s := range ai.remainingShips {
		if s.Name == shipName {
			ai.remainingShips = append(ai.remainingShips[:i], ai.remainingShips[i+1:]...)
			break
		}
	}

	// Find all contiguous hits connected to lastHit that form this ship,
	// and remove them from the hit stack.
	sunkCoords := ai.findSunkShipCoords(lastHit, shipName)
	newStack := make([]game.Coord, 0, len(ai.hitStack))
	for _, h := range ai.hitStack {
		isSunk := false
		for _, sc := range sunkCoords {
			if h == sc {
				isSunk = true
				break
			}
		}
		if !isSunk {
			newStack = append(newStack, h)
		}
	}
	ai.hitStack = newStack
}

// findSunkShipCoords finds the coordinates of a sunk ship by flood-filling
// from the last hit along hit cells. Returns up to shipLength coords.
func (ai *AI) findSunkShipCoords(start game.Coord, shipName string) []game.Coord {
	// Find the ship length.
	shipLen := 0
	for _, s := range ai.cfg.Ships {
		if s.Name == shipName {
			shipLen = s.Length
			break
		}
	}
	if shipLen == 0 {
		return []game.Coord{start}
	}

	// BFS from start along hit cells.
	visited := map[game.Coord]bool{start: true}
	queue := []game.Coord{start}
	var result []game.Coord

	for len(queue) > 0 && len(result) < shipLen {
		c := queue[0]
		queue = queue[1:]
		result = append(result, c)

		for _, adj := range []game.Coord{
			{X: c.X - 1, Y: c.Y},
			{X: c.X + 1, Y: c.Y},
			{X: c.X, Y: c.Y - 1},
			{X: c.X, Y: c.Y + 1},
		} {
			if adj.X >= 0 && adj.X < ai.boardSize && adj.Y >= 0 && adj.Y < ai.boardSize &&
				!visited[adj] && ai.knowledge[adj.Y][adj.X] == game.Hit {
				visited[adj] = true
				queue = append(queue, adj)
			}
		}
	}

	return result
}

// ShipPlacement represents a single ship placement decision.
type ShipPlacement struct {
	Ship   game.ShipConfig
	Start  game.Coord
	Orient game.Orientation
}
