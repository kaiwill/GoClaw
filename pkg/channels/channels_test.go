package channels

import (
	"context"
	"testing"
)

func TestChannelName(t *testing.T) {
	tests := []struct {
		name    string
		channel Channel
		want    string
	}{
		{
			name:    "Telegram",
			channel: NewTelegramChannel("test-token", []string{}, false),
			want:    "telegram",
		},
		{
			name:    "Discord",
			channel: NewDiscordChannel("test-token", []string{}),
			want:    "discord",
		},
		{
			name:    "Slack",
			channel: NewSlackChannel("test-token", []string{}),
			want:    "slack",
		},
		{
			name:    "WhatsApp",
			channel: NewWhatsAppChannel("test-token", "123456789", "verify", []string{}),
			want:    "whatsapp",
		},
		{
			name:    "Matrix",
			channel: NewMatrixChannel("https://matrix.org", "test-token", "!room:matrix.org", []string{}, false),
			want:    "matrix",
		},
		{
			name:    "DingTalk",
			channel: NewDingTalkChannel("client-id", "client-secret", []string{}),
			want:    "dingtalk",
		},
		{
			name:    "Email",
			channel: NewEmailChannel("smtp.gmail.com", 587, "user", "pass", "from@example.com", []string{}),
			want:    "email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.channel.Name(); got != tt.want {
				t.Errorf("Channel.Name() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChannelSupportsDraftUpdates(t *testing.T) {
	tests := []struct {
		name    string
		channel Channel
		want    bool
	}{
		{
			name:    "Telegram",
			channel: NewTelegramChannel("test-token", []string{}, false),
			want:    false,
		},
		{
			name:    "Discord",
			channel: NewDiscordChannel("test-token", []string{}),
			want:    false,
		},
		{
			name:    "Slack",
			channel: NewSlackChannel("test-token", []string{}),
			want:    false,
		},
		{
			name:    "WhatsApp",
			channel: NewWhatsAppChannel("test-token", "123456789", "verify", []string{}),
			want:    false,
		},
		{
			name:    "Matrix",
			channel: NewMatrixChannel("https://matrix.org", "test-token", "!room:matrix.org", []string{}, false),
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.channel.SupportsDraftUpdates(); got != tt.want {
				t.Errorf("Channel.SupportsDraftUpdates() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChannelStartTyping(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		channel Channel
	}{
		{
			name:    "Telegram",
			channel: NewTelegramChannel("test-token", []string{}, false),
		},
		{
			name:    "Discord",
			channel: NewDiscordChannel("test-token", []string{}),
		},
		{
			name:    "Slack",
			channel: NewSlackChannel("test-token", []string{}),
		},
		{
			name:    "WhatsApp",
			channel: NewWhatsAppChannel("test-token", "123456789", "verify", []string{}),
		},
		{
			name:    "Matrix",
			channel: NewMatrixChannel("https://matrix.org", "test-token", "!room:matrix.org", []string{}, false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.channel.StartTyping(ctx, "test-recipient")
			if err != nil {
				t.Errorf("Channel.StartTyping() error = %v", err)
			}
		})
	}
}

func TestNormalizeAllowedUsers(t *testing.T) {
	tests := []struct {
		name  string
		users []string
		want  int
	}{
		{
			name:  "normal users",
			users: []string{"user1", "user2", "user3"},
			want:  3,
		},
		{
			name:  "with empty strings",
			users: []string{"user1", "", "user2", "", "user3"},
			want:  3,
		},
		{
			name:  "with whitespace",
			users: []string{" user1 ", "  user2  ", "user3"},
			want:  3,
		},
		{
			name:  "duplicates",
			users: []string{"user1", "user2", "user1", "user3", "user2"},
			want:  3,
		},
		{
			name:  "empty list",
			users: []string{},
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeAllowedUsers(tt.users)
			if len(got) != tt.want {
				t.Errorf("normalizeAllowedUsers() = %v, want %d items", got, tt.want)
			}
		})
	}
}