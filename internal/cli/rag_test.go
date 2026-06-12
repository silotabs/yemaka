package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/config"
	"yemaka/internal/rag"
	"yemaka/internal/workspace"
)

func TestRunRAGVectorEvaluateIsResearchOnly(t *testing.T) {
	ctx := context.Background()
	store, err := rag.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("rag.Open() error = %v", err)
	}
	defer store.Close()

	app := &appContext{
		config: config.Default(),
		rag:    store,
	}
	var out bytes.Buffer
	if err := runRAG(ctx, app, []string{"vector", "evaluate"}, &out); err != nil {
		t.Fatalf("runRAG(vector evaluate) error = %v", err)
	}
	output := out.String()
	for _, want := range []string{
		"research_only: true",
		"approved: false",
		"recommendation: keep_sqlite_fts",
		"qdrant\tblocked",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestRunRAGInventoryAndPruneMissing(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(path, []byte("Permission approval works."), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	store, err := rag.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("rag.Open() error = %v", err)
	}
	defer store.Close()
	if _, err := store.IngestPath(ctx, "default", root, rag.DefaultConfig(), workspace.DefaultLimits()); err != nil {
		t.Fatalf("IngestPath() error = %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	app := &appContext{
		config: config.Default(),
		rag:    store,
	}
	var inventory bytes.Buffer
	if err := runRAG(ctx, app, []string{"inventory"}, &inventory); err != nil {
		t.Fatalf("runRAG(inventory) error = %v", err)
	}
	if output := inventory.String(); !strings.Contains(output, "missing") || !strings.Contains(output, "notes.txt") {
		t.Fatalf("inventory output = %q, want missing notes.txt", output)
	}

	var dryRun bytes.Buffer
	if err := runRAG(ctx, app, []string{"prune-missing"}, &dryRun); err != nil {
		t.Fatalf("runRAG(prune-missing) error = %v", err)
	}
	if output := dryRun.String(); !strings.Contains(output, "dry_run: true") || !strings.Contains(output, "documents_removed: 0") {
		t.Fatalf("dry-run output = %q, want dry-run without removal", output)
	}

	var pruned bytes.Buffer
	if err := runRAG(ctx, app, []string{"prune-missing", "--yes"}, &pruned); err != nil {
		t.Fatalf("runRAG(prune-missing --yes) error = %v", err)
	}
	if output := pruned.String(); !strings.Contains(output, "dry_run: false") || !strings.Contains(output, "documents_removed: 1") {
		t.Fatalf("prune output = %q, want one document removed", output)
	}
}
