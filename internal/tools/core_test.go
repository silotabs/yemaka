package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/workspace"
)

func TestProjectMapFindsManifestsAndTestCommand(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/test\n")
	writeTestFile(t, filepath.Join(root, "cmd", "app", "main.go"), "package main\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(root, "README.md"), "# Test\n")

	result, err := ProjectMap(root, workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("ProjectMap() error = %v", err)
	}
	if len(result.Manifests) == 0 || result.Manifests[0] != "go.mod" {
		t.Fatalf("Manifests = %+v, want go.mod", result.Manifests)
	}
	if strings.Join(result.TestCommand, " ") != "go test ./..." {
		t.Fatalf("TestCommand = %+v, want go test ./...", result.TestCommand)
	}
	if !strings.Contains(FormatProjectMap(result), "Detected test command") {
		t.Fatalf("formatted project map missing test command")
	}
}

func TestSymbolSearchFindsLocalSymbols(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "service.go"), "package service\n\ntype Runner struct{}\nfunc RunTask() error { return nil }\n")

	matches, err := SymbolSearch(context.Background(), root, "Run", workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("SymbolSearch() error = %v", err)
	}
	found := false
	for _, match := range matches {
		if match.Name == "RunTask" && match.Kind == "function" {
			found = true
		}
	}
	if !found {
		t.Fatalf("matches = %+v, want RunTask function", matches)
	}
}

func TestSecretScanReportsWithoutExposingValue(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "config.yaml"), "api_key: sk-abcdefghijklmnopqrstuvwxyz123456\n")
	writeTestFile(t, filepath.Join(root, ".env"), "PASSWORD=super-secret-value\n")

	findings, err := SecretScan(context.Background(), root, workspace.DefaultLimits())
	if err != nil {
		t.Fatalf("SecretScan() error = %v", err)
	}
	text := FormatSecretFindings(findings)
	if !strings.Contains(text, "config.yaml:1") {
		t.Fatalf("findings missing config.yaml line: %s", text)
	}
	if !strings.Contains(text, ".env") {
		t.Fatalf("findings missing .env path: %s", text)
	}
	if strings.Contains(text, "abcdefghijklmnopqrstuvwxyz") || strings.Contains(text, "super-secret-value") {
		t.Fatalf("secret value leaked in findings: %s", text)
	}
}

func TestPatchPreviewDoesNotWriteFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.md")
	writeTestFile(t, path, "before\n")

	diff, err := PatchPreview(context.Background(), root, "notes.md", "after\n", workspace.WriteOptions{MaxEditFileBytes: 200000})
	if err != nil {
		t.Fatalf("PatchPreview() error = %v", err)
	}
	if !strings.Contains(diff, "-before") || !strings.Contains(diff, "+after") {
		t.Fatalf("diff = %q, want before/after lines", diff)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "before\n" {
		t.Fatalf("file was modified by preview: %q", string(data))
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
