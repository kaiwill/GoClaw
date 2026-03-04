package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/zeroclaw-labs/goclaw/pkg/types"
)

const defaultOllamaBaseURL = "http://localhost:11434"

type OllamaProvider struct {
	baseURL    string
	httpClient *http.Client
}

type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaGenerateRequest struct {
	Model       string                 `json:"model"`
	Messages    []OllamaMessage        `json:"messages,omitempty"`
	Prompt      string                 `json:"prompt,omitempty"`
	Stream      bool                   `json:"stream"`
	Temperature float64                `json:"temperature,omitempty"`
	Options     map[string]interface{} `json:"options,omitempty"`
	Tools       []OllamaToolSpec       `json:"tools,omitempty"`
}

type OllamaToolSpec struct {
	Function OllamaFunctionSpec `json:"function"`
}

type OllamaFunctionSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type OllamaGenerateResponse struct {
	Model           string           `json:"model"`
	Response        string           `json:"response"`
	Done            bool             `json:"done"`
	Context         []int            `json:"context,omitempty"`
	TotalDuration   int64            `json:"total_duration,omitempty"`
	LoadDuration    int64            `json:"load_duration,omitempty"`
	PromptEvalCount *int             `json:"prompt_eval_count,omitempty"`
	EvalCount       *int             `json:"eval_count,omitempty"`
	Message         *OllamaMessage   `json:"message,omitempty"`
	ToolCalls       []OllamaToolCall `json:"tool_calls,omitempty"`
}

type OllamaToolCall struct {
	Function OllamaFunctionCall `json:"function"`
}

type OllamaFunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type OllamaModel struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
	Digest     string `json:"digest"`
}

type OllamaModelsResponse struct {
	Models []OllamaModel `json:"models"`
}

func NewOllamaProvider() *OllamaProvider {
	return NewOllamaProviderWithBaseURL("")
}

func NewOllamaProviderWithBaseURL(baseURL string) *OllamaProvider {
	if baseURL == "" {
		baseURL = defaultOllamaBaseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &OllamaProvider{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 120},
	}
}

func (p *OllamaProvider) Name() string {
	return "ollama"
}

func (p *OllamaProvider) Capabilities() types.ProviderCapabilities {
	return types.ProviderCapabilities{
		NativeToolCalling: true,
		Vision:            false,
	}
}

