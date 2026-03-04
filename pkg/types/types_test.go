package types

import (
	"testing"
)

func TestChatMessage_Role(t *testing.T) {
	tests := []struct {
		name  string
		msg   ChatMessage
		want  string
	}{
		{
			name: "user message",
			msg: ChatMessage{Role: RoleUser, Content: "hello"},
			want: RoleUser,
		},
		{
			name: "assistant message",
			msg: ChatMessage{Role: RoleAssistant, Content: "hi"},
			want: RoleAssistant,
		},
		{
			name: "system message",
			msg: ChatMessage{Role: RoleSystem, Content: "system"},
			want: RoleSystem,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.Role; got != tt.want {
				t.Errorf("ChatMessage.Role = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToolCall_IsValid(t *testing.T) {
	tests := []struct {
		name string
		tc   ToolCall
		want bool
	}{
		{
			name: "valid tool call",
			tc: ToolCall{
				ID:        "call_123",
				Name:      "test_tool",
				Arguments: `{"param": "value"}`,
			},
			want: true,
		},
		{
			name: "missing id",
			tc: ToolCall{
				Name:      "test_tool",
				Arguments: `{"param": "value"}`,
			},
			want: false,
		},
		{
			name: "missing name",
			tc: ToolCall{
				ID:        "call_123",
				Arguments: `{"param": "value"}`,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.tc.ID != "" && tt.tc.Name != ""
			if valid != tt.want {
				t.Errorf("ToolCall.IsValid() = %v, want %v", valid, tt.want)
			}
		})
	}
}

func TestTokenUsage_Total(t *testing.T) {
	tests := []struct {
		name  string
		usage TokenUsage
		want  int
	}{
		{
			name: "normal usage",
			usage: TokenUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
			},
			want: 150,
		},
		{
			name: "zero usage",
			usage: TokenUsage{},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.usage.PromptTokens + tt.usage.CompletionTokens
			if got != tt.want {
				t.Errorf("TokenUsage.Total() = %v, want %v", got, tt.want)
			}
		})
	}
}