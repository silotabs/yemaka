package rag

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/models"
	"yemaka/internal/workspace"
)

type fakeEmbedder struct{}

func (fakeEmbedder) Embed(ctx context.Context, req models.EmbeddingRequest) (models.EmbeddingResponse, error) {
	embeddings := make([][]float64, 0, len(req.Input))
	for _, input := range req.Input {
		input = strings.ToLower(input)
		switch {
		case strings.Contains(input, "fruit") || strings.Contains(input, "banana") || strings.Contains(input, "mango"):
			embeddings = append(embeddings, []float64{1, 0})
		case strings.Contains(input, "space") || strings.Contains(input, "rocket") || strings.Contains(input, "orbit"):
			embeddings = append(embeddings, []float64{0, 1})
		default:
			embeddings = append(embeddings, []float64{0.5, 0.5})
		}
	}
	return models.EmbeddingResponse{Embeddings: embeddings}, nil
}

func TestChunkTextUsesDeterministicApproxTokenSizes(t *testing.T) {
	content := strings.Repeat("alpha beta gamma delta. ", 800)
	chunks := ChunkText(content, 100, 20, 10)

	if len(chunks) < 2 {
		t.Fatalf("chunks len = %d, want at least 2", len(chunks))
	}
	if chunks[0].Index != 0 || chunks[1].Index != 1 {
		t.Fatalf("chunk indexes = %d, %d", chunks[0].Index, chunks[1].Index)
	}
	if chunks[1].StartByte >= chunks[0].EndByte {
		t.Fatalf("expected overlap, got first end %d second start %d", chunks[0].EndByte, chunks[1].StartByte)
	}
	if chunks[0].TokenEstimate == 0 {
		t.Fatal("TokenEstimate should be nonzero")
	}
}

func TestIngestAndSearchSQLiteFTS5(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "docs", "security.md"), "# Safety\nYemaka blocks dangerous shell commands and protects secrets.")
	writeFile(t, filepath.Join(root, "docs", "models.md"), "# Models\nSmall Ollama models use SQLite FTS5 retrieval.")
	writeFile(t, filepath.Join(root, ".env"), "SECRET=skip")

	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	cfg := DefaultConfig()
	cfg.ChunkSize = 40
	cfg.ChunkOverlap = 5
	result, err := store.IngestPath(ctx, "default", root, cfg, workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("IngestPath() error = %v", err)
	}
	if result.FilesIndexed != 2 {
		t.Fatalf("FilesIndexed = %d, want 2", result.FilesIndexed)
	}
	if result.ChunksCreated == 0 {
		t.Fatal("ChunksCreated should be nonzero")
	}
	if result.FilesSkipped == 0 {
		t.Fatal("FilesSkipped should include protected .env")
	}

	results, err := store.Search(ctx, "dangerous shell", 5)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search() returned no matches")
	}
	if results[0].Path != "docs/security.md" {
		t.Fatalf("top result path = %q, want docs/security.md", results[0].Path)
	}
	chunks, err := store.ListChunks(ctx, 10)
	if err != nil {
		t.Fatalf("ListChunks() error = %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("ListChunks() returned no chunks")
	}

	contextResult, err := store.BuildContext(ctx, "SQLite retrieval", 5, 2000)
	if err != nil {
		t.Fatalf("BuildContext() error = %v", err)
	}
	if !strings.Contains(contextResult.Text, "RETRIEVED CONTEXT") {
		t.Fatalf("context = %q, want retrieved context", contextResult.Text)
	}
	if len(contextResult.Sources) == 0 {
		t.Fatal("context sources should be nonempty")
	}
}

func TestIngestSingleMarkdownFile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "docs", "company.md")
	writeFile(t, path, "# Company\nYemaka TestCo launch codename is Stone Lantern.")

	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	result, err := store.IngestPath(ctx, "default", path, DefaultConfig(), workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("IngestPath(file) error = %v", err)
	}
	if result.Root != filepath.Dir(path) {
		t.Fatalf("Root = %q, want %q", result.Root, filepath.Dir(path))
	}
	if result.FilesIndexed != 1 {
		t.Fatalf("FilesIndexed = %d, want 1", result.FilesIndexed)
	}
	if result.ChunksCreated == 0 {
		t.Fatal("ChunksCreated should be nonzero")
	}

	results, err := store.Search(ctx, "Stone Lantern", 5)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search() returned no matches")
	}
	if results[0].Path != "company.md" {
		t.Fatalf("top result path = %q, want company.md", results[0].Path)
	}
}

