package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/zeroclaw-labs/goclaw/pkg/types"
)

type SlackChannel struct {
	botToken      string
	signingSecret string
	appToken      string
	allowedUsers  []string
	mentionOnly   bool
	httpClient    *http.Client
}

type SlackMessage struct {
	Type     string      `json:"type"`
	Channel  string      `json:"channel"`
	User     string      `json:"user"`
	Text     string      `json:"text"`
	Ts       string      `json:"ts"`
	ThreadTs string      `json:"thread_ts,omitempty"`
	Files    []SlackFile `json:"files,omitempty"`
	Subtype  string      `json:"subtype,omitempty"`
}

type SlackFile struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Mimetype   string `json:"mimetype"`
	URLPrivate string `json:"url_private"`
}

type SlackMessagePayload struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

type SlackResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func NewSlackChannel(botToken, signingSecret, appToken string, allowedUsers []string, mentionOnly bool) *SlackChannel {
	return &SlackChannel{
		botToken:      botToken,
		signingSecret: signingSecret,
		appToken:      appToken,
		allowedUsers:  allowedUsers,
		mentionOnly:   mentionOnly,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *SlackChannel) Name() string {
	return "slack"
}

func (c *SlackChannel) Send(ctx context.Context, message *types.SendMessage) error {
	channelID := message.Recipient
	if channelID == "" {
		return fmt.Errorf("recipient (channel ID) is required for Slack")
	}

	payload := SlackMessagePayload{
		Channel: channelID,
		Text:    message.Content,
	}

	if message.ThreadTS != nil && *message.ThreadTS != "" {
		payload.Text = message.Content
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://slack.com/api/chat.postMessage", strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Slack API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result SlackResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.OK {
		return fmt.Errorf("Slack API error: %s", result.Error)
	}

	return nil
}

func (c *SlackChannel) Listen(ctx context.Context, msgChan chan<- types.ChannelMessage) error {
	return fmt.Errorf("Slack Listen not implemented - use Events API instead")
}

func (c *SlackChannel) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://slack.com/api/auth.test", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.botToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	var result SlackResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.OK {
		return fmt.Errorf("Slack auth test failed: %s", result.Error)
	}

	return nil
}

func (c *SlackChannel) StartTyping(ctx context.Context, recipient string) error {
	payload := map[string]string{
		"channel": recipient,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST",
		"https://slack.com/api/chat.typing", strings.NewReader(string(body)))

	req.Header.Set("Authorization", "Bearer "+c.botToken)
	req.Header.Set("Content-Type", "application/json")

	c.httpClient.Do(req)
	return nil
}

func (c *SlackChannel) StopTyping(ctx context.Context, recipient string) error {
	return nil
}

func (c *SlackChannel) SupportsDraftUpdates() bool {
	return false
}

func (c *SlackChannel) SendDraft(ctx context.Context, message *types.SendMessage) (string, error) {
	return "", fmt.Errorf("draft updates not supported")
}

func (c *SlackChannel) UpdateDraft(ctx context.Context, recipient, messageID, text string) (string, error) {
	return "", fmt.Errorf("draft updates not supported")
}

func (c *SlackChannel) FinalizeDraft(ctx context.Context, recipient, messageID, text string) error {
	return fmt.Errorf("draft updates not supported")
}

func (c *SlackChannel) CancelDraft(ctx context.Context, recipient, messageID string) error {
	return fmt.Errorf("draft updates not supported")
}

func (c *SlackChannel) AddReaction(ctx context.Context, channelID, messageID, emoji string) error {
	emojiName := strings.Trim(emoji, ":")

	payload := map[string]string{
		"channel":   channelID,
		"timestamp": messageID,
		"name":      emojiName,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST",
		"https://slack.com/api/reactions.add", strings.NewReader(string(body)))

	req.Header.Set("Authorization", "Bearer "+c.botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *SlackChannel) RemoveReaction(ctx context.Context, channelID, messageID, emoji string) error {
	emojiName := strings.Trim(emoji, ":")

	payload := map[string]string{
		"channel":   channelID,
		"timestamp": messageID,
		"name":      emojiName,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST",
		"https://slack.com/api/reactions.remove", strings.NewReader(string(body)))

	req.Header.Set("Authorization", "Bearer "+c.botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func parseSlackMessage(msg SlackMessage) *types.ChannelMessage {
	content := msg.Text

	re := regexp.MustCompile(`<@[A-Z0-9]+>`)
	content = re.ReplaceAllString(content, "")

	re = regexp.MustCompile(`<#[A-Z0-9]+\|([^>]+)>`)
	content = re.ReplaceAllString(content, "#$1")

	re = regexp.MustCompile(`<([^|>]+)\|([^>]+)>`)
	content = re.ReplaceAllString(content, "$2")

	re = regexp.MustCompile(`<!([@#]|everyone|here)(\|[^>]+)?>`)
	content = re.ReplaceAllString(content, "@$1")

	return &types.ChannelMessage{
		ID:          msg.Ts,
		Sender:      msg.User,
		ReplyTarget: msg.Channel + ":" + msg.Ts,
		Content:     content,
		Channel:     "slack",
		Timestamp:   uint64(time.Now().Unix()),
		ThreadTS:    msg.ThreadTs,
	}
}

func (c *SlackChannel) isUserAllowed(userID string) bool {
	if len(c.allowedUsers) == 0 {
		return true
	}
	for _, u := range c.allowedUsers {
		if u == "*" || u == userID {
			return true
		}
	}
	return false
}

func extractSlackAttachments(files []SlackFile, client *http.Client) string {
	var results []string

	for _, file := range files {
		if strings.HasPrefix(file.Mimetype, "text/") {
			req, _ := http.NewRequest("GET", file.URLPrivate+"?token="+os.Getenv("SLACK_BOT_TOKEN"), nil)
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == 200 {
					body, _ := io.ReadAll(resp.Body)
					results = append(results, fmt.Sprintf("[%s]\n%s", file.Name, string(body)))
				}
			}
		}
	}

	return strings.Join(results, "\n---\n")
}
