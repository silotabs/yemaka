package contextcore

import "strings"

const contextTruncationMarker = "\n...[truncated for context budget]...\n"

func DefaultCompress(item Item, _ Limit) (string, bool) {
	compacted := CompactWhitespace(item.Content)
	return compacted, compacted != item.Content
}

func DefaultTruncate(item Item, limit Limit) (string, bool) {
	trimmed := TrimToLimit(item.Content, limit)
	return trimmed, trimmed != strings.TrimSpace(item.Content)
}

func CompactWhitespace(input string) string {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")
	lines := strings.Split(input, "\n")
	clean := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			if !blank {
				clean = append(clean, "")
			}
			blank = true
			continue
		}
		clean = append(clean, line)
		blank = false
	}
	return strings.TrimSpace(strings.Join(clean, "\n"))
}

func TrimToLimit(input string, limit Limit) string {
	input = strings.TrimSpace(input)
	if input == "" || limit.fitsText(input) {
		return input
	}
	target := limit.targetBytes()
	if target < 0 {
		return ""
	}
	if target == 0 {
		target = len(input)
	}

	output := trimMiddleBytes(input, target)
	for output != "" && !limit.fitsText(output) {
		nextTarget := target * 4 / 5
		if nextTarget >= target {
			nextTarget = target - 1
		}
		target = nextTarget
		if target <= 0 {
			return ""
		}
		output = trimMiddleBytes(input, target)
	}
	return strings.TrimSpace(output)
}

func trimMiddleBytes(input string, maxBytes int) string {
	input = strings.TrimSpace(input)
	if maxBytes <= 0 {
		return ""
	}
	if len(input) <= maxBytes {
		return input
	}
	if maxBytes <= len(contextTruncationMarker)+24 {
		return safePrefixBytes(input, maxBytes)
	}
	head := (maxBytes - len(contextTruncationMarker)) * 2 / 3
	tail := maxBytes - len(contextTruncationMarker) - head
	return strings.TrimSpace(safePrefixBytes(input, head)) + contextTruncationMarker + strings.TrimSpace(safeSuffixBytes(input, tail))
}

func safePrefixBytes(input string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(input) <= maxBytes {
		return input
	}
	end := 0
	for index := range input {
		if index > maxBytes {
			break
		}
		end = index
	}
	if end == 0 {
		return ""
	}
	return input[:end]
}

func safeSuffixBytes(input string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(input) <= maxBytes {
		return input
	}
	start := len(input)
	for index := range input {
		if len(input)-index <= maxBytes {
			start = index
			break
		}
	}
	if start >= len(input) {
		return ""
	}
	return input[start:]
}
