package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/zeroclaw-labs/goclaw/pkg/types"
)

type DiscordChannel struct {
	botToken      string
	guildID       string
	allowedUsers  []string
	listenToBots  bool
	mentionOnly   bool
	httpClient    *http.Client
	wsConn        interface{}
	typingHandles sync.Map
}

type DiscordMessage struct {
	ID          string              `json:"id"`
	ChannelID   string              `json:"channel_id"`
	GuildID     *string             `json:"guild_id"`
	Author      DiscordUser         `json:"author"`
	Content     string              `json:"content"`
	Timestamp   string              `json:"timestamp"`
	Attachments []DiscordAttachment `json:"attachments"`
	Mentions    []DiscordUser       `json:"mentions"`
}

type DiscordUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Bot      bool   `json:"bot"`
}

type DiscordAttachment struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	URL         string `json:"url"`
}

type DiscordWebhookPayload struct {
	Content string `json:"content"`
}

func NewDiscordChannel(botToken string, guildID string, allowedUsers []string, listenToBots, mentionOnly bool) *DiscordChannel {
	return &DiscordChannel{
		botToken:     botToken,
		guildID:      guildID,
		allowedUsers: allowedUsers,
		listenToBots: listenToBots,
		mentionOnly:  mentionOnly,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *DiscordChannel) Name() string {
	return "discord"
}

func (c *DiscordChannel) Send(ctx context.Context, message *types.SendMessage) error {
	channelID := message.Recipient
	if channelID == "" {
		return fmt.Errorf("recipient (channel ID) is required for Discord")
	}

	webhookURL := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", channelID)

	payload := DiscordWebhookPayload{
		Content: message.Content,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bot "+c.botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Discord API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *DiscordChannel) Listen(ctx context.Context, msgChan chan<- types.ChannelMessage) error {
	return fmt.Errorf("Discord Listen not implemented - use gateway WS instead")
}

func (c *DiscordChannel) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://discord.com/api/v10/users/@me", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bot "+c.botToken)

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

func (c *DiscordChannel) StartTyping(ctx context.Context, recipient string) error {
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/typing", recipient)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bot "+c.botToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *DiscordChannel) StopTyping(ctx context.Context, recipient string) error {
	return nil
}

func (c *DiscordChannel) SupportsDraftUpdates() bool {
	return false
}

func (c *DiscordChannel) SendDraft(ctx context.Context, message *types.SendMessage) (string, error) {
	return "", fmt.Errorf("draft updates not supported")
}

func (c *DiscordChannel) UpdateDraft(ctx context.Context, recipient, messageID, text string) (string, error) {
	return "", fmt.Errorf("draft updates not supported")
}

func (c *DiscordChannel) FinalizeDraft(ctx context.Context, recipient, messageID, text string) error {
	return fmt.Errorf("draft updates not supported")
}

func (c *DiscordChannel) CancelDraft(ctx context.Context, recipient, messageID string) error {
	return fmt.Errorf("draft updates not supported")
}

func (c *DiscordChannel) AddReaction(ctx context.Context, channelID, messageID, emoji string) error {
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages/%s/reactions/%s/@me",
		channelID, messageID, strings.Trim(emoji, ":"))

	req, err := http.NewRequestWithContext(ctx, "PUT", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bot "+c.botToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Discord API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *DiscordChannel) RemoveReaction(ctx context.Context, channelID, messageID, emoji string) error {
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages/%s/reactions/%s/@me",
		channelID, messageID, strings.Trim(emoji, ":"))

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bot "+c.botToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *DiscordChannel) isUserAllowed(userID string) bool {
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

func parseDiscordAttachmentMarkers(message string) (string, []DiscordAttachment) {
	re := regexp.MustCompile(`\[(IMAGE|DOCUMENT|VIDEO|AUDIO|VOICE):([^\]]+)\]`)
	matches := re.FindAllStringSubmatch(message, -1)

	var attachments []DiscordAttachment
	for _, match := range matches {
		if len(match) >= 3 {
			kind := match[1]
			target := match[2]

			attachment := DiscordAttachment{
				Filename: target,
			}

			switch strings.ToUpper(kind) {
			case "IMAGE":
				attachment.ContentType = "image"
			case "VIDEO":
				attachment.ContentType = "video"
			case "AUDIO":
				attachment.ContentType = "audio"
			default:
				attachment.ContentType = "application/octet-stream"
			}

			attachments = append(attachments, attachment)
		}
	}

	cleaned := re.ReplaceAllString(message, "")
	return cleaned, attachments
}

func processDiscordAttachments(attachments []DiscordAttachment, client *http.Client) string {
	var results []string

	for _, att := range attachments {
		if strings.HasPrefix(att.URL, "http") {
			req, _ := http.NewRequest("GET", att.URL, nil)
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == 200 {
					contentType := resp.Header.Get("Content-Type")
					if strings.HasPrefix(contentType, "text/") {
						body, _ := io.ReadAll(resp.Body)
						results = append(results, fmt.Sprintf("[%s]\n%s", att.Filename, string(body)))
					}
				}
			}
		}
	}

	return strings.Join(results, "\n---\n")
}
