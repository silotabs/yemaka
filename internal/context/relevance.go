package contextcore

import (
	"math"
	"sort"
	"strings"
)

type rankedItem struct {
	Item
	originalIndex int
}

func DefaultSourcePriority(source Source) int {
	switch source {
	case SourceRequest:
		return 1000
	case SourceTaskState:
		return 930
	case SourcePolicy:
		return 950
	case SourcePreferences:
		return 850
	case SourceSkills:
		return 760
	case SourceTools:
		return 740
	case SourceKnowledge:
		return 700
	case SourceMemory:
		return 660
	case SourceRAG:
		return 620
	default:
		return 100
	}
}

func SourcePriority(source Source, overrides map[Source]int) int {
	if overrides != nil {
		if priority, ok := overrides[source]; ok {
			return priority
		}
	}
	return DefaultSourcePriority(source)
}

func NormalizeRelevance(score float64) float64 {
	if math.IsNaN(score) || math.IsInf(score, 0) {
		return 0
	}
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func sortRankedItems(items []rankedItem, sourcePriorities map[Source]int) {
	sort.SliceStable(items, func(i int, j int) bool {
		left := items[i]
		right := items[j]
		if left.Required != right.Required {
			return left.Required
		}
		leftSourcePriority := SourcePriority(left.Source, sourcePriorities)
		rightSourcePriority := SourcePriority(right.Source, sourcePriorities)
		if leftSourcePriority != rightSourcePriority {
			return leftSourcePriority > rightSourcePriority
		}
		if left.Priority != right.Priority {
			return left.Priority > right.Priority
		}
		if left.Relevance != right.Relevance {
			return left.Relevance > right.Relevance
		}
		leftTokens := left.EstimatedTokens
		if leftTokens <= 0 {
			leftTokens = EstimateTokens(left.Content)
		}
		rightTokens := right.EstimatedTokens
		if rightTokens <= 0 {
			rightTokens = EstimateTokens(right.Content)
		}
		if leftTokens != rightTokens {
			return leftTokens < rightTokens
		}
		if left.Source != right.Source {
			return left.Source < right.Source
		}
		if titleCompare := strings.Compare(left.Title, right.Title); titleCompare != 0 {
			return titleCompare < 0
		}
		if idCompare := strings.Compare(left.ID, right.ID); idCompare != 0 {
			return idCompare < 0
		}
		return left.originalIndex < right.originalIndex
	})
}
