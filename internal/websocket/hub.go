// Package websocket provides WebSocket support for real-time streaming.
// This enables TUI and other clients to receive real-time updates.
package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client represents a connected WebSocket client
type Client struct {
	ID         string
	SessionID   string
	ClientID    string
	ClientType  string // tui, slack, web, etc.
	Connection  *websocket.Conn
	Send        chan []byte
	SubscribeTo string // Channel/stream to subscribe to
}

// Hub manages all WebSocket clients
type Hub struct {
	clients    map[string]*Client // clientID -> Client
	sessions   map[string][]string // sessionID -> []clientID
	register   chan *Client
	unregister chan *Client
	broadcast  chan *BroadcastMessage
	mu         sync.RWMutex
}

// BroadcastMessage is a message to broadcast to clients
type BroadcastMessage struct {
	SessionID string                 `json:"session_id,omitempty"`
	Channel   string                 `json:"channel,omitempty"`
	Data      interface{}            `json:"data"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// SessionUpdate represents a session update event
type SessionUpdate struct {
	Type      string                 `json:"type"` // created, updated, state_changed, completed, failed
	SessionID string                 `json:"session_id"`
	State     string                 `json:"state,omitempty"`
	Timestamp string                 `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		sessions:   make(map[string][]string),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *BroadcastMessage, 256),
	}
}

// Run starts the hub's event loop
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			if client.SessionID != "" {
				h.sessions[client.SessionID] = append(h.sessions[client.SessionID], client.ID)
			}
			h.mu.Unlock()
			log.Printf("WebSocket client registered: %s (session: %s)", client.ID, client.SessionID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				close(client.Send)
				// Remove from sessions mapping
				for sessionID, clientIDs := range h.sessions {
					for i, cid := range clientIDs {
						if cid == client.ID {
							h.sessions[sessionID] = append(clientIDs[:i], clientIDs[i+1:]...)
							break
						}
					}
				}
			}
			h.mu.Unlock()
			log.Printf("WebSocket client unregistered: %s", client.ID)

		case message := <-h.broadcast:
			h.mu.RLock()
			// Broadcast to all clients subscribed to session
			if message.SessionID != "" {
				clientIDs := h.sessions[message.SessionID]
				for _, clientID := range clientIDs {
					if client, ok := h.clients[clientID]; ok {
						select {
						case client.Send <- h.encodeMessage(message):
						default:
							// Client channel full, close it
							close(client.Send)
							delete(h.clients, clientID)
						}
					}
				}
			} else if message.Channel != "" {
				// Broadcast to all clients subscribed to channel
				for _, client := range h.clients {
					if client.SubscribeTo == message.Channel {
						select {
						case client.Send <- h.encodeMessage(message):
						default:
							close(client.Send)
							delete(h.clients, client.ID)
						}
					}
				}
			} else {
				// Broadcast to all clients
				for _, client := range h.clients {
					select {
					case client.Send <- h.encodeMessage(message):
					default:
						close(client.Send)
						delete(h.clients, client.ID)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// encodeMessage encodes a broadcast message to JSON
func (h *Hub) encodeMessage(msg *BroadcastMessage) []byte {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error encoding message: %v", err)
		return []byte(`{"error": "encode_error"}`)
	}
	return data
}

// PublishSessionUpdate publishes a session update to all subscribed clients
func (h *Hub) PublishSessionUpdate(update *SessionUpdate) {
	msg := &BroadcastMessage{
		SessionID: update.SessionID,
		Channel:   "sessions",
		Data:      update,
		Metadata: map[string]interface{}{
			"type":      "session_update",
			"timestamp": update.Timestamp,
		},
	}

	select {
	case h.broadcast <- msg:
	default:
		log.Printf("Broadcast channel full, dropping session update for %s", update.SessionID)
	}
}

// PublishToClient sends a message to a specific client
func (h *Hub) PublishToClient(clientID string, data interface{}) error {
	h.mu.RLock()
	client, ok := h.clients[clientID]
	h.mu.RUnlock()

	if !ok {
		return fmt.Errorf("client not found: %s", clientID)
	}

	msg := &BroadcastMessage{
		Data: data,
	}

	select {
	case client.Send <- h.encodeMessage(msg):
		return nil
	default:
		return fmt.Errorf("client channel full")
	}
}

// Handler returns an HTTP handler for WebSocket connections
func (h *Hub) Handler() http.Handler {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// Allow all origins in development
			// In production, implement proper CORS check
			return true
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}

		// Get client info from query params
		query := r.URL.Query()
		clientID := query.Get("client_id")
		if clientID == "" {
			clientID = fmt.Sprintf("client-%d", time.Now().UnixNano())
		}

		sessionID := query.Get("session_id")
		clientType := query.Get("client_type")
		if clientType == "" {
			clientType = "unknown"
		}

		subscribeTo := query.Get("subscribe")
		if subscribeTo == "" {
			subscribeTo = "all"
		}

		// Create client
		client := &Client{
			ID:         clientID,
			SessionID:   sessionID,
			ClientID:    clientID,
			ClientType:  clientType,
			Connection:  conn,
			Send:        make(chan []byte, 256),
			SubscribeTo: subscribeTo,
		}

		// Register client
		h.register <- client

		// Start client goroutine
		go client.writePump()
		go client.readPump(h)
	})
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Connection.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				// Hub closed the channel
				c.Connection.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Connection.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WebSocket write error for client %s: %v", c.ID, err)
				return
			}

		case <-ticker.C:
			// Send periodic ping
			if err := c.Connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump(h *Hub) {
	defer func() {
		h.unregister <- c
		c.Connection.Close()
	}()

	c.Connection.SetReadLimit(512)
	c.Connection.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Connection.SetPongHandler(func(string) error {
		c.Connection.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Connection.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error for client %s: %v", c.ID, err)
			}
			break
		}

		// Handle incoming message (could be client commands)
		// For now, just log it
		log.Printf("WebSocket message from client %s: %s", c.ID, string(message))
	}
}
