package ws

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"kaoshi/internal/auth"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // CORS 由中间件/网关层控制
	},
}

// SnapshotProvider 引擎提供状态快照（依赖注入，避免循环依赖）
type SnapshotProvider func(claims *auth.Claims) (*SyncData, error)

type Server struct {
	Hub      *Hub
	Snapshot SnapshotProvider
	// QuizIDByCode 管理端 ?quiz=<code> 房间解析（依赖注入，router 装配）
	QuizIDByCode func(code string) int64
}

// HandleWS /ws，token 走 Sec-WebSocket-Protocol 子协议（不下发 URL，避免进反代 access log）
func (s *Server) HandleWS(c *gin.Context) {
	token := c.GetHeader("Sec-WebSocket-Protocol")
	claims, err := auth.Parse(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "WS 鉴权失败"})
		return
	}
	if claims.Role != auth.RoleAdmin && claims.Role != auth.RoleUser {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "WS 鉴权失败"})
		return
	}
	if claims.Role == auth.RoleUser && claims.QuizID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "缺少答题活动"})
		return
	}

	// token 走子协议时回显，否则浏览器/undici 会因服务端未选择子协议而断开
	var respHeader http.Header
	if proto := c.GetHeader("Sec-WebSocket-Protocol"); proto != "" {
		respHeader = http.Header{"Sec-WebSocket-Protocol": []string{proto}}
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, respHeader)
	if err != nil {
		return
	}

	quizID := claims.QuizID
	// 管理端通过 ?quiz=<code> 指定房间
	if claims.Role == auth.RoleAdmin {
		if code := c.Query("quiz"); code != "" && s.QuizIDByCode != nil {
			quizID = s.QuizIDByCode(code)
		}
	}
	if quizID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "缺少答题活动"})
		return
	}

	client := &Client{
		hub:      s.Hub,
		send:     make(chan []byte, sendBuf),
		IsAdmin:  claims.Role == auth.RoleAdmin,
		UserID:   claims.UserID,
		Nickname: claims.Nick,
		QuizID:   quizID,
		closed:   make(chan struct{}),
	}
	client.room = s.Hub.Join(quizID, client)

	go s.writePump(conn, client)
	go s.readPump(conn, client, s)

	// 连接建立后立即下发全量状态（覆盖断线重连/刷新恢复）
	if s.Snapshot != nil {
		if claims.Role == auth.RoleAdmin {
			c2 := *claims
			c2.QuizID = quizID
			claims = &c2
		}
		if snap, err := s.Snapshot(claims); err == nil && snap != nil {
			snap.Quiz.ID = quizID
			client.Emit(EventSync, snap)
		}
	}
}

// readPump 读泵：处理客户端 ping，检测断开
func (s *Server) readPump(conn *websocket.Conn, c *Client, srv *Server) {
	defer func() {
		s.Hub.Leave(c)
		close(c.closed)
		conn.Close()
	}()
	conn.SetReadLimit(4096)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		switch msg.Event {
		case EventPing:
			_ = conn.SetReadDeadline(time.Now().Add(pongWait))
			c.Emit(EventPong, map[string]int64{"t": time.Now().UnixMilli()})
		}
	}
}

// writePump 写泵：心跳 ping + 消息下发
func (s *Server) writePump(conn *websocket.Conn, c *Client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()
	for {
		select {
		case raw, ok := <-c.send:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, raw); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-c.closed:
			return
		}
	}
}

// Emit 向该连接单发事件
func (c *Client) Emit(event string, data any) {
	msg := Message{Event: event, TS: time.Now().UnixMilli()}
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return
		}
		msg.Data = b
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case c.send <- raw:
	default:
	}
}