func (p *OllamaProvider) ConvertTools(tools []*types.ToolSpec) *ConvertToolsResult {
	if len(tools) == 0 {
		return &ConvertToolsResult{
			Type:         "prompt_guided",
			ToolsPayload: json.RawMessage("[]"),
			Instructions: BuildToolInstructionsText(tools),
		}
	}

	ollamaTools := make([]OllamaToolSpec, len(tools))
	for i, tool := range tools {
		ollamaTools[i] = OllamaToolSpec{
			Function: OllamaFunctionSpec{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		}
	}

	toolsPayload, _ := json.Marshal(ollamaTools)
	return &ConvertToolsResult{
		Type:         "ollama",
		ToolsPayload: toolsPayload,
		Instructions: BuildToolInstructionsText(tools),
	}
}

func (p *OllamaProvider) SimpleChat(ctx context.Context, message, model string, temperature float64) (string, error) {
	return p.ChatWithSystem(ctx, "", message, model, temperature)
}

func (p *OllamaProvider) ChatWithSystem(ctx context.Context, systemPrompt, message, model string, temperature float64) (string, error) {
	messages := make([]OllamaMessage, 0, 2)
	if systemPrompt != "" {
		messages = append(messages, OllamaMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}
	messages = append(messages, OllamaMessage{
		Role:    "user",
		Content: message,
	})

	req := OllamaGenerateRequest{
		Model:       model,
		Messages:    messages,
		Stream:      false,
		Temperature: temperature,
	}

	resp, err := p.doRequest(ctx, "/api/chat", req)
	if err != nil {
		return "", err
	}

	genResp, ok := resp.(OllamaGenerateResponse)
	if !ok {
		return "", fmt.Errorf("invalid response format")
	}

	if genResp.Message != nil {
		return genResp.Message.Content, nil
	}

	return genResp.Response, nil
}

func (p *OllamaProvider) ChatWithHistory(ctx context.Context, messages []types.ChatMessage, model string, temperature float64) (string, error) {
	nativeMessages := make([]OllamaMessage, len(messages))
	for i, msg := range messages {
		nativeMessages[i] = OllamaMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	req := OllamaGenerateRequest{
		Model:       model,
		Messages:    nativeMessages,
		Stream:      false,
		Temperature: temperature,
	}

	resp, err := p.doRequest(ctx, "/api/chat", req)
	if err != nil {
		return "", err
	}

	genResp, ok := resp.(OllamaGenerateResponse)
	if !ok {
		return "", fmt.Errorf("invalid response format")
	}

	if genResp.Message != nil {
		return genResp.Message.Content, nil
	}

	return genResp.Response, nil
}

func (p *OllamaProvider) Chat(ctx context.Context, request *ChatRequest, model string, temperature float64) (*types.ChatResponse, error) {
	nativeMessages := make([]OllamaMessage, len(request.Messages))
	for i, msg := range request.Messages {
		nativeMessages[i] = OllamaMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	req := OllamaGenerateRequest{
		Model:       model,
		Messages:    nativeMessages,
		Stream:      false,
		Temperature: temperature,
		Tools:       p.convertToolSpecs(request.Tools),
	}

	resp, err := p.doRequest(ctx, "/api/chat", req)
	if err != nil {
		return nil, err
	}

	genResp, ok := resp.(OllamaGenerateResponse)
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	return p.parseResponse(genResp), nil
}

func (p *OllamaProvider) ChatWithTools(ctx context.Context, messages []types.ChatMessage, tools []json.RawMessage, model string, temperature float64) (*types.ChatResponse, error) {
	nativeMessages := make([]OllamaMessage, len(messages))
	for i, msg := range messages {
		nativeMessages[i] = OllamaMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	req := OllamaGenerateRequest{
		Model:       model,
		Messages:    nativeMessages,
		Stream:      false,
		Temperature: temperature,
	}

	resp, err := p.doRequest(ctx, "/api/chat", req)
	if err != nil {
		return nil, err
	}

	genResp, ok := resp.(OllamaGenerateResponse)
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	return p.parseResponse(genResp), nil
}

func (p *OllamaProvider) SupportsNativeTools() bool {
	return true
}

func (p *OllamaProvider) SupportsVision() bool {
	return false
}

func (p *OllamaProvider) SupportsStreaming() bool {
	return true
}

func (p *OllamaProvider) StreamChatWithSystem(ctx context.Context, systemPrompt, message, model string, temperature float64, options types.StreamOptions) (<-chan types.StreamChunk, error) {
	messages := make([]OllamaMessage, 0, 2)
	if systemPrompt != "" {
		messages = append(messages, OllamaMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}
	messages = append(messages, OllamaMessage{
		Role:    "user",
		Content: message,
	})

	req := OllamaGenerateRequest{
		Model:       model,
		Messages:    messages,
		Stream:      true,
		Temperature: temperature,
	}

	ch := make(chan types.StreamChunk, 10)

	go func() {
		defer close(ch)

		body, err := json.Marshal(req)
		if err != nil {
			ch <- types.ErrorChunk(fmt.Sprintf("failed to marshal request: %v", err))
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewReader(body))
		if err != nil {
			ch <- types.ErrorChunk(fmt.Sprintf("failed to create request: %v", err))
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := p.httpClient.Do(httpReq)
		if err != nil {
			ch <- types.ErrorChunk(fmt.Sprintf("request failed: %v", err))
			return
		}
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)
		for {
			var genResp OllamaGenerateResponse
			if err := decoder.Decode(&genResp); err != nil {
				if err.Error() == "EOF" {
					ch <- types.FinalChunk()
				} else {
					ch <- types.ErrorChunk(fmt.Sprintf("decode error: %v", err))
				}
				return
			}

			delta := ""
			if genResp.Message != nil {
				delta = genResp.Message.Content
			} else {
				delta = genResp.Response
			}

			if genResp.Done {
				ch <- types.FinalChunk()
				return
			}

			ch <- types.NewStreamChunk(delta)
		}
	}()

	return ch, nil
}

func (p *OllamaProvider) StreamChatWithHistory(ctx context.Context, messages []types.ChatMessage, model string, temperature float64, options types.StreamOptions) (<-chan types.StreamChunk, error) {
	return p.StreamChatWithSystem(ctx, "", messages[len(messages)-1].Content, model, temperature, options)
}

func (p *OllamaProvider) Warmup(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/api/tags", nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Ollama not available (status %d)", resp.StatusCode)
	}

	return nil
}

func (p *OllamaProvider) doRequest(ctx context.Context, path string, req interface{}) (interface{}, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

func (p *OllamaProvider) parseResponse(resp OllamaGenerateResponse) *types.ChatResponse {
	text := ""
	if resp.Message != nil {
		text = resp.Message.Content
	} else {
		text = resp.Response
	}

	toolCalls := make([]types.ToolCall, 0, len(resp.ToolCalls))
	for _, tc := range resp.ToolCalls {
		toolCalls = append(toolCalls, types.ToolCall{
			ID:        "",
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	var usage *types.TokenUsage
	if resp.PromptEvalCount != nil || resp.EvalCount != nil {
		usage = &types.TokenUsage{}
		if resp.PromptEvalCount != nil {
			usage.InputTokens = uintPtr(uint64(*resp.PromptEvalCount))
		}
		if resp.EvalCount != nil {
			usage.OutputTokens = uintPtr(uint64(*resp.EvalCount))
		}
	}

	return &types.ChatResponse{
		Text:      &text,
		ToolCalls: toolCalls,
		Usage:     usage,
	}
}

func (p *OllamaProvider) convertToolSpecs(tools []*types.ToolSpec) []OllamaToolSpec {
	if len(tools) == 0 {
		return nil
	}

	result := make([]OllamaToolSpec, len(tools))
	for i, tool := range tools {
		result[i] = OllamaToolSpec{
			Function: OllamaFunctionSpec{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		}
	}
	return result
}
