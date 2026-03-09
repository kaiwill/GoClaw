package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/zeroclaw-labs/goclaw/pkg/types"
)

// WecomChannel implements the Channel interface for WeCom (企业微信) bots

type WecomChannel struct {
	botID     string
	botSecret string
	client    *http.Client
	baseURL   string
	
	// Access token management
	accessToken string
	tokenExpiry time.Time
	tokenMutex  chan struct{}
}

// NewWecomChannel creates a new WeCom channel instance
func NewWecomChannel(botID, botSecret string) *WecomChannel {
	log.Printf("WeCom: Creating channel with botID=%s", botID)
	return &WecomChannel{
		botID:     botID,
		botSecret: botSecret,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://qyapi.weixin.qq.com/cgi-bin",
		tokenMutex: make(chan struct{}, 1),
	}
}

// Name returns the channel name
func (c *WecomChannel) Name() string {
	return "wecom"
}

// accessTokenResponse represents the response from WeCom access token API
type accessTokenResponse struct {
	Errcode     int    `json:"errcode"`
	Errmsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// getAccessToken gets a valid access token, refreshing if necessary
func (c *WecomChannel) getAccessToken(ctx context.Context) (string, error) {
	// Check if token is still valid
	if time.Now().Before(c.tokenExpiry) && c.accessToken != "" {
		return c.accessToken, nil
	}

	// Acquire token mutex to prevent concurrent refresh
	c.tokenMutex <- struct{}{}
	defer func() { <-c.tokenMutex }()

	// Double-check token validity after acquiring mutex
	if time.Now().Before(c.tokenExpiry) && c.accessToken != "" {
		return c.accessToken, nil
	}

	// Request new access token
	url := fmt.Sprintf("%s/gettoken?corpid=%s&corpsecret=%s", c.baseURL, c.botID, c.botSecret)
	log.Printf("WeCom: Requesting access token from URL: %s", url)
	resp, err := c.client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}
	defer resp.Body.Close()

	var result accessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode access token response: %w", err)
	}

	if result.Errcode != 0 {
		return "", fmt.Errorf("WeCom API error: %d - %s", result.Errcode, result.Errmsg)
	}

	// Update token and expiry
	c.accessToken = result.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second) // Subtract 60s to avoid token expiration during requests

	log.Printf("WeCom: Access token updated, expires in %d seconds", result.ExpiresIn)

	return c.accessToken, nil
}

// Send sends a message through the WeCom channel
func (c *WecomChannel) Send(ctx context.Context, message *types.SendMessage) error {
	// 实现消息发送逻辑
	log.Printf("WeCom: Sending message to %s", message.Recipient)
	log.Printf("WeCom: Message content: %s", message.Content)
	
	// Get access token
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// Prepare message payload
	payload := map[string]interface{}{
		"touser":  message.Recipient,
		"msgtype": "text",
		"text": map[string]string{
			"content": message.Content,
		},
	}

	// Send message
	reqURL := fmt.Sprintf("%s/message/send?access_token=%s", c.baseURL, token)
	reqBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(string(reqBody)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if errcode := result["errcode"].(float64); errcode != 0 {
		return fmt.Errorf("WeCom API error: %v - %s", errcode, result["errmsg"])
	}

	log.Printf("WeCom: Message sent successfully")

	return nil
}

// listenResponse represents the response from WeCom listen API
type listenResponse struct {
	Errcode     int    `json:"errcode"`
	Errmsg      string `json:"errmsg"`
	Msgid       string `json:"msgid"`
	Msgtype     string `json:"msgtype"`
	Agentid     int    `json:"agentid"`
	Fromusername string `json:"FromUserName"`
	Createtime  int    `json:"CreateTime"`
	Text        struct {
		Content string `json:"Content"`
	} `json:"text"`
}

// Listen starts listening for incoming messages
func (c *WecomChannel) Listen(ctx context.Context, msgChan chan<- types.ChannelMessage) error {
	// 实现消息监听逻辑
	log.Printf("WeCom: Starting message listener (API mode)")

	// Create a ticker for periodic message checks
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("WeCom: Listener stopped")
			return nil
		case <-ticker.C:
			// Check for new messages using API
			// TODO: Implement message polling using WeCom API
			log.Printf("WeCom: Checking for new messages (API mode)")
		}
	}
}

// HealthCheck checks if the channel is healthy
func (c *WecomChannel) HealthCheck(ctx context.Context) error {
	// 实现健康检查逻辑
	log.Printf("WeCom: Health check")
	return nil
}

// StartTyping signals that the bot is processing a response
func (c *WecomChannel) StartTyping(ctx context.Context, recipient string) error {
	// 企业微信不支持typing状态
	return nil
}

// StopTyping stops any active typing indicator
func (c *WecomChannel) StopTyping(ctx context.Context, recipient string) error {
	// 企业微信不支持typing状态
	return nil
}

// SupportsDraftUpdates returns whether this channel supports progressive message updates
func (c *WecomChannel) SupportsDraftUpdates() bool {
	return false
}

// SendDraft sends an initial draft message
func (c *WecomChannel) SendDraft(ctx context.Context, message *types.SendMessage) (string, error) {
	return "", fmt.Errorf("WeCom does not support draft updates")
}

// UpdateDraft updates a previously sent draft message
func (c *WecomChannel) UpdateDraft(ctx context.Context, recipient, messageID, text string) (string, error) {
	return "", fmt.Errorf("WeCom does not support draft updates")
}

// FinalizeDraft finalizes a draft with the complete response
func (c *WecomChannel) FinalizeDraft(ctx context.Context, recipient, messageID, text string) error {
	return fmt.Errorf("WeCom does not support draft updates")
}

// CancelDraft cancels and removes a previously sent draft message
func (c *WecomChannel) CancelDraft(ctx context.Context, recipient, messageID string) error {
	return fmt.Errorf("WeCom does not support draft updates")
}

// AddReaction adds a reaction to a message
func (c *WecomChannel) AddReaction(ctx context.Context, channelID, messageID, emoji string) error {
	// 企业微信不支持消息 reactions
	return nil
}

// RemoveReaction removes a reaction from a message
func (c *WecomChannel) RemoveReaction(ctx context.Context, channelID, messageID, emoji string) error {
	// 企业微信不支持消息 reactions
	return nil
}