func TestNaturalLocalDocumentQuestionFindsSpecificFacts(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), "# Project\nGeneral project overview without launch contact facts.")
	writeFile(t, filepath.Join(root, "docs", "company.md"), "Yemaka TestCo was founded in 2026.\nThe launch codename is Stone Lantern.\nThe support email is local-only@example.test.\n")

	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	cfg := DefaultConfig()
	cfg.ChunkSize = 80
	cfg.ChunkOverlap = 10
	if _, err := store.IngestPath(ctx, "default", root, cfg, workspace.DefaultLimits()); err != nil {
		t.Fatalf("IngestPath() error = %v", err)
	}

	query := "Using my local documents, what is the launch codename and support email?"
	results, err := store.SearchWithConfig(ctx, query, cfg, nil)
	if err != nil {
		t.Fatalf("SearchWithConfig() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("SearchWithConfig() returned no matches")
	}
	if results[0].Path != "docs/company.md" {
		t.Fatalf("top result path = %q, want docs/company.md; results=%#v", results[0].Path, results)
	}

	contextResult, err := store.BuildContextWithConfig(ctx, query, cfg, nil, 2000)
	if err != nil {
		t.Fatalf("BuildContextWithConfig() error = %v", err)
	}
	for _, want := range []string{"docs/company.md", "Stone Lantern", "local-only@example.test"} {
		if !strings.Contains(contextResult.Text, want) {
			t.Fatalf("context missing %q:\n%s", want, contextResult.Text)
		}
	}
}

func TestIngestSingleUnsupportedFileReportsSkip(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "image.bin")
	if err := os.WriteFile(path, []byte{1, 2, 3}, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	result, err := store.IngestPath(ctx, "default", path, DefaultConfig(), workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("IngestPath(unsupported file) error = %v", err)
	}
	if result.FilesIndexed != 0 || result.FilesSkipped != 1 {
		t.Fatalf("result indexed=%d skipped=%d, want indexed=0 skipped=1", result.FilesIndexed, result.FilesSkipped)
	}
	if len(result.SkippedReasons) == 0 || !strings.Contains(result.SkippedReasons[0], "unsupported") {
		t.Fatalf("SkippedReasons = %#v, want unsupported reason", result.SkippedReasons)
	}
}

func TestIngestDirectoryReportsWorkspaceScanSkipReasons(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "image.bin")
	if err := os.WriteFile(path, []byte{1, 2, 3}, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	result, err := store.IngestPath(ctx, "default", root, DefaultConfig(), workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("IngestPath(directory) error = %v", err)
	}
	if result.FilesIndexed != 0 || result.FilesSkipped != 1 {
		t.Fatalf("result indexed=%d skipped=%d, want indexed=0 skipped=1", result.FilesIndexed, result.FilesSkipped)
	}
	if len(result.SkippedReasons) == 0 || !strings.Contains(result.SkippedReasons[0], "unsupported file type") {
		t.Fatalf("SkippedReasons = %#v, want workspace scan skip reason", result.SkippedReasons)
	}
}

func TestIngestUploadedMarkdownFile(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	result, err := store.IngestUploadedFile(ctx, "default", "company.md", []byte("# Company\nYemaka browser upload codename is Green Harbor."), DefaultConfig(), workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("IngestUploadedFile() error = %v", err)
	}
	if result.Root != uploadedDocumentRoot {
		t.Fatalf("Root = %q, want %q", result.Root, uploadedDocumentRoot)
	}
	if result.FilesIndexed != 1 {
		t.Fatalf("FilesIndexed = %d, want 1", result.FilesIndexed)
	}
	if result.ChunksCreated == 0 {
		t.Fatal("ChunksCreated should be nonzero")
	}

	results, err := store.Search(ctx, "Green Harbor", 5)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search() returned no matches")
	}
	if results[0].Path != "uploads/company.md" {
		t.Fatalf("top result path = %q, want uploads/company.md", results[0].Path)
	}
}

