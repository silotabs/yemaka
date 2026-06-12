package safety

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotRollbackRestoresExistingFile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	snapshots := filepath.Join(t.TempDir(), "snapshots")
	writeFile(t, filepath.Join(root, "README.md"), "before\n")

	manager := NewSnapshotManager(snapshots)
	manifest, err := manager.Create(ctx, root, []string{"README.md"}, "test", PathPolicy{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if manifest.ID == "" {
		t.Fatal("snapshot ID is empty")
	}
	if len(manifest.Files) != 1 || !manifest.Files[0].Existed {
		t.Fatalf("manifest files = %#v, want one existing file", manifest.Files)
	}

	writeFile(t, filepath.Join(root, "README.md"), "after\n")
	rolledBack, err := manager.Rollback(ctx, "last")
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if rolledBack.ID != manifest.ID {
		t.Fatalf("rollback ID = %q, want %q", rolledBack.ID, manifest.ID)
	}
	content := readFile(t, filepath.Join(root, "README.md"))
	if content != "before\n" {
		t.Fatalf("content = %q, want before", content)
	}
}

func TestSnapshotRollbackRemovesNewFile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	snapshots := filepath.Join(t.TempDir(), "snapshots")

	manager := NewSnapshotManager(snapshots)
	manifest, err := manager.Create(ctx, root, []string{"notes.md"}, "new file", PathPolicy{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].Existed {
		t.Fatalf("manifest files = %#v, want one non-existing file", manifest.Files)
	}

	writeFile(t, filepath.Join(root, "notes.md"), "new\n")
	if _, err := manager.Rollback(ctx, manifest.ID); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "notes.md")); !os.IsNotExist(err) {
		t.Fatalf("notes.md should have been removed, stat err = %v", err)
	}
}

func TestUnifiedDiffShowsChanges(t *testing.T) {
	diff := UnifiedDiff("README.md", "one\ntwo\n", "one\nthree\n")
	if !strings.Contains(diff, "-two") {
		t.Fatalf("diff missing removal: %s", diff)
	}
	if !strings.Contains(diff, "+three") {
		t.Fatalf("diff missing addition: %s", diff)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return string(data)
}
