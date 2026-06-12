package contextcore

import (
	"reflect"
	"strings"
	"testing"
)

func TestItemConstructorsNormalizeAndApplyOptions(t *testing.T) {
	metadata := map[string]string{"origin": "session"}
	item := MemoryItem(" memory-1 ", " Recent memory ", " alpha  \n\n beta ", 1.4,
		WithPriority(7),
		WithRequired(true),
		WithEstimatedTokens(42),
		WithMetadata(metadata),
	)
	metadata["origin"] = "mutated"

	if item.ID != "memory-1" {
		t.Fatalf("id = %q, want trimmed id", item.ID)
	}
	if item.Source != SourceMemory {
		t.Fatalf("source = %q, want %q", item.Source, SourceMemory)
	}
	if item.Title != "Recent memory" {
		t.Fatalf("title = %q, want trimmed title", item.Title)
	}
	if item.Content != "alpha\n\n beta" {
		t.Fatalf("content = %q, want compact whitespace", item.Content)
	}
	if item.Relevance != 1 {
		t.Fatalf("relevance = %v, want clamped relevance", item.Relevance)
	}
	if item.Priority != 7 || !item.Required || item.EstimatedTokens != 42 {
		t.Fatalf("options not applied: %+v", item)
	}
	if item.Metadata["origin"] != "session" {
		t.Fatalf("metadata = %v, want copied metadata", item.Metadata)
	}
}

func TestSourceSpecificConstructors(t *testing.T) {
	items := []Item{
		RAGItem("rag", "", "doc", 0.5),
		SkillItem("skill", "", "skill", 0.5),
		ToolItem("tool", "", "tool", 0.5),
		PolicyItem("policy", "", "policy", 0.5),
		PreferenceItem("preference", "", "pref", 0.5),
	}
	got := make([]Source, 0, len(items))
	for _, item := range items {
		got = append(got, item.Source)
	}
	want := []Source{SourceRAG, SourceSkills, SourceTools, SourcePolicy, SourcePreferences}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sources = %v, want %v", got, want)
	}
}

func TestRenderCompactSectionsGroupsSelectedItems(t *testing.T) {
	result := Compile(Input{
		Request: " Make a plan. ",
		Items: []Item{
			MemoryItem("memory-1", "Recent memory", "User likes short updates.", 0.8),
			RAGItem("rag-1", "Doc chunk", "Relevant doc line.", 0.7),
			MemoryItem("memory-2", "", "Second memory.", 0.7),
			PolicyItem("policy-1", "Guardrails", "Stay local-first.", 0.9, WithRequired(true)),
		},
		Options: Options{Budget: Budget{MaxBytes: 2048, MaxTokens: 512}},
	})

	got := RenderCompactSections(result)
	want := strings.Join([]string{
		"[Request]\nMake a plan.",
		"[Policy]\nGuardrails:\nStay local-first.",
		"[Memory]\nRecent memory:\nUser likes short updates.\n\nSecond memory.",
		"[Retrieved Context]\nDoc chunk:\nRelevant doc line.",
	}, "\n\n")
	if got != want {
		t.Fatalf("rendered sections:\n%s\nwant:\n%s", got, want)
	}
}

func TestSummarizeSourceUsageAndRenderSummary(t *testing.T) {
	result := Result{
		Request: CompiledItem{Source: SourceRequest, Content: "Ask", UsedBytes: 3, UsedTokens: 1},
		Items: []CompiledItem{
			{Source: SourceMemory, Content: "First", UsedBytes: 5, UsedTokens: 2},
			{Source: SourceMemory, Content: "Second", UsedBytes: 6, UsedTokens: 3},
			{Source: SourceRAG, Content: "Doc", UsedBytes: 3, UsedTokens: 1},
		},
	}

	summaries := SummarizeSourceUsage(result)
	wantSummaries := []SourceUsageSummary{
		{Source: SourceRequest, Items: 1, Bytes: 3, Tokens: 1},
		{Source: SourceMemory, Items: 2, Bytes: 11, Tokens: 5},
		{Source: SourceRAG, Items: 1, Bytes: 3, Tokens: 1},
	}
	if !reflect.DeepEqual(summaries, wantSummaries) {
		t.Fatalf("summaries = %+v, want %+v", summaries, wantSummaries)
	}

	got := RenderSourceUsageSummary(result)
	want := strings.Join([]string{
		"request: 1 item, 3 bytes, 1 token",
		"memory: 2 items, 11 bytes, 5 tokens",
		"rag: 1 item, 3 bytes, 1 token",
	}, "\n")
	if got != want {
		t.Fatalf("rendered usage = %q, want %q", got, want)
	}
}
