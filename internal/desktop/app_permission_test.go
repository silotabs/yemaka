package desktop

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yemaka/internal/agent"
	"yemaka/internal/memory"
	"yemaka/internal/routing"
)

func TestRecordPermissionDecisionRejectsForgedMemoryWriteApproval(t *testing.T) {
	app := newDomainPackDesktopTestApp(t)

	_, err := app.RecordPermissionDecision(PermissionDecisionInput{
		Request: agent.PermissionRequest{
			RequestID:            "perm_desktop_forged_memory",
			ToolName:             "memory_write",
			Command:              []string{"memory_write", "follow_up", "Email Alex tomorrow"},
			RiskLevel:            agent.RiskMedium,
			Reason:               "active skill requested saving a confirmed local memory",
			RequiresConfirmation: true,
			NextStep:             "Review the exact memory kind and content.",
		},
		Decision: "approved",
	})
	if err == nil || !strings.Contains(err.Error(), "pending permission request") {
		t.Fatalf("RecordPermissionDecision() error = %v, want pending request rejection", err)
	}
	memories, err := app.store.SearchMemories(context.Background(), "Alex tomorrow", 5)
	if err != nil {
		t.Fatalf("SearchMemories() error = %v", err)
	}
	if len(memories) != 0 {
		t.Fatalf("forged approval wrote memories: %#v", memories)
	}
}

func TestRecordPermissionDecisionAppliesStoredEditProposal(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte("module desktop-edit-test\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "notes.md"), []byte("before\n"), 0o644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}
	t.Chdir(workspace)

	app := newDomainPackDesktopTestApp(t)
	app.profile.Snapshots = filepath.Join(app.profile.Root, "snapshots")
	ctx := context.Background()
	conversation, err := app.store.CreateConversation(ctx, "desktop permission")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	user, err := app.store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "Create notes.md",
		Model:          "yemaka",
	})
	if err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	assistant, err := app.store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "I can do this, but edit_file needs your approval first.",
		Model:          "yemaka-executor",
		ParentID:       user.ID,
	})
	if err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}
	request := agent.PermissionRequest{
		RequestID:            "perm_desktop_edit",
		ToolName:             "edit_file",
		RiskLevel:            agent.RiskMedium,
		Reason:               "file edits require confirmation and snapshot flow",
		RequiresConfirmation: true,
		WorkspaceOnly:        true,
		DiffPreview:          true,
		SnapshotBeforeWrite:  true,
		RollbackSupported:    true,
		NextStep:             "Review the diff preview, then approve to create a snapshot and apply the workspace-only edit.",
	}
	if _, err := app.store.SaveToolRun(ctx, memory.ToolRun{
		ConversationID:     conversation.ID,
		UserMessageID:      user.ID,
		AssistantMessageID: assistant.ID,
		ParentMessageID:    user.ID,
		VariantIndex:       assistant.VariantIndex,
		ToolName:           "permission_request",
		Input:              map[string]any{"request_id": request.RequestID, "tool_name": request.ToolName},
		Output:             request,
		Status:             agent.ExecutionNeedsConfirmation,
		RiskLevel:          request.RiskLevel,
	}); err != nil {
		t.Fatalf("SaveToolRun(permission_request) error = %v", err)
	}
	if _, err := app.store.SaveToolRun(ctx, memory.ToolRun{
		ConversationID:     conversation.ID,
		UserMessageID:      user.ID,
		AssistantMessageID: assistant.ID,
		ParentMessageID:    user.ID,
		VariantIndex:       assistant.VariantIndex,
		ToolName:           "edit_proposal",
		Input:              map[string]any{"request_id": request.RequestID, "path": "notes.md"},
		Output:             agent.EditProposal{RequestID: request.RequestID, Path: "notes.md", Content: "Permission approval works.\n", Status: "ready_for_preview", DiffPreview: true, SnapshotBeforeWrite: true, RollbackSupported: true},
		Status:             "ready_for_preview",
		RiskLevel:          agent.RiskMedium,
	}); err != nil {
		t.Fatalf("SaveToolRun(edit_proposal) error = %v", err)
	}
	routeState, err := json.Marshal(routing.SessionContract{
		ActiveRoute:            routing.RouteFileWrite,
		TaskStatus:             routing.TaskStatusAwaitingApproval,
		PendingStep:            "edit_file",
		PendingApproval:        "edit_file",
		PendingOperationID:     request.RequestID,
		PendingOperationType:   "edit_file",
		PendingOperationTarget: "notes.md",
		PendingOperationStatus: "awaiting_approval",
		RouteLockStrength:      "pending_operation",
		LastOutcome:            routing.LastOutcomeCompleted,
		UpdatedAt:              time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatalf("marshal route state: %v", err)
	}
	if _, err := app.store.SaveConversationRouteState(ctx, memory.ConversationRouteState{
		ConversationID: conversation.ID,
		StateJSON:      string(routeState),
	}); err != nil {
		t.Fatalf("SaveConversationRouteState() error = %v", err)
	}

	result, err := app.RecordPermissionDecision(PermissionDecisionInput{
		Request:  request,
		Decision: "approved",
	})
	if err != nil {
		t.Fatalf("RecordPermissionDecision() error = %v", err)
	}
	if !result.Executed || result.ToolName != "edit_file" || result.ToolStatus != "completed" {
		t.Fatalf("result = %+v, want executed completed edit_file", result)
	}
	content, err := os.ReadFile(filepath.Join(workspace, "notes.md"))
	if err != nil {
		t.Fatalf("read notes.md: %v", err)
	}
	if string(content) != "Permission approval works.\n" {
		t.Fatalf("notes.md = %q, want approved content", string(content))
	}
	stored, ok, err := app.store.GetConversationRouteState(ctx, conversation.ID)
	if err != nil {
		t.Fatalf("GetConversationRouteState() error = %v", err)
	}
	if !ok {
		t.Fatal("conversation route state missing")
	}
	var next routing.SessionContract
	if err := json.Unmarshal([]byte(stored.StateJSON), &next); err != nil {
		t.Fatalf("unmarshal route state: %v", err)
	}
	if next.PendingOperationID != "" || next.PendingApproval != "" || next.PendingStep != "" || next.RouteLockStrength == "pending_operation" {
		t.Fatalf("route state = %+v, want desktop permission approval to clear pending operation", next)
	}
}

