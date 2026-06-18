package rag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"yemaka/internal/workspace"
)

func TestListDocumentsIncludesChunkAndEmbeddingCounts(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "docs", "snack.md"), "Banana and mango inventory notes.")

	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	cfg := DefaultConfig()
	cfg.ChunkSize = 80
	cfg.Embeddings.Enabled = true
	cfg.Embeddings.Model = "fake-embed"
	if _, err := store.IngestPath(ctx, "default", root, cfg, workspace.DefaultLimits()); err != nil {
		t.Fatalf("IngestPath() error = %v", err)
	}
	if _, err := store.EnsureEmbeddings(ctx, cfg, fakeEmbedder{}); err != nil {
		t.Fatalf("EnsureEmbeddings() error = %v", err)
	}

	items, err := store.ListDocuments(ctx, 10)
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	item := findInventoryItem(t, items, "docs/snack.md")
	if item.ChunkCount == 0 {
		t.Fatal("ChunkCount should be nonzero")
	}
	if item.EmbeddingCount != item.ChunkCount {
		t.Fatalf("EmbeddingCount = %d, want %d", item.EmbeddingCount, item.ChunkCount)
	}
	if item.Status != DocumentStatusIndexed || item.Missing || item.Stale {
		t.Fatalf("status = %q missing=%v stale=%v, want indexed", item.Status, item.Missing, item.Stale)
	}
	if item.ManagedSource != "filesystem" {
		t.Fatalf("ManagedSource = %q, want filesystem", item.ManagedSource)
	}
}

func TestListDocumentsEmptyReturnsNonNilSlice(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	items, err := store.ListDocuments(ctx, 10)
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if items == nil {
		t.Fatal("ListDocuments() returned nil slice, want empty slice")
	}
	if len(items) != 0 {
		t.Fatalf("len(items) = %d, want 0", len(items))
	}
}

func TestListDocumentsMarksDeletedAndChangedFilesystemSources(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	deletedPath := filepath.Join(root, "docs", "deleted.md")
	changedPath := filepath.Join(root, "docs", "changed.md")
	writeFile(t, deletedPath, "Deleted source inventory marker.")
	writeFile(t, changedPath, "Original source inventory marker.")

	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	if _, err := store.IngestPath(ctx, "default", root, DefaultConfig(), workspace.DefaultLimits()); err != nil {
		t.Fatalf("IngestPath() error = %v", err)
	}
	if err := os.Remove(deletedPath); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	writeFile(t, changedPath, "Changed source inventory marker.")

	items, err := store.ListDocuments(ctx, 10)
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	deleted := findInventoryItem(t, items, "docs/deleted.md")
	if deleted.Status != DocumentStatusMissing || !deleted.Missing || deleted.Stale {
		t.Fatalf("deleted status = %+v, want missing only", deleted)
	}
	changed := findInventoryItem(t, items, "docs/changed.md")
	if changed.Status != DocumentStatusStale || !changed.Stale || changed.Missing {
		t.Fatalf("changed status = %+v, want stale only", changed)
	}
	if changed.CurrentContentHash == "" || changed.CurrentContentHash == changed.ContentHash {
		t.Fatalf("changed hashes indexed=%q current=%q, want mismatch", changed.ContentHash, changed.CurrentContentHash)
	}
}

