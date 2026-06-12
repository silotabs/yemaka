package agent

import (
	"strings"
)

var modelThinkingOpenTags = []string{"<think>", "<thinking>", "<reasoning>"}
var modelThinkingCloseTags = []string{"</think>", "</thinking>", "</reasoning>"}

type modelThinkingFilter struct {
	pending    string
	inThinking bool
}

func (f *modelThinkingFilter) Filter(token string, final bool) string {
	if f == nil {
		return StripModelThinkingBlocks(token)
	}
	f.pending += token
	visible, rest, inThinking := consumeModelThinking(f.pending, f.inThinking, final)
	f.pending = rest
	f.inThinking = inThinking
	return visible
}

func StripModelThinkingBlocks(content string) string {
	var filter modelThinkingFilter
	return filter.Filter(content, true)
}

func consumeModelThinking(content string, inThinking bool, final bool) (string, string, bool) {
	var visible strings.Builder
	rest := content
	for {
		lower := strings.ToLower(rest)
		if inThinking {
			index, length := earliestTag(lower, modelThinkingCloseTags)
			if index < 0 {
				if final {
					return strings.TrimSpace(visible.String()), "", false
				}
				keep := suffixTagPrefixLength(lower, modelThinkingCloseTags)
				if keep == 0 && len(rest) <= maxTagLength(modelThinkingCloseTags) {
					keep = len(rest)
				}
				if keep == 0 {
					return visible.String(), "", true
				}
				return visible.String(), rest[len(rest)-keep:], true
			}
			rest = rest[index+length:]
			inThinking = false
			continue
		}
		index, length := earliestTag(lower, modelThinkingOpenTags)
		if index < 0 {
			if final {
				visible.WriteString(rest)
				return strings.TrimSpace(visible.String()), "", false
			}
			keep := suffixTagPrefixLength(lower, modelThinkingOpenTags)
			if keep == 0 {
				visible.WriteString(rest)
				return visible.String(), "", false
			}
			visible.WriteString(rest[:len(rest)-keep])
			return visible.String(), rest[len(rest)-keep:], false
		}
		visible.WriteString(rest[:index])
		rest = rest[index+length:]
		inThinking = true
	}
}

func earliestTag(content string, tags []string) (int, int) {
	bestIndex := -1
	bestLength := 0
	for _, tag := range tags {
		index := strings.Index(content, tag)
		if index < 0 {
			continue
		}
		if bestIndex < 0 || index < bestIndex {
			bestIndex = index
			bestLength = len(tag)
		}
	}
	return bestIndex, bestLength
}

func suffixTagPrefixLength(content string, tags []string) int {
	best := 0
	for _, tag := range tags {
		max := len(tag) - 1
		if max > len(content) {
			max = len(content)
		}
		for size := max; size > best; size-- {
			if strings.HasSuffix(content, tag[:size]) {
				best = size
				break
			}
		}
	}
	return best
}

func maxTagLength(tags []string) int {
	max := 0
	for _, tag := range tags {
		if len(tag) > max {
			max = len(tag)
		}
	}
	return max
}
