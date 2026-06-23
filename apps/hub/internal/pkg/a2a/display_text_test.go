package a2a

import "testing"

func TestSanitizeDisplayTextDropsStandaloneReasoning(t *testing.T) {
	text := "Okay, the user is asking for the weather today. I need to check the available Widget Skills first. Let me call jute_skill_list to see which skills are available."

	if got := sanitizeDisplayText(text); got != "" {
		t.Fatalf("sanitizeDisplayText() = %q, want empty", got)
	}
}

func TestSanitizeDisplayTextKeepsAnswerAfterReasoning(t *testing.T) {
	text := "Okay, the user asked for weather. I should call a tool.\n\nIt is 22 C and sunny."

	if got := sanitizeDisplayText(text); got != "It is 22 C and sunny." {
		t.Fatalf("sanitizeDisplayText() = %q", got)
	}
}
