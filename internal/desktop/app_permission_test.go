package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/agent"
	"yemaka/internal/memory"
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
	if _, err := app.store.SaveToolRun(context.Background(), memory.ToolRun{
		ToolName:  "permission_request",
		Input:     map[string]any{"request_id": request.RequestID, "tool_name": request.ToolName},
		Output:    request,
		Status:    agent.ExecutionNeedsConfirmation,
		RiskLevel: request.RiskLevel,
	}); err != nil {
		t.Fatalf("SaveToolRun(permission_request) error = %v", err)
	}
	if _, err := app.store.SaveToolRun(context.Background(), memory.ToolRun{
		ToolName:  "edit_proposal",
		Input:     map[string]any{"request_id": request.RequestID, "path": "notes.md"},
		Output:    agent.EditProposal{RequestID: request.RequestID, Path: "notes.md", Content: "Permission approval works.\n", Status: "ready_for_preview", DiffPreview: true, SnapshotBeforeWrite: true, RollbackSupported: true},
		Status:    "ready_for_preview",
		RiskLevel: agent.RiskMedium,
	}); err != nil {
		t.Fatalf("SaveToolRun(edit_proposal) error = %v", err)
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
