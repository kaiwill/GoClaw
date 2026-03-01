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

const defaultAnthropicBaseURL = "https://api.anthropic.com"

type AnthropicProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

type AnthropicMessage struct {
	Role    string                  `json:"role"`
	Content []AnthropicContentBlock `json:"content"`
}

type AnthropicContentBlock struct {
	Type         string                 `json:"type,omitempty"`
	Text         *string                `json:"text,omitempty"`
	Source       *AnthropicImageSource  `json:"source,omitempty"`
	ID           *string                `json:"id,omitempty"`
	Name         *string                `json:"name,omitempty"`
	Input        json.RawMessage        `json:"input,omitempty"`
	ToolUseID    *string                `json:"tool_use_id,omitempty"`
	CacheControl *AnthropicCacheControl `json:"cache_control,omitempty"`
}

type AnthropicImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type AnthropicCacheControl struct {
	Type string `json:"type"`
}

type AnthropicSystemPrompt struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type AnthropicChatRequest struct {
	Model       string              `json:"model"`
	MaxTokens   int                 `json:"max_tokens"`
	System      interface{}         `json:"system,omitempty"`
	Messages    []AnthropicMessage  `json:"messages"`
	Temperature float64             `json:"temperature"`
	Tools       []AnthropicToolSpec `json:"tools,omitempty"`
}

type AnthropicToolSpec struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	InputSchema  json.RawMessage        `json:"input_schema"`
	CacheControl *AnthropicCacheControl `json:"cache_control,omitempty"`
}

type AnthropicChatResponse struct {
	Content []AnthropicResponseBlock `json:"content"`
	Usage   *AnthropicUsage          `json:"usage,omitempty"`
}

