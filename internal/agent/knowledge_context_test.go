package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/config"
	contextcore "yemaka/internal/context"
	"yemaka/internal/knowledge"
	promptcore "yemaka/internal/prompt"
	"yemaka/internal/replay"
	"yemaka/internal/routing"
)

func TestKnowledgeContextProviderRequiresExplicitInfluence(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "memory.sqlite")
	store, err := knowledge.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	entity, err := store.UpsertEntity(ctx, knowledge.EntityInput{
		Name:     "Cogging torque",
		Kind:     "concept",
		Source:   "manual",
		Evidence: "Cogging torque is torque ripple from rotor and stator alignment.",
	})
	if err != nil {
		t.Fatalf("UpsertEntity() error = %v", err)
	}
	if _, err := store.ApplyReviewBatch(ctx, knowledge.ReviewBatchInput{
		Action:     "approve",
		ReviewedBy: "test",
		Targets:    []knowledge.ReviewTarget{{Type: "entity", ID: entity.ID}},
	}); err != nil {
		t.Fatalf("ApplyReviewBatch() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	cfg := config.Default()
	cfg.Memory.Database = dbPath
	cfg.Knowledge.Enabled = true
	cfg.Knowledge.InfluenceEnabled = false

	provider := NewLocalKnowledgeContextProvider(LocalKnowledgeContextConfig{Config: cfg})
	disabled, err := provider.RetrieveKnowledgeContext(ctx, "explain cogging torque")
	if err != nil {
		t.Fatalf("RetrieveKnowledgeContext disabled error = %v", err)
	}
	if strings.TrimSpace(disabled.Text) != "" || len(disabled.Sources) != 0 {
		t.Fatalf("disabled influence returned context: %+v", disabled)
	}

	cfg.Knowledge.InfluenceEnabled = true
	enabled, err := provider.RetrieveKnowledgeContext(ctx, "explain cogging torque")
	if err != nil {
		t.Fatalf("RetrieveKnowledgeContext enabled error = %v", err)
	}
	if !strings.Contains(enabled.Text, "Cogging torque") || !strings.Contains(enabled.Text, "Background only") {
		t.Fatalf("enabled influence missing approved background context:\n%s", enabled.Text)
	}
	if len(enabled.Sources) != 1 || enabled.Sources[0] != "kg:entity:"+entity.ID {
		t.Fatalf("Sources = %v, want approved entity source", enabled.Sources)
	}
}

func TestKnowledgeContextCompilesAsBackgroundWithoutChangingRoute(t *testing.T) {
	input := PlanInput{
		Content:          "Explain the concept of cogging torque.",
		KnowledgeContext: "Approved local knowledge graph context. Background only; not a current source of truth.\n- Cogging torque: torque ripple in electric motors.",
		KnowledgeSources: []string{"kg:entity:kgent_test"},
	}
	plan := BuildPlan(input)
	if plan.RouteCategory != routing.RouteChatExplanation {
		t.Fatalf("RouteCategory = %q, want chat explanation; plan=%+v", plan.RouteCategory, plan)
	}
	if plan.EvidencePolicy != EvidenceGeneralKnowledgeAllowed {
		t.Fatalf("EvidencePolicy = %q, want %q; plan=%+v", plan.EvidencePolicy, EvidenceGeneralKnowledgeAllowed, plan)
	}

	messages, compiled := PromptMessagesWithDiagnostics(plan, input, "system", promptcore.Options{MaxChars: 2800})
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(messages))
	}
	userPrompt := messages[1].Content
	if !strings.Contains(userPrompt, "APPROVED KNOWLEDGE GRAPH CONTEXT") {
		t.Fatalf("prompt missing knowledge context section:\n%s", userPrompt)
	}
	if !strings.Contains(userPrompt, "Background only; not a current source of truth") {
		t.Fatalf("prompt missing background-only guard:\n%s", userPrompt)
	}
	if !containsCompiledSource(compiled, "knowledge") {
		t.Fatalf("compiled sources = %+v, want knowledge source", compiled.Items)
	}
}

