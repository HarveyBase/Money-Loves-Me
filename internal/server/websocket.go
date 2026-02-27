package server

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WSMessage 表示一条 WebSocket 消息。
type WSMessage struct {
	Type string      `json:"type"` // "market"、"order"、"notification"
	Data interface{} `json:"data"`
}

// WSClient 表示一个已连接的 WebSocket 客户端。
type WSClient struct {
	conn *websocket.Conn
	send chan []byte
	hub  *WebSocketHub
}

// WebSocketHub 管理所有 WebSocket 客户端连接。
type WebSocketHub struct {
	mu         sync.RWMutex
	clients    map[*WSClient]bool
	broadcast  chan []byte
	register   chan *WSClient
	unregister chan *WSClient
}

// NewWebSocketHub 创建一个新的 WebSocketHub。
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*WSClient]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
	}
}

// Run 启动 Hub 的事件循环。
func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast 向所有已连接的客户端发送消息。
func (h *WebSocketHub) Broadcast(msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.broadcast <- data
}

// ClientCount 返回已连接的客户端数量。
func (h *WebSocketHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// HandleWebSocket 处理 WebSocket 升级请求。
func HandleWebSocket(hub *WebSocketHub) gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		client := &WSClient{
			conn: conn,
			send: make(chan []byte, 256),
			hub:  hub,
		}

		hub.register <- client

		go client.writePump()
		go client.readPump()
	}
}

func (c *WSClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *WSClient) writePump() {
	defer c.conn.Close()
	for message := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			break
		}
	}
}
