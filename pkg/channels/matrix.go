package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zeroclaw-labs/goclaw/pkg/types"
)

const (
	matrixMaxMessageLength = 40000
)

type MatrixChannel struct {
	homeserver     string
	accessToken    string
	roomID         string
	allowedUsers   []string
	mentionOnly    bool
	userID         string
	httpClient     *http.Client
	syncToken      string
	mu             sync.Mutex
}

type MatrixSyncResponse struct {
	NextBatch string `json:"next_batch"`
	Rooms     struct {
		Join map[string]struct {
			Timeline struct {
				Events []MatrixEvent `json:"events"`
			} `json:"timeline"`
		} `json:"join"`
	} `json:"rooms"`
}

type MatrixEvent struct {
	Type     string `json:"type"`
	Sender   string `json:"sender"`
	EventID  string `json:"event_id"`
	RoomID   string `json:"room_id"`
	Content  MatrixEventContent `json:"content"`
	OriginServerTs int64 `json:"origin_server_ts"`
}

type MatrixEventContent struct {
	MsgType string `json:"msgtype"`
	Body    string `json:"body"`
	FormattedBody string `json:"formatted_body"`
	RelatesTo *struct {
		EventID string `json:"event_id"`
		RelType string `json:"rel_type"`
	} `json:"m.relates_to"`
	Mentions *struct {
		UserIDs []string `json:"user_ids"`
	} `json:"m.mentions"`
}

type MatrixSendMessageRequest struct {
	MsgType string `json:"msgtype"`
	Body    string `json:"body"`
	FormattedBody *string `json:"formatted_body,omitempty"`
}

type MatrixSendMessageResponse struct {
	EventID string `json:"event_id"`
}

type MatrixWhoAmIResponse struct {
	UserID   string `json:"user_id"`
	DeviceID string `json:"device_id"`
}

