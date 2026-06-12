package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/safety"
)

func TestPlanWriteRejectsProtectedPath(t *testing.T) {
	_, err := PlanWrite(context.Background(), t.TempDir(), ".env", "SECRET=value\n", WriteOptions{})
	if err == nil {
		t.Fatal("expected protected path rejection")
	}
	if !strings.Contains(err.Error(), "protected") {
		t.Fatalf("error = %q, want protected", err)
	}
}

func TestApplyWriteCreatesSnapshotAndUpdatesFile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	snapshots := filepath.Join(t.TempDir(), "snapshots")
	writeFile(t, filepath.Join(root, "README.md"), "before\n")

	options := WriteOptions{
		SnapshotsRoot:       snapshots,
		SnapshotBeforeWrite: true,
		MaxEditFileBytes:    200000,
	}
	plan, err := PlanWrite(ctx, root, "README.md", "after\n", options)
	if err != nil {
		t.Fatalf("PlanWrite() error = %v", err)
	}
	if !plan.Changed {
		t.Fatal("plan should be changed")
	}
	if !strings.Contains(plan.Diff, "-before") || !strings.Contains(plan.Diff, "+after") {
		t.Fatalf("diff = %s", plan.Diff)
	}

	applied, err := ApplyWrite(ctx, root, plan, options)
	if err != nil {
		t.Fatalf("ApplyWrite() error = %v", err)
	}
	if applied.SnapshotID == "" {
		t.Fatal("SnapshotID is empty")
	}
	content := readTestFile(t, filepath.Join(root, "README.md"))
	if content != "after\n" {
		t.Fatalf("content = %q, want after", content)
	}
	verification := VerifyAppliedWrite(ctx, root, applied, options)
	if verification.Status != "pass" {
		t.Fatalf("verification status = %q, want pass; reasons: %v", verification.Status, verification.Reasons)
	}
	if !verification.RollbackAvailable {
		t.Fatal("RollbackAvailable = false, want true")
	}
	for _, want := range []string{"snapshot_manifest_matches_path", "snapshot_file_hash_verified"} {
		if !containsString(verification.Checks, want) {
			t.Fatalf("verification checks = %v, want %s", verification.Checks, want)
		}
	}
}

func TestApplyWriteCreatesNewFileSnapshotMarker(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	snapshots := filepath.Join(t.TempDir(), "snapshots")

	options := WriteOptions{
		SnapshotsRoot:       snapshots,
		SnapshotBeforeWrite: true,
		MaxEditFileBytes:    200000,
	}
	plan, err := PlanWrite(ctx, root, "docs/notes.md", "new\n", options)
	if err != nil {
		t.Fatalf("PlanWrite() error = %v", err)
	}
	if !plan.IsNew {
		t.Fatal("plan should identify new file")
	}
	applied, err := ApplyWrite(ctx, root, plan, options)
	if err != nil {
		t.Fatalf("ApplyWrite() error = %v", err)
	}
	if applied.SnapshotID == "" {
		t.Fatal("SnapshotID is empty")
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "notes.md")); err != nil {
		t.Fatalf("new file stat error = %v", err)
	}
	verification := VerifyAppliedWrite(ctx, root, applied, options)
	if verification.Status != "pass" {
		t.Fatalf("verification status = %q, want pass; reasons: %v", verification.Status, verification.Reasons)
	}
	if !containsString(verification.Checks, "snapshot_records_new_file") {
		t.Fatalf("verification checks = %v, want snapshot_records_new_file", verification.Checks)
	}
}

func TestVerifyAppliedWriteRequiresSnapshotToMatchChangedPath(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	snapshots := filepath.Join(t.TempDir(), "snapshots")
	writeFile(t, filepath.Join(root, "README.md"), "before\n")
	writeFile(t, filepath.Join(root, "other.md"), "other\n")

	options := WriteOptions{
		SnapshotsRoot:       snapshots,
		SnapshotBeforeWrite: true,
		MaxEditFileBytes:    200000,
	}
	plan, err := PlanWrite(ctx, root, "README.md", "after\n", options)
	if err != nil {
		t.Fatalf("PlanWrite() error = %v", err)
	}
	applied := plan
	applied.SnapshotID = createMismatchedSnapshot(t, ctx, root, snapshots)
	verification := VerifyAppliedWrite(ctx, root, applied, options)
	if verification.Status != "rollback_required" {
		t.Fatalf("verification status = %q, want rollback_required", verification.Status)
	}
	if !strings.Contains(strings.Join(verification.Reasons, " "), "snapshot does not include changed path") {
		t.Fatalf("verification reasons = %v, want changed path mismatch", verification.Reasons)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return string(data)
}

func createMismatchedSnapshot(t *testing.T, ctx context.Context, root string, snapshots string) string {
	t.Helper()
	manager := safety.NewSnapshotManager(snapshots)
	manifest, err := manager.Create(ctx, root, []string{"other.md"}, "mismatched", safety.PathPolicy{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	return manifest.ID
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
