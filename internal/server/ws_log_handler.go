package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/Soltus/encv-go/internal/service"
)

type WSLogHandler struct {
	inner slog.Handler
	hub   *service.WSHub
}

func NewWSLogHandler(inner slog.Handler, hub *service.WSHub) *WSLogHandler {
	return &WSLogHandler{inner: inner, hub: hub}
}

func (h *WSLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *WSLogHandler) Handle(ctx context.Context, r slog.Record) error {
	err := h.inner.Handle(ctx, r)
	if err != nil {
		return err
	}

	if r.Level < slog.LevelInfo {
		return nil
	}

	if h.hub != nil {
		levelStr := "info"
		switch {
		case r.Level >= slog.LevelError:
			levelStr = "error"
		case r.Level >= slog.LevelWarn:
			levelStr = "warn"
		default:
			levelStr = "info"
		}

		msg, _ := json.Marshal(map[string]string{
			"type":      "log",
			"level":     levelStr,
			"message":   r.Message,
			"timestamp": time.Now().Format("15:04:05"),
		})
		h.hub.BroadcastRaw(msg)
	}

	return nil
}

func (h *WSLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &WSLogHandler{
		inner: h.inner.WithAttrs(attrs),
		hub:   h.hub,
	}
}

func (h *WSLogHandler) WithGroup(name string) slog.Handler {
	return &WSLogHandler{
		inner: h.inner.WithGroup(name),
		hub:   h.hub,
	}
}
