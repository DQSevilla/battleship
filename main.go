package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// Restore active games from the database (survive server restart).
	activeGames, err := st.ListActiveGames()
	if err != nil {
		log.Printf("warning: failed to load active games: %v", err)
	} else {
		for i := range activeGames {
			rec := &activeGames[i]
			g, err := store.RestoreGame(rec)
			if err != nil {
				log.Printf("warning: failed to restore game %s: %v", rec.ID, err)
				continue
			}
			rooms.RestoreRoom(rec.RoomCode, g, rec.Mode)
			log.Printf("Restored game: room=%s id=%s phase=%s", rec.RoomCode, rec.ID, rec.Phase)
		}
		if len(activeGames) > 0 {
			log.Printf("Restored %d active game(s) from database", len(activeGames))
		}
	}

	allowedOrigin := os.Getenv("ALLOWED_ORIGIN") // e.g., "battleship.up.railway.app"
	wsHandler := ws.NewHandler(rooms, st, allowedOrigin)

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
		details := make([]store.GameDetail, len(games))
		for i := range games {
			details[i] = games[i].ToDetail()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(details)
	})

	// REST API: game detail with moves.
	mux.HandleFunc("/api/games/", func(w http.ResponseWriter, r *http.Request) {
		gameID := r.URL.Path[len("/api/games/"):]
		if gameID == "" {
			http.Error(w, "game ID required", http.StatusBadRequest)
			return
		}

		gameRec, err := st.GetGame(gameID)
		if err != nil {
			http.Error(w, "game not found", http.StatusNotFound)
			return
		}

		moves, err := st.GetMoves(gameID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		moveDetails := make([]store.MoveDetail, len(moves))
		for i := range moves {
			moveDetails[i] = moves[i].ToDetail()
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"game":  gameRec.ToDetail(),
			"moves": moveDetails,
		})
	})

	// Serve static frontend files.
	mux.Handle("/", http.FileServer(http.Dir("web")))

	addr := fmt.Sprintf(":%s", port)
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Graceful shutdown: listen for SIGINT/SIGTERM.
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Battleship server starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	log.Println("Server stopped")
}
