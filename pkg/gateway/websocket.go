package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zeroclaw-labs/goclaw/pkg/agent"
	"github.com/zeroclaw-labs/goclaw/pkg/types"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebSocketServer struct {
	addr       string
	server     *http.Server
	agent      *agent.Agent
	clients    map[string]*WebSocketClient
	clientsMu  sync.RWMutex
	httpServer *http.Server
}

type WebSocketClient struct {
	conn        *websocket.Conn
	clientID    string
	sendChan    chan []byte
	closeChan   chan struct{}
	mu          sync.Mutex
}

type WSMessage struct {
	Type         string                 `json:"type"`
	SessionID    string                 `json:"session_id,omitempty"`
	Data         map[string]interface{} `json:"data,omitempty"`
	Content      string                 `json:"content,omitempty"`
	Delta        string                 `json:"delta,omitempty"`
	Usage        *types.TokenUsage      `json:"usage,omitempty"`
	FinishReason string                 `json:"finish_reason,omitempty"`
	Error        string                 `json:"error,omitempty"`
}

type WSChatRequest struct {
	Messages    []types.ChatMessage `json:"messages"`
	Model       string              `json:"model"`
	Temperature float64             `json:"temperature,omitempty"`
	Stream      bool                `json:"stream,omitempty"`
}

type WSChatResponse struct {
	Type      string            `json:"type"`
	SessionID string            `json:"session_id,omitempty"`
	Content   string            `json:"content,omitempty"`
	Delta     string            `json:"delta,omitempty"`
	Usage     *types.TokenUsage `json:"usage,omitempty"`
	FinishReason string          `json:"finish_reason,omitempty"`
	Error     string            `json:"error,omitempty"`
}

func NewWebSocketServer(addr string, agent *agent.Agent) *WebSocketServer {
	return &WebSocketServer{
		addr:    addr,
		agent:   agent,
		clients: make(map[string]*WebSocketClient),
	}
}

func (s *WebSocketServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/ws/", s.handleWebSocket)

	s.httpServer = &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	go func() {
		log.Printf("WebSocket server starting on %s", s.addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("WebSocket server error: %v", err)
		}
	}()

	return nil
}

func (s *WebSocketServer) Stop(ctx context.Context) error {
	if s.httpServer != nil {
		s.clientsMu.Lock()
		for _, client := range s.clients {
			client.Close()
		}
		s.clientsMu.Unlock()

		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

func (s *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	clientID := fmt.Sprintf("ws_%d", time.Now().UnixNano())
	client := &WebSocketClient{
		conn:      conn,
		clientID:  clientID,
		sendChan:  make(chan []byte, 256),
		closeChan: make(chan struct{}),
	}

	s.clientsMu.Lock()
	s.clients[clientID] = client
	s.clientsMu.Unlock()

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, clientID)
		s.clientsMu.Unlock()
		client.Close()
	}()

	go client.writePump()

	client.readPump(s)
}

func (c *WebSocketClient) readPump(server *WebSocketServer) {
	defer func() {
		c.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			log.Printf("WebSocket message parse error: %v", err)
			continue
		}

		server.handleMessage(c, &wsMsg)
	}
}

func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case message, ok := <-c.sendChan:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.mu.Lock()
			err := c.conn.WriteMessage(websocket.TextMessage, message)
			c.mu.Unlock()

			if err != nil {
				return
			}

		case <-ticker.C:
			c.mu.Lock()
			err := c.conn.WriteMessage(websocket.PingMessage, nil)
			c.mu.Unlock()

			if err != nil {
				return
			}

		case <-c.closeChan:
			return
		}
	}
}

func (c *WebSocketClient) Send(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case c.sendChan <- data:
		return nil
	default:
		return fmt.Errorf("send channel full")
	}
}

func (c *WebSocketClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-c.closeChan:
	default:
		close(c.closeChan)
		c.conn.Close()
		close(c.sendChan)
	}
}

func (s *WebSocketServer) handleMessage(client *WebSocketClient, wsMsg *WSMessage) {
	switch wsMsg.Type {
	case "chat":
		s.handleChatRequest(client, wsMsg)
	case "ping":
		client.Send(WSMessage{
			Type: "pong",
			Data: map[string]interface{}{
				"timestamp": time.Now().Unix(),
			},
		})
	default:
		client.Send(WSMessage{
			Type: "error",
			Data: map[string]interface{}{
				"message": fmt.Sprintf("unknown message type: %s", wsMsg.Type),
			},
		})
	}
}

func (s *WebSocketServer) handleChatRequest(client *WebSocketClient, wsMsg *WSMessage) {
	data, err := json.Marshal(wsMsg.Data)
	if err != nil {
		client.Send(WSMessage{
			Type:  "error",
			Error: "invalid request data",
		})
		return
	}

	var req WSChatRequest
	if err := json.Unmarshal(data, &req); err != nil {
		client.Send(WSMessage{
			Type:  "error",
			Error: "failed to parse chat request",
		})
		return
	}

	model := req.Model
	if model == "" {
		model = "gpt-4o"
	}

	temperature := req.Temperature
	if temperature == 0 {
		temperature = 0.7
	}

	sessionID := wsMsg.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("session_%d", time.Now().UnixNano())
	}

	var lastMessage string
	for _, msg := range req.Messages {
		if msg.Role == types.RoleUser {
			lastMessage = msg.Content
		}
	}

	if req.Stream {
		go s.handleStreamingChat(client, sessionID, lastMessage, model, temperature)
	} else {
		s.handleNonStreamingChat(client, sessionID, lastMessage, model, temperature)
	}
}

func (s *WebSocketServer) handleNonStreamingChat(client *WebSocketClient, sessionID, message, model string, temperature float64) {
	response, err := s.agent.ProcessMessage(context.Background(), message)
	if err != nil {
		client.Send(WSMessage{
			Type:      "error",
			SessionID: sessionID,
			Error:     err.Error(),
		})
		return
	}

	finishReason := "stop"
	if response.HasToolCalls() {
		finishReason = "tool_calls"
	}

	client.Send(WSMessage{
		Type:         "chat.response",
		SessionID:    sessionID,
		Content:      response.TextOrEmpty(),
		Usage:        response.Usage,
		FinishReason: finishReason,
	})
}

func (s *WebSocketServer) handleStreamingChat(client *WebSocketClient, sessionID, message, model string, temperature float64) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	response, err := s.agent.ProcessMessage(ctx, message)
	if err != nil {
		client.Send(WSMessage{
			Type:      "error",
			SessionID: sessionID,
			Error:     err.Error(),
		})
		return
	}

	text := response.TextOrEmpty()

	chunkSize := 10
	for i := 0; i < len(text); i += chunkSize {
		end := i + chunkSize
		if end > len(text) {
			end = len(text)
		}

		delta := text[i:end]

		err := client.Send(WSMessage{
			Type:      "chat.chunk",
			SessionID: sessionID,
			Delta:     delta,
		})

		if err != nil {
			return
		}

		time.Sleep(50 * time.Millisecond)
	}

	finishReason := "stop"
	if response.HasToolCalls() {
		finishReason = "tool_calls"
	}

	client.Send(WSMessage{
		Type:         "chat.done",
		SessionID:    sessionID,
		Usage:        response.Usage,
		FinishReason: finishReason,
	})
}

func (s *WebSocketServer) Broadcast(msg interface{}) {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	for _, client := range s.clients {
		client.Send(msg)
	}
}

func (s *WebSocketServer) ClientCount() int {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	return len(s.clients)
}