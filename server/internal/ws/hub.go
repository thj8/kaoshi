package ws

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

// Hub 管理所有答题房间
type Hub struct {
	mu    sync.RWMutex
	rooms map[int64]*Room // quizID -> room
}

// Room 一个答题活动的连接房间
type Room struct {
	quizID int64
	mu     sync.RWMutex
	clients map[*Client]struct{}
	// 管理员连接（接收 statistics 等管理事件）
	admins map[*Client]struct{}
}

// Client 一条 WS 连接
type Client struct {
	hub      *Hub
	room     *Room
	send     chan []byte
	IsAdmin  bool
	UserID   int64
	Nickname string
	QuizID   int64
	closed   chan struct{}
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 25 * time.Second // 必须小于 pongWait
	sendBuf    = 64
)

func NewHub() *Hub {
	return &Hub{rooms: make(map[int64]*Room)}
}

func (h *Hub) Room(quizID int64) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[quizID]
	if !ok {
		r = &Room{quizID: quizID, clients: map[*Client]struct{}{}, admins: map[*Client]struct{}{}}
		h.rooms[quizID] = r
	}
	return r
}

func (h *Hub) Join(quizID int64, c *Client) *Room {
	r := h.Room(quizID)
	r.mu.Lock()
	defer r.mu.Unlock()
	if c.IsAdmin {
		r.admins[c] = struct{}{}
	} else {
		r.clients[c] = struct{}{}
	}
	return r
}

func (h *Hub) Leave(c *Client) {
	if c.room == nil {
		return
	}
	r := c.room
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, c)
	delete(r.admins, c)
}

// Broadcast 向房间内所有用户与管理员广播
func (h *Hub) Broadcast(quizID int64, event string, data any) {
	h.sendTo(quizID, event, data, nil)
}

// SendToUser 单播给指定用户（所有该用户连接）
func (h *Hub) SendToUser(quizID int64, userID int64, event string, data any) {
	h.sendTo(quizID, event, data, func(c *Client) bool { return c.UserID == userID })
}

// BroadcastUsers 仅广播给用户（不含管理员）
func (h *Hub) BroadcastUsers(quizID int64, event string, data any) {
	h.sendTo(quizID, event, data, func(c *Client) bool { return !c.IsAdmin })
}

// BroadcastAdmins 仅广播给管理员
func (h *Hub) BroadcastAdmins(quizID int64, event string, data any) {
	h.sendTo(quizID, event, data, func(c *Client) bool { return c.IsAdmin })
}

func (h *Hub) sendTo(quizID int64, event string, data any, filter func(*Client) bool) {
	h.mu.RLock()
	r, ok := h.rooms[quizID]
	h.mu.RUnlock()
	if !ok {
		return
	}
	msg := Message{Event: event, TS: time.Now().UnixMilli()}
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			log.Printf("[ws] marshal %s failed: %v", event, err)
			return
		}
		msg.Data = b
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	for c := range r.clients {
		if filter != nil && !filter(c) {
			continue
		}
		select {
		case c.send <- raw:
		default: // 慢消费者：丢弃，等它重连
		}
	}
	for c := range r.admins {
		if filter != nil && !filter(c) {
			continue
		}
		select {
		case c.send <- raw:
		default:
		}
	}
}

// UserCount 房间用户连接数（近似参与热度）
func (h *Hub) UserCount(quizID int64) int {
	h.mu.RLock()
	r, ok := h.rooms[quizID]
	h.mu.RUnlock()
	if !ok {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}
