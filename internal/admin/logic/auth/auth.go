package auth

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

// Manager 管理用户会话
type Manager struct {
	sessions map[string]time.Time // sessionID -> expiryTime
	mutex    sync.RWMutex
	duration time.Duration
}

// NewManager 创建一个新的会话管理器
func NewManager(duration time.Duration) *Manager {
	m := &Manager{
		sessions: make(map[string]time.Time),
		duration: duration,
	}
	// 启动后台清理任务
	go m.cleanupExpired()
	return m
}

// CreateSession 创建一个新的会话ID
func (m *Manager) CreateSession() string {
	b := make([]byte, 32)
	rand.Read(b)
	sessionID := base64.URLEncoding.EncodeToString(b)

	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.sessions[sessionID] = time.Now().Add(m.duration)
	return sessionID
}

// ValidateSession 检查给定的会话ID是否有效
func (m *Manager) ValidateSession(sessionID string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	expiry, ok := m.sessions[sessionID]
	return ok && time.Now().Before(expiry)
}

// DestroySession 使一个会话失效
func (m *Manager) DestroySession(sessionID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.sessions, sessionID)
}

// cleanupExpired 定期清理过期的会话
func (m *Manager) cleanupExpired() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mutex.Lock()
		for id, expiry := range m.sessions {
			if time.Now().After(expiry) {
				delete(m.sessions, id)
			}
		}
		m.mutex.Unlock()
	}
}

// SetSessionCookie 在响应中设置会话Cookie
func SetSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "encv_session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   0, //表示会话Cookie，浏览器关闭时失效
	})
}

// ClearSessionCookie 清除会话Cookie
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "encv_session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1, // 立即过期
		HttpOnly: true,
	})
}
