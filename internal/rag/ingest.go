package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"

	"yemaka/internal/safety"
	"yemaka/internal/workspace"
)

const maxIngestSkipReasons = 8
const uploadedDocumentRoot = "browser_upload"

type extractableFile struct {
	Path string
	Size int64
}

func (s *Store) IngestPath(ctx context.Context, profileID string, rootPath string, cfg Config, limits workspace.Limits) (IngestResult, error) {
	cfg = applyConfigDefaults(cfg)
	limits = applyRAGWorkspaceLimits(cfg, limits)
	rootPath = safety.NormalizeUserSuppliedPath(rootPath)

	target, err := singleFileTarget(rootPath)
	if err != nil {
		return IngestResult{}, err
	}
	if target != nil {
		return s.ingestSingleFile(ctx, profileID, *target, cfg, limits)
	}

	scan, err := workspace.Scan(rootPath, limits)
	if err != nil {
		return IngestResult{}, err
	}

	result := IngestResult{
		Root:         scan.Root,
		FilesSkipped: skippedWithoutExtractableDocs(scan.Skipped),
	}
	result.addWorkspaceScanSkipReasons(scan.Skipped)
	seen := map[string]bool{}
	for _, file := range scan.Files {
		seen[file.Path] = true
		read, err := workspace.ReadFile(ctx, scan.Root, file.Path, limits)
		if err != nil {
			result.FilesSkipped++
			result.addSkipReason(file.Path, err)
			continue
		}
		created, unchanged, err := s.ingestExtracted(ctx, profileID, scan.Root, extractedDocument{
			Path:     read.Path,
			Content:  read.Content,
			Size:     read.Size,
			MimeType: "text/plain",
		}, cfg)
		if err != nil {
			return result, err
		}
		if unchanged {
			result.FilesUnchanged++
			continue
		}
		if created == 0 {
			result.FilesSkipped++
			continue
		}
		result.FilesIndexed++
		result.ChunksCreated += created
		result.BytesIndexed += read.Size
	}

	extraFiles, extraSkipped, err := scanExtractableDocuments(scan.Root, seen, limits, len(scan.Files), scan.TotalIndexedSize)
	if err != nil {
		return result, err
	}
	result.FilesSkipped += extraSkipped
	for _, file := range extraFiles {
		extracted, err := extractDocument(ctx, scan.Root, file.Path, limits.MaxFileBytes)
		if err != nil {
			result.FilesSkipped++
			result.addSkipReason(file.Path, err)
			continue
		}
		created, unchanged, err := s.ingestExtracted(ctx, profileID, scan.Root, extracted, cfg)
		if err != nil {
			return result, err
		}
		if unchanged {
			result.FilesUnchanged++
			continue
		}
		if created == 0 {
			result.FilesSkipped++
			continue
		}
		result.FilesIndexed++
		result.ChunksCreated += created
		result.BytesIndexed += file.Size
	}
	return result, nil
}

type ingestFileTarget struct {
	Root string
	Path string
	Size int64
}

func singleFileTarget(path string) (*ingestFileTarget, error) {
	if path == "" {
		path = "."
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve ingest path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("stat ingest path: %w", err)
	}
	if info.IsDir() {
		return nil, nil
	}
	return &ingestFileTarget{
		Root: filepath.Clean(filepath.Dir(abs)),
		Path: filepath.Base(abs),
		Size: info.Size(),
	}, nil
}

func (s *Store) ingestSingleFile(ctx context.Context, profileID string, target ingestFileTarget, cfg Config, limits workspace.Limits) (IngestResult, error) {
	result := IngestResult{Root: target.Root}
	var doc extractedDocument
	var err error

	if isExtractableDocument(target.Path) {
		doc, err = extractDocument(ctx, target.Root, target.Path, limits.MaxFileBytes)
	} else {
		read, readErr := workspace.ReadFile(ctx, target.Root, target.Path, limits)
		if readErr == nil {
			doc = extractedDocument{
				Path:     read.Path,
				Content:  read.Content,
				Size:     read.Size,
				MimeType: "text/plain",
			}
		}
		err = readErr
	}
	if err != nil {
		result.FilesSkipped = 1
		result.addSkipReason(target.Path, err)
		return result, nil
	}

	created, unchanged, err := s.ingestExtracted(ctx, profileID, target.Root, doc, cfg)
	if err != nil {
		return result, err
	}
	if unchanged {
		result.FilesUnchanged = 1
		return result, nil
	}
	if created == 0 {
		result.FilesSkipped = 1
		return result, nil
	}
	result.FilesIndexed = 1
	result.ChunksCreated = created
	result.BytesIndexed = doc.Size
	return result, nil
}

