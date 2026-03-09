package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zeroclaw-labs/goclaw/pkg/types"
)

const (
	larkAPIBase          = "https://open.feishu.cn"
	larkBotCallbackTopic = "/v1.0/im/bot/messages/get"
)

type LarkChannel struct {
	appID          string
	appSecret      string
	allowedUsers    []string
	sessionWebhooks map[string]string
	webhooksMutex   sync.RWMutex
	httpClient      *http.Client
	processedMsgs   map[string]time.Time
	msgsMutex       sync.RWMutex
}

type LarkGatewayRequest struct {
	AppID        string `json:"app_id"`
	AppSecret     string `json:"app_secret"`
	Subscriptions []struct {
		Type  string `json:"type"`
		Topic string `json:"topic"`
	} `json:"subscriptions"`
}

type LarkGatewayResponse struct {
	Endpoint string `json:"endpoint"`
	Ticket   string `json:"ticket"`
}

type LarkStreamFrame struct {
	Type    string            `json:"type"`
	Headers map[string]string `json:"headers,omitempty"`
	Data    json.RawMessage   `json:"data"`
}

type LarkMessageData struct {
	Content struct {
		Content string `json:"content"`
	} `json:"content"`
	SenderID       string `json:"sender_id"`
	ChatID         string `json:"chat_id"`
	ChatType       string `json:"chat_type"`
	SessionWebhook string `json:"session_webhook"`
	Text           struct {
		Content string `json:"content"`
	} `json:"text"`
	SenderNick string `json:"sender_nick"`
}

type LarkSendMessageRequest struct {
	ReceiveID string `json:"receive_id"`
	ReceiveIDType string `json:"receive_id_type"`
	MsgType  string `json:"msg_type"`
	Content  string `json:"content"`
}

type LarkMessageContent struct {
	Text string `json:"text"`
}

func NewLarkChannel(appID, appSecret string, allowedUsers []string) *LarkChannel {
	log.Printf("Lark: initializing with appID=%s, allowedUsers=%v", appID, allowedUsers)
	return &LarkChannel{
		appID:          appID,
		appSecret:       appSecret,
		allowedUsers:     normalizeAllowedUsers(allowedUsers),
		sessionWebhooks:  make(map[string]string),
		httpClient:       &http.Client{Timeout: 30 * time.Second},
		processedMsgs:    make(map[string]time.Time),
	}
}

func (c *LarkChannel) Name() string {
	return "lark"
}

func (c *LarkChannel) Send(ctx context.Context, message *types.SendMessage) error {
	c.webhooksMutex.RLock()
	webhookURL, ok := c.sessionWebhooks[message.Recipient]
	c.webhooksMutex.RUnlock()

	log.Printf("Lark Send: looking for webhook for recipient=%s, found=%v", message.Recipient, ok)

	if !ok {
		c.webhooksMutex.RLock()
		log.Printf("Lark Send: available webhooks: %+v", c.sessionWebhooks)
		c.webhooksMutex.RUnlock()
		return fmt.Errorf("no session webhook found for chat %s. The user must send a message first to establish a session", message.Recipient)
	}

	content := LarkMessageContent{
		Text: message.Content,
	}
	contentJSON, _ := json.Marshal(content)

	req := LarkSendMessageRequest{
		ReceiveID:     message.Recipient,
		ReceiveIDType: "chat_id",
		MsgType:       "text",
		Content:       string(contentJSON),
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", webhookURL, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Lark webhook reply failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *LarkChannel) Listen(ctx context.Context, msgChan chan<- types.ChannelMessage) error {
	log.Printf("Lark: Listen started, appID=%s", c.appID)
	for {
		select {
		case <-ctx.Done():
			log.Printf("Lark: context cancelled")
			return ctx.Err()
		default:
			if err := c.connectAndListen(ctx, msgChan); err != nil {
				log.Printf("Lark: connection error: %v, reconnecting in 5 seconds...", err)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(5 * time.Second):
					continue
				}
			}
		}
	}
}