func TestIngestUploadedFolderPathPreservesRelativePath(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	result, err := store.IngestUploadedFile(ctx, "default", "project/docs/company.md", []byte("# Company\nFolder upload codename is Bright Orchard."), DefaultConfig(), workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("IngestUploadedFile(folder path) error = %v", err)
	}
	if result.FilesIndexed != 1 {
		t.Fatalf("FilesIndexed = %d, want 1", result.FilesIndexed)
	}

	results, err := store.Search(ctx, "Bright Orchard", 5)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search() returned no matches")
	}
	if results[0].Path != "uploads/project/docs/company.md" {
		t.Fatalf("top result path = %q, want uploads/project/docs/company.md", results[0].Path)
	}
}

func TestIngestUploadedUnsupportedFileReportsSkip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	result, err := store.IngestUploadedFile(ctx, "default", "photo.bin", []byte{1, 2, 3}, DefaultConfig(), workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("IngestUploadedFile(unsupported) error = %v", err)
	}
	if result.FilesIndexed != 0 || result.FilesSkipped != 1 {
		t.Fatalf("result indexed=%d skipped=%d, want indexed=0 skipped=1", result.FilesIndexed, result.FilesSkipped)
	}
	if len(result.SkippedReasons) == 0 || !strings.Contains(result.SkippedReasons[0], "unsupported") {
		t.Fatalf("SkippedReasons = %#v, want unsupported reason", result.SkippedReasons)
	}
}

func TestIngestUploadedTraversalPathReportsSkip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	result, err := store.IngestUploadedFile(ctx, "default", "../company.md", []byte("outside"), DefaultConfig(), workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("IngestUploadedFile(traversal) error = %v", err)
	}
	if result.FilesIndexed != 0 || result.FilesSkipped != 1 {
		t.Fatalf("result indexed=%d skipped=%d, want indexed=0 skipped=1", result.FilesIndexed, result.FilesSkipped)
	}
	if len(result.SkippedReasons) == 0 || !strings.Contains(result.SkippedReasons[0], "invalid") {
		t.Fatalf("SkippedReasons = %#v, want invalid reason", result.SkippedReasons)
	}
}

func TestIngestUploadedProtectedFileReportsSkip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	result, err := store.IngestUploadedFile(ctx, "default", ".env", []byte("SECRET=value"), DefaultConfig(), workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("IngestUploadedFile(protected) error = %v", err)
	}
	if result.FilesIndexed != 0 || result.FilesSkipped != 1 {
		t.Fatalf("result indexed=%d skipped=%d, want indexed=0 skipped=1", result.FilesIndexed, result.FilesSkipped)
	}
	if len(result.SkippedReasons) == 0 || !strings.Contains(result.SkippedReasons[0], "protected") {
		t.Fatalf("SkippedReasons = %#v, want protected reason", result.SkippedReasons)
	}
}

func TestSearchSanitizesStackTraceLikeTokens(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	_, err = store.Search(ctx, "index-P89FhvJf.js:6 Uncaught Error: motion elements require key", 5)
	if err != nil {
		t.Fatalf("Search() stack trace query error = %v", err)
	}
}

func TestIngestSkipsUnchangedDocuments(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), "# Yemaka\nLocal RAG with FTS5.")

	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	cfg := DefaultConfig()
	first, err := store.IngestPath(ctx, "default", root, cfg, workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("first IngestPath() error = %v", err)
	}
	if first.FilesIndexed != 1 {
		t.Fatalf("first FilesIndexed = %d, want 1", first.FilesIndexed)
	}

	second, err := store.IngestPath(ctx, "default", root, cfg, workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("second IngestPath() error = %v", err)
	}
	if second.FilesUnchanged != 1 {
		t.Fatalf("second FilesUnchanged = %d, want 1", second.FilesUnchanged)
	}
	if second.ChunksCreated != 0 {
		t.Fatalf("second ChunksCreated = %d, want 0", second.ChunksCreated)
	}
}

