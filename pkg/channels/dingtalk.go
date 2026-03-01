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
	dingTalkBotCallbackTopic = "/v1.0/im/bot/messages/get"
	dingTalkAPIBase          = "https://api.dingtalk.com"
)

type DingTalkChannel struct {
	clientID        string
	clientSecret    string
	allowedUsers    []string
	sessionWebhooks map[string]string
	webhooksMutex   sync.RWMutex
	httpClient      *http.Client
}

type DingTalkGatewayRequest struct {
	ClientID      string `json:"clientId"`
	ClientSecret  string `json:"clientSecret"`
	Subscriptions []struct {
		Type  string `json:"type"`
		Topic string `json:"topic"`
	} `json:"subscriptions"`
}

type DingTalkGatewayResponse struct {
	Endpoint string `json:"endpoint"`
	Ticket   string `json:"ticket"`
}

type DingTalkStreamFrame struct {
	Type    string            `json:"type"`
	Headers map[string]string `json:"headers,omitempty"`
	Data    json.RawMessage   `json:"data"`
}

type DingTalkMessageData struct {
	Content struct {
		Content string `json:"content"`
	} `json:"content"`
	SenderID         string `json:"senderStaffId"`
	ConversationID   string `json:"conversationId"`
	ConversationType string `json:"conversationType"`
	ChatbotCorpID    string `json:"chatbotCorpId"`
	SessionWebhook   string `json:"sessionWebhook"`
	Text             struct {
		Content string `json:"content"`
	} `json:"text"`
	SenderNick string `json:"senderNick"`
}

type DingTalkSendMessageRequest struct {
	MsgType  string `json:"msgtype"`
	Markdown struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	} `json:"markdown"`
}

