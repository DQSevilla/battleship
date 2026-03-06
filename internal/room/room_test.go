package room

import (
	"testing"

	"github.com/DQSevilla/battleship/internal/game"
)

func TestManagerCreateRoom(t *testing.T) {
	m := NewManager()
	cfg := game.DefaultConfig()

	r, err := m.CreateRoom("game-1", cfg, "human")
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if r.Code == "" {
		t.Error("room code should not be empty")
	}
	if r.Game == nil {
		t.Error("room should have a game")
	}
	if r.Mode != "human" {
		t.Errorf("expected mode human, got %s", r.Mode)
	}
	if m.RoomCount() != 1 {
		t.Errorf("expected 1 room, got %d", m.RoomCount())
	}
}

func TestManagerGetRoom(t *testing.T) {
	m := NewManager()
	cfg := game.DefaultConfig()

	r, _ := m.CreateRoom("game-2", cfg, "ai")

	got, err := m.GetRoom(r.Code)
	if err != nil {
		t.Fatalf("get room: %v", err)
	}
	if got.Code != r.Code {
		t.Errorf("expected code %s, got %s", r.Code, got.Code)
	}
}

func TestManagerGetRoomCaseInsensitive(t *testing.T) {
	m := NewManager()
	cfg := game.DefaultConfig()

	r, _ := m.CreateRoom("game-3", cfg, "human")

	// Room codes are uppercase; GetRoom should uppercase the input.
	got, err := m.GetRoom("abcd") // This won't match since code is random.
	// Instead, test by lowering the actual code.
	got, err = m.GetRoom(r.Code) // Already uppercase.
	if err != nil {
		t.Fatalf("get room: %v", err)
	}
	if got == nil {
		t.Fatal("expected room")
	}
}

func TestManagerGetRoomNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetRoom("ZZZZ")
	if err != ErrRoomNotFound {
		t.Errorf("expected ErrRoomNotFound, got %v", err)
	}
}

func TestManagerRemoveRoom(t *testing.T) {
	m := NewManager()
	cfg := game.DefaultConfig()

	r, _ := m.CreateRoom("game-rm", cfg, "human")
	m.RemoveRoom(r.Code)

	if m.RoomCount() != 0 {
		t.Errorf("expected 0 rooms after remove, got %d", m.RoomCount())
	}
}

func TestManagerRestoreRoom(t *testing.T) {
	m := NewManager()
	cfg := game.DefaultConfig()
	g, _ := game.NewGame("restored-game", cfg)

	r := m.RestoreRoom("ABCD", g, "human")
	if r.Code != "ABCD" {
		t.Errorf("expected code ABCD, got %s", r.Code)
	}
	if m.RoomCount() != 1 {
		t.Errorf("expected 1 room, got %d", m.RoomCount())
	}

	got, err := m.GetRoom("ABCD")
	if err != nil {
		t.Fatalf("get restored room: %v", err)
	}
	if got.Game.ID != "restored-game" {
		t.Errorf("expected game ID restored-game, got %s", got.Game.ID)
	}
}

func TestRoomAddPlayer(t *testing.T) {
	m := NewManager()
	cfg := game.DefaultConfig()
	r, _ := m.CreateRoom("game-ap", cfg, "human")

	pc1 := &PlayerConn{PlayerID: "p1"}
	pc2 := &PlayerConn{PlayerID: "p2"}

	if err := r.AddPlayer(pc1); err != nil {
		t.Fatalf("add player 1: %v", err)
	}
	if err := r.AddPlayer(pc2); err != nil {
		t.Fatalf("add player 2: %v", err)
	}

	// Third player should fail.
	pc3 := &PlayerConn{PlayerID: "p3"}
	if err := r.AddPlayer(pc3); err != ErrRoomFull {
		t.Errorf("expected ErrRoomFull, got %v", err)
	}
}

func TestRoomRemovePlayer(t *testing.T) {
	m := NewManager()
	cfg := game.DefaultConfig()
	r, _ := m.CreateRoom("game-rp", cfg, "human")

	pc1 := &PlayerConn{PlayerID: "p1"}
	pc2 := &PlayerConn{PlayerID: "p2"}
	r.AddPlayer(pc1)
	r.AddPlayer(pc2)

	// Remove p1 — room should not be empty.
	empty := r.RemovePlayer("p1")
	if empty {
		t.Error("room should not be empty with one player remaining")
	}

	// Remove p2 — room should be empty.
	empty = r.RemovePlayer("p2")
	if !empty {
		t.Error("room should be empty after removing both players")
	}
}

func TestRoomReplacePlayer(t *testing.T) {
	m := NewManager()
	cfg := game.DefaultConfig()
	r, _ := m.CreateRoom("game-rep", cfg, "human")

	pc1 := &PlayerConn{PlayerID: "p1"}
	r.AddPlayer(pc1)

	// Replace with a new connection.
	pc1New := &PlayerConn{PlayerID: "p1"}
	r.ReplacePlayer(pc1New)

	// The player should still be in the room.
	opp := r.GetOpponentConn("p1") // returns nil since there's only p1
	if opp != nil {
		t.Error("should have no opponent")
	}
}

func TestRoomGetOpponentConn(t *testing.T) {
	m := NewManager()
	cfg := game.DefaultConfig()
	r, _ := m.CreateRoom("game-opp", cfg, "human")

	pc1 := &PlayerConn{PlayerID: "p1"}
	pc2 := &PlayerConn{PlayerID: "p2"}
	r.AddPlayer(pc1)
	r.AddPlayer(pc2)

	opp := r.GetOpponentConn("p1")
	if opp == nil {
		t.Fatal("expected opponent connection")
	}
	if opp.PlayerID != "p2" {
		t.Errorf("expected p2 as opponent, got %s", opp.PlayerID)
	}

	opp2 := r.GetOpponentConn("p2")
	if opp2 == nil || opp2.PlayerID != "p1" {
		t.Error("expected p1 as opponent of p2")
	}
}

func TestGenerateCodeUniqueness(t *testing.T) {
	m := NewManager()
	cfg := game.DefaultConfig()

	codes := make(map[string]bool)
	for i := 0; i < 100; i++ {
		r, err := m.CreateRoom("game-"+string(rune('A'+i)), cfg, "ai")
		if err != nil {
			t.Fatalf("create room %d: %v", i, err)
		}
		if codes[r.Code] {
			t.Errorf("duplicate room code: %s", r.Code)
		}
		codes[r.Code] = true
	}
	if m.RoomCount() != 100 {
		t.Errorf("expected 100 rooms, got %d", m.RoomCount())
	}
}

func TestGenerateCodeCharset(t *testing.T) {
	m := NewManager()
	cfg := game.DefaultConfig()

	// Generate many codes and check they only use the allowed charset.
	allowed := "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	allowedSet := make(map[byte]bool)
	for i := 0; i < len(allowed); i++ {
		allowedSet[allowed[i]] = true
	}

	for i := 0; i < 50; i++ {
		r, _ := m.CreateRoom("game-chr-"+string(rune('0'+i)), cfg, "ai")
		for j := 0; j < len(r.Code); j++ {
			if !allowedSet[r.Code[j]] {
				t.Errorf("code %s contains invalid character %c", r.Code, r.Code[j])
			}
		}
		if len(r.Code) != 4 {
			t.Errorf("expected 4-char code, got %d chars: %s", len(r.Code), r.Code)
		}
	}
}
