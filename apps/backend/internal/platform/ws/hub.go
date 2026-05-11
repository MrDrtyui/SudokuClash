package ws

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
)

type Connection struct {
	UserID  string
	MatchID string
	Socket  *websocket.Conn
	Send    chan []byte
}

type Hub struct {
	mu      sync.RWMutex
	matches map[string]map[string]*Connection
}

func NewHub() *Hub {
	return &Hub{matches: make(map[string]map[string]*Connection)}
}

func (h *Hub) Register(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.matches[conn.MatchID] == nil {
		h.matches[conn.MatchID] = make(map[string]*Connection)
	}
	h.matches[conn.MatchID][conn.UserID] = conn
}

func (h *Hub) Unregister(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if match := h.matches[conn.MatchID]; match != nil {
		delete(match, conn.UserID)
		if len(match) == 0 {
			delete(h.matches, conn.MatchID)
		}
	}
	close(conn.Send)
	_ = conn.Socket.Close()
}

func (h *Hub) Broadcast(matchID string, event string, payload any) {
	message, _ := json.Marshal(map[string]any{
		"event": event,
		"data":  payload,
	})
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, conn := range h.matches[matchID] {
		select {
		case conn.Send <- message:
		default:
		}
	}
}
