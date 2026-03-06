package game

import "errors"

// Config validation errors.
var (
	ErrBoardTooSmall     = errors.New("board size must be at least 2")
	ErrNoShips           = errors.New("at least one ship is required")
	ErrInvalidShipLength = errors.New("ship length must be at least 1")
	ErrShipTooLong       = errors.New("ship length exceeds board size")
	ErrTooManyShipCells  = errors.New("total ship cells exceed board area")
)

// Placement errors.
var (
	ErrOutOfBounds   = errors.New("ship placement out of bounds")
	ErrOverlap       = errors.New("ship overlaps with existing ship")
	ErrAlreadyPlaced = errors.New("ship already placed")
	ErrUnknownShip   = errors.New("unknown ship name")
	ErrPlacementDone = errors.New("all ships already placed")
)

// Firing errors.
var (
	ErrNotYourTurn    = errors.New("not your turn")
	ErrAlreadyFired   = errors.New("already fired at this coordinate")
	ErrInvalidCoord   = errors.New("coordinate out of bounds")
	ErrGameNotFiring  = errors.New("game is not in firing phase")
	ErrGameNotPlacing = errors.New("game is not in placement phase")
	ErrGameFinished   = errors.New("game is already finished")
)

// Game state errors.
var (
	ErrGameFull       = errors.New("game already has two players")
	ErrPlayerNotFound = errors.New("player not in this game")
)
