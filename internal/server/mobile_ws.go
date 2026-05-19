package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}

var (
	wsClients   = make(map[*wsClient]struct{})
	wsClientsMu sync.RWMutex
)

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	client := &wsClient{
		conn: conn,
		send: make(chan []byte, 256),
	}

	wsClientsMu.Lock()
	wsClients[client] = struct{}{}
	wsClientsMu.Unlock()
	slog.Info("WebSocket client connected", "remote", conn.RemoteAddr())

	defer func() {
		wsClientsMu.Lock()
		delete(wsClients, client)
		wsClientsMu.Unlock()
		close(client.send)
		slog.Info("WebSocket client disconnected", "remote", conn.RemoteAddr())
	}()

	statusMsg, _ := json.Marshal(WSMessage{
		Type: "server:status",
		Data: map[string]interface{}{
			"online": true,
		},
	})
	if err := conn.WriteMessage(websocket.TextMessage, statusMsg); err != nil {
		slog.Error("Failed to send status message", "error", err)
		return
	}

	go s.writePump(client)

	for {
		_, messageBytes, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("WebSocket read error", "error", err)
			} else {
				slog.Debug("WebSocket connection closed", "remote", conn.RemoteAddr())
			}
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			slog.Warn("Failed to unmarshal WebSocket message", "error", err)
			continue
		}

		switch msg.Type {
		case "ping":
			pongMsg, _ := json.Marshal(WSMessage{Type: "pong"})
			if err := conn.WriteMessage(websocket.TextMessage, pongMsg); err != nil {
				slog.Error("Failed to send pong", "error", err)
			}
		default:
			slog.Debug("Unhandled WebSocket message type", "type", msg.Type)
		}
	}
}

func (s *Server) writePump(client *wsClient) {
	defer func() {
		client.conn.Close()
	}()
	for msg := range client.send {
		if err := client.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			slog.Error("WebSocket write error", "error", err)
			return
		}
	}
}

func (s *Server) BroadcastMessage(msgType string, data interface{}) {
	msg, err := json.Marshal(WSMessage{
		Type: msgType,
		Data: data,
	})
	if err != nil {
		slog.Error("Failed to marshal broadcast message", "error", err)
		return
	}

	wsClientsMu.RLock()
	defer wsClientsMu.RUnlock()

	for client := range wsClients {
		select {
		case client.send <- msg:
		default:
			slog.Warn("WebSocket client send buffer full, skipping")
		}
	}
}
