// Package channels defines the Channel interface for messaging platform integrations.
package channels

import (
	"context"

	"github.com/zeroclaw-labs/goclaw/pkg/types"
)

// Channel is the core trait for messaging platform integrations.
// Implementations include Telegram, Discord, Slack, etc.
type Channel interface {
	// Name returns a human-readable channel name
	Name() string

	// Send sends a message through this channel
	Send(ctx context.Context, message *types.SendMessage) error

	// Listen starts listening for incoming messages (long-running).
	// Messages are sent through the provided channel.
	Listen(ctx context.Context, msgChan chan<- types.ChannelMessage) error

	// HealthCheck checks if the channel is healthy
	HealthCheck(ctx context.Context) error

	// StartTyping signals that the bot is processing a response (e.g., "typing" indicator)
	StartTyping(ctx context.Context, recipient string) error

	// StopTyping stops any active typing indicator
	StopTyping(ctx context.Context, recipient string) error

	// SupportsDraftUpdates returns whether this channel supports progressive message updates via draft edits
	SupportsDraftUpdates() bool

	// SendDraft sends an initial draft message.
	// Returns a platform-specific message ID for later edits.
	SendDraft(ctx context.Context, message *types.SendMessage) (messageID string, err error)

	// UpdateDraft updates a previously sent draft message with new accumulated content.
	// Returns empty string to keep current draft ID, or a new ID when a continuation message was created.
	UpdateDraft(ctx context.Context, recipient, messageID, text string) (newID string, err error)

	// FinalizeDraft finalizes a draft with the complete response (e.g., apply Markdown formatting)
	FinalizeDraft(ctx context.Context, recipient, messageID, text string) error

	// CancelDraft cancels and removes a previously sent draft message
	CancelDraft(ctx context.Context, recipient, messageID string) error

	// AddReaction adds a reaction (emoji) to a message
	AddReaction(ctx context.Context, channelID, messageID, emoji string) error

	// RemoveReaction removes a reaction (emoji) from a message
	RemoveReaction(ctx context.Context, channelID, messageID, emoji string) error
}

// ChannelFactory creates a channel from configuration
type ChannelFactory interface {
	// Type returns the channel type identifier
	Type() string

	// Create creates a new channel instance from configuration
	Create(config map[string]interface{}) (Channel, error)

	// ValidateConfig validates the channel configuration
	ValidateConfig(config map[string]interface{}) error
}
