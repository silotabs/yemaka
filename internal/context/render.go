package contextcore

import (
	"fmt"
	"strings"
)

type Section struct {
	Source  Source
	Title   string
	Content string
}

type SourceUsageSummary struct {
	Source Source
	Items  int
	Bytes  int
	Tokens int
}

func CompactSections(result Result) []Section {
	sections := make([]Section, 0, len(result.Items)+1)
	sectionIndexes := map[Source]int{}

	add := func(item CompiledItem, includeItemTitle bool) {
		content := renderCompiledItem(item, includeItemTitle)
		if content == "" {
			return
		}
		source := normalizeSource(item.Source)
		if index, ok := sectionIndexes[source]; ok {
			sections[index].Content += "\n\n" + content
			return
		}
		sectionIndexes[source] = len(sections)
		sections = append(sections, Section{
			Source:  source,
			Title:   SourceTitle(source),
			Content: content,
		})
	}

	add(result.Request, false)
	for _, item := range result.Items {
		add(item, true)
	}
	return sections
}

func RenderCompactSections(result Result) string {
	sections := CompactSections(result)
	if len(sections) == 0 {
		return ""
	}
	rendered := make([]string, 0, len(sections))
	for _, section := range sections {
		rendered = append(rendered, "["+section.Title+"]\n"+section.Content)
	}
	return strings.Join(rendered, "\n\n")
}

func SourceTitle(source Source) string {
	switch normalizeSource(source) {
	case SourceRequest:
		return "Request"
	case SourceTaskState:
		return "Task State"
	case SourcePolicy:
		return "Policy"
	case SourcePreferences:
		return "Preferences"
	case SourceSkills:
		return "Skills"
	case SourceTools:
		return "Tools"
	case SourceMemory:
		return "Memory"
	case SourceKnowledge:
		return "Knowledge"
	case SourceRAG:
		return "Retrieved Context"
	case SourceUnknown:
		return "Context"
	default:
		value := strings.TrimSpace(string(source))
		if value == "" {
			return "Context"
		}
		return value
	}
}

func SummarizeSourceUsage(result Result) []SourceUsageSummary {
	summaries := map[Source]SourceUsageSummary{}
	order := make([]Source, 0, len(result.Items)+1)

	add := func(item CompiledItem) {
		if strings.TrimSpace(item.Content) == "" && item.UsedBytes <= 0 && item.UsedTokens <= 0 {
			return
		}
		source := normalizeSource(item.Source)
		summary, ok := summaries[source]
		if !ok {
			order = append(order, source)
			summary.Source = source
		}
		summary.Items++
		summary.Bytes += item.UsedBytes
		summary.Tokens += item.UsedTokens
		summaries[source] = summary
	}

	add(result.Request)
	for _, item := range result.Items {
		add(item)
	}

	output := make([]SourceUsageSummary, 0, len(order))
	for _, source := range order {
		output = append(output, summaries[source])
	}
	return output
}

func RenderSourceUsageSummary(result Result) string {
	summaries := SummarizeSourceUsage(result)
	if len(summaries) == 0 {
		return ""
	}
	lines := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		itemLabel := "items"
		if summary.Items == 1 {
			itemLabel = "item"
		}
		byteLabel := "bytes"
		if summary.Bytes == 1 {
			byteLabel = "byte"
		}
		tokenLabel := "tokens"
		if summary.Tokens == 1 {
			tokenLabel = "token"
		}
		lines = append(lines, fmt.Sprintf("%s: %d %s, %d %s, %d %s", summary.Source, summary.Items, itemLabel, summary.Bytes, byteLabel, summary.Tokens, tokenLabel))
	}
	return strings.Join(lines, "\n")
}

func renderCompiledItem(item CompiledItem, includeTitle bool) string {
	content := CompactWhitespace(item.Content)
	if content == "" {
		return ""
	}
	if !includeTitle {
		return content
	}
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return content
	}
	return title + ":\n" + content
}
