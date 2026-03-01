package providers

import (
	"context"
	"testing"
)

func TestProviderCapabilities(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
	}{
		{
			name:     "OpenAI",
			provider: NewOpenAIProvider("test-key"),
		},
		{
			name:     "Anthropic",
			provider: NewAnthropicProvider("test-key"),
		},
		{
			name:     "Gemini",
			provider: NewGeminiProvider("test-key"),
		},
		{
			name:     "GLM",
			provider: NewGLMProvider("test-key"),
		},
		{
			name:     "Ollama",
			provider: NewOllamaProvider(),
		},
		{
			name:     "Bedrock",
			provider: NewBedrockProvider("test-key", "test-secret", "", "us-east-1"),
		},
		{
			name:     "OpenRouter",
			provider: NewOpenRouterProvider("test-key"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := tt.provider.Capabilities()

			if caps.MaxTokens <= 0 {
				t.Errorf("Provider %s: MaxTokens should be > 0, got %d", tt.name, caps.MaxTokens)
			}

			if tt.name == "OpenAI" || tt.name == "Anthropic" || tt.name == "Gemini" {
				if !caps.SupportsTools {
					t.Errorf("Provider %s: should support tools", tt.name)
				}
			}

			if tt.name == "OpenAI" || tt.name == "Gemini" {
				if !caps.SupportsVision {
					t.Errorf("Provider %s: should support vision", tt.name)
				}
			}
		})
	}
}

func TestProviderName(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		want     string
	}{
		{
			name:     "OpenAI",
			provider: NewOpenAIProvider("test-key"),
			want:     "openai",
		},
		{
			name:     "Anthropic",
			provider: NewAnthropicProvider("test-key"),
			want:     "anthropic",
		},
		{
			name:     "Gemini",
			provider: NewGeminiProvider("test-key"),
			want:     "gemini",
		},
		{
			name:     "GLM",
			provider: NewGLMProvider("test-key"),
			want:     "glm",
		},
		{
			name:     "Ollama",
			provider: NewOllamaProvider(),
			want:     "ollama",
		},
		{
			name:     "Bedrock",
			provider: NewBedrockProvider("test-key", "test-secret", "", "us-east-1"),
			want:     "bedrock",
		},
		{
			name:     "OpenRouter",
			provider: NewOpenRouterProvider("test-key"),
			want:     "openrouter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.provider.Name(); got != tt.want {
				t.Errorf("Provider.Name() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSupportedProviders(t *testing.T) {
	providers := SupportedProviders()

	expected := []string{
		"openai",
		"anthropic",
		"gemini",
		"glm",
		"ollama",
	}

	if len(providers) != len(expected) {
		t.Errorf("SupportedProviders() returned %d items, want %d", len(providers), len(expected))
	}

	for i, p := range expected {
		if i >= len(providers) || providers[i] != p {
			t.Errorf("SupportedProviders()[%d] = %v, want %v", i, providers[i], p)
		}
	}
}