type AnthropicResponseBlock struct {
	Type  string          `json:"type"`
	Text  *string         `json:"text,omitempty"`
	ID    *string         `json:"id,omitempty"`
	Name  *string         `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type AnthropicUsage struct {
	InputTokens  *int64 `json:"input_tokens,omitempty"`
	OutputTokens *int64 `json:"output_tokens,omitempty"`
}

func NewAnthropicProvider(apiKey string) *AnthropicProvider {
	return NewAnthropicProviderWithBaseURL("", apiKey)
}

func NewAnthropicProviderWithBaseURL(baseURL, apiKey string) *AnthropicProvider {
	if baseURL == "" {
		baseURL = defaultAnthropicBaseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &AnthropicProvider{
		apiKey:     strings.TrimSpace(apiKey),
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 120},
	}
}

func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

func (p *AnthropicProvider) Capabilities() types.ProviderCapabilities {
	return types.ProviderCapabilities{
		NativeToolCalling: true,
		Vision:            true,
	}
}

func (p *AnthropicProvider) ConvertTools(tools []*types.ToolSpec) *ConvertToolsResult {
	if len(tools) == 0 {
		return &ConvertToolsResult{
			Type:         "anthropic",
			ToolsPayload: json.RawMessage("[]"),
		}
	}

	anthropicTools := make([]AnthropicToolSpec, len(tools))
	for i, tool := range tools {
		anthropicTools[i] = AnthropicToolSpec{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.Parameters,
		}
	}

	// Cache the last tool definition
	if len(anthropicTools) > 0 {
		anthropicTools[len(anthropicTools)-1].CacheControl = &AnthropicCacheControl{Type: "ephemeral"}
	}

	toolsPayload, _ := json.Marshal(anthropicTools)
	return &ConvertToolsResult{
		Type:         "anthropic",
		ToolsPayload: toolsPayload,
	}
}

func (p *AnthropicProvider) SimpleChat(ctx context.Context, message, model string, temperature float64) (string, error) {
	return p.ChatWithSystem(ctx, "", message, model, temperature)
}

func (p *AnthropicProvider) ChatWithSystem(ctx context.Context, systemPrompt, message, model string, temperature float64) (string, error) {
	if p.apiKey == "" {
		return "", fmt.Errorf("Anthropic credentials not set. Set ANTHROPIC_API_KEY")
	}

	messages := []AnthropicMessage{
		{
			Role: "user",
			Content: []AnthropicContentBlock{
				{Type: "text", Text: &message},
			},
		},
	}

	var system interface{}
	if systemPrompt != "" {
		system = systemPrompt
	}

	req := AnthropicChatRequest{
		Model:       model,
		MaxTokens:   4096,
		System:      system,
		Messages:    messages,
		Temperature: temperature,
	}

	resp, err := p.doRequest(ctx, "/v1/messages", req)
	if err != nil {
		return "", err
	}

	chatResp, ok := resp.(AnthropicChatResponse)
	if !ok {
		return "", fmt.Errorf("invalid response format")
	}

	return p.parseTextResponse(chatResp)
}

func (p *AnthropicProvider) ChatWithHistory(ctx context.Context, messages []types.ChatMessage, model string, temperature float64) (string, error) {
	if p.apiKey == "" {
		return "", fmt.Errorf("Anthropic credentials not set. Set ANTHROPIC_API_KEY")
	}

	_, nativeMessages := p.convertMessages(messages)

	req := AnthropicChatRequest{
		Model:       model,
		MaxTokens:   4096,
		Messages:    nativeMessages,
		Temperature: temperature,
	}

	resp, err := p.doRequest(ctx, "/v1/messages", req)
	if err != nil {
		return "", err
	}

	chatResp, ok := resp.(AnthropicChatResponse)
	if !ok {
		return "", fmt.Errorf("invalid response format")
	}

	return p.parseTextResponse(chatResp)
}

func (p *AnthropicProvider) Chat(ctx context.Context, request *ChatRequest, model string, temperature float64) (*types.ChatResponse, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("Anthropic credentials not set. Set ANTHROPIC_API_KEY")
	}

	systemPrompt, nativeMessages := p.convertMessages(request.Messages)

	tools := p.convertToolSpecs(request.Tools)

	req := AnthropicChatRequest{
		Model:       model,
		MaxTokens:   4096,
		System:      systemPrompt,
		Messages:    nativeMessages,
		Temperature: temperature,
		Tools:       tools,
	}

	resp, err := p.doRequest(ctx, "/v1/messages", req)
	if err != nil {
		return nil, err
	}

	chatResp, ok := resp.(AnthropicChatResponse)
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	return p.parseResponse(chatResp), nil
}

func (p *AnthropicProvider) ChatWithTools(ctx context.Context, messages []types.ChatMessage, tools []json.RawMessage, model string, temperature float64) (*types.ChatResponse, error) {
	toolSpecs := make([]*types.ToolSpec, len(tools))
	for i, tool := range tools {
		var spec map[string]interface{}
		if err := json.Unmarshal(tool, &spec); err != nil {
			continue
		}
		if fn, ok := spec["function"].(map[string]interface{}); ok {
			name, _ := fn["name"].(string)
			description, _ := fn["description"].(string)
			params, _ := json.Marshal(fn["parameters"])
			toolSpecs[i] = &types.ToolSpec{
				Name:        name,
				Description: description,
				Parameters:  params,
			}
		}
	}

	request := &ChatRequest{
		Messages: messages,
		Tools:    toolSpecs,
	}
	return p.Chat(ctx, request, model, temperature)
}

func (p *AnthropicProvider) SupportsNativeTools() bool {
	return true
}

func (p *AnthropicProvider) SupportsVision() bool {
	return true
}

func (p *AnthropicProvider) SupportsStreaming() bool {
	return false
}

func (p *AnthropicProvider) StreamChatWithSystem(ctx context.Context, systemPrompt, message, model string, temperature float64, options types.StreamOptions) (<-chan types.StreamChunk, error) {
	ch := make(chan types.StreamChunk, 1)
	close(ch)
	return ch, nil
}

func (p *AnthropicProvider) StreamChatWithHistory(ctx context.Context, messages []types.ChatMessage, model string, temperature float64, options types.StreamOptions) (<-chan types.StreamChunk, error) {
	ch := make(chan types.StreamChunk, 1)
	close(ch)
	return ch, nil
}

func (p *AnthropicProvider) Warmup(ctx context.Context) error {
	if p.apiKey == "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/messages", strings.NewReader("{}"))
	if err != nil {
		return err
	}

	p.applyAuth(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (p *AnthropicProvider) doRequest(ctx context.Context, path string, req interface{}) (interface{}, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	p.applyAuth(httpReq)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Anthropic API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result AnthropicChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

func (p *AnthropicProvider) applyAuth(req *http.Request) {
	if strings.HasPrefix(p.apiKey, "sk-ant-oat01-") {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	} else {
		req.Header.Set("x-api-key", p.apiKey)
	}
}

func (p *AnthropicProvider) parseTextResponse(resp AnthropicChatResponse) (string, error) {
	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != nil {
			return *block.Text, nil
		}
	}
	return "", fmt.Errorf("no response from Anthropic")
}

func (p *AnthropicProvider) parseResponse(resp AnthropicChatResponse) *types.ChatResponse {
	var textParts []string
	var toolCalls []types.ToolCall

	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != nil {
			textParts = append(textParts, *block.Text)
		}
		if block.Type == "tool_use" {
			id := ""
			if block.ID != nil {
				id = *block.ID
			}
			name := ""
			if block.Name != nil {
				name = *block.Name
			}
			toolCalls = append(toolCalls, types.ToolCall{
				ID:        id,
				Name:      name,
				Arguments: block.Input,
			})
		}
	}

	var text *string
	if len(textParts) > 0 {
		combined := strings.Join(textParts, "\n")
		text = &combined
	}

	var usage *types.TokenUsage
	if resp.Usage != nil {
		usage = &types.TokenUsage{}
		if resp.Usage.InputTokens != nil {
			usage.InputTokens = uintPtr(uint64(*resp.Usage.InputTokens))
		}
		if resp.Usage.OutputTokens != nil {
			usage.OutputTokens = uintPtr(uint64(*resp.Usage.OutputTokens))
		}
	}

	return &types.ChatResponse{
		Text:      text,
		ToolCalls: toolCalls,
		Usage:     usage,
	}
}

func (p *AnthropicProvider) convertMessages(messages []types.ChatMessage) (interface{}, []AnthropicMessage) {
	var systemText string
	var nativeMessages []AnthropicMessage

	for _, msg := range messages {
		role := string(msg.Role)
		switch role {
		case "system":
			systemText = msg.Content
		case "assistant":
			nativeMessages = append(nativeMessages, p.convertAssistantMessage(msg.Content))
		case "tool":
			nativeMessages = append(nativeMessages, p.convertToolResultMessage(msg.Content))
		default:
			nativeMessages = append(nativeMessages, p.convertUserMessage(msg.Content))
		}
	}

	var system interface{}
	if systemText != "" {
		system = systemText
	}

	return system, nativeMessages
}

func (p *AnthropicProvider) convertUserMessage(content string) AnthropicMessage {
	return AnthropicMessage{
		Role: "user",
		Content: []AnthropicContentBlock{
			{Type: "text", Text: &content},
		},
	}
}

func (p *AnthropicProvider) convertAssistantMessage(content string) AnthropicMessage {
	var blocks []AnthropicContentBlock

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &data); err == nil {
		if text, ok := data["content"].(string); ok && text != "" {
			blocks = append(blocks, AnthropicContentBlock{Type: "text", Text: &text})
		}
		if tc, ok := data["tool_calls"].([]interface{}); ok {
			for _, tcItem := range tc {
				if tcMap, ok := tcItem.(map[string]interface{}); ok {
					id, _ := tcMap["id"].(string)
					name, _ := tcMap["name"].(string)
					args, _ := json.Marshal(tcMap["arguments"])
					blocks = append(blocks, AnthropicContentBlock{
						Type:  "tool_use",
						ID:    &id,
						Name:  &name,
						Input: args,
					})
				}
			}
		}
	}

	if len(blocks) == 0 {
		blocks = []AnthropicContentBlock{{Type: "text", Text: &content}}
	}

	return AnthropicMessage{
		Role:    "assistant",
		Content: blocks,
	}
}

func (p *AnthropicProvider) convertToolResultMessage(content string) AnthropicMessage {
	var toolUseID, result string

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &data); err == nil {
		if tcID, ok := data["tool_call_id"].(string); ok {
			toolUseID = tcID
		}
		if c, ok := data["content"].(string); ok {
			result = c
		}
	}

	return AnthropicMessage{
		Role: "user",
		Content: []AnthropicContentBlock{
			{
				Type:      "tool_result",
				ToolUseID: &toolUseID,
				Text:      &result,
			},
		},
	}
}

func (p *AnthropicProvider) convertToolSpecs(tools []*types.ToolSpec) []AnthropicToolSpec {
	if len(tools) == 0 {
		return nil
	}

	result := make([]AnthropicToolSpec, len(tools))
	for i, tool := range tools {
		result[i] = AnthropicToolSpec{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.Parameters,
		}
	}

	if len(result) > 0 {
		result[len(result)-1].CacheControl = &AnthropicCacheControl{Type: "ephemeral"}
	}

	return result
}