func (c *LarkChannel) connectAndListen(ctx context.Context, msgChan chan<- types.ChannelMessage) error {
	log.Printf("Lark: registering gateway connection...")

	gw, err := c.registerConnection()
	if err != nil {
		log.Printf("Lark: failed to register connection: %v", err)
		return err
	}
	log.Printf("Lark: gateway response - endpoint: %s, ticket: %s", gw.Endpoint, gw.Ticket)

	wsURL := fmt.Sprintf("%s?ticket=%s", gw.Endpoint, gw.Ticket)
	log.Printf("Lark: connecting to stream WebSocket: %s", wsURL)

	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		log.Printf("Lark: failed to connect to WebSocket: %v", err)
		return err
	}
	defer conn.Close()

	log.Printf("Lark: connected and listening for messages...")

	for {
		select {
		case <-ctx.Done():
			log.Printf("Lark: context cancelled")
			return ctx.Err()
		default:
			conn.SetReadDeadline(time.Now().Add(120 * time.Second))

			messageType, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("Lark: WebSocket closed unexpectedly: %v", err)
					return err
				}
				log.Printf("Lark: read error: %v", err)
				return err
			}

			log.Printf("Lark: received message type=%d, raw=%s", messageType, string(message))

			var frame LarkStreamFrame
			if err := json.Unmarshal(message, &frame); err != nil {
				log.Printf("Lark: failed to parse frame: %v", err)
				continue
			}

			log.Printf("Lark: parsed frame type=%s", frame.Type)

			switch frame.Type {
			case "SYSTEM":
				c.handleSystemFrame(conn, &frame)
			case "EVENT", "CALLBACK":
				c.handleCallbackFrame(ctx, msgChan, &frame)
			default:
				log.Printf("Lark: unknown frame type: %s", frame.Type)
			}
		}
	}
}

func (c *LarkChannel) handleSystemFrame(conn *websocket.Conn, frame *LarkStreamFrame) {
	messageID := ""
	if frame.Headers != nil {
		messageID = frame.Headers["messageId"]
	}

	pong := map[string]interface{}{
		"code": 200,
		"headers": map[string]string{
			"contentType": "application/json",
			"messageId":   messageID,
		},
		"message": "OK",
		"data":    "",
	}

	if err := conn.WriteJSON(pong); err != nil {
		log.Printf("Lark: failed to send pong: %v", err)
	}
}

func (c *LarkChannel) handleCallbackFrame(ctx context.Context, msgChan chan<- types.ChannelMessage, frame *LarkStreamFrame) {
	log.Printf("Lark: handling callback frame, data=%s", string(frame.Data))

	data, err := c.parseStreamData(frame.Data)
	if err != nil {
		log.Printf("Lark: failed to parse stream data: %v", err)
		return
	}

	log.Printf("Lark: parsed data: %+v", data)

	content := strings.TrimSpace(data.Text.Content)
	if content == "" {
		content = strings.TrimSpace(data.Content.Content)
	}

	if content == "" {
		log.Printf("Lark: empty content, skipping")
		return
	}

	senderID := data.SenderID
	if senderID == "" {
		senderID = "unknown"
	}

	log.Printf("Lark: senderID=%s, allowedUsers=%v", senderID, c.allowedUsers)

	if !c.isUserAllowed(senderID) {
		log.Printf("Lark: ignoring message from unauthorized user: %s", senderID)
		return
	}

	msgKey := senderID + ":" + content
	var messageID string
	if frame.Headers != nil {
		messageID = frame.Headers["messageId"]
	}

	c.msgsMutex.Lock()
	if lastTime, exists := c.processedMsgs[msgKey]; exists && time.Since(lastTime) < 30*time.Second {
		c.msgsMutex.Unlock()
		log.Printf("Lark: duplicate message detected, skipping: %s (last seen %v ago)", msgKey, time.Since(lastTime))
		return
	}
	c.processedMsgs[msgKey] = time.Now()

	for k, t := range c.processedMsgs {
		if time.Since(t) > 2*time.Minute {
			delete(c.processedMsgs, k)
		}
	}
	c.msgsMutex.Unlock()

	log.Printf("Lark: processing new message: %s (messageId=%s)", msgKey, messageID)

	chatID := c.resolveChatID(data, senderID)

	if data.SessionWebhook != "" {
		c.webhooksMutex.Lock()
		c.sessionWebhooks[chatID] = data.SessionWebhook
		c.sessionWebhooks[senderID] = data.SessionWebhook
		c.webhooksMutex.Unlock()
		log.Printf("Lark: stored session webhook for chat %s and sender %s", chatID, senderID)
	}

	log.Printf("Lark: received message from %s (chatID=%s): %s", senderID, chatID, content)

	msg := types.ChannelMessage{
		ID:          fmt.Sprintf("lark_%d", time.Now().UnixNano()),
		Channel:     "lark",
		Sender:      senderID,
		ReplyTarget: chatID,
		Content:     content,
		Timestamp:   uint64(time.Now().Unix()),
	}

	select {
	case msgChan <- msg:
		log.Printf("Lark: message sent to channel successfully")
	case <-ctx.Done():
		log.Printf("Lark: context done while sending message")
	default:
		log.Printf("Lark: WARNING - msgChan not ready, message dropped")
	}
}

