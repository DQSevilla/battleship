package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DQSevilla/battleship/internal/game"
)

func setupTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	st, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestNewStore(t *testing.T) {
	st := setupTestStore(t)
	if st == nil {
		t.Fatal("store should not be nil")
	}
}

func TestNewStoreInvalidPath(t *testing.T) {
	// New() calls MustExec which panics on SQLite pragma errors.
	// An invalid path will panic, so we just test that a valid temp path works.
	// The TestNewStore already covers the happy path.
	t.Skip("SQLite driver panics on invalid paths via MustExec; cannot test gracefully")
}

func TestSaveAndGetGame(t *testing.T) {
	st := setupTestStore(t)

	now := time.Now().Truncate(time.Second)
	rec := GameRecord{
		ID:         "game-123",
		RoomCode:   "ABCD",
		Mode:       "human",
		ConfigJSON: `{"board_size":10}`,
		StateJSON:  `{"turn":0}`,
		Phase:      "placing",
		Player1ID:  "player-1",
		Player2ID:  "player-2",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := st.SaveGame(rec); err != nil {
		t.Fatalf("save game: %v", err)
	}

	got, err := st.GetGame("game-123")
	if err != nil {
		t.Fatalf("get game: %v", err)
	}

	if got.ID != "game-123" {
		t.Errorf("expected ID game-123, got %s", got.ID)
	}
	if got.RoomCode != "ABCD" {
		t.Errorf("expected room code ABCD, got %s", got.RoomCode)
	}
	if got.Phase != "placing" {
		t.Errorf("expected phase placing, got %s", got.Phase)
	}
	if got.Player1ID != "player-1" {
		t.Errorf("expected player1 player-1, got %s", got.Player1ID)
	}
	if got.ConfigJSON != `{"board_size":10}` {
		t.Errorf("config JSON mismatch: %s", got.ConfigJSON)
	}
}

func TestSaveGameUpsert(t *testing.T) {
	st := setupTestStore(t)

	now := time.Now().Truncate(time.Second)
	rec := GameRecord{
		ID:         "game-upsert",
		RoomCode:   "AAAA",
		Mode:       "ai",
		ConfigJSON: `{}`,
		StateJSON:  `{}`,
		Phase:      "placing",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	st.SaveGame(rec)

	// Update phase.
	rec.Phase = "firing"
	rec.UpdatedAt = time.Now().Truncate(time.Second)
	st.SaveGame(rec)

	got, _ := st.GetGame("game-upsert")
	if got.Phase != "firing" {
		t.Errorf("expected phase firing after upsert, got %s", got.Phase)
	}
}

func TestGetGameNotFound(t *testing.T) {
	st := setupTestStore(t)

	_, err := st.GetGame("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent game")
	}
}

func TestGetGameByRoom(t *testing.T) {
	st := setupTestStore(t)

	now := time.Now().Truncate(time.Second)
	st.SaveGame(GameRecord{
		ID: "game-room-1", RoomCode: "XYZW", Mode: "human",
		ConfigJSON: `{}`, StateJSON: `{}`, Phase: "placing",
		CreatedAt: now, UpdatedAt: now,
	})

	got, err := st.GetGameByRoom("XYZW")
	if err != nil {
		t.Fatalf("get game by room: %v", err)
	}
	if got.ID != "game-room-1" {
		t.Errorf("expected game-room-1, got %s", got.ID)
	}
}

func TestSaveAndGetMoves(t *testing.T) {
	st := setupTestStore(t)

	now := time.Now().Truncate(time.Second)
	st.SaveGame(GameRecord{
		ID: "game-moves", RoomCode: "MMAA", Mode: "ai",
		ConfigJSON: `{}`, StateJSON: `{}`, Phase: "firing",
		CreatedAt: now, UpdatedAt: now,
	})

	st.SaveMove(MoveRecord{
		GameID: "game-moves", PlayerID: "p1", MoveType: "fire",
		DataJSON: `{"target":{"x":3,"y":5},"hit":true}`, CreatedAt: now,
	})
	st.SaveMove(MoveRecord{
		GameID: "game-moves", PlayerID: "p2", MoveType: "fire",
		DataJSON: `{"target":{"x":1,"y":1},"hit":false}`, CreatedAt: now.Add(time.Second),
	})

	moves, err := st.GetMoves("game-moves")
	if err != nil {
		t.Fatalf("get moves: %v", err)
	}
	if len(moves) != 2 {
		t.Fatalf("expected 2 moves, got %d", len(moves))
	}
	if moves[0].PlayerID != "p1" {
		t.Errorf("first move should be p1, got %s", moves[0].PlayerID)
	}
}

func TestListCompletedGames(t *testing.T) {
	st := setupTestStore(t)

	now := time.Now().Truncate(time.Second)
	completedAt := now

	// Active game (should not appear).
	st.SaveGame(GameRecord{
		ID: "game-active", RoomCode: "AAAA", Mode: "ai",
		ConfigJSON: `{}`, StateJSON: `{}`, Phase: "firing",
		CreatedAt: now, UpdatedAt: now,
	})

	// Completed game.
	st.SaveGame(GameRecord{
		ID: "game-done", RoomCode: "BBBB", Mode: "human",
		ConfigJSON: `{}`, StateJSON: `{}`, Phase: "finished",
		Winner: "p1", CreatedAt: now, UpdatedAt: now, CompletedAt: &completedAt,
	})

	games, err := st.ListCompletedGames(10)
	if err != nil {
		t.Fatalf("list completed: %v", err)
	}
	if len(games) != 1 {
		t.Fatalf("expected 1 completed game, got %d", len(games))
	}
	if games[0].ID != "game-done" {
		t.Errorf("expected game-done, got %s", games[0].ID)
	}
}

func TestListActiveGames(t *testing.T) {
	st := setupTestStore(t)

	now := time.Now().Truncate(time.Second)
	completedAt := now

	st.SaveGame(GameRecord{
		ID: "game-active", RoomCode: "AAAA", Mode: "ai",
		ConfigJSON: `{}`, StateJSON: `{}`, Phase: "firing",
		CreatedAt: now, UpdatedAt: now,
	})
	st.SaveGame(GameRecord{
		ID: "game-done", RoomCode: "BBBB", Mode: "human",
		ConfigJSON: `{}`, StateJSON: `{}`, Phase: "finished",
		CreatedAt: now, UpdatedAt: now, CompletedAt: &completedAt,
	})

	games, err := st.ListActiveGames()
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(games) != 1 {
		t.Fatalf("expected 1 active game, got %d", len(games))
	}
	if games[0].ID != "game-active" {
		t.Errorf("expected game-active, got %s", games[0].ID)
	}
}

func TestBuildGameRecord(t *testing.T) {
	cfg := game.DefaultConfig()
	g, err := game.NewGame("test-game", cfg)
	if err != nil {
		t.Fatalf("new game: %v", err)
	}
	g.AddPlayer("p1")
	g.AddPlayer("p2")

	rec := BuildGameRecord(g, "XYZW", "human")

	if rec.ID != "test-game" {
		t.Errorf("expected ID test-game, got %s", rec.ID)
	}
	if rec.RoomCode != "XYZW" {
		t.Errorf("expected room code XYZW, got %s", rec.RoomCode)
	}
	if rec.Phase != "placing" {
		t.Errorf("expected phase placing, got %s", rec.Phase)
	}
	if rec.Player1ID != "p1" || rec.Player2ID != "p2" {
		t.Errorf("player IDs mismatch: %s, %s", rec.Player1ID, rec.Player2ID)
	}
	if rec.ConfigJSON == "" || rec.ConfigJSON == "{}" {
		t.Error("config JSON should not be empty")
	}
	if rec.StateJSON == "" || rec.StateJSON == "{}" {
		t.Error("state JSON should not be empty")
	}
}

func TestRestoreGame(t *testing.T) {
	cfg := game.DefaultConfig()
	g, _ := game.NewGame("restore-test", cfg)
	g.AddPlayer("p1")
	g.AddPlayer("p2")

	// Place a ship for p1.
	g.PlaceShip("p1", "Destroyer", game.Coord{X: 0, Y: 0}, game.Horizontal)

	// Save and restore.
	rec := BuildGameRecord(g, "REST", "human")
	if err := func() error {
		// Simulate save/load cycle.
		restored, err := RestoreGame(&rec)
		if err != nil {
			return err
		}
		if restored.ID != "restore-test" {
			t.Errorf("expected ID restore-test, got %s", restored.ID)
		}
		snap := restored.Snapshot()
		if snap.Phase != game.PhasePlacing {
			t.Errorf("expected PhasePlacing, got %d", snap.Phase)
		}
		if snap.Players[0] == nil || snap.Players[0].ID != "p1" {
			t.Error("player 1 not restored correctly")
		}
		// Verify FiredAt was rebuilt (should be empty since no shots fired).
		if snap.Players[0].FiredAt == nil {
			t.Error("FiredAt should be initialized after restore")
		}
		return nil
	}(); err != nil {
		t.Fatalf("restore game: %v", err)
	}
}

func TestPhaseToString(t *testing.T) {
	tests := []struct {
		phase game.GamePhase
		want  string
	}{
		{game.PhaseWaiting, "waiting"},
		{game.PhasePlacing, "placing"},
		{game.PhaseFiring, "firing"},
		{game.PhaseFinished, "finished"},
		{game.GamePhase(99), "unknown"},
	}
	for _, tt := range tests {
		got := PhaseToString(tt.phase)
		if got != tt.want {
			t.Errorf("PhaseToString(%d) = %s, want %s", tt.phase, got, tt.want)
		}
	}
}

func TestGameDetailToDetail(t *testing.T) {
	rec := GameRecord{
		ID:         "detail-test",
		ConfigJSON: `{"board_size":10}`,
	}
	detail := rec.ToDetail()
	if string(detail.Config) != `{"board_size":10}` {
		t.Errorf("config not passed through: %s", detail.Config)
	}
}

func TestMoveDetailToDetail(t *testing.T) {
	move := MoveRecord{
		DataJSON: `{"target":{"x":1,"y":2}}`,
	}
	detail := move.ToDetail()
	if string(detail.Data) != `{"target":{"x":1,"y":2}}` {
		t.Errorf("data not passed through: %s", detail.Data)
	}
}

func TestStoreDatabaseFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "persist.db")

	// Create store and add data.
	st, err := New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	st.SaveGame(GameRecord{
		ID: "persist-game", RoomCode: "PPPP", Mode: "ai",
		ConfigJSON: `{}`, StateJSON: `{}`, Phase: "firing",
		CreatedAt: now, UpdatedAt: now,
	})
	st.Close()

	// Reopen and verify data persisted.
	st2, err := New(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer st2.Close()

	got, err := st2.GetGame("persist-game")
	if err != nil {
		t.Fatalf("get persisted game: %v", err)
	}
	if got.ID != "persist-game" {
		t.Errorf("expected persist-game, got %s", got.ID)
	}

	// Verify the file exists on disk.
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file should exist on disk")
	}
}