func TestRecordPermissionDecisionRunsStoredMemoryWriteOnce(t *testing.T) {
	app := newDomainPackDesktopTestApp(t)
	request := agent.PermissionRequest{
		RequestID:            "perm_desktop_memory",
		ToolName:             "memory_write",
		Command:              []string{"memory_write", "follow_up", "Email Alex tomorrow"},
		RiskLevel:            agent.RiskMedium,
		Reason:               "active skill requested saving a confirmed local memory",
		RequiresConfirmation: true,
		NextStep:             "Review the exact memory kind and content.",
	}
	if _, err := app.store.SaveToolRun(context.Background(), memory.ToolRun{
		ToolName:  "permission_request",
		Input:     map[string]any{"request_id": request.RequestID, "tool_name": request.ToolName},
		Output:    request,
		Status:    agent.ExecutionNeedsConfirmation,
		RiskLevel: request.RiskLevel,
	}); err != nil {
		t.Fatalf("SaveToolRun(permission_request) error = %v", err)
	}

	result, err := app.RecordPermissionDecision(PermissionDecisionInput{
		Request:  request,
		Decision: "approved",
	})
	if err != nil {
		t.Fatalf("RecordPermissionDecision() first error = %v", err)
	}
	if !result.Executed || result.ToolName != "memory_write" {
		t.Fatalf("result = %+v, want executed memory_write", result)
	}

	if _, err := app.RecordPermissionDecision(PermissionDecisionInput{
		Request:  request,
		Decision: "approved",
	}); err == nil || !strings.Contains(err.Error(), "already decided") {
		t.Fatalf("RecordPermissionDecision() replay error = %v, want already decided", err)
	}

	memories, err := app.store.SearchMemories(context.Background(), "Alex tomorrow", 10)
	if err != nil {
		t.Fatalf("SearchMemories() error = %v", err)
	}
	if len(memories) != 1 {
		t.Fatalf("SearchMemories() len = %d, want one approved memory", len(memories))
	}
}
