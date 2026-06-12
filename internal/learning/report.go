package learning

import (
	"context"
	"fmt"
	"strings"
	"time"

	"yemaka/internal/extensions"
	"yemaka/internal/memory"
)

type Report struct {
	GeneratedAt           string                    `json:"generatedAt"`
	WorkflowSuccesses     int                       `json:"workflowSuccesses"`
	WorkflowFailures      int                       `json:"workflowFailures"`
	Corrections           int                       `json:"corrections"`
	ExtensionGenerations  int                       `json:"extensionGenerations"`
	RecentLearningMemory  []LearningMemory          `json:"recentLearningMemory"`
	ExtensionFailureTrend []extensions.FailureTrend `json:"extensionFailureTrend"`
	Suggestions           []string                  `json:"suggestions"`
	AutomaticTraining     bool                      `json:"automaticTraining"`
}

type LearningMemory struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Source     string `json:"source"`
	Snippet    string `json:"snippet"`
	Importance int    `json:"importance"`
	UpdatedAt  string `json:"updatedAt"`
}

func BuildReport(ctx context.Context, store *memory.Store, extensionStore *extensions.Store) (Report, error) {
	if store == nil {
		return Report{}, fmt.Errorf("memory store is required")
	}
	memories, err := store.ListMemories(ctx, 200)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		GeneratedAt:       time.Now().UTC().Format(time.RFC3339),
		AutomaticTraining: false,
	}
	for _, item := range memories {
		switch item.Kind {
		case KindWorkflowSuccess:
			report.WorkflowSuccesses++
		case KindWorkflowFailure:
			report.WorkflowFailures++
		case KindCorrection:
			report.Corrections++
		case KindRouteCorrection:
			report.Corrections++
		case "extension_generation":
			report.ExtensionGenerations++
		}
		if isLearningKind(item.Kind) && len(report.RecentLearningMemory) < 12 {
			report.RecentLearningMemory = append(report.RecentLearningMemory, LearningMemory{
				ID:         item.ID,
				Kind:       item.Kind,
				Source:     item.Source,
				Snippet:    compact(item.Content, 180),
				Importance: item.Importance,
				UpdatedAt:  item.UpdatedAt,
			})
		}
	}
	if extensionStore != nil {
		failures, err := extensionStore.Failures(10)
		if err != nil {
			return Report{}, err
		}
		report.ExtensionFailureTrend = failures
	}
	report.Suggestions = learningSuggestions(report)
	return report, nil
}

func isLearningKind(kind string) bool {
	switch kind {
	case KindWorkflowSuccess, KindWorkflowFailure, KindCorrection, KindRouteCorrection, KindJobRunSummary, "extension_generation":
		return true
	default:
		return false
	}
}

func learningSuggestions(report Report) []string {
	suggestions := []string{}
	if report.WorkflowSuccesses > 0 {
		suggestions = append(suggestions, "Create a reusable skill from repeated successful conversations with `yemaka skill create-from-session <conversation_id>`.")
	}
	if report.Corrections > 0 {
		suggestions = append(suggestions, "Use correction memories when improving skills with `yemaka skill improve-from-session <skill_name> <conversation_id>`.")
	}
	for _, failure := range report.ExtensionFailureTrend {
		if failure.Failures >= 2 {
			suggestions = append(suggestions, "Extension "+failure.Name+" has repeated failures; inspect it before reuse or regenerate a safer replacement.")
		}
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "No self-learning action is suggested yet; complete more local tasks first.")
	}
	return uniqueSuggestions(suggestions)
}

func uniqueSuggestions(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