func NewDingTalkChannel(clientID, clientSecret string, allowedUsers []string) *DingTalkChannel {
	log.Printf("DingTalk: initializing with clientID=%s, allowedUsers=%v", clientID, allowedUsers)
	return &DingTalkChannel{
		clientID:        clientID,
		clientSecret:    clientSecret,
		allowedUsers:    normalizeAllowedUsers(allowedUsers),
		sessionWebhooks: make(map[string]string),
		httpClient:      &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *DingTalkChannel) Name() string {
	return "dingtalk"
}

func (c *DingTalkChannel) Send(ctx context.Context, message *types.SendMessage) error {
	c.webhooksMutex.RLock()
	webhookURL, ok := c.sessionWebhooks[message.Recipient]
	c.webhooksMutex.RUnlock()

	log.Printf("DingTalk Send: looking for webhook for recipient=%s, found=%v", message.Recipient, ok)

	if !ok {
		c.webhooksMutex.RLock()
		log.Printf("DingTalk Send: available webhooks: %+v", c.sessionWebhooks)
		c.webhooksMutex.RUnlock()
		return fmt.Errorf("no session webhook found for chat %s. The user must send a message first to establish a session", message.Recipient)
	}

	title := "GoClaw"
	if message.Subject != nil {
		title = *message.Subject
	}

	req := DingTalkSendMessageRequest{
		MsgType: "markdown",
	}
	req.Markdown.Title = title
	req.Markdown.Text = message.Content

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
		return fmt.Errorf("DingTalk webhook reply failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *DingTalkChannel) Listen(ctx context.Context, msgChan chan<- types.ChannelMessage) error {
	log.Printf("DingTalk: Listen started, clientID=%s", c.clientID)
	for {
		select {
		case <-ctx.Done():
			log.Printf("DingTalk: context cancelled")
			return ctx.Err()
		default:
			if err := c.connectAndListen(ctx, msgChan); err != nil {
				log.Printf("DingTalk: connection error: %v, reconnecting in 5 seconds...", err)
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

func (c *DingTalkChannel) connectAndListen(ctx context.Context, msgChan chan<- types.ChannelMessage) error {
	log.Printf("DingTalk: registering gateway connection...")

	gw, err := c.registerConnection()
	if err != nil {
		log.Printf("DingTalk: failed to register connection: %v", err)
		return err
	}
	log.Printf("DingTalk: gateway response - endpoint: %s, ticket: %s", gw.Endpoint, gw.Ticket)

	wsURL := fmt.Sprintf("%s?ticket=%s", gw.Endpoint, gw.Ticket)
	log.Printf("DingTalk: connecting to stream WebSocket: %s", wsURL)

	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		log.Printf("DingTalk: failed to connect to WebSocket: %v", err)
		return err
	}
	defer conn.Close()

	log.Printf("DingTalk: connected and listening for messages...")

	for {
		select {
		case <-ctx.Done():
			log.Printf("DingTalk: context cancelled")
			return ctx.Err()
		default:
			conn.SetReadDeadline(time.Now().Add(120 * time.Second))

			messageType, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("DingTalk: WebSocket closed unexpectedly: %v", err)
					return err
				}
				log.Printf("DingTalk: read error: %v", err)
				return err
			}

			log.Printf("DingTalk: received message type=%d, raw=%s", messageType, string(message))

			var frame DingTalkStreamFrame
			if err := json.Unmarshal(message, &frame); err != nil {
				log.Printf("DingTalk: failed to parse frame: %v", err)
				continue
			}

			log.Printf("DingTalk: parsed frame type=%s", frame.Type)

			switch frame.Type {
			case "SYSTEM":
				c.handleSystemFrame(conn, &frame)
			case "EVENT", "CALLBACK":
				c.handleCallbackFrame(ctx, msgChan, &frame)
			default:
				log.Printf("DingTalk: unknown frame type: %s", frame.Type)
			}
		}
	}
}

func (c *DingTalkChannel) handleSystemFrame(conn *websocket.Conn, frame *DingTalkStreamFrame) {
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
		log.Printf("DingTalk: failed to send pong: %v", err)
	}
}

func (c *DingTalkChannel) handleCallbackFrame(ctx context.Context, msgChan chan<- types.ChannelMessage, frame *DingTalkStreamFrame) {
	log.Printf("DingTalk: handling callback frame, data=%s", string(frame.Data))

	data, err := c.parseStreamData(frame.Data)
	if err != nil {
		log.Printf("DingTalk: failed to parse stream data: %v", err)
		return
	}

	log.Printf("DingTalk: parsed data: %+v", data)

	// Extract message content
	content := strings.TrimSpace(data.Text.Content)
	if content == "" {
		content = strings.TrimSpace(data.Content.Content)
	}

	if content == "" {
		log.Printf("DingTalk: empty content, skipping")
		return
	}

	senderID := data.SenderID
	if senderID == "" {
		senderID = "unknown"
	}

	log.Printf("DingTalk: senderID=%s, allowedUsers=%v", senderID, c.allowedUsers)

	if !c.isUserAllowed(senderID) {
		log.Printf("DingTalk: ignoring message from unauthorized user: %s", senderID)
		return
	}

	chatID := c.resolveChatID(data, senderID)

	// Store session webhook for later replies
	if data.SessionWebhook != "" {
		c.webhooksMutex.Lock()
		c.sessionWebhooks[chatID] = data.SessionWebhook
		c.sessionWebhooks[senderID] = data.SessionWebhook
		c.webhooksMutex.Unlock()
		log.Printf("DingTalk: stored session webhook for chat %s and sender %s", chatID, senderID)
	}

	log.Printf("DingTalk: received message from %s (chatID=%s): %s", senderID, chatID, content)

	// Send message to channel with panic prevention
	msg := types.ChannelMessage{
		ID:          fmt.Sprintf("dingtalk_%d", time.Now().UnixNano()),
		Channel:     "dingtalk",
		Sender:      senderID,
		ReplyTarget: chatID,
		Content:     content,
		Timestamp:   uint64(time.Now().Unix()),
	}

	// Use non-blocking send to prevent panic
	select {
	case msgChan <- msg:
		log.Printf("DingTalk: message sent to channel successfully")
	case <-ctx.Done():
		log.Printf("DingTalk: context done while sending message")
	default:
		log.Printf("DingTalk: WARNING - msgChan not ready, message dropped")
	}
}

func (c *DingTalkChannel) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1.0/gateway/connections/open", dingTalkAPIBase)
	body := DingTalkGatewayRequest{
		ClientID:     c.clientID,
		ClientSecret: c.clientSecret,
		Subscriptions: []struct {
			Type  string `json:"type"`
			Topic string `json:"topic"`
		}{
			{Type: "CALLBACK", Topic: dingTalkBotCallbackTopic},
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

func (c *DingTalkChannel) StartTyping(ctx context.Context, recipient string) error {
	return nil
}

func (c *DingTalkChannel) StopTyping(ctx context.Context, recipient string) error {
	return nil
}

func (c *DingTalkChannel) SupportsDraftUpdates() bool {
	return false
}

func (c *DingTalkChannel) SendDraft(ctx context.Context, message *types.SendMessage) (string, error) {
	return "", fmt.Errorf("draft updates not supported")
}

func (c *DingTalkChannel) UpdateDraft(ctx context.Context, recipient, messageID, text string) (string, error) {
	return "", fmt.Errorf("draft updates not supported")
}

func (c *DingTalkChannel) FinalizeDraft(ctx context.Context, recipient, messageID, text string) error {
	return fmt.Errorf("draft updates not supported")
}

func (c *DingTalkChannel) CancelDraft(ctx context.Context, recipient, messageID string) error {
	return fmt.Errorf("draft updates not supported")
}

func (c *DingTalkChannel) AddReaction(ctx context.Context, channelID, messageID, emoji string) error {
	return fmt.Errorf("reactions not supported")
}

func (c *DingTalkChannel) RemoveReaction(ctx context.Context, channelID, messageID, emoji string) error {
	return fmt.Errorf("reactions not supported")
}

func (c *DingTalkChannel) isUserAllowed(userID string) bool {
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

func (c *DingTalkChannel) resolveChatID(data *DingTalkMessageData, senderID string) string {
	isPrivateChat := data.ConversationType == "1"

	if isPrivateChat {
		return senderID
	}

	if data.ConversationID != "" {
		return data.ConversationID
	}

	return senderID
}

func (c *DingTalkChannel) parseStreamData(data json.RawMessage) (*DingTalkMessageData, error) {
	var result DingTalkMessageData

	if err := json.Unmarshal(data, &result); err == nil {
		// Handle conversationType as number
		if result.ConversationType == "" {
			var raw map[string]interface{}
			if json.Unmarshal(data, &raw) == nil {
				if ct, ok := raw["conversationType"].(float64); ok {
					result.ConversationType = fmt.Sprintf("%d", int(ct))
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

func (c *DingTalkChannel) registerConnection() (*DingTalkGatewayResponse, error) {
	body := DingTalkGatewayRequest{
		ClientID:     c.clientID,
		ClientSecret: c.clientSecret,
		Subscriptions: []struct {
			Type  string `json:"type"`
			Topic string `json:"topic"`
		}{
			{Type: "CALLBACK", Topic: dingTalkBotCallbackTopic},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1.0/gateway/connections/open", dingTalkAPIBase), strings.NewReader(string(jsonBody)))
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
		return nil, fmt.Errorf("DingTalk gateway registration failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result DingTalkGatewayResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