func TestIngestExtractsPDFAndDOCXDocuments(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writePDF(t, filepath.Join(root, "docs", "handout.pdf"), "Yemaka PDF ingestion extracts simple text.")
	writeDOCX(t, filepath.Join(root, "docs", "notes.docx"), "Yemaka DOCX ingestion keeps class notes searchable.")

	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	cfg := DefaultConfig()
	cfg.ChunkSize = 40
	cfg.ChunkOverlap = 5
	result, err := store.IngestPath(ctx, "default", root, cfg, workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("IngestPath() error = %v", err)
	}
	if result.FilesIndexed != 2 {
		t.Fatalf("FilesIndexed = %d, want 2", result.FilesIndexed)
	}

	pdfMatches, err := store.Search(ctx, "PDF simple text", 5)
	if err != nil {
		t.Fatalf("Search PDF error = %v", err)
	}
	if len(pdfMatches) == 0 || pdfMatches[0].Path != "docs/handout.pdf" {
		t.Fatalf("PDF search = %#v, want docs/handout.pdf", pdfMatches)
	}

	docxMatches, err := store.Search(ctx, "class notes searchable", 5)
	if err != nil {
		t.Fatalf("Search DOCX error = %v", err)
	}
	if len(docxMatches) == 0 || docxMatches[0].Path != "docs/notes.docx" {
		t.Fatalf("DOCX search = %#v, want docs/notes.docx", docxMatches)
	}
}

