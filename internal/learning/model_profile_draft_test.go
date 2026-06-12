package learning

import (
	"context"
	"strings"
	"testing"

	"yemaka/internal/memory"
	"yemaka/internal/modelprofiles"
)

func TestDraftModelProfileFromConversationUsesReviewedTraitsWithoutTranscriptLeak(t *testing.T) {
	ctx := context.Background()
	store := testMemoryStore(t)
	conversation, err := store.CreateConversation(ctx, "Engineering concept")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "Explain private project cogging torque. api_key = sk-testsecretvalue1234567890",
	}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Model:          "small:2b",
		Content:        "Here is a concise explanation:\n- It is a torque ripple effect.\n- Use local context when available.\n\nDo you want a motor-design example?",
	}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}

	profile, report, err := DraftModelProfileFromConversation(ctx, store, ModelProfileDraftInput{
		ConversationID: conversation.ID,
		Name:           "Reviewed Engineering Helper",
	})
	if err != nil {
		t.Fatalf("DraftModelProfileFromConversation() error = %v", err)
	}
	if report.Schema != ModelProfileDraftSchema || report.Eligibility != "eligible" {
		t.Fatalf("report = %+v, want eligible schema", report)
	}
	if report.Readiness.Status != "ready" || report.Readiness.Score == 0 {
		t.Fatalf("readiness = %+v, want ready non-zero score", report.Readiness)
	}
	if profile.BaseModel != "small:2b" || profile.Metadata.RefinedFrom != conversation.ID {
		t.Fatalf("profile metadata = %+v", profile)
	}
	rendered, err := modelprofiles.RenderValidatedModelfile(profile)
	if err != nil {
		t.Fatalf("RenderValidatedModelfile() error = %v", err)
	}
	for _, leaked := range []string{"sk-testsecretvalue", "private project", "torque ripple effect"} {
		if strings.Contains(rendered, leaked) {
			t.Fatalf("draft leaked transcript content %q:\n%s", leaked, rendered)
		}
	}
	for _, want := range []string{"reviewed successful conversation", "concise", "structured", "clarifying", "local-first"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("draft missing %q:\n%s", want, rendered)
		}
	}
}

func TestDraftModelProfileFromConversationBlocksFailedToolRuns(t *testing.T) {
	ctx := context.Background()
	store := testMemoryStore(t)
	conversation, err := store.CreateConversation(ctx, "Search task")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{ConversationID: conversation.ID, Role: "user", Content: "search current news"}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{ConversationID: conversation.ID, Role: "assistant", Model: "small:2b", Content: "Search failed because provider is unavailable."}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}
	if _, err := store.SaveToolRun(ctx, memory.ToolRun{
		ConversationID: conversation.ID,
		ToolName:       "internet_search",
		Status:         "failed",
	}); err != nil {
		t.Fatalf("SaveToolRun() error = %v", err)
	}

	_, report, err := DraftModelProfileFromConversation(ctx, store, ModelProfileDraftInput{ConversationID: conversation.ID})
	if err == nil {
		t.Fatal("DraftModelProfileFromConversation() error = nil, want failed tool run rejection")
	}
	if report.Eligibility != "blocked" || report.FailedToolRuns != 1 {
		t.Fatalf("report = %+v, want blocked failed tool count", report)
	}
}

func TestDraftModelProfileFromConversationRequiresBaseModelWhenConversationHasNoAssistantModel(t *testing.T) {
	ctx := context.Background()
	store := testMemoryStore(t)
	conversation, err := store.CreateConversation(ctx, "Chat")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{ConversationID: conversation.ID, Role: "user", Content: "hello"}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{ConversationID: conversation.ID, Role: "assistant", Content: "hello back"}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}

	if _, _, err := DraftModelProfileFromConversation(ctx, store, ModelProfileDraftInput{ConversationID: conversation.ID}); err == nil {
		t.Fatal("DraftModelProfileFromConversation() error = nil, want base model requirement")
	}
	profile, report, err := DraftModelProfileFromConversation(ctx, store, ModelProfileDraftInput{
		ConversationID: conversation.ID,
		BaseModel:      "small:2b",
	})
	if err != nil {
		t.Fatalf("DraftModelProfileFromConversation(with base) error = %v", err)
	}
	if report.Eligibility != "eligible" || profile.BaseModel != "small:2b" {
		t.Fatalf("draft with base = profile %+v report %+v", profile, report)
	}
}
