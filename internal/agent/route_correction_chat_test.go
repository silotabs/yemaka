package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/config"
	"yemaka/internal/learning"
	"yemaka/internal/memory"
	"yemaka/internal/models"
	"yemaka/internal/routing"
)

func TestChatRouteCorrectionProposalApprovalAndReuse(t *testing.T) {
	ctx := context.Background()
	store := openRouteCorrectionChatStore(t, ctx)
	defer store.Close()
	conversation, err := store.CreateConversation(ctx, "route correction")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "search @gmail.com from indexed document",
	}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "I would need internet search for that.",
	}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}

	service := &Service{Memory: store}
	output := ""
	err = service.Chat(ctx, ChatInput{
		ConversationID: conversation.ID,
		Content:        "No, I meant local ingested documents, not internet. Remember that.",
	}, collectRouteCorrectionOutput(&output))
	if err != nil {
		t.Fatalf("Chat(correction) error = %v", err)
	}
	if !strings.Contains(output, "Save this for next time?") {
		t.Fatalf("correction response = %q, want approval question", output)
	}
	if strings.Contains(output, "rag_search") || strings.Contains(output, "internet_fetch") {
		t.Fatalf("correction response exposed tool jargon: %q", output)
	}
	corrections, err := learning.ListRouteCorrections(ctx, store, 10)
	if err != nil {
		t.Fatalf("ListRouteCorrections() error = %v", err)
	}
	if len(corrections) != 1 {
		t.Fatalf("corrections = %+v, want one pending correction", corrections)
	}
	pending := corrections[0]
	if pending.ApprovalStatus != routing.RouteCorrectionStatusPending || pending.IntendedRouteCategory != routing.RouteRAGSearch {
		t.Fatalf("pending correction = %+v, want pending RAG correction", pending)
	}
	assertAgentStringContainsAll(t, pending.Tags, []string{"source:local_documents", "route:rag_search"})

	output = ""
	err = service.Chat(ctx, ChatInput{
		ConversationID: conversation.ID,
		Content:        "yes, save it",
	}, collectRouteCorrectionOutput(&output))
	if err != nil {
		t.Fatalf("Chat(approval) error = %v", err)
	}
	if !strings.Contains(output, "Saved.") {
		t.Fatalf("approval response = %q, want saved confirmation", output)
	}
	corrections, err = learning.ListRouteCorrections(ctx, store, 10)
	if err != nil {
		t.Fatalf("ListRouteCorrections() after approval error = %v", err)
	}
	if len(corrections) != 1 || corrections[0].ApprovalStatus != routing.RouteCorrectionStatusApproved {
		t.Fatalf("corrections after approval = %+v, want approved correction", corrections)
	}

	input := PlanInput{Content: "Search @gmail.com in ingested documents"}
	if err := service.attachMemoryContext(ctx, &input, nil); err != nil {
		t.Fatalf("attachMemoryContext() error = %v", err)
	}
	plan := BuildPlan(input)
	if plan.RouteCategory != routing.RouteRAGSearch || plan.TaskType != TaskRAG {
		t.Fatalf("plan=%+v, want learned RAG route", plan)
	}
	if !containsRouteSubstring(plan.RouteReasons, "approved route correction") {
		t.Fatalf("RouteReasons = %v, want approved correction reason", plan.RouteReasons)
	}
	if containsRouteSubstring(plan.ToolsNeeded, "internet_search") || containsRouteSubstring(plan.ToolsNeeded, "internet_fetch") {
		t.Fatalf("ToolsNeeded = %v, want no internet tools", plan.ToolsNeeded)
	}
}

func TestChatRouteCorrectionDoesNotSaveAmbiguousTeaching(t *testing.T) {
	ctx := context.Background()
	store := openRouteCorrectionChatStore(t, ctx)
	defer store.Close()
	conversation, err := store.CreateConversation(ctx, "ambiguous correction")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "check it",
	}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}

	service := &Service{Memory: store}
	output := ""
	err = service.Chat(ctx, ChatInput{
		ConversationID: conversation.ID,
		Content:        "That was wrong. Don't do that.",
	}, collectRouteCorrectionOutput(&output))
	if err != nil {
		t.Fatalf("Chat(ambiguous correction) error = %v", err)
	}
	if !strings.Contains(output, "What should Yemaka do for similar prompts") {
		t.Fatalf("ambiguous correction response = %q, want clarification", output)
	}
	corrections, err := learning.ListRouteCorrections(ctx, store, 10)
	if err != nil {
		t.Fatalf("ListRouteCorrections() error = %v", err)
	}
	if len(corrections) != 0 {
		t.Fatalf("corrections = %+v, want none for ambiguous teaching", corrections)
	}
}

func TestChatBareApprovalWithoutPendingCorrectionIsOrdinaryChat(t *testing.T) {
	ctx := context.Background()
	store := openRouteCorrectionChatStore(t, ctx)
	defer store.Close()
	service := &Service{
		Router:  models.NewRouter(config.Default()),
		Runtime: staticRuntime{text: "ordinary chat"},
		Memory:  store,
	}
	output := ""
	if err := service.Chat(ctx, ChatInput{Content: "yes, save it"}, collectRouteCorrectionOutput(&output)); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if !strings.Contains(output, "ordinary chat") {
		t.Fatalf("output = %q, want ordinary model response", output)
	}
	corrections, err := learning.ListRouteCorrections(ctx, store, 10)
	if err != nil {
		t.Fatalf("ListRouteCorrections() error = %v", err)
	}
	if len(corrections) != 0 {
		t.Fatalf("corrections = %+v, want no correction from bare approval", corrections)
	}
}

func TestRouteCorrectionChatDoesNotHijackFactualFileStateComplaint(t *testing.T) {
	ctx := context.Background()
	store := openRouteCorrectionChatStore(t, ctx)
	defer store.Close()
	conversation, err := store.CreateConversation(ctx, "factual file state")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	service := &Service{Memory: store}
	handled, err := service.handleRouteCorrectionChat(
		ctx,
		conversation.ID,
		"",
		"I can not find /users/silo/documents/tavily-implementation.md. check the folder and make sure the file was created",
		"",
		"",
		toolRunMetadata{SessionID: newToolRunSessionID()},
		func(Event) error { return nil },
	)
	if err != nil {
		t.Fatalf("handleRouteCorrectionChat() error = %v", err)
	}
	if handled {
		t.Fatal("handleRouteCorrectionChat() handled factual file-state complaint as a route correction")
	}
	corrections, err := learning.ListRouteCorrections(ctx, store, 10)
	if err != nil {
		t.Fatalf("ListRouteCorrections() error = %v", err)
	}
	if len(corrections) != 0 {
		t.Fatalf("corrections = %+v, want none for factual file-state complaint", corrections)
	}
}

func openRouteCorrectionChatStore(t *testing.T, ctx context.Context) *memory.Store {
	t.Helper()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	return store
}

func collectRouteCorrectionOutput(output *string) EventHandler {
	return func(event Event) error {
		if event.Type == EventModelToken {
			*output += event.Token
		}
		return nil
	}
}

func assertAgentStringContainsAll(t *testing.T, got []string, want []string) {
	t.Helper()
	for _, expected := range want {
		found := false
		for _, value := range got {
			if value == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("values = %v, want %q", got, expected)
		}
	}
}
