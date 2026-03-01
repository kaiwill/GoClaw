package providers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/zeroclaw-labs/goclaw/pkg/types"
)

const (
	bedrockEndpointPrefix = "bedrock-runtime"
	bedrockSigningService = "bedrock"
	defaultRegion         = "us-east-1"
	awsAlgorithm          = "AWS4-HMAC-SHA256"
	awsRequest            = "aws4_request"
)

type BedrockProvider struct {
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
	region          string
	httpClient      *http.Client
}

type BedrockConverseRequest struct {
	Messages      []BedrockMessage `json:"messages"`
	InferenceConfig struct {
		MaxTokens     int32   `json:"maxTokens,omitempty"`
		Temperature   float32 `json:"temperature,omitempty"`
		TopP          float32 `json:"topP,omitempty"`
		StopSequences []string `json:"stopSequences,omitempty"`
	} `json:"inferenceConfig"`
	Tools []BedrockTool `json:"tools,omitempty"`
}

type BedrockMessage struct {
	Role    string           `json:"role"`
	Content []BedrockContent `json:"content"`
}

type BedrockContent struct {
	Text  *string `json:"text,omitempty"`
	Image *struct {
		Source struct {
			Type     string `json:"type"`
			Bytes    []byte `json:"bytes,omitempty"`
			MimeType string `json:"mediaType,omitempty"`
		} `json:"source"`
	} `json:"image,omitempty"`
	ToolUse *struct {
		ToolID string                 `json:"toolUseId"`
		Input  map[string]interface{} `json:"input"`
		Name   string                 `json:"name"`
	} `json:"toolUse,omitempty"`
	ToolResult *struct {
		ToolUseID string `json:"toolUseId"`
		Content   string `json:"content"`
		Status    string `json:"status"`
	} `json:"toolResult,omitempty"`
}

type BedrockTool struct {
	ToolSpec struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		InputSchema map[string]interface{} `json:"inputSchema"`
	} `json:"toolSpec"`
}

type BedrockConverseResponse struct {
	Output struct {
		Message struct {
			Role    string          `json:"role"`
			Content []BedrockContent `json:"content"`
		} `json:"message"`
	} `json:"output"`
	StopReason string `json:"stopReason"`
	Usage      struct {
		InputTokens  int `json:"inputTokens"`
		OutputTokens int `json:"outputTokens"`
		TotalTokens  int `json:"totalTokens"`
	} `json:"usage"`
}

type BedrockErrorResponse struct {
	Message string `json:"message"`
}

