package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zeroclaw-labs/goclaw/pkg/types"
)

const (
	whatsappAPIBase = "https://graph.facebook.com/v18.0"
)

type WhatsAppChannel struct {
	accessToken     string
	phoneNumberID   string
	verifyToken     string
	allowedNumbers  []string
	httpClient      *http.Client
	webhookURL      string
}

type WhatsAppWebhookPayload struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"`
		Changes []struct {
			Value struct {
				MessagingProduct string `json:"messaging_product"`
				Metadata         struct {
					DisplayPhoneNumber string `json:"display_phone_number"`
					PhoneNumberID      string `json:"phone_number_id"`
				} `json:"metadata"`
				Messages []WhatsAppMessage `json:"messages"`
			} `json:"value"`
			Field string `json:"field"`
		} `json:"changes"`
	} `json:"entry"`
}

type WhatsAppMessage struct {
	From      string `json:"from"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Text      *struct {
		Body string `json:"body"`
	} `json:"text"`
}

type WhatsAppSendMessageRequest struct {
	MessagingProduct string `json:"messaging_product"`
	RecipientType   string `json:"recipient_type"`
	To              string `json:"to"`
	Type            string `json:"type"`
	Text            struct {
		PreviewURL bool   `json:"preview_url"`
		Body       string `json:"body"`
	} `json:"text"`
}

type WhatsAppSendMessageResponse struct {
	MessagingProduct string `json:"messaging_product"`
	Contacts        []struct {
		Input string `json:"input"`
		WaID  string `json:"wa_id"`
	} `json:"contacts"`
	Messages []struct {
		ID string `json:"id"`
	} `json:"messages"`
}

func NewWhatsAppChannel(accessToken, phoneNumberID, verifyToken string, allowedNumbers []string) *WhatsAppChannel {
	return &WhatsAppChannel{
		accessToken:    accessToken,
		phoneNumberID:  phoneNumberID,
		verifyToken:    verifyToken,
		allowedNumbers: normalizeAllowedUsers(allowedNumbers),
		httpClient:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *WhatsAppChannel) Name() string {
	return "whatsapp"
}

func (c *WhatsAppChannel) Send(ctx context.Context, message *types.SendMessage) error {
	recipient := message.Recipient
	if recipient == "" {
		return fmt.Errorf("recipient is required for WhatsApp")
	}

	req := WhatsAppSendMessageRequest{
		MessagingProduct: "whatsapp",
		RecipientType:   "individual",
		To:              strings.TrimPrefix(recipient, "+"),
		Type:            "text",
	}
	req.Text.Body = message.Content
	req.Text.PreviewURL = false

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s/messages", whatsappAPIBase, c.phoneNumberID)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result WhatsAppSendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Messages) == 0 {
		return fmt.Errorf("no message ID returned from WhatsApp API")
	}

	return nil
}

func (c *WhatsAppChannel) Listen(ctx context.Context, msgChan chan<- types.ChannelMessage) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// WhatsApp uses webhook mode, not polling
			// This is a placeholder to keep the channel alive
			// Actual messages come via webhook endpoint
		}
	}
}

func (c *WhatsAppChannel) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/%s", whatsappAPIBase, c.phoneNumberID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)

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

func (c *WhatsAppChannel) StartTyping(ctx context.Context, recipient string) error {
	return nil
}

func (c *WhatsAppChannel) StopTyping(ctx context.Context, recipient string) error {
	return nil
}

func (c *WhatsAppChannel) SupportsDraftUpdates() bool {
	return false
}

func (c *WhatsAppChannel) SendDraft(ctx context.Context, message *types.SendMessage) (string, error) {
	return "", fmt.Errorf("draft updates not supported")
}

func (c *WhatsAppChannel) UpdateDraft(ctx context.Context, recipient, messageID, text string) (string, error) {
	return "", fmt.Errorf("draft updates not supported")
}

func (c *WhatsAppChannel) FinalizeDraft(ctx context.Context, recipient, messageID, text string) error {
	return fmt.Errorf("draft updates not supported")
}

func (c *WhatsAppChannel) CancelDraft(ctx context.Context, recipient, messageID string) error {
	return fmt.Errorf("draft updates not supported")
}

func (c *WhatsAppChannel) AddReaction(ctx context.Context, channelID, messageID, emoji string) error {
	return fmt.Errorf("reactions not supported")
}

func (c *WhatsAppChannel) RemoveReaction(ctx context.Context, channelID, messageID, emoji string) error {
	return fmt.Errorf("reactions not supported")
}

func (c *WhatsAppChannel) ParseWebhookPayload(payload []byte) []types.ChannelMessage {
	var webhook WhatsAppWebhookPayload
	if err := json.Unmarshal(payload, &webhook); err != nil {
		return nil
	}

	var messages []types.ChannelMessage

	for _, entry := range webhook.Entry {
		for _, change := range entry.Changes {
			if change.Field != "messages" {
				continue
			}

			for _, msg := range change.Value.Messages {
				if msg.Type != "text" || msg.Text == nil {
					continue
				}

				sender := msg.From
				if !strings.HasPrefix(sender, "+") {
					sender = "+" + sender
				}

				if !c.isNumberAllowed(sender) {
					continue
				}

				timestamp := uint64(time.Now().Unix())
				if ts, err := parseTimestamp(msg.Timestamp); err == nil {
					timestamp = ts
				}

				messages = append(messages, types.ChannelMessage{
					ID:          msg.ID,
					Sender:      sender,
					ReplyTarget: sender,
					Content:     msg.Text.Body,
					Channel:     "whatsapp",
					Timestamp:   timestamp,
					ThreadTS:    "",
				})
			}
		}
	}

	return messages
}

func (c *WhatsAppChannel) isNumberAllowed(phone string) bool {
	if len(c.allowedNumbers) == 0 {
		return true
	}

	for _, allowed := range c.allowedNumbers {
		if allowed == "*" || allowed == phone {
			return true
		}
	}

	return false
}

func (c *WhatsAppChannel) VerifyToken() string {
	return c.verifyToken
}

func parseTimestamp(ts string) (uint64, error) {
	var timestamp uint64
	_, err := fmt.Sscanf(ts, "%d", &timestamp)
	return timestamp, err
}