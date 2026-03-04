package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/zeroclaw-labs/goclaw/pkg/types"
)

const (
	openRouterAPIBase = "https://openrouter.ai/api/v1"
)

type OpenRouterProvider struct {
	apiKey     string
	httpClient *http.Client
}

type OpenRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenRouterChatRequest struct {
	Model       string              `json:"model"`
	Messages    []OpenRouterMessage `json:"messages"`
	Tools       []OpenRouterTool    `json:"tools,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
	MaxTokens   int32               `json:"max_tokens,omitempty"`
	Stream      bool                `json:"stream"`
}

type OpenRouterTool struct {
	Type     string                 `json:"type"`
	Function struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Parameters  map[string]interface{} `json:"parameters"`
	} `json:"function"`
}

type OpenRouterChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int `json:"index"`
		Message      struct {
			Role      string  `json:"role"`
			Content   string  `json:"content,omitempty"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type OpenRouterErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func NewOpenRouterProvider(apiKey string) *OpenRouterProvider {
	return &OpenRouterProvider{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (p *OpenRouterProvider) Name() string {
	return "openrouter"
}

func (p *OpenRouterProvider) Chat(ctx context.Context, request *ChatRequest, model string, temperature float64) (*types.ChatResponse, error) {
	openRouterReq := p.buildRequest(request, model, temperature)

	body, err := json.Marshal(openRouterReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", openRouterAPIBase)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("HTTP-Referer", "https://github.com/zeroclaw-labs/goclaw")
	httpReq.Header.Set("X-Title", "GoClaw")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		var errResp OpenRouterErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("OpenRouter API error: %s", errResp.Error.Message)
		}
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var openRouterResp OpenRouterChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&openRouterResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return p.parseResponse(&openRouterResp), nil
}

func (p *OpenRouterProvider) Capabilities() types.ProviderCapabilities {
	return types.ProviderCapabilities{
		NativeToolCalling: true,
		Vision:            true,
	}
}

func (p *OpenRouterProvider) buildRequest(request *ChatRequest, model string, temperature float64) *OpenRouterChatRequest {
	openRouterReq := &OpenRouterChatRequest{
		Model:       model,
		Messages:    make([]OpenRouterMessage, 0, len(request.Messages)),
		Temperature: temperature,
		MaxTokens:   8192,
		Stream:      false,
	}

	for _, msg := range request.Messages {
		openRouterReq.Messages = append(openRouterReq.Messages, OpenRouterMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	if len(request.Tools) > 0 {
		openRouterReq.Tools = make([]OpenRouterTool, 0, len(request.Tools))
		for _, tool := range request.Tools {
			var parameters map[string]interface{}
			if err := json.Unmarshal(tool.Parameters, &parameters); err == nil {
				openRouterReq.Tools = append(openRouterReq.Tools, OpenRouterTool{
					Type: "function",
					Function: struct {
						Name        string                 `json:"name"`
						Description string                 `json:"description"`
						Parameters  map[string]interface{} `json:"parameters"`
					}{
						Name:        tool.Name,
						Description: tool.Description,
						Parameters:  parameters,
					},
				})
			}
		}
	}

	return openRouterReq
}

func (p *OpenRouterProvider) parseResponse(resp *OpenRouterChatResponse) *types.ChatResponse {
	if len(resp.Choices) == 0 {
		inputTokens := uint64(resp.Usage.PromptTokens)
		outputTokens := uint64(resp.Usage.CompletionTokens)
		return &types.ChatResponse{
			Usage: &types.TokenUsage{
				InputTokens:  &inputTokens,
				OutputTokens: &outputTokens,
			},
		}
	}

	choice := resp.Choices[0]
	inputTokens := uint64(resp.Usage.PromptTokens)
	outputTokens := uint64(resp.Usage.CompletionTokens)
	response := &types.ChatResponse{
		Usage: &types.TokenUsage{
			InputTokens:  &inputTokens,
			OutputTokens: &outputTokens,
		},
	}

	if choice.Message.Content != "" {
		response.Text = &choice.Message.Content
	}

	for _, toolCall := range choice.Message.ToolCalls {
		response.ToolCalls = append(response.ToolCalls, types.ToolCall{
			ID:   toolCall.ID,
			Name: toolCall.Function.Name,
			Arguments: json.RawMessage(toolCall.Function.Arguments),
		})
	}

	return response
}