package main

import (
	"encoding/json"
	"log"
	"sync"
)

type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]*Client),
	}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	h.clients[c.id] = c
	h.mu.Unlock()

	// Send the new client a list of existing peers
	h.mu.RLock()
	peers := make([]PeerInfo, 0, len(h.clients)-1)
	for id, existing := range h.clients {
		if id != c.id {
			peers = append(peers, PeerInfo{ID: id, Name: existing.name})
		}
	}
	h.mu.RUnlock()

	c.send <- mustMarshal(Message{
		Type:  "peers",
		Peers: peers,
	})

	// Broadcast join to everyone else
	h.broadcast(c.id, Message{
		Type: "peer-joined",
		From: c.id,
		Name: c.name,
	})

	log.Printf("[+] %s (%s) joined — %d peers online", c.name, c.id, h.count())
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	delete(h.clients, c.id)
	h.mu.Unlock()

	h.broadcast(c.id, Message{
		Type: "peer-left",
		From: c.id,
	})

	log.Printf("[-] %s (%s) left — %d peers online", c.name, c.id, h.count())
}

// Route forwards a message from one client to a specific target client
func (h *Hub) Route(msg Message) {
	h.mu.RLock()
	target, ok := h.clients[msg.To]
	h.mu.RUnlock()

	if !ok {
		log.Printf("[!] target %s not found for message type %s", msg.To, msg.Type)
		return
	}

	target.send <- mustMarshal(msg)
}

// broadcast sends a message to all clients except the sender
func (h *Hub) broadcast(excludeID string, msg Message) {
	data := mustMarshal(msg)
	h.mu.RLock()
	defer h.mu.RUnlock()
	for id, c := range h.clients {
		if id != excludeID {
			select {
			case c.send <- data:
			default:
				log.Printf("[!] send buffer full for %s, dropping message", c.name)
			}
		}
	}
}

func (h *Hub) count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func mustMarshal(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("[!] marshal error: %v", err)
		return []byte("{}")
	}
	return data
}
