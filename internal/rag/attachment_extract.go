package rag

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"yemaka/internal/workspace"
)

type UploadedExtraction struct {
	Path     string
	Content  string
	Size     int64
	MimeType string
}

func ExtractUploadedFile(ctx context.Context, name string, data []byte, limits workspace.Limits) (UploadedExtraction, error) {
	cleanPath, err := cleanUploadedDocumentPath(name, limits)
	if err != nil {
		return UploadedExtraction{}, err
	}
	if limits.MaxFileBytes > 0 && int64(len(data)) > limits.MaxFileBytes {
		return UploadedExtraction{}, fmt.Errorf("uploaded document exceeds max read size: %s", cleanPath)
	}

	baseName := filepath.Base(cleanPath)
	if isExtractableDocument(baseName) {
		tmpDir, err := os.MkdirTemp("", "yemaka-chat-attachment-*")
		if err != nil {
			return UploadedExtraction{}, fmt.Errorf("create attachment extraction workspace: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		tmpPath := filepath.Join(tmpDir, baseName)
		if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
			return UploadedExtraction{}, fmt.Errorf("stage attachment: %w", err)
		}
		doc, err := extractDocument(ctx, tmpDir, baseName, limits.MaxFileBytes)
		if err != nil {
			return UploadedExtraction{}, err
		}
		doc.Path = cleanPath
		return UploadedExtraction{
			Path:     doc.Path,
			Content:  doc.Content,
			Size:     doc.Size,
			MimeType: doc.MimeType,
		}, nil
	}
	if workspace.LanguageForPath(baseName) == "" {
		return UploadedExtraction{}, fmt.Errorf("unsupported attachment type: %s", cleanPath)
	}
	if bytes.Contains(data, []byte{0}) {
		return UploadedExtraction{}, fmt.Errorf("binary attachment is blocked: %s", cleanPath)
	}
	return UploadedExtraction{
		Path:     cleanPath,
		Content:  decodeTextBytes(data),
		Size:     int64(len(data)),
		MimeType: "text/plain",
	}, nil
}
