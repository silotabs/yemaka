package rag

import (
	"context"
	"strings"
	"testing"

	"yemaka/internal/workspace"
)

func TestExtractUploadedFileReadsTextWithoutIndexing(t *testing.T) {
	result, err := ExtractUploadedFile(context.Background(), "notes.txt", []byte("local attachment context"), workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("ExtractUploadedFile() error = %v", err)
	}
	if result.Path != "notes.txt" {
		t.Fatalf("Path = %q, want redacted basename", result.Path)
	}
	if result.Content != "local attachment context" {
		t.Fatalf("Content = %q", result.Content)
	}
}

func TestExtractUploadedFileBlocksBinaryTextAttachment(t *testing.T) {
	_, err := ExtractUploadedFile(context.Background(), "notes.txt", []byte{'o', 'k', 0, 'x'}, workspace.DefaultLimits())
	if err == nil || !strings.Contains(err.Error(), "binary attachment is blocked") {
		t.Fatalf("ExtractUploadedFile() error = %v, want binary block", err)
	}
}