func NewMatrixChannel(homeserver, accessToken, roomID string, allowedUsers []string, mentionOnly bool) *MatrixChannel {
	return &MatrixChannel{
		homeserver:   strings.TrimSuffix(homeserver, "/"),
		accessToken:  accessToken,
		roomID:       roomID,
		allowedUsers: normalizeAllowedUsers(allowedUsers),
		mentionOnly:  mentionOnly,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *MatrixChannel) Name() string {
	return "matrix"
}

func (c *MatrixChannel) Send(ctx context.Context, message *types.SendMessage) error {
	recipient := message.Recipient
	if recipient == "" {
		recipient = c.roomID
	}

	content := message.Content
	if len(content) > matrixMaxMessageLength {
		content = content[:matrixMaxMessageLength]
	}

	req := MatrixSendMessageRequest{
		MsgType: "m.text",
		Body:    content,
	}

	if message.ThreadTS != nil && *message.ThreadTS != "" {
		// Thread support would require additional RelatesTo field
		// For now, skip thread support
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/send/m.room.message/%d",
		c.homeserver, recipient, time.Now().UnixNano()/1000000)

	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(string(body)))
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

	var result MatrixSendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

func (c *MatrixChannel) Listen(ctx context.Context, msgChan chan<- types.ChannelMessage) error {
	if err := c.initUserID(ctx); err != nil {
		return fmt.Errorf("failed to initialize user ID: %w", err)
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			events, err := c.sync(ctx)
			if err != nil {
				continue
			}

			for _, event := range events {
				msg := c.parseEvent(event)
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

func (c *MatrixChannel) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/_matrix/client/v3/account/whoami", c.homeserver)
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

func (c *MatrixChannel) StartTyping(ctx context.Context, recipient string) error {
	url := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/typing/%s", c.homeserver, recipient, c.userID)
	body := map[string]interface{}{
		"typing":   true,
		"timeout":  30000,
	}

	return c.sendRequest(ctx, "PUT", url, body)
}

func (c *MatrixChannel) StopTyping(ctx context.Context, recipient string) error {
	url := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/typing/%s", c.homeserver, recipient, c.userID)
	body := map[string]interface{}{
		"typing": false,
	}

	return c.sendRequest(ctx, "PUT", url, body)
}

func (c *MatrixChannel) SupportsDraftUpdates() bool {
	return false
}

func (c *MatrixChannel) SendDraft(ctx context.Context, message *types.SendMessage) (string, error) {
	return "", fmt.Errorf("draft updates not supported")
}

func (c *MatrixChannel) UpdateDraft(ctx context.Context, recipient, messageID, text string) (string, error) {
	return "", fmt.Errorf("draft updates not supported")
}

func (c *MatrixChannel) FinalizeDraft(ctx context.Context, recipient, messageID, text string) error {
	return fmt.Errorf("draft updates not supported")
}

func (c *MatrixChannel) CancelDraft(ctx context.Context, recipient, messageID string) error {
	return fmt.Errorf("draft updates not supported")
}

func (c *MatrixChannel) AddReaction(ctx context.Context, channelID, messageID, emoji string) error {
	url := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/send/m.reaction/%d",
		c.homeserver, channelID, time.Now().UnixNano()/1000000)

	body := map[string]interface{}{
		"m.relates_to": map[string]interface{}{
			"event_id": messageID,
			"rel_type": "m.annotation",
			"key":      emoji,
		},
	}

	return c.sendRequest(ctx, "PUT", url, body)
}

func (c *MatrixChannel) RemoveReaction(ctx context.Context, channelID, messageID, emoji string) error {
	return fmt.Errorf("reaction removal requires redaction - not implemented")
}

func (c *MatrixChannel) initUserID(ctx context.Context) error {
	if c.userID != "" {
		return nil
	}

	url := fmt.Sprintf("%s/_matrix/client/v3/account/whoami", c.homeserver)
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
		return fmt.Errorf("failed to get user ID: status %d", resp.StatusCode)
	}

	var result MatrixWhoAmIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	c.userID = result.UserID
	return nil
}

func (c *MatrixChannel) sync(ctx context.Context) ([]MatrixEvent, error) {
	url := fmt.Sprintf("%s/_matrix/client/v3/sync", c.homeserver)
	if c.syncToken != "" {
		url += "?since=" + c.syncToken
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("sync failed: status %d", resp.StatusCode)
	}

	var result MatrixSyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	c.syncToken = result.NextBatch

	var events []MatrixEvent
	for roomID, room := range result.Rooms.Join {
		if roomID != c.roomID {
			continue
		}

		for _, event := range room.Timeline.Events {
			if event.Type == "m.room.message" && event.Content.MsgType == "m.text" {
				events = append(events, event)
			}
		}
	}

	return events, nil
}

func (c *MatrixChannel) parseEvent(event MatrixEvent) *types.ChannelMessage {
	if event.Sender == c.userID {
		return nil
	}

	if len(c.allowedUsers) > 0 {
		allowed := false
		for _, user := range c.allowedUsers {
			if user == "*" || user == event.Sender {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil
		}
	}

	if c.mentionOnly {
		mentioned := false
		if event.Content.Mentions != nil {
			for _, userID := range event.Content.Mentions.UserIDs {
				if userID == c.userID {
					mentioned = true
					break
				}
			}
		}
		if strings.Contains(event.Content.Body, c.userID) {
			mentioned = true
		}
		if !mentioned {
			return nil
		}
	}

	threadTS := ""
	if event.Content.RelatesTo != nil && event.Content.RelatesTo.RelType == "m.thread" {
		threadTS = event.Content.RelatesTo.EventID
	}

	content := stripToolCallTags(event.Content.Body)

	return &types.ChannelMessage{
		ID:          event.EventID,
		Sender:      event.Sender,
		ReplyTarget: event.RoomID + ":" + event.EventID,
		Content:     content,
		Channel:     "matrix",
		Timestamp:   uint64(event.OriginServerTs / 1000),
		ThreadTS:    threadTS,
	}
}

func (c *MatrixChannel) sendRequest(ctx context.Context, method, url string, body interface{}) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}