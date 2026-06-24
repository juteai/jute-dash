package a2a

import "testing"

func TestSanitizeDisplayTextKeepsStandaloneReasoning(t *testing.T) {
	text := "Okay, the user is asking for the weather today. I need to check the available Widget Skills first. Let me call jute_skill_list to see which skills are available."

	if got := sanitizeDisplayText(text); got != text {
		t.Fatalf("sanitizeDisplayText() = %q, want %q", got, text)
	}
}

func TestSanitizeDisplayTextUnwrapsTaggedReasoning(t *testing.T) {
	text := "<think>I should not hide this.</think>\n\nHi there."

	if got := sanitizeDisplayText(text); got != "I should not hide this.\n\nHi there." {
		t.Fatalf("sanitizeDisplayText() = %q", got)
	}
}
