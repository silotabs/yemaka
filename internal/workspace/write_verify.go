package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"yemaka/internal/safety"
)

type WriteVerification struct {
	Status            string   `json:"status"`
	Checks            []string `json:"checks"`
	Reasons           []string `json:"reasons"`
	SnapshotID        string   `json:"snapshotId"`
	RollbackAvailable bool     `json:"rollbackAvailable"`
}

func VerifyAppliedWrite(ctx context.Context, root string, plan ChangePlan, options WriteOptions) WriteVerification {
	result := WriteVerification{
		Status:     "pass",
		SnapshotID: plan.SnapshotID,
		Checks: []string{
			"diff_present",
			"content_matches",
		},
	}
	if options.SnapshotBeforeWrite && plan.Changed {
		result.Checks = append(result.Checks, "snapshot_created", "rollback_available")
	}
	if plan.Diff == "" {
		result.Status = "needs_follow_up"
		result.Reasons = append(result.Reasons, "diff preview is empty")
	}
	if options.SnapshotBeforeWrite && plan.Changed {
		if plan.SnapshotID == "" {
			result.Status = "rollback_required"
			result.Reasons = append(result.Reasons, "snapshot was required but not created")
		} else if checks, err := verifySnapshotMatchesPlan(plan.SnapshotID, root, plan, options); err != nil {
			result.Status = "rollback_required"
			result.Reasons = append(result.Reasons, err.Error())
		} else {
			result.Checks = append(result.Checks, checks...)
			result.RollbackAvailable = true
		}
	}

	if err := verifyWrittenContent(ctx, root, plan, options); err != nil {
		result.Status = "rollback_required"
		result.Reasons = append(result.Reasons, err.Error())
	}
	if len(result.Reasons) == 0 {
		result.Reasons = append(result.Reasons, "write verification passed")
	}
	return result
}

func verifyWrittenContent(ctx context.Context, root string, plan ChangePlan, options WriteOptions) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	abs, rel, exists, err := safety.ResolveWritePath(safety.PathPolicy{
		WorkspaceRoot:  root,
		FollowSymlinks: options.FollowSymlinks,
		IncludeHidden:  options.IncludeHidden,
	}, plan.Path)
	if err != nil {
		return fmt.Errorf("verify written path: %w", err)
	}
	if !exists {
		return fmt.Errorf("written file is missing: %s", rel)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Errorf("read written file: %w", err)
	}
	if string(data) != plan.After {
		return fmt.Errorf("written content does not match planned content: %s", rel)
	}
	return nil
}

func verifySnapshotMatchesPlan(id string, workspaceRoot string, plan ChangePlan, options WriteOptions) ([]string, error) {
	if options.SnapshotsRoot == "" {
		return nil, fmt.Errorf("snapshot root is empty")
	}
	manager := safety.NewSnapshotManager(options.SnapshotsRoot)
	manifest, err := manager.Read(id)
	if err != nil {
		return nil, fmt.Errorf("snapshot is not readable: %w", err)
	}
	resolvedRoot, err := safety.ResolveWorkspace(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("verify snapshot workspace: %w", err)
	}
	if manifest.WorkspaceRoot != resolvedRoot {
		return nil, fmt.Errorf("snapshot workspace mismatch: %s", manifest.WorkspaceRoot)
	}
	checks := []string{"snapshot_readable", "snapshot_workspace_matches"}
	var matched *safety.SnapshotFile
	for index := range manifest.Files {
		if manifest.Files[index].Path == plan.Path {
			matched = &manifest.Files[index]
			break
		}
	}
	if matched == nil {
		return nil, fmt.Errorf("snapshot does not include changed path: %s", plan.Path)
	}
	checks = append(checks, "snapshot_manifest_matches_path")
	if plan.IsNew {
		if matched.Existed {
			return nil, fmt.Errorf("snapshot expected new file marker but recorded existing file: %s", plan.Path)
		}
		checks = append(checks, "snapshot_records_new_file")
		return checks, nil
	}
	if !matched.Existed {
		return nil, fmt.Errorf("snapshot expected existing file but recorded new file: %s", plan.Path)
	}
	snapshotPath := filepath.Join(options.SnapshotsRoot, manifest.ID, "files", filepath.FromSlash(plan.Path))
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		return nil, fmt.Errorf("read snapshot file: %w", err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != matched.SHA256 {
		return nil, fmt.Errorf("snapshot hash mismatch: %s", plan.Path)
	}
	if int64(len(data)) != matched.SizeBytes {
		return nil, fmt.Errorf("snapshot size mismatch: %s", plan.Path)
	}
	checks = append(checks, "snapshot_file_hash_verified")
	return checks, nil
}
