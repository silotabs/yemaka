package internet

import (
	"regexp"
	"strings"
)

var (
	scriptStylePattern = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	tagPattern         = regexp.MustCompile(`(?s)<[^>]+>`)
	spacePattern       = regexp.MustCompile(`\s+`)
)

func ExtractText(body string, contentType string) string {
	text := body
	if strings.Contains(strings.ToLower(contentType), "html") || strings.Contains(text, "<") {
		text = scriptStylePattern.ReplaceAllString(text, " ")
		text = tagPattern.ReplaceAllString(text, " ")
	}
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", `"`)
	text = strings.ReplaceAll(text, "&#39;", "'")
	return strings.TrimSpace(spacePattern.ReplaceAllString(text, " "))
}
