package prompt

import (
	"fmt"
	"strings"
)

type Section struct {
	Name     string
	Content  string
	Priority int
}

type SectionStat struct {
	Name          string
	OriginalChars int
	UsedChars     int
	Truncated     bool
}

type Result struct {
	Content        string
	Stats          []SectionStat
	OriginalChars  int
	OptimizedChars int
	Truncated      bool
}

func writeBlock(builder *strings.Builder, name string, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	if builder.Len() > 0 {
		builder.WriteString("\n\n")
	}
	builder.WriteString(block(name, content))
}

func block(name string, content string) string {
	return fmt.Sprintf("%s:\n%s", strings.TrimSpace(name), strings.TrimSpace(content))
}

func statsForUnchanged(sections []Section) []SectionStat {
	stats := make([]SectionStat, 0, len(sections))
	for _, section := range sections {
		stats = append(stats, SectionStat{Name: section.Name, OriginalChars: len(section.Content), UsedChars: len(section.Content)})
	}
	return stats
}