func (s *Store) IngestUploadedFile(ctx context.Context, profileID string, name string, data []byte, cfg Config, limits workspace.Limits) (IngestResult, error) {
	cfg = applyConfigDefaults(cfg)
	limits = applyRAGWorkspaceLimits(cfg, limits)

	result := IngestResult{Root: uploadedDocumentRoot}
	cleanPath, err := cleanUploadedDocumentPath(name, limits)
	if err != nil {
		result.FilesSkipped = 1
		result.addSkipReason(name, err)
		return result, nil
	}
	if limits.MaxFileBytes > 0 && int64(len(data)) > limits.MaxFileBytes {
		result.FilesSkipped = 1
		result.addSkipReason(cleanPath, fmt.Errorf("uploaded document exceeds max read size: %s", cleanPath))
		return result, nil
	}

	baseName := pathpkg.Base(cleanPath)
	docPath := pathpkg.Join("uploads", cleanPath)
	var doc extractedDocument
	if isExtractableDocument(baseName) {
		tmpDir, err := os.MkdirTemp("", "yemaka-rag-upload-*")
		if err != nil {
			return result, fmt.Errorf("create upload extraction workspace: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		tmpPath := filepath.Join(tmpDir, baseName)
		if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
			return result, fmt.Errorf("stage uploaded document: %w", err)
		}
		doc, err = extractDocument(ctx, tmpDir, baseName, limits.MaxFileBytes)
		if err != nil {
			result.FilesSkipped = 1
			result.addSkipReason(cleanPath, err)
			return result, nil
		}
		doc.Path = docPath
	} else if workspace.LanguageForPath(baseName) != "" {
		doc = extractedDocument{
			Path:     docPath,
			Content:  string(data),
			Size:     int64(len(data)),
			MimeType: "text/plain",
		}
	} else {
		result.FilesSkipped = 1
		result.addSkipReason(cleanPath, fmt.Errorf("unsupported document type: %s", cleanPath))
		return result, nil
	}

	created, unchanged, err := s.ingestExtracted(ctx, profileID, uploadedDocumentRoot, doc, cfg)
	if err != nil {
		return result, err
	}
	if unchanged {
		result.FilesUnchanged = 1
		return result, nil
	}
	if created == 0 {
		result.FilesSkipped = 1
		return result, nil
	}
	result.FilesIndexed = 1
	result.ChunksCreated = created
	result.BytesIndexed = doc.Size
	return result, nil
}

func cleanUploadedDocumentPath(name string, limits workspace.Limits) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("uploaded document name is required")
	}
	name = strings.ReplaceAll(filepath.ToSlash(name), "\\", "/")
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("uploaded document path must be relative")
	}
	cleanPath := pathpkg.Clean(name)
	if cleanPath == "." || cleanPath == "/" || cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return "", fmt.Errorf("uploaded document path is invalid")
	}
	for _, segment := range strings.Split(cleanPath, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("uploaded document path is invalid")
		}
		if safety.IsProtectedPath(segment) || safety.IsProtectedPath(cleanPath) {
			return "", fmt.Errorf("protected document file cannot be ingested: %s", cleanPath)
		}
		if !limits.IncludeHidden && (safety.IsHiddenPath(segment) || safety.IsHiddenPath(cleanPath)) {
			return "", fmt.Errorf("hidden document file cannot be ingested: %s", cleanPath)
		}
	}
	return cleanPath, nil
}

