package ws

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/DQSevilla/battleship/internal/game"
	"github.com/DQSevilla/battleship/internal/room"
)

func setupTestServer(t *testing.T) (*httptest.Server, *room.Manager) {
	t.Helper()
	rooms := room.NewManager()
	handler := NewHandler(rooms)
	server := httptest.NewServer(handler)
	t.Cleanup(func() { server.Close() })
	return server, rooms
}

func connectWS(t *testing.T, server *httptest.Server) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	wsURL := "ws" + server.URL[4:] // http -> ws
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	t.Cleanup(func() { conn.Close(websocket.StatusNormalClosure, "") })
	return conn
}

func sendMsg(t *testing.T, conn *websocket.Conn, msg ClientMessage) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write error: %v", err)
	}
}

func readMsg(t *testing.T, conn *websocket.Conn) ServerMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}

	var msg ServerMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	return msg
}

func TestCreateAndJoinGame(t *testing.T) {
	server, rooms := setupTestServer(t)

	// Player 1 creates a game.
	conn1 := connectWS(t, server)
	sendMsg(t, conn1, ClientMessage{Type: MsgCreateGame, Mode: "human"})

	msg1 := readMsg(t, conn1)
	if msg1.Type != MsgGameCreated {
		t.Fatalf("expected game_created, got %s: %s", msg1.Type, msg1.Message)
	}
	if msg1.RoomCode == "" {
		t.Fatal("expected room code")
	}
	if msg1.PlayerID == "" {
		t.Fatal("expected player ID")
	}

	// Verify room exists.
	if rooms.RoomCount() != 1 {
		t.Fatalf("expected 1 room, got %d", rooms.RoomCount())
	}

	// Player 2 joins the game.
	conn2 := connectWS(t, server)
	sendMsg(t, conn2, ClientMessage{Type: MsgJoinGame, RoomCode: msg1.RoomCode})

	msg2 := readMsg(t, conn2)
	if msg2.Type != MsgGameJoined {
		t.Fatalf("expected game_joined, got %s: %s", msg2.Type, msg2.Message)
	}

	// Both players should receive game_start.
	start2 := readMsg(t, conn2)
	if start2.Type != MsgGameStart {
		t.Fatalf("expected game_start for player 2, got %s", start2.Type)
	}

	start1 := readMsg(t, conn1)
	if start1.Type != MsgGameStart {
		t.Fatalf("expected game_start for player 1, got %s", start1.Type)
	}
}

