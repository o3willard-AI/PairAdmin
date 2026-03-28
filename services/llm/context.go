package llm

import (
	"fmt"
	"math"
)

// SystemPrompt is the locked system prompt for the terminal assistant.
const SystemPrompt = "You are a terminal assistant. The user shares their terminal output with you. Help them understand errors, suggest commands, and explain output."

// BuildMessages assembles the canonical message slice for an LLM request.
// If terminalContext is non-empty, it is prepended to the user input as a fenced code block.
// The message order is: [system, user].
func BuildMessages(systemPrompt, terminalContext, userInput string) []Message {
	userContent := userInput
	if terminalContext != "" {
		userContent = fmt.Sprintf("```terminal\n%s\n```\n\n%s", terminalContext, userInput)
	}

	return []Message{
		{Role: RoleSystem, Content: systemPrompt},
		{Role: RoleUser, Content: userContent},
	}
}

// EstimateTokens provides a fast approximation of token count based on character length.
// Uses the rule of thumb: ~4 characters per token (standard for English text).
func EstimateTokens(text string) int {
	return int(math.Ceil(float64(len(text)) / 4.0))
}