func NewBedrockProvider(accessKeyID, secretAccessKey, sessionToken, region string) *BedrockProvider {
	if region == "" {
		region = defaultRegion
	}

	return &BedrockProvider{
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
		sessionToken:    sessionToken,
		region:          region,
		httpClient:      &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *BedrockProvider) Name() string {
	return "bedrock"
}

func (p *BedrockProvider) Chat(ctx context.Context, request *ChatRequest, model string, temperature float64) (*types.ChatResponse, error) {
	bedrockReq := p.buildRequest(request, model, temperature)

	body, err := json.Marshal(bedrockReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("https://%s.%s.amazonaws.com/model/%s/converse",
		bedrockEndpointPrefix, p.region, model)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	now := time.Now().UTC()
	if err := p.signRequest(httpReq, body, now, model); err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		var errResp BedrockErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Message != "" {
			return nil, fmt.Errorf("Bedrock API error: %s", errResp.Message)
		}
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var bedrockResp BedrockConverseResponse
	if err := json.NewDecoder(resp.Body).Decode(&bedrockResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return p.parseResponse(&bedrockResp), nil
}

func (p *BedrockProvider) Capabilities() types.ProviderCapabilities {
	return types.ProviderCapabilities{
		NativeToolCalling: true,
		Vision:            true,
	}
}

func (p *BedrockProvider) buildRequest(request *ChatRequest, model string, temperature float64) *BedrockConverseRequest {
	bedrockReq := &BedrockConverseRequest{
		Messages: make([]BedrockMessage, 0, len(request.Messages)),
	}
	bedrockReq.InferenceConfig.Temperature = float32(temperature)
	bedrockReq.InferenceConfig.MaxTokens = 4096

	for _, msg := range request.Messages {
		content := []BedrockContent{{Text: &msg.Content}}
		bedrockReq.Messages = append(bedrockReq.Messages, BedrockMessage{
			Role:    string(msg.Role),
			Content: content,
		})
	}

	if len(request.Tools) > 0 {
		bedrockReq.Tools = make([]BedrockTool, 0, len(request.Tools))
		for _, tool := range request.Tools {
			var inputSchema map[string]interface{}
			if err := json.Unmarshal(tool.Parameters, &inputSchema); err == nil {
				bedrockReq.Tools = append(bedrockReq.Tools, BedrockTool{
					ToolSpec: struct {
						Name        string                 `json:"name"`
						Description string                 `json:"description"`
						InputSchema map[string]interface{} `json:"inputSchema"`
					}{
						Name:        tool.Name,
						Description: tool.Description,
						InputSchema: inputSchema,
					},
				})
			}
		}
	}

	return bedrockReq
}

func (p *BedrockProvider) parseResponse(resp *BedrockConverseResponse) *types.ChatResponse {
	inputTokens := uint64(resp.Usage.InputTokens)
	outputTokens := uint64(resp.Usage.OutputTokens)
	response := &types.ChatResponse{
		Usage: &types.TokenUsage{
			InputTokens:  &inputTokens,
			OutputTokens: &outputTokens,
		},
	}

	var contentBuilder strings.Builder
	for _, content := range resp.Output.Message.Content {
		if content.Text != nil {
			contentBuilder.WriteString(*content.Text)
		}
		if content.ToolUse != nil {
			response.ToolCalls = append(response.ToolCalls, types.ToolCall{
				ID:   content.ToolUse.ToolID,
				Name: content.ToolUse.Name,
				Arguments: func() json.RawMessage {
					if b, err := json.Marshal(content.ToolUse.Input); err == nil {
						return b
					}
					return json.RawMessage("{}")
				}(),
			})
		}
	}

	if contentBuilder.Len() > 0 {
		contentStr := contentBuilder.String()
		response.Text = &contentStr
	}

	return response
}

func (p *BedrockProvider) signRequest(req *http.Request, body []byte, now time.Time, model string) error {
	region := p.region
	service := bedrockSigningService
	dateStamp := now.Format("20060102")
	timeStamp := now.Format("20060102T150405Z")

	host := req.URL.Host

	hashedPayload := hex.EncodeToString(hashSHA256(body))

	canonicalHeaders := strings.Join([]string{
		"host:" + host,
		"x-amz-content-sha256:" + hashedPayload,
		"x-amz-date:" + timeStamp,
	}, "\n") + "\n"

	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	canonicalRequest := strings.Join([]string{
		req.Method,
		req.URL.RequestURI(),
		"",
		canonicalHeaders,
		signedHeaders,
		hashedPayload,
	}, "\n")

	credentialScope := strings.Join([]string{
		dateStamp,
		region,
		service,
		awsRequest,
	}, "/")

	stringToSign := strings.Join([]string{
		awsAlgorithm,
		timeStamp,
		credentialScope,
		hex.EncodeToString(hashSHA256([]byte(canonicalRequest))),
	}, "\n")

	signingKey := p.getSignatureKey(dateStamp, region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	authorizationHeader := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		awsAlgorithm,
		p.accessKeyID,
		credentialScope,
		signedHeaders,
		signature,
	)

	req.Header.Set("Host", host)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Amz-Date", timeStamp)
	req.Header.Set("X-Amz-Content-Sha256", hashedPayload)
	req.Header.Set("Authorization", authorizationHeader)

	if p.sessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", p.sessionToken)
	}

	return nil
}

func hashSHA256(data []byte) []byte {
	h := sha256.New()
	h.Write(data)
	return h.Sum(nil)
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func (p *BedrockProvider) getSignatureKey(date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+p.secretAccessKey), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, awsRequest)
	return kSigning
}

func (p *BedrockProvider) createCanonicalQuery(req *http.Request) string {
	var params []string
	for k, vals := range req.URL.Query() {
		for _, v := range vals {
			params = append(params, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	sort.Strings(params)
	return strings.Join(params, "&")
}