func TestAdvancedExtractionGuardsAndDocxTables(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writePDFWithPages(t, filepath.Join(root, "docs", "large.pdf"), maxExtractedPDFPages+1)
	writeDOCXXML(t, filepath.Join(root, "docs", "table.docx"), `<?xml version="1.0" encoding="UTF-8"?>`+
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`+
		`<w:body><w:tbl><w:tr>`+
		`<w:tc><w:p><w:r><w:t>Alpha</w:t></w:r></w:p></w:tc>`+
		`<w:tc><w:p><w:r><w:t>Beta</w:t></w:r></w:p></w:tc>`+
		`</w:tr></w:tbl></w:body></w:document>`)
	writeFile(t, filepath.Join(root, "docs", "unsupported.rtf"), "{\\rtf1 unsupported}")

	if _, err := extractDocument(ctx, root, "docs/large.pdf", 300000); err == nil || !strings.Contains(err.Error(), "page count") {
		t.Fatalf("large PDF error = %v, want page count guard", err)
	}
	table, err := extractDocument(ctx, root, "docs/table.docx", 300000)
	if err != nil {
		t.Fatalf("extract table docx error = %v", err)
	}
	if !strings.Contains(table.Content, "Alpha") || !strings.Contains(table.Content, "Beta") || strings.Contains(table.Content, "AlphaBeta") {
		t.Fatalf("table content = %q, want separated cell text", table.Content)
	}
	if _, err := extractDocument(ctx, root, "docs/unsupported.rtf", 300000); err == nil || !strings.Contains(err.Error(), "supported document types") {
		t.Fatalf("unsupported document error = %v", err)
	}
}

func TestIngestReportsSkippedExtractionReasons(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writePDFWithPages(t, filepath.Join(root, "docs", "large.pdf"), maxExtractedPDFPages+1)

	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	result, err := store.IngestPath(ctx, "default", root, DefaultConfig(), workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("IngestPath() error = %v", err)
	}
	if result.FilesSkipped == 0 {
		t.Fatal("FilesSkipped = 0, want skipped large PDF")
	}
	if len(result.SkippedReasons) == 0 || !strings.Contains(result.SkippedReasons[0], "page count") {
		t.Fatalf("SkippedReasons = %#v, want page count reason", result.SkippedReasons)
	}
}

func TestOptionalEmbeddingsAreExplicitAndSearchable(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "docs", "snack.md"), "Banana and mango are useful quick snacks.")
	writeFile(t, filepath.Join(root, "docs", "space.md"), "Rocket engines help spacecraft reach orbit.")

	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	cfg := DefaultConfig()
	cfg.ChunkSize = 40
	cfg.ChunkOverlap = 5
	if cfg.Embeddings.Enabled {
		t.Fatal("embeddings should be disabled by default")
	}
	if _, err := store.IngestPath(ctx, "default", root, cfg, workspace.DefaultLimits()); err != nil {
		t.Fatalf("IngestPath() error = %v", err)
	}
	disabled, err := store.EnsureEmbeddings(ctx, cfg, fakeEmbedder{})
	if err != nil {
		t.Fatalf("EnsureEmbeddings disabled error = %v", err)
	}
	if disabled.ChunksEmbedded != 0 {
		t.Fatalf("disabled ChunksEmbedded = %d, want 0", disabled.ChunksEmbedded)
	}

	cfg.Embeddings.Enabled = true
	cfg.Embeddings.Model = "fake-embed"
	cfg.Embeddings.BatchSize = 1
	indexed, err := store.EnsureEmbeddings(ctx, cfg, fakeEmbedder{})
	if err != nil {
		t.Fatalf("EnsureEmbeddings enabled error = %v", err)
	}
	if indexed.ChunksEmbedded == 0 {
		t.Fatal("expected embedded chunks")
	}
	if indexed.Dimensions != 2 {
		t.Fatalf("Dimensions = %d, want 2", indexed.Dimensions)
	}

	results, err := store.SearchWithConfig(ctx, "fruit", cfg, fakeEmbedder{})
	if err != nil {
		t.Fatalf("SearchWithConfig() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected embedding search results")
	}
	if results[0].Path != "docs/snack.md" {
		t.Fatalf("top result path = %q, want docs/snack.md", results[0].Path)
	}
	if results[0].Source != "embedding" {
		t.Fatalf("top result source = %q, want embedding", results[0].Source)
	}
}

func TestSearchWithConfigReranksExactPathAndHeadingMatches(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "docs", "notes.md"), "# Notes\nA brief mention says model routing can be useful.")
	writeFile(t, filepath.Join(root, "docs", "model-routing.md"), "# Model routing\nYemaka model routing chooses low-memory local models for small machines.")
	writeFile(t, filepath.Join(root, "docs", "routing.md"), "# Routing\nRouting advice mentions models without the exact phrase.")

	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	cfg := DefaultConfig()
	cfg.ChunkSize = 80
	cfg.ChunkOverlap = 5
	cfg.Rerank.CandidateLimit = 10
	if _, err := store.IngestPath(ctx, "default", root, cfg, workspace.DefaultLimits()); err != nil {
		t.Fatalf("IngestPath() error = %v", err)
	}

	results, err := store.SearchWithConfig(ctx, "model routing", cfg, nil)
	if err != nil {
		t.Fatalf("SearchWithConfig() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("SearchWithConfig() returned no matches")
	}
	if results[0].Path != "docs/model-routing.md" {
		t.Fatalf("top result path = %q, want docs/model-routing.md; results=%#v", results[0].Path, results)
	}
	if results[0].Score <= 0 {
		t.Fatalf("top result score = %f, want > 0", results[0].Score)
	}
	reasons := strings.Join(results[0].Explanation, ",")
	if !strings.Contains(reasons, "exact_phrase") || !strings.Contains(reasons, "heading") || !strings.Contains(reasons, "path_title") {
		t.Fatalf("top result explanation = %v, want exact phrase, heading, and path/title boosts", results[0].Explanation)
	}
}

func TestSearchWithConfigAppliesSourceDiversityCap(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "docs", "dense.md"), strings.Repeat("model routing exact phrase in dense source.\n\n", 8))
	writeFile(t, filepath.Join(root, "docs", "second.md"), "# Model routing\nA second source also explains model routing.")

	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	cfg := DefaultConfig()
	cfg.ChunkSize = 45
	cfg.ChunkOverlap = 0
	cfg.TopK = 2
	cfg.Rerank.CandidateLimit = 10
	cfg.Rerank.SourceDiversity = 1
	if _, err := store.IngestPath(ctx, "default", root, cfg, workspace.DefaultLimits()); err != nil {
		t.Fatalf("IngestPath() error = %v", err)
	}

	results, err := store.SearchWithConfig(ctx, "model routing", cfg, nil)
	if err != nil {
		t.Fatalf("SearchWithConfig() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("result count = %d, want 2; results=%#v", len(results), results)
	}
	if results[0].Path == results[1].Path {
		t.Fatalf("source diversity cap returned duplicate path %q", results[0].Path)
	}
}

func TestEvaluateVectorDBResearchGateKeepsSQLiteDefault(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "docs", "fruit.md"), "# Fruit\nBanana and mango notes for retrieval.")

	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	cfg := DefaultConfig()
	cfg.ChunkSize = 80
	cfg.Embeddings.Enabled = true
	if _, err := store.IngestPath(ctx, "default", root, cfg, workspace.DefaultLimits()); err != nil {
		t.Fatalf("IngestPath() error = %v", err)
	}
	if _, err := store.EnsureEmbeddings(ctx, cfg, fakeEmbedder{}); err != nil {
		t.Fatalf("EnsureEmbeddings() error = %v", err)
	}

	report, err := store.EvaluateVectorDB(ctx, cfg)
	if err != nil {
		t.Fatalf("EvaluateVectorDB() error = %v", err)
	}
	if report.Approved {
		t.Fatal("vector DB gate should not auto-approve a vector database")
	}
	if report.Recommendation != "keep_sqlite_fts" {
		t.Fatalf("Recommendation = %q, want keep_sqlite_fts", report.Recommendation)
	}
	if !report.SQLiteFTSActive {
		t.Fatal("SQLite FTS should remain active")
	}
	if !report.SQLiteEmbeddingsActive {
		t.Fatal("SQLite-stored embeddings should be reported active")
	}
	if report.Stats.Documents != 1 || report.Stats.Chunks == 0 || report.Stats.Embeddings == 0 {
		t.Fatalf("Stats = %+v, want indexed docs/chunks/embeddings", report.Stats)
	}
	var sawBlockedService bool
	var sawResearchCandidate bool
	for _, candidate := range report.Candidates {
		if candidate.RequiresBackgroundService && candidate.Status == "blocked" {
			sawBlockedService = true
		}
		if candidate.Name == "faiss" && candidate.Status == "research_only" {
			sawResearchCandidate = true
		}
	}
	if !sawBlockedService {
		t.Fatalf("Candidates = %+v, want background-service candidates blocked", report.Candidates)
	}
	if !sawResearchCandidate {
		t.Fatalf("Candidates = %+v, want local research-only candidate", report.Candidates)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func writePDF(t *testing.T, path string, text string) {
	t.Helper()
	content := "%PDF-1.4\n" +
		"1 0 obj << /Length 64 >>\nstream\n" +
		"BT /F1 12 Tf 72 720 Td (" + text + ") Tj ET\n" +
		"endstream\nendobj\n%%EOF\n"
	writeFile(t, path, content)
}

func writePDFWithPages(t *testing.T, path string, pages int) {
	t.Helper()
	var builder strings.Builder
	builder.WriteString("%PDF-1.4\n")
	for i := 0; i < pages; i++ {
		builder.WriteString("1 0 obj << /Type /Page >> endobj\n")
	}
	builder.WriteString("2 0 obj << /Length 32 >>\nstream\nBT (bounded text) Tj ET\nendstream\nendobj\n%%EOF\n")
	writeFile(t, path, builder.String())
}

func writeDOCX(t *testing.T, path string, text string) {
	t.Helper()
	writeDOCXXML(t, path, `<?xml version="1.0" encoding="UTF-8"?>`+
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`+
		`<w:body><w:p><w:r><w:t>`+text+`</w:t></w:r></w:p></w:body></w:document>`)
}

func writeDOCXXML(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	doc, err := writer.Create("word/document.xml")
	if err != nil {
		t.Fatalf("Create document.xml error = %v", err)
	}
	if _, err := doc.Write([]byte(content)); err != nil {
		t.Fatalf("Write document.xml error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close zip error = %v", err)
	}
}
