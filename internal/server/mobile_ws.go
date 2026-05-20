package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Soltus/encv-go/internal/service"
	"github.com/gorilla/websocket"
)

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	wsHub := s.mobileSvc.GetWSHub()

	conn, err := wsHub.Upgrade(w, r)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}

	client := wsHub.RegisterClient(conn)
	defer wsHub.UnregisterClient(client)

	wsHub.StartWritePump(client)

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

		var msg service.WSMessage
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			slog.Warn("Failed to unmarshal WebSocket message", "error", err)
			continue
		}

		switch msg.Type {
		case "ping":
			wsHub.HandlePing(conn)
		default:
			slog.Debug("Unhandled WebSocket message type", "type", msg.Type)
		}
	}
}

func (s *Server) BroadcastMessage(msgType string, data interface{}) {
	s.mobileSvc.GetWSHub().Broadcast(msgType, data)
}
