// Package realtime delivers alerts to browsers over WebSocket.
//
// The design is the "hub" pattern:
//
//   - One Hub goroutine owns the list of connected clients. Nothing else
//     touches that list, so it needs no mutex: all changes arrive through
//     channels and are handled one at a time.
//   - Each client has two goroutines: readPump (notices disconnects and
//     pongs) and writePump (sends alerts and pings).
//   - Publish puts a message on a buffered channel and returns immediately,
//     so the HTTP handler that detected the event is never slowed down.
package realtime

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second // max time to write one message
	pongWait       = 45 * time.Second // drop the client if no pong arrives in this time
	pingPeriod     = 30 * time.Second // how often we ping; must be less than pongWait
	maxMessageSize = 512              // clients are not expected to send data
	sendBuffer     = 64               // alerts queued per client before it counts as too slow
)

// client is one connected browser tab.
type client struct {
	conn *websocket.Conn
	send chan []byte // alerts waiting to be written to this client
}

type Hub struct {
	clients    map[*client]bool // owned by Run; never touched from other goroutines
	register   chan *client
	unregister chan *client
	broadcast  chan []byte
	upgrader   websocket.Upgrader
}

// NewHub creates a hub. allowedOrigins is the list of frontend origins that
// may connect; "*" allows any origin.
func NewHub(allowedOrigins []string) *Hub {
	return &Hub{
		clients:    make(map[*client]bool),
		register:   make(chan *client),
		unregister: make(chan *client),
		broadcast:  make(chan []byte, 256),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return originAllowed(r.Header.Get("Origin"), allowedOrigins)
			},
		},
	}
}

// Run is the hub's event loop. Start it once in its own goroutine.
// It stops when ctx is cancelled (server shutdown).
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case c := <-h.register:
			h.clients[c] = true
			log.Printf("websocket: client connected (%d online)", len(h.clients))

		case c := <-h.unregister:
			h.remove(c)

		case message := <-h.broadcast:
			for c := range h.clients {
				select {
				case c.send <- message:
				default:
					// This client's queue is full: it is too slow or dead.
					// Drop it rather than block every other client.
					h.remove(c)
				}
			}

		case <-ctx.Done():
			for c := range h.clients {
				h.remove(c)
			}
			return
		}
	}
}

// remove forgets a client and closes its send channel, which tells its
// writePump to finish. Only called from Run.
func (h *Hub) remove(c *client) {
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
		log.Printf("websocket: client disconnected (%d online)", len(h.clients))
	}
}

// Publish queues a message for every connected client and returns at once.
// It never blocks: if the queue is full the message is dropped and logged
// (it is still stored in the database).
func (h *Hub) Publish(message any) {
	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("websocket: cannot encode alert: %v", err)
		return
	}
	select {
	case h.broadcast <- data:
	default:
		log.Printf("websocket: broadcast queue full, alert dropped")
	}
}

// ServeWS upgrades an HTTP request to a WebSocket and registers the client.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade has already written the HTTP error response.
		log.Printf("websocket: upgrade failed: %v", err)
		return
	}

	c := &client{conn: conn, send: make(chan []byte, sendBuffer)}
	h.register <- c

	go h.writePump(c)
	go h.readPump(c)
}

// readPump reads from the connection until it fails. We ignore any data the
// client sends; the reason to read is that it is the only way to learn that
// the client has gone away and to receive pong replies.
func (h *Hub) readPump(c *client) {
	defer func() {
		h.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		// Each pong proves the client is alive, so extend the deadline.
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// writePump is the only goroutine that writes to this connection (a
// WebSocket connection supports just one writer at a time). It sends queued
// alerts and a ping every pingPeriod.
func (h *Hub) writePump(c *client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel: say goodbye and stop.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func originAllowed(origin string, allowed []string) bool {
	if origin == "" {
		return true // not a browser (curl, tests)
	}
	for _, candidate := range allowed {
		if candidate == "*" || candidate == origin {
			return true
		}
	}
	return false
}
