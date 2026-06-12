package agent

import (
	"strings"
	"testing"

	"yemaka/internal/memory"
)

func TestSplitMemoryContextSkipsNoisyWorkflowSuccess(t *testing.T) {
	results := []memory.MemorySearchResult{
		{
			ID:         "noisy",
			Kind:       "workflow_success",
			Content:    "Conversation: conv_1\nOutcome: success (completed)\nGoal: hello\nTools: agent_verifier(pass)\nLesson: retrieve this before similar tasks",
			Importance: 3,
			Source:     "workflow:conv_1",
		},
		{
			ID:         "useful",
			Kind:       "workflow_success",
			Content:    "Conversation: conv_2\nOutcome: success (completed)\nGoal: Run tests\nTools: run_tests(completed), agent_verifier(pass)\nLesson: tests passed",
			Importance: 3,
			Source:     "workflow:conv_2",
		},
		{
			ID:         "preference",
			Kind:       "preference",
			Content:    "Prefer concise answers.",
			Importance: 5,
			Source:     "user",
		},
	}

	profile, task, sources := splitMemoryContext(results)
	if !strings.Contains(profile, "Prefer concise answers") {
		t.Fatalf("profile = %q, want preference", profile)
	}
	if strings.Contains(task, "conv_1") || containsString(sources, "workflow:conv_1") {
		t.Fatalf("task = %q sources = %#v, noisy workflow should be skipped", task, sources)
	}
	if !strings.Contains(task, "conv_2") || !containsString(sources, "workflow:conv_2") {
		t.Fatalf("task = %q sources = %#v, useful workflow should be included", task, sources)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
