package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/DQSevilla/battleship/internal/room"
	"github.com/DQSevilla/battleship/internal/store"
	"github.com/DQSevilla/battleship/internal/ws"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "battleship.db"
	}

	// Initialize persistence.
	st, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer st.Close()
	log.Printf("Database initialized at %s", dbPath)

	rooms := room.NewManager()
	wsHandler := ws.NewHandler(rooms, st)

	mux := http.NewServeMux()

	// WebSocket endpoint.
	mux.Handle("/ws", wsHandler)

	// REST API: game history.
	mux.HandleFunc("/api/games", func(w http.ResponseWriter, r *http.Request) {
		games, err := st.ListCompletedGames(50)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(games)
	})

	// REST API: game detail with moves.
	mux.HandleFunc("/api/games/", func(w http.ResponseWriter, r *http.Request) {
		gameID := r.URL.Path[len("/api/games/"):]
		if gameID == "" {
			http.Error(w, "game ID required", http.StatusBadRequest)
			return
		}

		game, err := st.GetGame(gameID)
		if err != nil {
			http.Error(w, "game not found", http.StatusNotFound)
			return
		}

		moves, err := st.GetMoves(gameID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"game":  game,
			"moves": moves,
		})
	})

	// Serve static frontend files.
	mux.Handle("/", http.FileServer(http.Dir("web")))

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Battleship server starting on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
