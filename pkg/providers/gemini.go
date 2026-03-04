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

const defaultGeminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"

type GeminiProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text string `json:"text,omitempty"`
}

type GeminiGenerateRequest struct {
	Contents          []GeminiContent `json:"contents"`
	SystemInstruction *GeminiContent  `json:"systemInstruction,omitempty"`
	GenerationConfig  GeminiGenConfig `json:"generationConfig"`
}

type GeminiGenConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens uint32  `json:"maxOutputTokens"`
}

type GeminiGenerateResponse struct {
	Candidates    []GeminiCandidate    `json:"candidates,omitempty"`
	Error         *GeminiError         `json:"error,omitempty"`
	UsageMetadata *GeminiUsageMetadata `json:"usageMetadata,omitempty"`
}

type GeminiCandidate struct {
	Content *GeminiCandidateContent `json:"content,omitempty"`
}

type GeminiCandidateContent struct {
	Parts []GeminiResponsePart `json:"parts"`
}

type GeminiResponsePart struct {
	Text    string `json:"text,omitempty"`
	Thought bool   `json:"thought,omitempty"`
}

type GeminiError struct {
	Message string `json:"message"`
}

type GeminiUsageMetadata struct {
	PromptTokenCount     *int64 `json:"promptTokenCount,omitempty"`
	CandidatesTokenCount *int64 `json:"candidatesTokenCount,omitempty"`
}

func NewGeminiProvider(apiKey string) *GeminiProvider {
	return NewGeminiProviderWithBaseURL("", apiKey)
}

func NewGeminiProviderWithBaseURL(baseURL, apiKey string) *GeminiProvider {
	if baseURL == "" {
		baseURL = defaultGeminiBaseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &GeminiProvider{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 120},
	}
}

func (p *GeminiProvider) Name() string {
	return "gemini"
}

func (p *GeminiProvider) Capabilities() types.ProviderCapabilities {
	return types.ProviderCapabilities{
		NativeToolCalling: true,
		Vision:            true,
	}
}

func (p *GeminiProvider) ConvertTools(tools []*types.ToolSpec) *ConvertToolsResult {
	if len(tools) == 0 {
		return &ConvertToolsResult{
			Type:         "gemini",
			ToolsPayload: json.RawMessage("[]"),
		}
	}

	functionDeclarations := make([]map[string]interface{}, len(tools))
	for i, tool := range tools {
		functionDeclarations[i] = map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  tool.Parameters,
		}
	}

	toolsPayload, _ := json.Marshal(functionDeclarations)
	return &ConvertToolsResult{
		Type:         "gemini",
		ToolsPayload: toolsPayload,
	}
}

func (p *GeminiProvider) SimpleChat(ctx context.Context, message, model string, temperature float64) (string, error) {
	return p.ChatWithSystem(ctx, "", message, model, temperature)
}

func (p *GeminiProvider) ChatWithSystem(ctx context.Context, systemPrompt, message, model string, temperature float64) (string, error) {
	if p.apiKey == "" {
		return "", fmt.Errorf("Gemini API key not set. Set GEMINI_API_KEY")
	}

	contents := []GeminiContent{
		{
			Role:  "user",
			Parts: []GeminiPart{{Text: message}},
		},
	}

	var systemInstruction *GeminiContent
	if systemPrompt != "" {
		systemInstruction = &GeminiContent{
			Parts: []GeminiPart{{Text: systemPrompt}},
		}
	}

	req := GeminiGenerateRequest{
		Contents:          contents,
		SystemInstruction: systemInstruction,
		GenerationConfig: GeminiGenConfig{
			Temperature:     temperature,
			MaxOutputTokens: 2048,
		},
	}

	resp, err := p.doRequest(ctx, model+":generateContent", req)
	if err != nil {
		return "", err
	}

	genResp, ok := resp.(GeminiGenerateResponse)
	if !ok {
		return "", fmt.Errorf("invalid response format")
	}

	return p.parseTextResponse(genResp)
}

func (p *GeminiProvider) ChatWithHistory(ctx context.Context, messages []types.ChatMessage, model string, temperature float64) (string, error) {
	if p.apiKey == "" {
		return "", fmt.Errorf("Gemini API key not set. Set GEMINI_API_KEY")
	}

	contents := p.convertMessages(messages)

	req := GeminiGenerateRequest{
		Contents: contents,
		GenerationConfig: GeminiGenConfig{
			Temperature:     temperature,
			MaxOutputTokens: 2048,
		},
	}

	resp, err := p.doRequest(ctx, model+":generateContent", req)
	if err != nil {
		return "", err
	}

	genResp, ok := resp.(GeminiGenerateResponse)
	if !ok {
		return "", fmt.Errorf("invalid response format")
	}

	return p.parseTextResponse(genResp)
}

