package a2a

import (
	"regexp"
	"strings"
)

var hiddenReasoningBlocks = []*regexp.Regexp{
	regexp.MustCompile(`(?is)<think>.*?</think>`),
	regexp.MustCompile(`(?is)<thinking>.*?</thinking>`),
	regexp.MustCompile(`(?is)<reasoning>.*?</reasoning>`),
	regexp.MustCompile(`(?is)<scratchpad>.*?</scratchpad>`),
	regexp.MustCompile("(?is)```(?:thinking|reasoning|scratchpad)\\s+.*?```"),
}

func displayTextFromMessage(msg message) string {
	return displayTextFromParts(msg.Parts)
}

func displayTextFromOptionalMessage(msg *message) string {
	if msg == nil {
		return ""
	}
	return displayTextFromMessage(*msg)
}

func displayTextFromParts(parts []part) string {
	return sanitizeDisplayText(textFromParts(parts))
}

func sanitizeDisplayText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	for _, pattern := range hiddenReasoningBlocks {
		text = pattern.ReplaceAllString(text, "")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	paragraphs := splitParagraphs(text)
	for len(paragraphs) > 1 && looksLikeReasoningParagraph(paragraphs[0]) {
		paragraphs = paragraphs[1:]
	}
	return strings.TrimSpace(strings.Join(paragraphs, "\n\n"))
}

func splitParagraphs(text string) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	raw := strings.Split(normalized, "\n\n")
	paragraphs := make([]string, 0, len(raw))
	for _, paragraph := range raw {
		if trimmed := strings.TrimSpace(paragraph); trimmed != "" {
			paragraphs = append(paragraphs, trimmed)
		}
	}
	return paragraphs
}

func looksLikeReasoningParagraph(paragraph string) bool {
	lower := strings.ToLower(strings.TrimSpace(paragraph))
	if strings.HasPrefix(lower, "okay, the user") ||
		strings.HasPrefix(lower, "the user ") ||
		strings.HasPrefix(lower, "we need ") ||
		strings.HasPrefix(lower, "i need ") ||
		strings.HasPrefix(lower, "i should ") ||
		strings.HasPrefix(lower, "let me ") {
		return true
	}
	signals := 0
	for _, phrase := range []string{
		"the user",
		"i should",
		"i'll",
		"i will",
		"no need to",
		"need to call",
		"call any function",
		"call tools",
		"use the tool",
		"tool choice",
		"final answer",
	} {
		if strings.Contains(lower, phrase) {
			signals++
		}
	}
	return signals >= 2
}