func TestPruneMissingDocumentsRemovesMissingOnlyAndPreservesUploads(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	missingPath := filepath.Join(root, "docs", "missing.md")
	stalePath := filepath.Join(root, "docs", "stale.md")
	writeFile(t, missingPath, "Missing prune marker source.")
	writeFile(t, stalePath, "Stale prune marker source.")

	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	if _, err := store.IngestPath(ctx, "default", root, DefaultConfig(), workspace.DefaultLimits()); err != nil {
		t.Fatalf("IngestPath() error = %v", err)
	}
	if _, err := store.IngestUploadedFile(ctx, "default", "missing.md", []byte("Upload prune marker source."), DefaultConfig(), workspace.DefaultLimits()); err != nil {
		t.Fatalf("IngestUploadedFile() error = %v", err)
	}
	if err := os.Remove(missingPath); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	writeFile(t, stalePath, "Changed stale prune marker source.")

	dryRun, err := store.PruneMissingDocuments(ctx, true)
	if err != nil {
		t.Fatalf("PruneMissingDocuments(dryRun) error = %v", err)
	}
	if dryRun.DocumentsMatched != 1 || dryRun.DocumentsRemoved != 0 || dryRun.ChunksRemoved != 0 {
		t.Fatalf("dryRun = %+v, want one matched and no removals", dryRun)
	}
	if dryRun.ChunksMatched == 0 || dryRun.FTSRowsMatched == 0 {
		t.Fatalf("dryRun = %+v, want chunk and FTS matches", dryRun)
	}

	pruned, err := store.PruneMissingDocuments(ctx, false)
	if err != nil {
		t.Fatalf("PruneMissingDocuments() error = %v", err)
	}
	if pruned.DocumentsRemoved != 1 || pruned.ChunksRemoved == 0 || pruned.FTSRowsRemoved == 0 {
		t.Fatalf("pruned = %+v, want missing document, chunks, and FTS removed", pruned)
	}

	items, err := store.ListDocuments(ctx, 10)
	if err != nil {
		t.Fatalf("ListDocuments() after prune error = %v", err)
	}
	for _, item := range items {
		if item.Path == "docs/missing.md" {
			t.Fatalf("missing filesystem document survived prune: %+v", item)
		}
	}
	upload := findInventoryItem(t, items, "uploads/missing.md")
	if upload.ManagedSource != "upload" || upload.Missing || upload.Stale {
		t.Fatalf("upload inventory = %+v, want preserved upload", upload)
	}
	stale := findInventoryItem(t, items, "docs/stale.md")
	if stale.Status != DocumentStatusStale {
		t.Fatalf("stale inventory = %+v, want stale document preserved", stale)
	}

	missingMatches, err := store.Search(ctx, "Missing prune marker", 5)
	if err != nil {
		t.Fatalf("Search(missing) error = %v", err)
	}
	if len(missingMatches) != 0 {
		t.Fatalf("missing search returned %#v, want no removed filesystem matches", missingMatches)
	}
	uploadMatches, err := store.Search(ctx, "Upload prune marker", 5)
	if err != nil {
		t.Fatalf("Search(upload) error = %v", err)
	}
	if len(uploadMatches) == 0 || uploadMatches[0].Path != "uploads/missing.md" {
		t.Fatalf("upload search = %#v, want preserved upload match", uploadMatches)
	}
}

func TestPruneMissingDocumentsScansBeyondInventoryLimit(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	total := maxDocumentInventoryLimit + 3
	for index := 0; index < total; index++ {
		content := fmt.Sprintf("Missing document prune cap marker %03d.", index)
		path := fmt.Sprintf("docs/missing-%03d.md", index)
		if _, err := store.replaceDocument(ctx, documentToStore{
			ProfileID: "default",
			Root:      root,
			Path:      path,
			Hash:      contentHash(content),
			SizeBytes: int64(len(content)),
			MimeType:  "text/plain",
			Chunks:    []Chunk{{Index: 0, Content: content, EndByte: len(content), TokenEstimate: 1}},
		}); err != nil {
			t.Fatalf("replaceDocument(%d) error = %v", index, err)
		}
	}

	dryRun, err := store.PruneMissingDocuments(ctx, true)
	if err != nil {
		t.Fatalf("PruneMissingDocuments(dryRun) error = %v", err)
	}
	if dryRun.DocumentsMatched != total || dryRun.ChunksMatched != total || dryRun.FTSRowsMatched != total {
		t.Fatalf("dryRun = %+v, want all %d missing documents matched", dryRun, total)
	}
	if dryRun.DocumentsRemoved != 0 || dryRun.ChunksRemoved != 0 || dryRun.FTSRowsRemoved != 0 {
		t.Fatalf("dryRun = %+v, want no removals", dryRun)
	}

	pruned, err := store.PruneMissingDocuments(ctx, false)
	if err != nil {
		t.Fatalf("PruneMissingDocuments() error = %v", err)
	}
	if pruned.DocumentsRemoved != total || pruned.ChunksRemoved != total || pruned.FTSRowsRemoved != total {
		t.Fatalf("pruned = %+v, want all %d missing documents removed", pruned, total)
	}

	items, err := store.ListDocuments(ctx, maxDocumentInventoryLimit)
	if err != nil {
		t.Fatalf("ListDocuments() after prune error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("ListDocuments() after prune returned %d item(s), want 0", len(items))
	}
}

func findInventoryItem(t *testing.T, items []DocumentInventoryItem, path string) DocumentInventoryItem {
	t.Helper()
	for _, item := range items {
		if item.Path == path {
			return item
		}
	}
	t.Fatalf("inventory missing %q in %#v", path, items)
	return DocumentInventoryItem{}
}
