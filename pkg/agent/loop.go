// Package agent provides the core agent functionality for GoClaw.
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zeroclaw-labs/goclaw/pkg/providers"
	"github.com/zeroclaw-labs/goclaw/pkg/tools"
	"github.com/zeroclaw-labs/goclaw/pkg/types"
)

// Constants for loop configuration.
const (
	// DefaultMaxToolIterations is the default maximum number of tool iterations.
	DefaultMaxToolIterations = 15
	// DefaultMaxHistoryMessages is the default maximum number of history messages.
	DefaultMaxHistoryMessages = 50
	// CompactionKeepRecentMessages is the number of recent messages to keep after compaction.
	CompactionKeepRecentMessages = 20
	// CompactionMaxSourceChars is the maximum number of characters for compaction source.
	CompactionMaxSourceChars = 12000
	// CompactionMaxSummaryChars is the maximum number of characters for compaction summary.
	CompactionMaxSummaryChars = 2000
	// ProgressMinIntervalMS is the minimum interval between progress sends.
	ProgressMinIntervalMS = 500
	// DraftClearSentinel is the sentinel value to clear draft text.
	DraftClearSentinel = "\x00CLEAR\x00"
)

// ToolCallLoop executes the tool call loop.
func (a *Agent) ToolCallLoop(ctx context.Context, message string) (*types.ChatResponse, error) {
	// Build initial prompt
	prompt, err := a.buildPrompt(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Initialize loop state
	iterations := 0
	maxIterations := a.config.MaxToolIterations
	if maxIterations == 0 {
		maxIterations = DefaultMaxToolIterations
	}

	// Conversation history
	history := []types.ChatMessage{
		{Role: types.RoleSystem, Content: prompt},
		{Role: types.RoleUser, Content: message},
	}

	// Loop until no more tool calls or max iterations reached
	for iterations < maxIterations {
		// Call LLM
		response, err := a.provider.Chat(ctx, &providers.ChatRequest{
			Messages: history,
			Tools:    a.toolSpecs,
		}, a.modelName, a.temperature)
		if err != nil {
			return nil, fmt.Errorf("LLM call failed: %w", err)
		}

		// Check if there are tool calls
		if !response.HasToolCalls() {
			// No more tool calls, return response
			return response, nil
		}

		// Execute tools
		toolResults, err := a.executeTools(ctx, response.ToolCalls)
		if err != nil {
			return nil, fmt.Errorf("tool execution failed: %w", err)
		}

		// Add tool results to history
		for _, result := range toolResults {
			history = append(history, types.ChatMessage{
				Role:    types.RoleTool,
				Content: result.Output,
			})
		}

		iterations++
	}

	// Max iterations reached
	return nil, fmt.Errorf("tool call loop exceeded maximum iterations: %d", maxIterations)
}

// buildPrompt builds the initial prompt for the agent.
func (a *Agent) buildPrompt(ctx context.Context, message string) (string, error) {
	// Build context
	context, err := a.buildContext(ctx, message)
	if err != nil {
		return "", fmt.Errorf("failed to build context: %w", err)
	}

	// Build system prompt
	prompt := a.promptBuilder.Build(context, message)
	return prompt, nil
}

// executeTools executes multiple tool calls.
func (a *Agent) executeTools(ctx context.Context, toolCalls []types.ToolCall) ([]ToolExecutionResult, error) {
	results := make([]ToolExecutionResult, len(toolCalls))

	for i, call := range toolCalls {
		result, err := a.executeTool(ctx, call)
		if err != nil {
			return nil, fmt.Errorf("failed to execute tool %s: %w", call.Name, err)
		}
		results[i] = result
	}

	return results, nil
}

// executeTool executes a single tool call.
func (a *Agent) executeTool(ctx context.Context, call types.ToolCall) (ToolExecutionResult, error) {
	// Find the tool
	var tool tools.Tool
	for _, t := range a.tools {
		if t.Name() == call.Name {
			tool = t
			break
		}
	}

	if tool == nil {
		return ToolExecutionResult{
			ToolCallID: call.ID,
			Output:     fmt.Sprintf("Unknown tool: %s", call.Name),
			Success:    false,
			Error:      fmt.Sprintf("tool %s not found", call.Name),
		}, nil
	}

	// Execute the tool
	var args map[string]interface{}
	json.Unmarshal(call.Arguments, &args)
	result, err := tool.Execute(ctx, args)
	if err != nil {
		return ToolExecutionResult{
			ToolCallID: call.ID,
			Output:     fmt.Sprintf("Error executing %s: %v", call.Name, err),
			Success:    false,
			Error:      err.Error(),
		}, nil
	}

	// Scrub credentials from output
	output := scrubCredentials(result.Output)

	return ToolExecutionResult{
		ToolCallID: call.ID,
		Output:     output,
		Success:    result.Success,
		Error:      result.Error,
	}, nil
}

// trimHistory trims the conversation history to prevent unbounded growth.
func (a *Agent) trimHistory(maxHistory int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Nothing to trim if within limit
	hasSystem := len(a.history) > 0 && a.history[0].Type == "chat" &&
		a.history[0].Chat != nil && a.history[0].Chat.Role == types.RoleSystem

	nonSystemCount := len(a.history)
	if hasSystem {
		nonSystemCount--
	}

	if nonSystemCount <= maxHistory {
		return
	}

	start := 0
	if hasSystem {
		start = 1
	}

	toRemove := nonSystemCount - maxHistory
	if toRemove > 0 {
		a.history = append(a.history[:start], a.history[start+toRemove:]...)
	}
}

// autoCompactHistory automatically compacts the conversation history.
func (a *Agent) autoCompactHistory(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	maxHistory := a.config.MaxHistoryMessages
	if maxHistory == 0 {
		maxHistory = DefaultMaxHistoryMessages
	}

	// Check if compaction is needed
	hasSystem := len(a.history) > 0 && a.history[0].Type == "chat" &&
		a.history[0].Chat != nil && a.history[0].Chat.Role == types.RoleSystem

	nonSystemCount := len(a.history)
	if hasSystem {
		nonSystemCount--
	}

	if nonSystemCount <= maxHistory {
		return nil
	}

	// Calculate how many messages to compact
	start := 0
	if hasSystem {
		start = 1
	}

	keepRecent := CompactionKeepRecentMessages
	if keepRecent > nonSystemCount {
		keepRecent = nonSystemCount
	}

	compactCount := nonSystemCount - keepRecent
	if compactCount <= 0 {
		return nil
	}

	compactEnd := start + compactCount

	// Extract messages to compact
	toCompact := make([]types.ChatMessage, compactCount)
	for i := 0; i < compactCount; i++ {
		if a.history[start+i].Chat != nil {
			toCompact[i] = *a.history[start+i].Chat
		}
	}

	// Build transcript
	transcript := buildCompactionTranscript(toCompact)

	// Call LLM to summarize
	summary, err := a.summarizeConversation(ctx, transcript)
	if err != nil {
		return fmt.Errorf("failed to summarize conversation: %w", err)
	}

	// Apply summary
	a.applyCompactionSummary(start, compactEnd, summary)

	return nil
}

// buildCompactionTranscript builds the transcript for compaction.
func buildCompactionTranscript(messages []types.ChatMessage) string {
	var transcript string
	for _, msg := range messages {
		role := msg.Role
		transcript += fmt.Sprintf("%s: %s\n", role, msg.Content)
	}

	// Truncate if too long
	if len(transcript) > CompactionMaxSourceChars {
		transcript = transcript[:CompactionMaxSourceChars] + "..."
	}

	return transcript
}

// summarizeConversation summarizes the conversation for compaction.
func (a *Agent) summarizeConversation(ctx context.Context, transcript string) (string, error) {
	summarizerSystem := "You are a conversation compaction engine. Summarize older chat history into concise context for future turns. Preserve: user preferences, commitments, decisions, unresolved tasks, key facts. Omit: filler, repeated chit-chat, verbose tool logs. Output plain text bullet points only."

	summarizerUser := fmt.Sprintf("Summarize the following conversation history for context preservation. Keep it short (max 12 bullet points).\n\n%s", transcript)

	response, err := a.provider.Chat(ctx, &providers.ChatRequest{
		Messages: []types.ChatMessage{
			{Role: types.RoleSystem, Content: summarizerSystem},
			{Role: types.RoleUser, Content: summarizerUser},
		},
	}, a.modelName, 0.2)
	if err != nil {
		return "", fmt.Errorf("LLM call failed: %w", err)
	}

	summary := response.TextOrEmpty()
	if len(summary) > CompactionMaxSummaryChars {
		summary = summary[:CompactionMaxSummaryChars] + "..."
	}

	return summary, nil
}

// applyCompactionSummary applies the compaction summary to the history.
func (a *Agent) applyCompactionSummary(start, compactEnd int, summary string) {
	// Replace compacted messages with summary
	summaryMsg := types.ConversationMessage{
		Type: "chat",
		Chat: &types.ChatMessage{
			Role:    types.RoleAssistant,
			Content: fmt.Sprintf("[Compaction summary]\n%s", summary),
		},
	}

	// Replace the range with the summary
	a.history = append(a.history[:start], append([]types.ConversationMessage{summaryMsg}, a.history[compactEnd:]...)...)
}

// truncateToolArgsForProgress truncates tool arguments for progress display.
func truncateToolArgsForProgress(name string, args map[string]interface{}, maxLen int) string {
	// TODO: Implement argument truncation
	return ""
}
