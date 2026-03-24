package channels

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zeroclaw-labs/goclaw/pkg/types"
)

const (
	weComWsDefaultURL = "wss://openws.work.weixin.qq.com"
	weComWsPingInterval = 30 * time.Second
	weComWsPongTimeout = 10 * time.Second
	weComWsReconnectBaseDelay = 1 * time.Second
	weComWsReconnectMaxDelay = 30 * time.Second
	weComWsMaxReconnectAttempts = 100
)

type WecomWSFrame struct {
	Cmd     string                 `json:"cmd,omitempty"`
	Headers map[string]interface{} `json:"headers"`
	Body    interface{}            `json:"body,omitempty"`
	Errcode int                    `json:"errcode,omitempty"`
	Errmsg  string                 `json:"errmsg,omitempty"`
}

type WecomWSClient struct {
	botID      string
	secret     string
	wsURL      string
	conn       *websocket.Conn
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	
	connected     bool
	authenticated bool
	mu            sync.RWMutex
	
	heartbeatTicker *time.Ticker
	heartbeatDone   chan struct{}
	
	reconnectAttempts int
	
	msgChan chan<- types.ChannelMessage
	
	onConnected      func()
	onAuthenticated  func()
	onDisconnected   func(reason string)
	onMessage        func(frame *WecomWSFrame)
	onError          func(err error)
}

func NewWecomWSClient(botID, secret string, wsURL string) *WecomWSClient {
	if wsURL == "" {
		wsURL = weComWsDefaultURL
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &WecomWSClient{
		botID:      botID,
		secret:     secret,
		wsURL:      wsURL,
		ctx:        ctx,
		cancel:     cancel,
		connected:  false,
		authenticated: false,
		heartbeatDone: make(chan struct{}),
		msgChan:    nil,
		reconnectAttempts: 0,
	}
}

func (c *WecomWSClient) SetMessageHandler(fn func(frame *WecomWSFrame)) {
	c.mu.Lock()
	c.onMessage = fn
	c.mu.Unlock()
}

func (c *WecomWSClient) SetConnectedHandler(fn func()) {
	c.mu.Lock()
	c.onConnected = fn
	c.mu.Unlock()
}

func (c *WecomWSClient) SetAuthenticatedHandler(fn func()) {
	c.mu.Lock()
	c.onAuthenticated = fn
	c.mu.Unlock()
}

func (c *WecomWSClient) SetDisconnectedHandler(fn func(reason string)) {
	c.mu.Lock()
	c.onDisconnected = fn
	c.mu.Unlock()
}

func (c *WecomWSClient) SetErrorHandler(fn func(err error)) {
	c.mu.Lock()
	c.onError = fn
	c.mu.Unlock()
}

func (c *WecomWSClient) Connect() error {
	c.mu.Lock()
	c.connected = false
	c.authenticated = false
	c.reconnectAttempts = 0
	c.mu.Unlock()
	
	err := c.connect()
	if err != nil {
		return err
	}
	
	return nil
}

func (c *WecomWSClient) connect() error {
	log.Printf("WeCom: Connecting to WebSocket: %s", c.wsURL)
	
	dialer := &websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	
	conn, _, err := dialer.Dial(c.wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to dial WebSocket: %w", err)
	}
	
	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.mu.Unlock()
	
	c.wg.Add(2)
	go c.readLoop()
	go c.heartbeatLoop()
	
	if c.onConnected != nil {
		c.onConnected()
	}
	
	return nil
}

func (c *WecomWSClient) readLoop() {
	defer c.wg.Done()
	
	c.conn.SetReadLimit(512 * 1024)
	
	// Handle server ping messages by sending back pong responses
	c.conn.SetPingHandler(func(appData string) error {
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()
		
		if conn == nil {
			return fmt.Errorf("connection closed")
		}
		
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(weComWsPongTimeout))
	})
	
	for {
		select {
		case <-c.ctx.Done():
			log.Printf("WeCom: readLoop stopped")
			return
		default:
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WeCom: read error: %v", err)
					c.handleError(err)
				}
				c.handleDisconnect(err.Error())
				return
			}
			
			//log.Printf("WeCom: Received raw message: %s", string(message))
			
			var frame WecomWSFrame
			if err := json.Unmarshal(message, &frame); err != nil {
				log.Printf("WeCom: failed to parse WebSocket message: %v", err)
				continue
			}
			
			c.handleFrame(&frame)
		}
	}
}

func (c *WecomWSClient) handleFrame(frame *WecomWSFrame) {
	//log.Printf("WeCom: Received frame: cmd=%s, headers=%+v, body=%+v, errcode=%d", 
	//	frame.Cmd, frame.Headers, frame.Body, frame.Errcode)
	
	c.mu.RLock()
	onMessage := c.onMessage
	c.mu.RUnlock()
	
	if onMessage != nil {
		onMessage(frame)
	}
}

