package prompt

import "strings"

func BuildUserPrompt(taskPlan string, sections []Section, userRequest string, options Options) Result {
	options = applyDefaults(options)
	normalized := normalizeSections(sections)

	var original strings.Builder
	writeBlock(&original, "TASK PLAN", strings.TrimSpace(taskPlan))
	for _, section := range normalized {
		writeBlock(&original, section.Name, section.Content)
	}
	writeBlock(&original, "USER REQUEST", strings.TrimSpace(userRequest))
	originalText := strings.TrimSpace(original.String())
	if options.Disabled || len(originalText) <= options.MaxChars {
		return Result{
			Content:        originalText,
			Stats:          statsForUnchanged(normalized),
			OriginalChars:  len(originalText),
			OptimizedChars: len(originalText),
		}
	}

	mandatory := strings.TrimSpace(block("TASK PLAN", strings.TrimSpace(taskPlan)) + "\n\n" + block("USER REQUEST", strings.TrimSpace(userRequest)))
	remaining := options.MaxChars - len(mandatory) - 2
	if remaining <= 0 {
		content := trimMiddle(mandatory, options.MaxChars)
		return Result{
			Content:        content,
			OriginalChars:  len(originalText),
			OptimizedChars: len(content),
			Truncated:      len(content) < len(originalText),
		}
	}

	allocations := allocateSections(normalized, remaining, options)
	var builder strings.Builder
	writeBlock(&builder, "TASK PLAN", strings.TrimSpace(taskPlan))
	stats := make([]SectionStat, 0, len(normalized))
	for index, section := range normalized {
		limit := allocations[index]
		if limit <= 0 {
			stats = append(stats, SectionStat{Name: section.Name, OriginalChars: len(section.Content), Truncated: len(section.Content) > 0})
			continue
		}
		content := trimMiddle(section.Content, limit)
		writeBlock(&builder, section.Name, content)
		stats = append(stats, SectionStat{
			Name:          section.Name,
			OriginalChars: len(section.Content),
			UsedChars:     len(content),
			Truncated:     len(content) < len(section.Content),
		})
	}
	writeBlock(&builder, "USER REQUEST", strings.TrimSpace(userRequest))
	content := strings.TrimSpace(builder.String())
	if len(content) > options.MaxChars {
		content = trimMiddle(content, options.MaxChars)
	}
	return Result{
		Content:        content,
		Stats:          stats,
		OriginalChars:  len(originalText),
		OptimizedChars: len(content),
		Truncated:      len(content) < len(originalText),
	}
}
