package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/config"
	"yemaka/internal/memory"
	"yemaka/internal/models"
	"yemaka/internal/replay"
)

func TestChatPersistsReplayTraceWhenStoreAvailable(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	replayStore := replay.NewStore(filepath.Join(t.TempDir(), "replay"))
	service := &Service{
		Router:      models.NewRouter(cfg),
		Runtime:     staticRuntime{text: "local response"},
		Memory:      store,
		ReplayStore: replayStore,
	}

	err = service.Chat(ctx, ChatInput{Content: "hello with token=super-secret-value"}, func(Event) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	files, err := replayStore.List()
	if err != nil {
		t.Fatalf("replay List() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("replay files = %d, want 1: %+v", len(files), files)
	}
	trace, err := replayStore.Load(files[0].ID)
	if err != nil {
		t.Fatalf("replay Load() error = %v", err)
	}
	if trace.Schema != replay.TraceSchema {
		t.Fatalf("trace.Schema = %q, want %q", trace.Schema, replay.TraceSchema)
	}
	if trace.FinalResult.Status != "completed" {
		t.Fatalf("FinalResult.Status = %q, want completed", trace.FinalResult.Status)
	}
	if len(trace.ToolsCalled) != 0 {
		t.Fatalf("ToolsCalled = %+v, want no tool calls for chat-only run", trace.ToolsCalled)
	}
	if trace.Route.ShouldUseInternet {
		t.Fatalf("Route.ShouldUseInternet = true, want false for local chat default")
	}
	data, err := json.Marshal(trace)
	if err != nil {
		t.Fatalf("marshal trace: %v", err)
	}
	if strings.Contains(string(data), "super-secret-value") {
		t.Fatalf("replay trace leaked secret: %s", data)
	}
}

func TestConversationMessageActivitiesHydrateSourcesAndStepsFromReplay(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	replayStore := replay.NewStore(filepath.Join(t.TempDir(), "replay"))
	service := &Service{
		Router:      models.NewRouter(cfg),
		Runtime:     staticRuntime{text: "The docs say Yemaka is local-first."},
		Memory:      store,
		ReplayStore: replayStore,
	}

	var conversationID string
	err = service.Ask(ctx, AskInput{
		Content:          "Summarize the workspace docs.",
		WorkspaceContext: "README.md: Yemaka is local-first.\ndocs/status.md: beta-ready routing.",
		SourceKind:       "workspace",
		Sources:          []string{"README.md", "docs/status.md"},
	}, func(event Event) error {
		if event.Type == EventAgentCompleted {
			conversationID = event.Data["conversation_id"]
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if conversationID == "" {
		t.Fatal("conversation id was not emitted")
	}
	messages, err := store.ListConversationMessages(ctx, conversationID, 20)
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	activities, err := ConversationMessageActivities(ctx, store, replayStore, conversationID, messages)
	if err != nil {
		t.Fatalf("ConversationMessageActivities() error = %v", err)
	}
	var assistant memory.Message
	for _, msg := range memory.WithInferredResponseParents(messages) {
		if msg.Role == "assistant" {
			assistant = msg
			break
		}
	}
	if assistant.ID == "" {
		t.Fatal("assistant message was not saved")
	}
	activity := activities[assistant.ID]
	if activity.SourceKind != "workspace" {
		t.Fatalf("SourceKind = %q, want workspace", activity.SourceKind)
	}
	if strings.Join(activity.Sources, ",") != "README.md,docs/status.md" {
		t.Fatalf("Sources = %+v, want replay workspace sources", activity.Sources)
	}
	trace := strings.Join(activity.Trace, "\n")
	for _, want := range []string{"task:", "plan:", "executor: not_required", "model:", "workspace: 2 sources", "verification: pass"} {
		if !strings.Contains(trace, want) {
			t.Fatalf("trace missing %q:\n%s", want, trace)
		}
	}
}

func TestReplayPersistenceKeepsApprovalGateForRiskyRuns(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	replayStore := replay.NewStore(filepath.Join(t.TempDir(), "replay"))
	var modelRequests []models.ChatRequest
	toolRan := false
	service := &Service{
		Router:      models.NewRouter(cfg),
		Runtime:     recordingRuntime{text: "model should not run", requests: &modelRequests},
		Memory:      store,
		ReplayStore: replayStore,
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			toolRan = true
			return ExecutionResult{Status: "completed", Context: "should not run"}, nil
		},
	}

	var permission Event
	err = service.Ask(ctx, AskInput{Content: "Delete build output with rm -rf build"}, func(event Event) error {
		if event.Type == EventPermissionRequested {
			permission = event
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if toolRan {
		t.Fatal("tool executor ran before explicit approval")
	}
	if len(modelRequests) != 0 {
		t.Fatalf("model requests = %d, want 0 before approval", len(modelRequests))
	}
	if permission.Type != EventPermissionRequested {
		t.Fatal("permission request event was not emitted")
	}
	if permission.Data["requires_confirmation"] != "true" {
		t.Fatalf("requires_confirmation = %q, want true", permission.Data["requires_confirmation"])
	}

	files, err := replayStore.List()
	if err != nil {
		t.Fatalf("replay List() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("replay files = %d, want 1: %+v", len(files), files)
	}
	trace, err := replayStore.Load(files[0].ID)
	if err != nil {
		t.Fatalf("replay Load() error = %v", err)
	}
	if trace.FinalResult.Status != ExecutionNeedsConfirmation {
		t.Fatalf("FinalResult.Status = %q, want %q", trace.FinalResult.Status, ExecutionNeedsConfirmation)
	}
	if len(trace.PermissionsRequested) != 1 {
		t.Fatalf("PermissionsRequested = %+v, want one approval request", trace.PermissionsRequested)
	}
	if trace.PermissionsRequested[0].Status != "needs_confirmation" {
		t.Fatalf("permission status = %q, want needs_confirmation", trace.PermissionsRequested[0].Status)
	}
	if len(trace.ToolsCalled) != 0 {
		t.Fatalf("ToolsCalled = %+v, want none before approval", trace.ToolsCalled)
	}
}

func hasReplayToolConsideration(values []replay.ToolConsideration, name string, status string) bool {
	for _, value := range values {
		if value.Name == name && value.Status == status {
			return true
		}
	}
	return false
}
