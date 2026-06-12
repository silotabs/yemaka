package prompt

import "strings"

func normalizeSections(sections []Section) []Section {
	normalized := make([]Section, 0, len(sections))
	for _, section := range sections {
		name := strings.TrimSpace(section.Name)
		content := strings.TrimSpace(section.Content)
		if name == "" || content == "" {
			continue
		}
		if section.Priority <= 0 {
			section.Priority = 50
		}
		section.Name = name
		section.Content = compactSectionWhitespace(content)
		normalized = append(normalized, section)
	}
	return normalized
}

func compactSectionWhitespace(input string) string {
	lines := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
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

func trimMiddle(input string, maxChars int) string {
	input = strings.TrimSpace(input)
	if maxChars <= 0 || len(input) <= maxChars {
		return input
	}
	marker := "\n\n...[trimmed for low-memory prompt budget]...\n\n"
	if maxChars <= len(marker)+80 {
		return strings.TrimSpace(input[:maxChars])
	}
	head := (maxChars - len(marker)) * 2 / 3
	tail := maxChars - len(marker) - head
	return strings.TrimSpace(input[:head]) + marker + strings.TrimSpace(input[len(input)-tail:])
}