func (c *LarkChannel) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1.0/gateway/connections/open", larkAPIBase)
	body := LarkGatewayRequest{
		AppID:     c.appID,
		AppSecret:  c.appSecret,
		Subscriptions: []struct {
			Type  string `json:"type"`
			Topic string `json:"topic"`
		}{
			{Type: "CALLBACK", Topic: larkBotCallbackTopic},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (c *LarkChannel) StartTyping(ctx context.Context, recipient string) error {
	return nil
}

func (c *LarkChannel) StopTyping(ctx context.Context, recipient string) error {
	return nil
}

func (c *LarkChannel) SupportsDraftUpdates() bool {
	return false
}

func (c *LarkChannel) SendDraft(ctx context.Context, message *types.SendMessage) (string, error) {
	return "", fmt.Errorf("draft updates not supported")
}

func (c *LarkChannel) UpdateDraft(ctx context.Context, recipient, messageID, text string) (string, error) {
	return "", fmt.Errorf("draft updates not supported")
}

func (c *LarkChannel) FinalizeDraft(ctx context.Context, recipient, messageID, text string) error {
	return fmt.Errorf("draft updates not supported")
}

func (c *LarkChannel) CancelDraft(ctx context.Context, recipient, messageID string) error {
	return fmt.Errorf("draft updates not supported")
}

func (c *LarkChannel) AddReaction(ctx context.Context, channelID, messageID, emoji string) error {
	return fmt.Errorf("reactions not supported")
}

func (c *LarkChannel) RemoveReaction(ctx context.Context, channelID, messageID, emoji string) error {
	return fmt.Errorf("reactions not supported")
}

func (c *LarkChannel) isUserAllowed(userID string) bool {
	if len(c.allowedUsers) == 0 {
		return true
	}

	for _, user := range c.allowedUsers {
		if user == "*" || user == userID {
			return true
		}
	}

	return false
}

func (c *LarkChannel) resolveChatID(data *LarkMessageData, senderID string) string {
	isPrivateChat := data.ChatType == "p2p"

	if isPrivateChat {
		return senderID
	}

	if data.ChatID != "" {
		return data.ChatID
	}

	return senderID
}

func (c *LarkChannel) parseStreamData(data json.RawMessage) (*LarkMessageData, error) {
	var result LarkMessageData

	if err := json.Unmarshal(data, &result); err == nil {
		if result.ChatType == "" {
			var raw map[string]interface{}
			if json.Unmarshal(data, &raw) == nil {
				if ct, ok := raw["chat_type"].(string); ok {
					result.ChatType = ct
				}
			}
		}
		return &result, nil
	}

	var strData string
	if err := json.Unmarshal(data, &strData); err == nil {
		if err := json.Unmarshal([]byte(strData), &result); err == nil {
			return &result, nil
		}
	}

	return nil, fmt.Errorf("failed to parse stream data")
}

func (c *LarkChannel) registerConnection() (*LarkGatewayResponse, error) {
	body := LarkGatewayRequest{
		AppID:     c.appID,
		AppSecret:  c.appSecret,
		Subscriptions: []struct {
			Type  string `json:"type"`
			Topic string `json:"topic"`
		}{
			{Type: "CALLBACK", Topic: larkBotCallbackTopic},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1.0/gateway/connections/open", larkAPIBase), strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Lark gateway registration failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result LarkGatewayResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}