func TestFullGameOverWebSocket(t *testing.T) {
	server, _ := setupTestServer(t)

	// Player 1 creates.
	conn1 := connectWS(t, server)
	sendMsg(t, conn1, ClientMessage{Type: MsgCreateGame, Mode: "human"})
	created := readMsg(t, conn1)
	if created.Type != MsgGameCreated {
		t.Fatalf("expected game_created, got %s", created.Type)
	}
	p1ID := created.PlayerID
	roomCode := created.RoomCode

	// Player 2 joins.
	conn2 := connectWS(t, server)
	sendMsg(t, conn2, ClientMessage{Type: MsgJoinGame, RoomCode: roomCode})
	joined := readMsg(t, conn2)
	if joined.Type != MsgGameJoined {
		t.Fatalf("expected game_joined, got %s", joined.Type)
	}
	p2ID := joined.PlayerID

	// Consume game_start messages.
	_ = readMsg(t, conn2) // game_start
	_ = readMsg(t, conn1) // game_start

	// Both players place their single ship (using default config, but we'll
	// place all 5 ships to be correct).
	ships := created.Ships

	// Player 1 places all ships horizontally, one per row starting at column 0.
	for i, ship := range ships {
		sendMsg(t, conn1, ClientMessage{
			Type:        MsgPlaceShip,
			ShipName:    ship.Name,
			Start:       &game.Coord{X: 0, Y: i},
			Orientation: game.Horizontal,
		})
		placeResult := readMsg(t, conn1)
		if placeResult.Type != MsgPlaceResult {
			t.Fatalf("p1 expected place_result for %s, got %s: %s", ship.Name, placeResult.Type, placeResult.Message)
		}
	}

	// Player 2 places all ships horizontally, one per row starting at column 0.
	// Note: P2 may receive an opponent_ready message interleaved with place_results
	// since P1 already finished placing. We handle this by reading flexibly.
	for i, ship := range ships {
		sendMsg(t, conn2, ClientMessage{
			Type:        MsgPlaceShip,
			ShipName:    ship.Name,
			Start:       &game.Coord{X: 0, Y: i},
			Orientation: game.Horizontal,
		})
		// Read messages until we get a place_result, consuming opponent_ready if it arrives.
		for {
			msg := readMsg(t, conn2)
			if msg.Type == MsgOpponentReady {
				continue // consume and keep reading
			}
			if msg.Type != MsgPlaceResult {
				t.Fatalf("p2 expected place_result for %s, got %s: %s", ship.Name, msg.Type, msg.Message)
			}
			break
		}
	}

	// Both should get all_placed. Either may also get opponent_ready first.
	readAllPlaced := func(conn *websocket.Conn, label string) ServerMessage {
		for {
			m := readMsg(t, conn)
			if m.Type == MsgOpponentReady {
				continue
			}
			if m.Type != MsgAllPlaced {
				t.Fatalf("expected all_placed for %s, got %s: %s", label, m.Type, m.Message)
			}
			return m
		}
	}

	allPlaced1 := readAllPlaced(conn1, "p1")
	_ = readAllPlaced(conn2, "p2")

	// Player 1 should have first turn.
	if allPlaced1.YourTurn == nil || !*allPlaced1.YourTurn {
		t.Fatal("player 1 should have first turn")
	}

	// Build list of all ship cell coordinates.
	// Ships placed horizontally: row i has ship[i] from (0,i) to (length-1, i).
	_ = p2ID
	type target struct{ x, y int }
	var targets []target
	for i, ship := range ships {
		for x := 0; x < ship.Length; x++ {
			targets = append(targets, target{x, i})
		}
	}

	p2MissIdx := 0 // P2 fires misses at column 9, incrementing rows
	for shotIdx, tgt := range targets {
		isLast := shotIdx == len(targets)-1

		// Player 1 fires.
		sendMsg(t, conn1, ClientMessage{
			Type:   MsgFire,
			Target: &game.Coord{X: tgt.x, Y: tgt.y},
		})

		// P1 gets fire_result.
		fireResult := readMsg(t, conn1)
		if fireResult.Type != MsgFireResult {
			t.Fatalf("shot %d: expected fire_result, got %s: %s", shotIdx, fireResult.Type, fireResult.Message)
		}
		if fireResult.Hit == nil || !*fireResult.Hit {
			t.Fatalf("shot %d: expected hit at (%d,%d)", shotIdx, tgt.x, tgt.y)
		}

		// P2 gets opponent_fired.
		opFired := readMsg(t, conn2)
		if opFired.Type != MsgOpponentFired {
			t.Fatalf("shot %d: expected opponent_fired, got %s", shotIdx, opFired.Type)
		}

		if isLast {
			// Both get game_over.
			gameOver1 := readMsg(t, conn1)
			if gameOver1.Type != MsgGameOver {
				t.Fatalf("expected game_over for p1, got %s", gameOver1.Type)
			}
			if gameOver1.YouWin == nil || !*gameOver1.YouWin {
				t.Fatal("player 1 should win")
			}
			if gameOver1.Winner != p1ID {
				t.Fatalf("expected winner %s, got %s", p1ID, gameOver1.Winner)
			}

			gameOver2 := readMsg(t, conn2)
			if gameOver2.Type != MsgGameOver {
				t.Fatalf("expected game_over for p2, got %s", gameOver2.Type)
			}
			if gameOver2.YouWin == nil || *gameOver2.YouWin {
				t.Fatal("player 2 should lose")
			}
		} else {
			// Both get turn_update.
			turn1 := readMsg(t, conn1)
			if turn1.Type != MsgTurnUpdate {
				t.Fatalf("shot %d: expected turn_update for p1, got %s", shotIdx, turn1.Type)
			}
			turn2 := readMsg(t, conn2)
			if turn2.Type != MsgTurnUpdate {
				t.Fatalf("shot %d: expected turn_update for p2, got %s", shotIdx, turn2.Type)
			}

			// Player 2's turn — fire a miss at far columns (away from P1's ships at col 0).
			missX := 9 - (p2MissIdx / 10)
			missY := p2MissIdx % 10
			sendMsg(t, conn2, ClientMessage{
				Type:   MsgFire,
				Target: &game.Coord{X: missX, Y: missY},
			})
			p2MissIdx++

			// P2 gets fire_result.
			p2Result := readMsg(t, conn2)
			if p2Result.Type != MsgFireResult {
				t.Fatalf("shot %d: expected fire_result for p2, got %s: %s", shotIdx, p2Result.Type, p2Result.Message)
			}

			// P1 gets opponent_fired.
			p1Notif := readMsg(t, conn1)
			if p1Notif.Type != MsgOpponentFired {
				t.Fatalf("shot %d: expected opponent_fired for p1, got %s", shotIdx, p1Notif.Type)
			}

			// Both get turn_update (back to p1).
			turn1 = readMsg(t, conn1)
			if turn1.Type != MsgTurnUpdate {
				t.Fatalf("shot %d: expected turn_update for p1 (back), got %s", shotIdx, turn1.Type)
			}
			turn2 = readMsg(t, conn2)
			if turn2.Type != MsgTurnUpdate {
				t.Fatalf("shot %d: expected turn_update for p2 (back), got %s", shotIdx, turn2.Type)
			}
		}
	}
}
