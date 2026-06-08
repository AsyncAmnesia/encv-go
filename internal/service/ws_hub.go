package service

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}

type WSHub struct {
	clients  map[*wsClient]struct{}
	mu       sync.RWMutex
	upgrader websocket.Upgrader
}

func NewWSHub() *WSHub {
	return &WSHub{
		clients: make(map[*wsClient]struct{}),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (h *WSHub) Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	return h.upgrader.Upgrade(w, r, nil)
}

func (h *WSHub) RegisterClient(conn *websocket.Conn) *wsClient {
	client := &wsClient{
		conn: conn,
		send: make(chan []byte, 1024),
	}

	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()

	slog.Info("WebSocket client connected", "remote", conn.RemoteAddr())

	statusMsg, _ := json.Marshal(WSMessage{
		Type: "server:status",
		Data: map[string]interface{}{
			"online": true,
		},
	})
	if err := conn.WriteMessage(websocket.TextMessage, statusMsg); err != nil {
		slog.Error("Failed to send status message", "error", err)
	}

	return client
}

func (h *WSHub) UnregisterClient(client *wsClient) {
	h.mu.Lock()
	delete(h.clients, client)
	h.mu.Unlock()

	close(client.send)
	client.conn.Close()

	slog.Info("WebSocket client disconnected", "remote", client.conn.RemoteAddr())
}

func (h *WSHub) HandlePing(conn *websocket.Conn) {
	pongMsg, _ := json.Marshal(WSMessage{Type: "pong"})
	if err := conn.WriteMessage(websocket.TextMessage, pongMsg); err != nil {
		slog.Error("Failed to send pong", "error", err)
	}
}

func (h *WSHub) Broadcast(msgType string, data interface{}) {
	msg, err := json.Marshal(WSMessage{
		Type: msgType,
		Data: data,
	})
	if err != nil {
		slog.Error("Failed to marshal broadcast message", "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	dropped := 0
	for client := range h.clients {
		select {
		case client.send <- msg:
		default:
			dropped++
		}
	}
	if dropped > 0 {
		slog.Warn("WebSocket Broadcast: messages dropped", "type", msgType, "dropped", dropped)
	}
}

func (h *WSHub) BroadcastRaw(msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	dropped := 0
	for client := range h.clients {
		select {
		case client.send <- msg:
		default:
			dropped++
		}
	}
	if dropped > 0 {
		slog.Warn("WebSocket BroadcastRaw: messages dropped", "dropped", dropped, "clients", len(h.clients))
	}
}

func (h *WSHub) StartWritePump(client *wsClient) {
	go func() {
		defer client.conn.Close()
		for msg := range client.send {
			if err := client.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				slog.Error("WebSocket write error", "error", err)
				return
			}
		}
	}()
}
