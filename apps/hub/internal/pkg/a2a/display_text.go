package a2a

import (
	"regexp"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

var visibleReasoningBlocks = []*regexp.Regexp{
	regexp.MustCompile(`(?is)<think>(.*?)</think>`),
	regexp.MustCompile(`(?is)<thinking>(.*?)</thinking>`),
	regexp.MustCompile(`(?is)<reasoning>(.*?)</reasoning>`),
	regexp.MustCompile(`(?is)<scratchpad>(.*?)</scratchpad>`),
	regexp.MustCompile("(?is)```(?:thinking|reasoning|scratchpad)\\s+(.*?)```"),
}

func sanitizeDisplayText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	for _, pattern := range visibleReasoningBlocks {
		text = pattern.ReplaceAllString(text, "$1")
	}
	return strings.TrimSpace(text)
}

func displayTextFromSDKMessage(msg *a2a.Message) string {
	if msg == nil {
		return ""
	}
	return displayTextFromSDKParts(msg.Parts)
}

func displayTextFromOptionalSDKMessage(msg *a2a.Message) string {
	return displayTextFromSDKMessage(msg)
}

func displayTextFromSDKParts(parts []*a2a.Part) string {
	return sanitizeDisplayText(textFromSDKParts(parts))
}

func textFromSDKParts(parts []*a2a.Part) string {
	var chunks []string
	for _, item := range parts {
		if item.Text() != "" {
			if text := strings.TrimSpace(item.Text()); text != "" {
				chunks = append(chunks, text)
			}
		}
	}
	return strings.Join(chunks, "\n\n")
}