func TestKnowledgeContextDoesNotSatisfyCurrentNewsRoute(t *testing.T) {
	input := PlanInput{
		Content:          "Search latest updates about electric motor tariffs.",
		KnowledgeContext: "Approved local knowledge graph context. Background only; not a current source of truth.\n- Electric motor tariffs: older internal planning note.",
		KnowledgeSources: []string{"kg:entity:kgent_tariffs"},
	}
	plan := BuildPlan(input)
	if plan.RouteCategory != routing.RouteInternetSearch {
		t.Fatalf("RouteCategory = %q, want internet search; plan=%+v", plan.RouteCategory, plan)
	}
	if plan.EvidencePolicy != EvidenceFreshInternetRequired {
		t.Fatalf("EvidencePolicy = %q, want %q; plan=%+v", plan.EvidencePolicy, EvidenceFreshInternetRequired, plan)
	}
	messages := PromptMessages(plan, input, "system "+ContextGroundingGuidance()+AnswerContractGuidance(plan))
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(messages))
	}
	systemPrompt := messages[0].Content
	userPrompt := messages[1].Content
	if strings.Contains(userPrompt, "APPROVED KNOWLEDGE GRAPH CONTEXT") {
		t.Fatalf("current-news prompt included knowledge graph context:\n%s", userPrompt)
	}
	if !strings.Contains(systemPrompt, "not proof for routes that require local documents, workspace files, current internet evidence, or tool results") {
		t.Fatalf("system prompt missing KG current-evidence guard:\n%s", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "fresh internet/search evidence") {
		t.Fatalf("system prompt missing fresh internet evidence contract:\n%s", systemPrompt)
	}
}

func TestKnowledgeContextDoesNotEnterLocalDocumentRoute(t *testing.T) {
	input := PlanInput{
		Content:          "Answer from my indexed documents: explain cogging torque.",
		SourceKind:       "local_documents",
		WorkspaceContext: "Retrieved local document context about motor design.",
		Sources:          []string{"docs:motor-notes.md"},
		KnowledgeContext: "Approved local knowledge graph context. Background only; not a current source of truth.\n- Cogging torque: older graph note.",
		KnowledgeSources: []string{"kg:entity:kgent_cogging"},
	}
	plan := BuildPlan(input)
	if plan.EvidencePolicy != EvidenceLocalDocumentsRequired {
		t.Fatalf("EvidencePolicy = %q, want %q; plan=%+v", plan.EvidencePolicy, EvidenceLocalDocumentsRequired, plan)
	}
	messages, compiled := PromptMessagesWithDiagnostics(plan, input, "system "+ContextGroundingGuidance()+AnswerContractGuidance(plan), promptcore.Options{MaxChars: 2800})
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(messages))
	}
	userPrompt := messages[1].Content
	if strings.Contains(userPrompt, "APPROVED KNOWLEDGE GRAPH CONTEXT") {
		t.Fatalf("local-document prompt included knowledge graph context:\n%s", userPrompt)
	}
	if containsCompiledSource(compiled, "knowledge") {
		t.Fatalf("compiled sources = %+v, want no knowledge source", compiled.Items)
	}
}

func TestReplayTraceRecordsKnowledgeContextSources(t *testing.T) {
	input := PlanInput{
		Content:          "Explain the concept of cogging torque.",
		KnowledgeContext: "Approved local knowledge graph context.",
		KnowledgeSources: []string{"kg:entity:kgent_test"},
	}
	trace := ReplayTraceFromPlan(input, BuildPlan(input))
	if got := replayAttributeValue(trace.Attributes, "knowledge_sources"); got != "kg:entity:kgent_test" {
		t.Fatalf("knowledge_sources = %q, want kg source; attrs=%+v", got, trace.Attributes)
	}
	if !hasReplayDocumentSource(trace.MemoryUsed, "knowledge") {
		t.Fatalf("MemoryUsed = %+v, want knowledge source recorded", trace.MemoryUsed)
	}
}

func containsCompiledSource(result contextcore.Result, source string) bool {
	for _, item := range result.Items {
		if string(item.Source) == source {
			return true
		}
	}
	return false
}

func hasReplayDocumentSource(refs []replay.DocumentRef, source string) bool {
	for _, ref := range refs {
		if ref.Source == source {
			return true
		}
	}
	return false
}
