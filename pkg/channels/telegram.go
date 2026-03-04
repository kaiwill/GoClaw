package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zeroclaw-labs/goclaw/pkg/types"
)

const (
	telegramMaxMessageLength     = 4096
	telegramContinuationOverhead = 30
	telegramAPIBase              = "https://api.telegram.org"
)

type TelegramChannel struct {
	botToken              string
	allowedUsers          []string
	pairing               *PairingGuard
	httpClient            *http.Client
	apiBase               string
	mentionOnly           bool
	botUsername           string
	streamMode            StreamMode
	draftUpdateIntervalMs uint64
	mu                    sync.Mutex
}

type TelegramUpdate struct {
	UpdateID      int64            `json:"update_id"`
	Message       *TelegramMessage `json:"message"`
	EditedMessage *TelegramMessage `json:"edited_message"`
}

type TelegramMessage struct {
	MessageID int64         `json:"message_id"`
	From      *TelegramUser `json:"from"`
	Chat      TelegramChat  `json:"chat"`
	Text      *string       `json:"text"`
}

type TelegramUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type TelegramChat struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	Type      string `json:"type"`
}

type TelegramSendMessageRequest struct {
	ChatID                string `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode,omitempty"`
	ReplyToMessageID      int64  `json:"reply_to_message_id,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
}

type TelegramSendMessageResponse struct {
	OK     bool             `json:"ok"`
	Result *TelegramMessage `json:"result"`
}

type TelegramGetUpdatesResponse struct {
	OK     bool             `json:"ok"`
	Result []TelegramUpdate `json:"result"`
}

type TelegramUserInfoResponse struct {
	OK     bool          `json:"ok"`
	Result *TelegramUser `json:"result"`
}

type StreamMode string

const (
	StreamModeOff StreamMode = "off"
)

type PairingGuard struct {
	allowedUsers []string
}

func NewTelegramChannel(botToken string, allowedUsers []string, mentionOnly bool) *TelegramChannel {
	return &TelegramChannel{
		botToken:              botToken,
		allowedUsers:          normalizeAllowedUsers(allowedUsers),
		mentionOnly:           mentionOnly,
		httpClient:            &http.Client{Timeout: 30 * time.Second},
		apiBase:               telegramAPIBase,
		streamMode:            StreamModeOff,
		draftUpdateIntervalMs: 1000,
	}
}

func (c *TelegramChannel) Name() string {
	return "telegram"
}

func (c *TelegramChannel) Send(ctx context.Context, message *types.SendMessage) error {
	chatID := message.Recipient
	if chatID == "" {
		return fmt.Errorf("recipient is required for Telegram")
	}

	chunks := splitMessageForTelegram(message.Content)
	for i, chunk := range chunks {
		req := TelegramSendMessageRequest{
			ChatID:    chatID,
			Text:      chunk,
			ParseMode: "Markdown",
		}

		if message.ThreadTS != nil && *message.ThreadTS != "" {
			if threadID, err := strconv.ParseInt(*message.ThreadTS, 10, 64); err == nil {
				req.ReplyToMessageID = threadID
			}
		}

		if err := c.sendRequest(ctx, "sendMessage", req); err != nil {
			if i == 0 {
				return fmt.Errorf("failed to send message: %w", err)
			}
		}
	}
	return nil
}

func (c *TelegramChannel) Listen(ctx context.Context, msgChan chan<- types.ChannelMessage) error {
	offset := int64(0)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			updates, err := c.getUpdates(ctx, offset)
			if err != nil {
				continue
			}

			for _, update := range updates {
				offset = update.UpdateID + 1

				msg := c.parseUpdate(update)
				if msg != nil {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case msgChan <- *msg:
					}
				}
			}
		}
	}
}

func (c *TelegramChannel) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiBase+"/bot"+c.botToken+"/getMe", nil)
	if err != nil {
		return err
	}

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

func (c *TelegramChannel) StartTyping(ctx context.Context, recipient string) error {
	req := map[string]string{
		"chat_id": recipient,
	}
	return c.sendRequest(ctx, "sendChatAction", req)
}

func (c *TelegramChannel) StopTyping(ctx context.Context, recipient string) error {
	return nil
}

func (c *TelegramChannel) SupportsDraftUpdates() bool {
	return false
}

func (c *TelegramChannel) SendDraft(ctx context.Context, message *types.SendMessage) (string, error) {
	return "", fmt.Errorf("draft updates not supported")
}

func (c *TelegramChannel) UpdateDraft(ctx context.Context, recipient, messageID, text string) (string, error) {
	return "", fmt.Errorf("draft updates not supported")
}

func (c *TelegramChannel) FinalizeDraft(ctx context.Context, recipient, messageID, text string) error {
	return fmt.Errorf("draft updates not supported")
}

func (c *TelegramChannel) CancelDraft(ctx context.Context, recipient, messageID string) error {
	return fmt.Errorf("draft updates not supported")
}

func (c *TelegramChannel) AddReaction(ctx context.Context, channelID, messageID, emoji string) error {
	req := map[string]interface{}{
		"chat_id":    channelID,
		"message_id": messageID,
		"reaction": []map[string]string{
			{"type": "emoji", "emoji": emoji},
		},
	}
	return c.sendRequest(ctx, "setMessageReaction", req)
}

func (c *TelegramChannel) RemoveReaction(ctx context.Context, channelID, messageID, emoji string) error {
	req := map[string]interface{}{
		"chat_id":    channelID,
		"message_id": messageID,
		"reaction":   []interface{}{},
	}
	return c.sendRequest(ctx, "setMessageReaction", req)
}

func (c *TelegramChannel) getUpdates(ctx context.Context, offset int64) ([]TelegramUpdate, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/bot%s/getUpdates?timeout=1&offset=%d", c.apiBase, c.botToken, offset), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("getUpdates failed: %s", string(body))
	}

	var result TelegramGetUpdatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Result, nil
}

func (c *TelegramChannel) sendRequest(ctx context.Context, method string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/bot%s/%s", c.apiBase, c.botToken, method),
		strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result TelegramSendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.OK {
		return fmt.Errorf("API error: not OK")
	}

	return nil
}

func (c *TelegramChannel) parseUpdate(update TelegramUpdate) *types.ChannelMessage {
	var message *TelegramMessage
	if update.Message != nil {
		message = update.Message
	} else if update.EditedMessage != nil {
		message = update.EditedMessage
	}

	if message == nil || message.Text == nil {
		return nil
	}

	from := ""
	if message.From != nil {
		if message.From.Username != "" {
			from = message.From.Username
		} else if message.From.FirstName != "" {
			from = message.From.FirstName
			if message.From.LastName != "" {
				from += " " + message.From.LastName
			}
		}
	}

	chatID := strconv.FormatInt(message.Chat.ID, 10)

	if c.mentionOnly && message.From != nil {
		username := c.botUsername
		if username == "" {
			username = c.getBotUsername(context.Background())
		}
		if username != "" {
			mentioned := false
			re := regexp.MustCompile("@" + username)
			if re.FindString(*message.Text) != "" {
				mentioned = true
			}
			if !mentioned {
				return nil
			}
		}
	}

	text := *message.Text

	if len(c.allowedUsers) > 0 && message.From != nil {
		allowed := false
		userID := strconv.FormatInt(message.From.ID, 10)
		for _, u := range c.allowedUsers {
			if u == userID || u == from {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil
		}
	}

	return &types.ChannelMessage{
		ID:          strconv.FormatInt(message.MessageID, 10),
		Sender:      from,
		ReplyTarget: chatID + ":" + strconv.FormatInt(message.MessageID, 10),
		Content:     stripToolCallTags(text),
		Channel:     "telegram",
		Timestamp:   uint64(time.Now().Unix()),
		ThreadTS:    "",
	}
}

func (c *TelegramChannel) getBotUsername(ctx context.Context) string {
	if c.botUsername != "" {
		return c.botUsername
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.apiBase+"/bot"+c.botToken+"/getMe", nil)
	if err != nil {
		return ""
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var result TelegramUserInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}

	if result.OK && result.Result != nil {
		c.botUsername = result.Result.Username
	}

	return c.botUsername
}

func splitMessageForTelegram(message string) []string {
	if len(message) <= telegramMaxMessageLength {
		return []string{message}
	}

	var chunks []string
	chunkLimit := telegramMaxMessageLength - telegramContinuationOverhead

	for len(message) > 0 {
		if len(message) <= telegramMaxMessageLength {
			chunks = append(chunks, message)
			break
		}

		hardSplit := len(message)
		if len(message) > chunkLimit {
			hardSplit = chunkLimit
		}

		chunkEnd := hardSplit

		if hardSplit < len(message) {
			searchArea := message[:hardSplit]

			if pos := strings.LastIndex(searchArea, "\n"); pos > chunkLimit/2 {
				chunkEnd = pos + 1
			} else if pos := strings.LastIndex(searchArea, " "); pos > chunkLimit/2 {
				chunkEnd = pos + 1
			}
		}

		chunks = append(chunks, message[:chunkEnd])
		message = message[chunkEnd:]
	}

	return chunks
}

func normalizeAllowedUsers(users []string) []string {
	result := make([]string, 0, len(users))
	seen := make(map[string]bool)

	for _, user := range users {
		user = strings.TrimSpace(user)
		if user == "" {
			continue
		}
		if !seen[user] {
			seen[user] = true
			result = append(result, user)
		}
	}

	return result
}

func stripToolCallTags(message string) string {
	re := regexp.MustCompile(`<tool_call>.*?</tool_call>`)
	return re.ReplaceAllString(message, "")
}

func generatePairingCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	code := make([]byte, 6)
	for i := range code {
		code[i] = chars[i%len(chars)]
	}
	return string(code)
}
