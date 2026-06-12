package contextcore

import "strings"

// ItemOption applies small deterministic adjustments to an Item constructor.
type ItemOption func(*Item)

// NewItem creates a normalized generic context item.
func NewItem(source Source, id string, title string, content string, relevance float64, options ...ItemOption) Item {
	item := Item{
		ID:        strings.TrimSpace(id),
		Source:    normalizeSource(source),
		Title:     strings.TrimSpace(title),
		Content:   CompactWhitespace(content),
		Relevance: NormalizeRelevance(relevance),
	}
	for _, option := range options {
		if option != nil {
			option(&item)
		}
	}
	return item
}

func MemoryItem(id string, title string, content string, relevance float64, options ...ItemOption) Item {
	return NewItem(SourceMemory, id, title, content, relevance, options...)
}

func KnowledgeItem(id string, title string, content string, relevance float64, options ...ItemOption) Item {
	return NewItem(SourceKnowledge, id, title, content, relevance, options...)
}

func TaskStateItem(id string, title string, content string, relevance float64, options ...ItemOption) Item {
	return NewItem(SourceTaskState, id, title, content, relevance, options...)
}

func RAGItem(id string, title string, content string, relevance float64, options ...ItemOption) Item {
	return NewItem(SourceRAG, id, title, content, relevance, options...)
}

func SkillItem(id string, title string, content string, relevance float64, options ...ItemOption) Item {
	return NewItem(SourceSkills, id, title, content, relevance, options...)
}

func ToolItem(id string, title string, content string, relevance float64, options ...ItemOption) Item {
	return NewItem(SourceTools, id, title, content, relevance, options...)
}

func PolicyItem(id string, title string, content string, relevance float64, options ...ItemOption) Item {
	return NewItem(SourcePolicy, id, title, content, relevance, options...)
}

func PreferenceItem(id string, title string, content string, relevance float64, options ...ItemOption) Item {
	return NewItem(SourcePreferences, id, title, content, relevance, options...)
}

func WithPriority(priority int) ItemOption {
	return func(item *Item) {
		item.Priority = priority
	}
}

func WithRequired(required bool) ItemOption {
	return func(item *Item) {
		item.Required = required
	}
}

func WithEstimatedTokens(tokens int) ItemOption {
	return func(item *Item) {
		if tokens > 0 {
			item.EstimatedTokens = tokens
		}
	}
}

func WithMetadata(metadata map[string]string) ItemOption {
	return func(item *Item) {
		item.Metadata = copyMetadata(metadata)
	}
}
