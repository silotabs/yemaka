package contextcore

import (
	"strings"
	"testing"
)

func TestCompileOrdersGenericSourcesByDefaultPriority(t *testing.T) {
	result := Compile(Input{
		Request: "Explain the release blockers.",
		Items: []Item{
			{ID: "rag", Source: SourceRAG, Content: "Document chunk", Relevance: 1},
			{ID: "memory", Source: SourceMemory, Content: "Recent conversation memory", Relevance: 1},
			{ID: "tool", Source: SourceTools, Content: "Available tool contract", Relevance: 0.2},
			{ID: "skill", Source: SourceSkills, Content: "Relevant skill instructions", Relevance: 0.3},
			{ID: "preference", Source: SourcePreferences, Content: "User prefers concise answers", Relevance: 0.1},
			{ID: "policy", Source: SourcePolicy, Content: "Do not write files without approval", Relevance: 0.1},
		},
		Options: Options{Budget: Budget{MaxBytes: 2048, MaxTokens: 512}},
	})

	if result.Request.Source != SourceRequest {
		t.Fatalf("request source = %q, want %q", result.Request.Source, SourceRequest)
	}
	got := ids(result.Items)
	want := []string{"policy", "preference", "skill", "tool", "memory", "rag"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("item order = %v, want %v", got, want)
	}
}

func TestCompileSelectsMostRelevantItemsWithinBudget(t *testing.T) {
	result := Compile(Input{
		Request: "Need context",
		Items: []Item{
			{ID: "low", Source: SourceMemory, Content: strings.Repeat("low ", 15), Relevance: 0.1},
			{ID: "high", Source: SourceMemory, Content: strings.Repeat("high ", 12), Relevance: 0.95},
			{ID: "medium", Source: SourceMemory, Content: strings.Repeat("medium ", 12), Relevance: 0.5},
		},
		Options: Options{
			Budget: Budget{MaxBytes: 73, MinItemBytes: 30},
		},
	})

	if got := ids(result.Items); len(got) != 1 || got[0] != "high" {
		t.Fatalf("selected ids = %v, want only high relevance item", got)
	}
	if result.Stats.UsedBytes > 73 {
		t.Fatalf("used bytes = %d, want <= 73", result.Stats.UsedBytes)
	}
}

func TestCompileUsesDeterministicTruncationHook(t *testing.T) {
	truncateCalled := false
	result := Compile(Input{
		Request: "Short request",
		Items: []Item{
			{ID: "doc", Source: SourceRAG, Content: strings.Repeat("abcdef ", 40), Relevance: 1},
		},
		Options: Options{
			Budget: Budget{MaxBytes: 96, MinItemBytes: 1},
			Hooks: Hooks{
				Truncate: func(item Item, limit Limit) (string, bool) {
					truncateCalled = true
					return TrimToLimit(item.Content, limit), true
				},
			},
		},
	})

	if !truncateCalled {
		t.Fatal("truncate hook was not called")
	}
	if len(result.Items) != 1 {
		t.Fatalf("selected items = %d, want 1; dropped=%v", len(result.Items), result.Dropped)
	}
	if !result.Items[0].Truncated {
		t.Fatalf("compiled item was not marked truncated: %+v", result.Items[0])
	}
	if result.Stats.UsedBytes > 96 {
		t.Fatalf("used bytes = %d, want <= 96", result.Stats.UsedBytes)
	}
}

func TestCompileEnforcesTokenBudget(t *testing.T) {
	result := Compile(Input{
		Request: "alpha beta",
		Items: []Item{
			{ID: "first", Source: SourceMemory, Content: "one two three four", Relevance: 0.9},
			{ID: "second", Source: SourceMemory, Content: "five six seven eight", Relevance: 0.8},
		},
		Options: Options{
			Budget: Budget{MaxTokens: 8, MinItemTokens: 1},
		},
	})

	if result.Stats.UsedTokens > 8 {
		t.Fatalf("used tokens = %d, want <= 8", result.Stats.UsedTokens)
	}
	if got := ids(result.Items); len(got) != 1 || got[0] != "first" {
		t.Fatalf("selected ids = %v, want first item only", got)
	}
}

func TestCompileRespectsPerSourceBudget(t *testing.T) {
	result := Compile(Input{
		Request: "Summarize local context",
		Items: []Item{
			{ID: "memory", Source: SourceMemory, Content: strings.Repeat("memory ", 20), Relevance: 1},
			{ID: "rag", Source: SourceRAG, Content: strings.Repeat("rag ", 20), Relevance: 1},
		},
		Options: Options{
			Budget: Budget{
				MaxBytes:     240,
				MinItemBytes: 1,
				PerSource: map[Source]Limit{
					SourceMemory: {MaxBytes: 48},
				},
			},
		},
	})

	memoryUsage := result.Stats.BySource[SourceMemory]
	if memoryUsage.Bytes > 48 {
		t.Fatalf("memory bytes = %d, want <= 48", memoryUsage.Bytes)
	}
	if got := ids(result.Items); strings.Join(got, ",") != "memory,rag" {
		t.Fatalf("selected ids = %v, want memory and rag", got)
	}
	if !result.Items[0].Truncated {
		t.Fatalf("memory item should be truncated to per-source budget: %+v", result.Items[0])
	}
}

func ids(items []CompiledItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}
