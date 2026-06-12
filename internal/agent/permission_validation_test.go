package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/memory"
)

func TestValidateStoredPermissionApprovalAllowsStoredEditProposalFallback(t *testing.T) {
	ctx := context.Background()
	store := openPermissionValidationStore(t)

	request := PermissionRequest{
		RequestID:           "perm_edit_fallback",
		ToolName:            "edit_file",
		Command:             []string{"edit_file", "README.md"},
		RiskLevel:           RiskMedium,
		Reason:              "file edits require confirmation and snapshot flow",
		DiffPreview:         true,
		SnapshotBeforeWrite: true,
		RollbackSupported:   true,
	}
	if _, err := store.SaveToolRun(ctx, memory.ToolRun{
		ToolName: "edit_proposal",
		Input:    map[string]any{"request_id": request.RequestID, "path": "README.md"},
		Output: EditProposal{
			RequestID:           request.RequestID,
			Path:                "README.md",
			Status:              "ready_for_preview",
			DiffPreview:         true,
			SnapshotBeforeWrite: true,
			RollbackSupported:   true,
		},
		Status:    "ready_for_preview",
		RiskLevel: RiskMedium,
	}); err != nil {
		t.Fatalf("SaveToolRun(edit_proposal) error = %v", err)
	}

	if err := ValidateStoredPermissionApproval(ctx, store, request); err != nil {
		t.Fatalf("ValidateStoredPermissionApproval() error = %v", err)
	}
}

func TestValidateStoredPermissionApprovalRejectsMismatchedEditProposalFallback(t *testing.T) {
	ctx := context.Background()
	store := openPermissionValidationStore(t)

	request := PermissionRequest{
		RequestID:           "perm_edit_mismatch",
		ToolName:            "edit_file",
		Command:             []string{"edit_file", "README.md"},
		RiskLevel:           RiskMedium,
		DiffPreview:         true,
		SnapshotBeforeWrite: true,
		RollbackSupported:   true,
	}
	if _, err := store.SaveToolRun(ctx, memory.ToolRun{
		ToolName: "edit_proposal",
		Input:    map[string]any{"request_id": request.RequestID, "path": "docs/README.md"},
		Output: EditProposal{
			RequestID:           request.RequestID,
			Path:                "docs/README.md",
			Status:              "ready_for_preview",
			DiffPreview:         true,
			SnapshotBeforeWrite: true,
			RollbackSupported:   true,
		},
		Status:    "ready_for_preview",
		RiskLevel: RiskMedium,
	}); err != nil {
		t.Fatalf("SaveToolRun(edit_proposal) error = %v", err)
	}

	err := ValidateStoredPermissionApproval(ctx, store, request)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("ValidateStoredPermissionApproval() error = %v, want mismatch", err)
	}
}

func openPermissionValidationStore(t *testing.T) *memory.Store {
	t.Helper()
	store, err := memory.Open(context.Background(), filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}
