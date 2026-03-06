package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/DQSevilla/battleship/internal/room"
	"github.com/DQSevilla/battleship/internal/ws"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	rooms := room.NewManager()
	wsHandler := ws.NewHandler(rooms)

	mux := http.NewServeMux()

	// WebSocket endpoint.
	mux.Handle("/ws", wsHandler)

	// Serve static frontend files.
	mux.Handle("/", http.FileServer(http.Dir("web")))

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Battleship server starting on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