func (s *Store) ingestExtracted(ctx context.Context, profileID string, root string, doc extractedDocument, cfg Config) (created int, unchanged bool, err error) {
	hash := contentHash(doc.Content)
	existingID, existingHash, err := s.existingDocument(ctx, root, doc.Path)
	if err != nil {
		return 0, false, err
	}
	if existingID != "" && existingHash == hash {
		return 0, true, nil
	}

	chunks := ChunkText(doc.Content, cfg.ChunkSize, cfg.ChunkOverlap, cfg.MaxChunksPerFile)
	if len(chunks) == 0 {
		return 0, false, nil
	}
	created, err = s.replaceDocument(ctx, documentToStore{
		ProfileID:  profileID,
		Root:       root,
		Path:       doc.Path,
		ExistingID: existingID,
		Hash:       hash,
		SizeBytes:  doc.Size,
		MimeType:   doc.MimeType,
		Chunks:     chunks,
	})
	return created, false, err
}

func scanExtractableDocuments(root string, seen map[string]bool, limits workspace.Limits, scannedFiles int, scannedBytes int64) ([]extractableFile, int, error) {
	var files []extractableFile
	skipped := 0
	currentBytes := scannedBytes
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			skipped++
			return nil
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			skipped++
			return nil
		}
		rel = filepath.ToSlash(rel)
		name := entry.Name()
		if entry.IsDir() {
			if shouldSkipExtractableDir(rel, name, limits) {
				return filepath.SkipDir
			}
			return nil
		}
		if seen[rel] || !isExtractableDocument(rel) {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 && !limits.FollowSymlinks {
			skipped++
			return nil
		}
		if safety.IsProtectedPath(rel) || !limits.IncludeHidden && safety.IsHiddenPath(rel) {
			skipped++
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			skipped++
			return nil
		}
		if info.Size() > limits.MaxFileBytes {
			skipped++
			return nil
		}
		if scannedFiles+len(files) >= limits.MaxFilesScanned {
			return filepath.SkipAll
		}
		if currentBytes+info.Size() > limits.MaxTotalScanBytes {
			return filepath.SkipAll
		}
		files = append(files, extractableFile{Path: rel, Size: info.Size()})
		currentBytes += info.Size()
		return nil
	})
	if err != nil {
		return nil, skipped, fmt.Errorf("scan extractable documents: %w", err)
	}
	return files, skipped, nil
}

func shouldSkipExtractableDir(rel string, name string, limits workspace.Limits) bool {
	switch name {
	case ".git", "node_modules", "vendor", "dist", "build", "target", ".cache", ".svelte-kit", "wailsjs":
		return true
	}
	return safety.IsProtectedPath(rel) || !limits.IncludeHidden && safety.IsHiddenPath(rel)
}

func skippedWithoutExtractableDocs(skipped []workspace.SkipInfo) int {
	count := 0
	for _, item := range skipped {
		if item.Reason == "unsupported file type" && isExtractableDocument(item.Path) {
			continue
		}
		count++
	}
	return count
}

