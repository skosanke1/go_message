// backend/main.go
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

/*
Minimal WebSocket server with a Hub (connection manager) and a modular Game interface.
- Default game is EchoGame (sends replies only to the sender).
- Swap in BroadcastGame to broadcast to all clients.
*/

// websocket timing constants
const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// allow cross-origin in dev (be careful in production)
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // TODO: tighten origin check in production
	},
}

// Message is the JSON envelope for messages
type Message struct {
	Type    string `json:"type"`              // e.g., "message", "guess", "system"
	Sender  string `json:"sender,omitempty"`  // e.g., user id
	Payload string `json:"payload,omitempty"` // freeform payload
}

// Client represents a connected websocket client
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	id   string
}

// readPump reads messages from the websocket and passes them to the game
func (c *Client) readPump(game Game) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
		game.OnDisconnect(c)
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("unexpected close: %v", err)
			}
			break
		}
		var m Message
		if err := json.Unmarshal(raw, &m); err != nil {
			// if not JSON, wrap as a simple message
			m = Message{Type: "message", Sender: c.id, Payload: string(raw)}
		}
		if m.Sender == "" {
			m.Sender = c.id
		}
		game.OnMessage(c, m)
	}
}

// writePump writes messages from the send channel to the websocket
func (c *Client) writePump() {
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
				// hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			// write a single TextMessage (JSON expected)
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			// send ping
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Hub holds registered clients and broadcasts messages.
type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mu         sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = true
			h.mu.Unlock()
			log.Printf("client registered: %s (total %d)", c.id, len(h.clients))
		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
				log.Printf("client unregistered: %s (total %d)", c.id, len(h.clients))
			}
			h.mu.Unlock()
		case msg := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- msg:
				default:
					// if client send buffer full, close connection
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Game interface - plug-in game logic
type Game interface {
	OnConnect(c *Client)
	OnMessage(c *Client, msg Message)
	OnDisconnect(c *Client)
}

/* ----------------------------
   Example game implementations
   ---------------------------- */

// EchoGame replies only to the sender with "Echo: <payload>"
type EchoGame struct {
	hub *Hub
}

func NewEchoGame(h *Hub) *EchoGame { return &EchoGame{hub: h} }

func (g *EchoGame) OnConnect(c *Client) {
	s := Message{Type: "system", Payload: "Welcome! (EchoGame). Your id: " + c.id}
	b, _ := json.Marshal(s)
	c.send <- b
}

func (g *EchoGame) OnMessage(c *Client, msg Message) {
	// simple behavior: send echo to the sending client
	out := Message{Type: "echo", Sender: "server", Payload: "Echo: " + msg.Payload}
	b, _ := json.Marshal(out)
	c.send <- b
}

func (g *EchoGame) OnDisconnect(c *Client) {
	// nothing for now
}

// BroadcastGame publishes any incoming message to all connected clients
type BroadcastGame struct {
	hub *Hub
}

func NewBroadcastGame(h *Hub) *BroadcastGame { return &BroadcastGame{hub: h} }

func (g *BroadcastGame) OnConnect(c *Client) {
	s := Message{Type: "system", Payload: "Welcome! (BroadcastGame)."}
	b, _ := json.Marshal(s)
	c.send <- b
}

func (g *BroadcastGame) OnMessage(c *Client, msg Message) {
	// broadcast message to everyone (converted to JSON)
	b, _ := json.Marshal(msg)
	g.hub.broadcast <- b
}

func (g *BroadcastGame) OnDisconnect(c *Client) {
	// nothing
}

/* ----------------------------
   WebSocket upgrade / HTTP
   ---------------------------- */

func serveWs(hub *Hub, game Game, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}
	client := &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 256),
		id:   r.RemoteAddr,
	}
	hub.register <- client
	game.OnConnect(client)

	// start pumps
	go client.writePump()
	go client.readPump(game)
}

func spaHandler(distDir string) http.HandlerFunc {
	fs := http.FileServer(http.Dir(distDir))
	return func(w http.ResponseWriter, r *http.Request) {
		// If the requested file exists, serve it; otherwise serve index.html (SPA fallback)
		path := distDir + r.URL.Path
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, distDir+"/index.html")
	}
}

func main() {
	addr := flag.String("addr", ":8080", "http service address")
	staticDir := flag.String("static", "../frontend/dist", "path to frontend build (Vite: dist)")
	mode := flag.String("mode", "echo", "game mode: echo|broadcast")
	flag.Parse()

	hub := NewHub()
	go hub.Run()

	// choose game
	var game Game
	switch *mode {
	case "broadcast":
		game = NewBroadcastGame(hub)
	default:
		game = NewEchoGame(hub)
	}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, game, w, r)
	})

	// serve frontend static files if present
	log.Printf("serving static from %s", *staticDir)
	http.HandleFunc("/", spaHandler(*staticDir))

	log.Printf("listening on %s (mode=%s)", *addr, *mode)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
