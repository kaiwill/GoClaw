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

const defaultGLMBaseURL = "https://open.bigmodel.cn/api/paas/v4"

type GLMProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

type GLMMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type GLMChatRequest struct {
	Model       string        `json:"model"`
	Messages    []GLMMessage  `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Tools       []GLMToolSpec `json:"tools,omitempty"`
}

type GLMToolSpec struct {
	Type     string          `json:"type"`
	Function GLMFunctionSpec `json:"function"`
}

type GLMFunctionSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type GLMChatResponse struct {
	ID      string      `json:"id"`
	Choices []GLMChoice `json:"choices"`
	Usage   *GLMUsage   `json:"usage,omitempty"`
}

type GLMChoice struct {
	Message GLMMessage `json:"message"`
}

type GLMUsage struct {
	PromptTokens     *int64 `json:"prompt_tokens,omitempty"`
	CompletionTokens *int64 `json:"completion_tokens,omitempty"`
}

func NewGLMProvider(apiKey string) *GLMProvider {
	return NewGLMProviderWithBaseURL("", apiKey)
}

func NewGLMProviderWithBaseURL(baseURL, apiKey string) *GLMProvider {
	if baseURL == "" {
		baseURL = defaultGLMBaseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &GLMProvider{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 120},
	}
}

func (p *GLMProvider) Name() string {
	return "glm"
}

func (p *GLMProvider) Capabilities() types.ProviderCapabilities {
	return types.ProviderCapabilities{
		NativeToolCalling: true,
		Vision:            false,
	}
}

func (p *GLMProvider) ConvertTools(tools []*types.ToolSpec) *ConvertToolsResult {
	if len(tools) == 0 {
		return &ConvertToolsResult{
			Type:         "glm",
			ToolsPayload: json.RawMessage("[]"),
		}
	}

	glmTools := make([]GLMToolSpec, len(tools))
	for i, tool := range tools {
		glmTools[i] = GLMToolSpec{
			Type: "function",
			Function: GLMFunctionSpec{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		}
	}

	toolsPayload, _ := json.Marshal(glmTools)
	return &ConvertToolsResult{
		Type:         "glm",
		ToolsPayload: toolsPayload,
	}
}

func (p *GLMProvider) SimpleChat(ctx context.Context, message, model string, temperature float64) (string, error) {
	return p.ChatWithSystem(ctx, "", message, model, temperature)
}

func (p *GLMProvider) ChatWithSystem(ctx context.Context, systemPrompt, message, model string, temperature float64) (string, error) {
	if p.apiKey == "" {
		return "", fmt.Errorf("GLM API key not set")
	}

	messages := make([]GLMMessage, 0, 2)
	if systemPrompt != "" {
		messages = append(messages, GLMMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}
	messages = append(messages, GLMMessage{
		Role:    "user",
		Content: message,
	})

	req := GLMChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
	}

	resp, err := p.doRequest(ctx, "/chat/completions", req)
	if err != nil {
		return "", err
	}

	chatResp, ok := resp.(GLMChatResponse)
	if !ok {
		return "", fmt.Errorf("invalid response format")
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response from GLM")
	}

	msg := chatResp.Choices[0].Message
	switch v := msg.Content.(type) {
	case string:
		return v, nil
	default:
		return "", fmt.Errorf("unexpected content type")
	}
}

func (p *GLMProvider) ChatWithHistory(ctx context.Context, messages []types.ChatMessage, model string, temperature float64) (string, error) {
	if p.apiKey == "" {
		return "", fmt.Errorf("GLM API key not set")
	}

	nativeMessages := p.convertMessages(messages)

	req := GLMChatRequest{
		Model:       model,
		Messages:    nativeMessages,
		Temperature: temperature,
	}

	resp, err := p.doRequest(ctx, "/chat/completions", req)
	if err != nil {
		return "", err
	}

	chatResp, ok := resp.(GLMChatResponse)
	if !ok {
		return "", fmt.Errorf("invalid response format")
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response from GLM")
	}

	msg := chatResp.Choices[0].Message
	switch v := msg.Content.(type) {
	case string:
		return v, nil
	default:
		return "", fmt.Errorf("unexpected content type")
	}
}

func (p *GLMProvider) Chat(ctx context.Context, request *ChatRequest, model string, temperature float64) (*types.ChatResponse, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("GLM API key not set")
	}

	nativeMessages := p.convertMessages(request.Messages)
	tools := p.convertToolSpecs(request.Tools)

	req := GLMChatRequest{
		Model:       model,
		Messages:    nativeMessages,
		Temperature: temperature,
		Tools:       tools,
	}

	resp, err := p.doRequest(ctx, "/chat/completions", req)
	if err != nil {
		return nil, err
	}

	chatResp, ok := resp.(GLMChatResponse)
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	return p.parseResponse(chatResp), nil
}

func (p *GLMProvider) ChatWithTools(ctx context.Context, messages []types.ChatMessage, tools []json.RawMessage, model string, temperature float64) (*types.ChatResponse, error) {
	nativeMessages := p.convertMessages(messages)

	req := GLMChatRequest{
		Model:       model,
		Messages:    nativeMessages,
		Temperature: temperature,
	}

	resp, err := p.doRequest(ctx, "/chat/completions", req)
	if err != nil {
		return nil, err
	}

	chatResp, ok := resp.(GLMChatResponse)
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	return p.parseResponse(chatResp), nil
}

func (p *GLMProvider) SupportsNativeTools() bool {
	return true
}

func (p *GLMProvider) SupportsVision() bool {
	return false
}

func (p *GLMProvider) SupportsStreaming() bool {
	return false
}

func (p *GLMProvider) StreamChatWithSystem(ctx context.Context, systemPrompt, message, model string, temperature float64, options types.StreamOptions) (<-chan types.StreamChunk, error) {
	ch := make(chan types.StreamChunk, 1)
	close(ch)
	return ch, nil
}

func (p *GLMProvider) StreamChatWithHistory(ctx context.Context, messages []types.ChatMessage, model string, temperature float64, options types.StreamOptions) (<-chan types.StreamChunk, error) {
	ch := make(chan types.StreamChunk, 1)
	close(ch)
	return ch, nil
}

func (p *GLMProvider) Warmup(ctx context.Context) error {
	if p.apiKey == "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("warmup failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (p *GLMProvider) doRequest(ctx context.Context, path string, req interface{}) (interface{}, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GLM API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result GLMChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

func (p *GLMProvider) parseResponse(resp GLMChatResponse) *types.ChatResponse {
	var text string

	if len(resp.Choices) > 0 {
		msg := resp.Choices[0].Message
		switch v := msg.Content.(type) {
		case string:
			text = v
		}
	}

	var usage *types.TokenUsage
	if resp.Usage != nil {
		usage = &types.TokenUsage{}
		if resp.Usage.PromptTokens != nil {
			usage.InputTokens = uintPtr(uint64(*resp.Usage.PromptTokens))
		}
		if resp.Usage.CompletionTokens != nil {
			usage.OutputTokens = uintPtr(uint64(*resp.Usage.CompletionTokens))
		}
	}

	return &types.ChatResponse{
		Text:  &text,
		Usage: usage,
	}
}

func (p *GLMProvider) convertMessages(messages []types.ChatMessage) []GLMMessage {
	result := make([]GLMMessage, len(messages))
	for i, msg := range messages {
		result[i] = GLMMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}
	return result
}

func (p *GLMProvider) convertToolSpecs(tools []*types.ToolSpec) []GLMToolSpec {
	if len(tools) == 0 {
		return nil
	}

	result := make([]GLMToolSpec, len(tools))
	for i, tool := range tools {
		result[i] = GLMToolSpec{
			Type: "function",
			Function: GLMFunctionSpec{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		}
	}
	return result
}