func applyConfigDefaults(cfg Config) Config {
	defaults := DefaultConfig()
	if cfg.Mode == "" {
		cfg.Mode = defaults.Mode
	}
	if cfg.ChunkSize == 0 {
		cfg.ChunkSize = defaults.ChunkSize
	}
	if cfg.ChunkOverlap == 0 {
		cfg.ChunkOverlap = defaults.ChunkOverlap
	}
	if cfg.TopK == 0 {
		cfg.TopK = defaults.TopK
	}
	if cfg.MaxFileBytes == 0 {
		cfg.MaxFileBytes = defaults.MaxFileBytes
	}
	if cfg.MaxTotalBytes == 0 {
		cfg.MaxTotalBytes = defaults.MaxTotalBytes
	}
	if cfg.MaxChunksPerFile == 0 {
		cfg.MaxChunksPerFile = defaults.MaxChunksPerFile
	}
	if cfg.Rerank.CandidateLimit == 0 {
		cfg.Rerank.CandidateLimit = defaults.Rerank.CandidateLimit
	}
	if cfg.Rerank.SourceDiversity == 0 {
		cfg.Rerank.SourceDiversity = defaults.Rerank.SourceDiversity
	}
	if cfg.Embeddings.Provider == "" {
		cfg.Embeddings.Provider = defaults.Embeddings.Provider
	}
	if cfg.Embeddings.Model == "" {
		cfg.Embeddings.Model = defaults.Embeddings.Model
	}
	if cfg.Embeddings.BatchSize == 0 {
		cfg.Embeddings.BatchSize = defaults.Embeddings.BatchSize
	}
	if cfg.Embeddings.MaxTextChars == 0 {
		cfg.Embeddings.MaxTextChars = defaults.Embeddings.MaxTextChars
	}
	if cfg.Embeddings.CandidateLimit == 0 {
		cfg.Embeddings.CandidateLimit = defaults.Embeddings.CandidateLimit
	}
	if cfg.VectorDB.Provider == "" {
		cfg.VectorDB.Provider = defaults.VectorDB.Provider
	}
	if !cfg.VectorDB.ResearchOnly {
		cfg.VectorDB.ResearchOnly = defaults.VectorDB.ResearchOnly
	}
	if !cfg.VectorDB.LocalOnly {
		cfg.VectorDB.LocalOnly = defaults.VectorDB.LocalOnly
	}
	if cfg.VectorDB.MaxRAMMB == 0 {
		cfg.VectorDB.MaxRAMMB = defaults.VectorDB.MaxRAMMB
	}
	if cfg.VectorDB.MaxStorageMB == 0 {
		cfg.VectorDB.MaxStorageMB = defaults.VectorDB.MaxStorageMB
	}
	if cfg.VectorDB.MinBenefitPercent == 0 {
		cfg.VectorDB.MinBenefitPercent = defaults.VectorDB.MinBenefitPercent
	}
	return cfg
}

func applyRAGWorkspaceLimits(cfg Config, limits workspace.Limits) workspace.Limits {
	defaults := workspace.DefaultLimits()
	if limits.MaxFilesScanned == 0 {
		limits.MaxFilesScanned = defaults.MaxFilesScanned
	}
	if limits.MaxFileBytes == 0 || limits.MaxFileBytes > cfg.MaxFileBytes {
		limits.MaxFileBytes = cfg.MaxFileBytes
	}
	if limits.MaxTotalScanBytes == 0 || limits.MaxTotalScanBytes > cfg.MaxTotalBytes {
		limits.MaxTotalScanBytes = cfg.MaxTotalBytes
	}
	if limits.MaxSearchResults == 0 {
		limits.MaxSearchResults = defaults.MaxSearchResults
	}
	if limits.MaxContextFiles == 0 {
		limits.MaxContextFiles = defaults.MaxContextFiles
	}
	if limits.MaxContextChars == 0 {
		limits.MaxContextChars = defaults.MaxContextChars
	}
	return limits
}

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func FormatIngestResult(result IngestResult) string {
	text := fmt.Sprintf(
		"root: %s\nfiles_indexed: %d\nfiles_skipped: %d\nfiles_unchanged: %d\nchunks_created: %d\nbytes_indexed: %d",
		result.Root,
		result.FilesIndexed,
		result.FilesSkipped,
		result.FilesUnchanged,
		result.ChunksCreated,
		result.BytesIndexed,
	)
	if len(result.SkippedReasons) > 0 {
		text += "\nskipped_reasons:"
		for _, reason := range result.SkippedReasons {
			text += "\n- " + reason
		}
	}
	return text
}

func (r *IngestResult) addSkipReason(path string, err error) {
	if r == nil || err == nil || len(r.SkippedReasons) >= maxIngestSkipReasons {
		return
	}
	path = filepath.ToSlash(path)
	reason := err.Error()
	for _, existing := range r.SkippedReasons {
		if existing == reason || existing == fmt.Sprintf("%s: %s", path, reason) {
			return
		}
	}
	if path == "" {
		r.SkippedReasons = append(r.SkippedReasons, reason)
		return
	}
	r.SkippedReasons = append(r.SkippedReasons, fmt.Sprintf("%s: %s", path, reason))
}

func (r *IngestResult) addWorkspaceScanSkipReasons(skipped []workspace.SkipInfo) {
	if r == nil {
		return
	}
	for _, item := range skipped {
		if item.Reason == "unsupported file type" && isExtractableDocument(item.Path) {
			continue
		}
		r.addSkipReason(item.Path, fmt.Errorf("%s", item.Reason))
	}
}
