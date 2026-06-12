package prompt

import (
	"strings"
	"testing"
)

func TestBuildUserPromptPreservesEssentialSections(t *testing.T) {
	result := BuildUserPrompt("Goal: explain\nTask type: chat", []Section{
		{Name: "PROFILE MEMORY", Content: "Prefer concise answers.", Priority: 80},
		{Name: "RETRIEVED CONTEXT", Content: strings.Repeat("local context ", 800), Priority: 90},
		{Name: "SKILL INSTRUCTIONS", Content: "Use sources.", Priority: 85},
	}, "Explain Yemaka", Options{MaxChars: 2200, LowMemory: true})

	if !strings.Contains(result.Content, "TASK PLAN:") || !strings.Contains(result.Content, "USER REQUEST:") {
		t.Fatalf("prompt missing mandatory sections:\n%s", result.Content)
	}
	if !strings.Contains(result.Content, "PROFILE MEMORY:") || !strings.Contains(result.Content, "SKILL INSTRUCTIONS:") {
		t.Fatalf("prompt missing important sections:\n%s", result.Content)
	}
	if len(result.Content) > 2200 {
		t.Fatalf("prompt len = %d, want <= 2200", len(result.Content))
	}
	if !result.Truncated {
		t.Fatal("expected optimizer to truncate oversized context")
	}
}

func TestBuildUserPromptLeavesSmallPromptUnchanged(t *testing.T) {
	result := BuildUserPrompt("Goal: hello", []Section{
		{Name: "TASK MEMORY", Content: "Previous greeting.", Priority: 70},
	}, "Hello", Options{MaxChars: 6000})

	if result.Truncated {
		t.Fatal("small prompt should not be marked truncated")
	}
	if strings.Contains(result.Content, "trimmed for low-memory") {
		t.Fatalf("small prompt was unexpectedly trimmed:\n%s", result.Content)
	}
	if !strings.Contains(result.Content, "TASK MEMORY:\nPrevious greeting.") {
		t.Fatalf("task memory missing:\n%s", result.Content)
	}
}
