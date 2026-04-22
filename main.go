package main

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins — Cloudflare sits in front and handles security.
	// In production you can restrict this to share.zenlith.dev
	CheckOrigin: func(r *http.Request) bool { return true },
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func wsHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[!] upgrade error: %v", err)
			return
		}

		id := generateID()
		name := randomName()
		client := NewClient(id, name, hub, conn)

		hub.Register(client)

		// Run read and write pumps in separate goroutines
		go client.WritePump()
		client.ReadPump() // blocks until connection closes
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	hub := NewHub()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsHandler(hub))
	mux.HandleFunc("/health", healthHandler)

	log.Printf("[*] share-zenlith signaling server starting on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("[!] server error: %v", err)
	}
}