func (c *WecomWSClient) heartbeatLoop() {
	defer c.wg.Done()
	
	ticker := time.NewTicker(weComWsPingInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-c.ctx.Done():
			log.Printf("WeCom: heartbeatLoop stopped")
			return
		case <-c.heartbeatDone:
			log.Printf("WeCom: heartbeatLoop stopped (manual)")
			return
		case <-ticker.C:
			if !c.isConnected() {
				continue
			}
			
			reqID := generateReqID("ping")
			frame := &WecomWSFrame{
				Cmd:     "ping",
				Headers: map[string]interface{}{"req_id": reqID},
			}
			
			if err := c.sendFrame(frame); err != nil {
				log.Printf("WeCom: failed to send heartbeat: %v", err)
				c.handleError(err)
				c.handleDisconnect("heartbeat failed")
				return
			}
		}
	}
}

func (c *WecomWSClient) sendFrame(frame *WecomWSFrame) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()
	
	if conn == nil {
		return fmt.Errorf("WebSocket not connected")
	}
	
	data, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("failed to marshal frame: %w", err)
	}
	
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("failed to send frame: %w", err)
	}
	
	return nil
}

func (c *WecomWSClient) SendAuth() error {
	reqID := generateReqID("aibot_subscribe")
	frame := &WecomWSFrame{
		Cmd: "aibot_subscribe",
		Headers: map[string]interface{}{
			"req_id": reqID,
		},
		Body: map[string]interface{}{
			"bot_id": c.botID,
			"secret": c.secret,
		},
	}
	
	if err := c.sendFrame(frame); err != nil {
		return fmt.Errorf("failed to send auth: %w", err)
	}
	
	log.Printf("WeCom: Auth frame sent")
	return nil
}

func (c *WecomWSClient) SendReply(reqID string, body interface{}, cmd string) error {
	if cmd == "" {
		cmd = "aibot_respond_msg"
	}
	
	frame := &WecomWSFrame{
		Cmd:     cmd,
		Headers: map[string]interface{}{"req_id": reqID},
		Body:    body,
	}
	
	return c.sendFrame(frame)
}

func (c *WecomWSClient) SendStreamReply(reqID, streamID, content string, finish bool) error {
	body := map[string]interface{}{
		"msgtype": "stream",
		"stream": map[string]interface{}{
			"id":      streamID,
			"content": content,
			"finish":  finish,
		},
	}
	
	return c.SendReply(reqID, body, "")
}

func (c *WecomWSClient) SendTextReply(reqID, content string) error {
	body := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]interface{}{
			"content": content,
		},
	}
	
	return c.SendReply(reqID, body, "")
}

func (c *WecomWSClient) SendMarkdownReply(reqID, content string) error {
	body := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"content": content,
		},
	}
	
	return c.SendReply(reqID, body, "")
}

func (c *WecomWSClient) SendTemplateCardReply(reqID string, card interface{}) error {
	body := map[string]interface{}{
		"msgtype":      "template_card",
		"template_card": card,
	}
	
	return c.SendReply(reqID, body, "")
}

func (c *WecomWSClient) Disconnect() {
	log.Printf("WeCom: Disconnecting WebSocket")
	
	c.cancel()
	c.mu.Lock()
	if c.heartbeatDone != nil {
		close(c.heartbeatDone)
	}
	c.mu.Unlock()
	
	c.wg.Wait()
	
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connected = false
	c.authenticated = false
	c.mu.Unlock()
	
	log.Printf("WeCom: WebSocket disconnected")
}

func (c *WecomWSClient) isConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && c.conn != nil
}

func (c *WecomWSClient) isAuthenticated() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.authenticated
}

func (c *WecomWSClient) handleDisconnect(reason string) {
	c.mu.Lock()
	c.connected = false
	c.authenticated = false
	c.mu.Unlock()
	
	if c.onDisconnected != nil {
		c.onDisconnected(reason)
	}
}

func (c *WecomWSClient) handleError(err error) {
	c.mu.RLock()
	onError := c.onError
	c.mu.RUnlock()
	
	if onError != nil {
		onError(err)
	}
}

func generateReqID(prefix string) string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%s_%d_%s", prefix, time.Now().UnixNano(), base64.URLEncoding.EncodeToString(b))
}

func generateStreamID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func (c *WecomWSClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && c.conn != nil
}

func (c *WecomWSClient) SendFrame(frame *WecomWSFrame) error {
	return c.sendFrame(frame)
}