func (p *GeminiProvider) Chat(ctx context.Context, request *ChatRequest, model string, temperature float64) (*types.ChatResponse, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("Gemini API key not set. Set GEMINI_API_KEY")
	}

	contents := p.convertMessages(request.Messages)

	var systemInstruction *GeminiContent
	for _, msg := range request.Messages {
		if string(msg.Role) == "system" {
			systemInstruction = &GeminiContent{
				Parts: []GeminiPart{{Text: msg.Content}},
			}
			break
		}
	}

	req := GeminiGenerateRequest{
		Contents:          contents,
		SystemInstruction: systemInstruction,
		GenerationConfig: GeminiGenConfig{
			Temperature:     temperature,
			MaxOutputTokens: 2048,
		},
	}

	resp, err := p.doRequest(ctx, model+":generateContent", req)
	if err != nil {
		return nil, err
	}

	genResp, ok := resp.(GeminiGenerateResponse)
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	return p.parseResponse(genResp), nil
}

func (p *GeminiProvider) ChatWithTools(ctx context.Context, messages []types.ChatMessage, tools []json.RawMessage, model string, temperature float64) (*types.ChatResponse, error) {
	contents := p.convertMessages(messages)

	req := GeminiGenerateRequest{
		Contents: contents,
		GenerationConfig: GeminiGenConfig{
			Temperature:     temperature,
			MaxOutputTokens: 2048,
		},
	}

	resp, err := p.doRequest(ctx, model+":generateContent", req)
	if err != nil {
		return nil, err
	}

	genResp, ok := resp.(GeminiGenerateResponse)
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	return p.parseResponse(genResp), nil
}

func (p *GeminiProvider) SupportsNativeTools() bool {
	return true
}

func (p *GeminiProvider) SupportsVision() bool {
	return true
}

func (p *GeminiProvider) SupportsStreaming() bool {
	return false
}

func (p *GeminiProvider) StreamChatWithSystem(ctx context.Context, systemPrompt, message, model string, temperature float64, options types.StreamOptions) (<-chan types.StreamChunk, error) {
	ch := make(chan types.StreamChunk, 1)
	close(ch)
	return ch, nil
}

func (p *GeminiProvider) StreamChatWithHistory(ctx context.Context, messages []types.ChatMessage, model string, temperature float64, options types.StreamOptions) (<-chan types.StreamChunk, error) {
	ch := make(chan types.StreamChunk, 1)
	close(ch)
	return ch, nil
}

func (p *GeminiProvider) Warmup(ctx context.Context) error {
	if p.apiKey == "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models?key="+p.apiKey, nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (p *GeminiProvider) doRequest(ctx context.Context, path string, req interface{}) (interface{}, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s?key=%s", p.baseURL, path, p.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
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
		return nil, fmt.Errorf("Gemini API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result GeminiGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("Gemini API error: %s", result.Error.Message)
	}

	return result, nil
}

func (p *GeminiProvider) parseTextResponse(resp GeminiGenerateResponse) (string, error) {
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return "", fmt.Errorf("no response from Gemini")
	}

	var answerParts, thinkingParts []string
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Thought {
			thinkingParts = append(thinkingParts, part.Text)
		} else {
			answerParts = append(answerParts, part.Text)
		}
	}

	if len(answerParts) > 0 {
		return strings.Join(answerParts, ""), nil
	}
	if len(thinkingParts) > 0 {
		return strings.Join(thinkingParts, ""), nil
	}

	return "", fmt.Errorf("no response from Gemini")
}

func (p *GeminiProvider) parseResponse(resp GeminiGenerateResponse) *types.ChatResponse {
	text, _ := p.parseTextResponse(resp)

	var usage *types.TokenUsage
	if resp.UsageMetadata != nil {
		usage = &types.TokenUsage{}
		if resp.UsageMetadata.PromptTokenCount != nil {
			usage.InputTokens = uintPtr(uint64(*resp.UsageMetadata.PromptTokenCount))
		}
		if resp.UsageMetadata.CandidatesTokenCount != nil {
			usage.OutputTokens = uintPtr(uint64(*resp.UsageMetadata.CandidatesTokenCount))
		}
	}

	return &types.ChatResponse{
		Text:  &text,
		Usage: usage,
	}
}

func (p *GeminiProvider) convertMessages(messages []types.ChatMessage) []GeminiContent {
	contents := make([]GeminiContent, 0, len(messages))

	for _, msg := range messages {
		role := string(msg.Role)
		if role == "system" {
			continue
		}

		contents = append(contents, GeminiContent{
			Role:  role,
			Parts: []GeminiPart{{Text: msg.Content}},
		})
	}

	return contents
}
