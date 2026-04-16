package llm

import "testing"

func TestSanitizeAssistantContentRemovesThinkBlocks(t *testing.T) {
	input := "<think>internal reasoning</think>\n\nFinal answer"
	if got := sanitizeAssistantContent(input); got != "Final answer" {
		t.Fatalf("unexpected sanitized content: %q", got)
	}
}

func TestSanitizeAssistantContentKeepsRegularFormatting(t *testing.T) {
	input := "<thinking>hidden</thinking>\nLine 1\n\nLine 2"
	if got := sanitizeAssistantContent(input); got != "Line 1\n\nLine 2" {
		t.Fatalf("unexpected sanitized content: %q", got)
	}
}
