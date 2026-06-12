package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanSkipsIgnoredHiddenAndProtectedFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), "# Test")
	writeFile(t, filepath.Join(root, "main.go"), "package main")
	writeFile(t, filepath.Join(root, "node_modules", "pkg", "index.js"), "console.log('skip')")
	writeFile(t, filepath.Join(root, ".hidden.md"), "skip")
	writeFile(t, filepath.Join(root, ".env"), "skip")
	writeFile(t, filepath.Join(root, "memory.sqlite"), "skip")

	result, err := Scan(root, DefaultLimits())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(result.Files) != 2 {
		t.Fatalf("Files len = %d, want 2: %#v", len(result.Files), result.Files)
	}
	if result.LanguageCounts["Go"] != 1 {
		t.Fatalf("Go count = %d, want 1", result.LanguageCounts["Go"])
	}
	if result.LanguageCounts["Markdown"] != 1 {
		t.Fatalf("Markdown count = %d, want 1", result.LanguageCounts["Markdown"])
	}
	if !contains(result.ImportantFiles, "README.md") {
		t.Fatalf("ImportantFiles = %#v, want README.md", result.ImportantFiles)
	}
	if len(result.Skipped) < 3 {
		t.Fatalf("Skipped len = %d, want at least 3", len(result.Skipped))
	}
}

func TestReadFileAndSearchFiles(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), "# Test\nSQLite keeps memory light.\n")
	writeFile(t, filepath.Join(root, "internal", "app.go"), "package internal\n")

	read, err := ReadFile(ctx, root, "README.md", DefaultLimits())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if read.Path != "README.md" {
		t.Fatalf("Path = %q, want README.md", read.Path)
	}
	if !strings.Contains(read.Content, "SQLite") {
		t.Fatalf("Content = %q, want SQLite", read.Content)
	}

	matches, err := SearchFiles(ctx, root, "sqlite", DefaultLimits())
	if err != nil {
		t.Fatalf("SearchFiles() error = %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches len = %d, want 1", len(matches))
	}
	if matches[0].Path != "README.md" || matches[0].Line != 2 {
		t.Fatalf("match = %#v, want README.md line 2", matches[0])
	}

	list, err := ListFiles(ctx, root, filepath.Join(root, "internal"), DefaultLimits())
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}
	if list.Path != "internal" {
		t.Fatalf("list.Path = %q, want internal", list.Path)
	}
	if len(list.Entries) != 1 || list.Entries[0].Path != "internal/app.go" {
		t.Fatalf("entries = %#v, want internal/app.go", list.Entries)
	}
}

func TestStatPathReportsFileDirectoryAndMissingWorkspacePath(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), "# Test\n")
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	file, err := StatPath(ctx, root, "README.md", DefaultLimits())
	if err != nil {
		t.Fatalf("StatPath(file) error = %v", err)
	}
	if file.Path != "README.md" || !file.Exists || file.IsDir || file.Size == 0 {
		t.Fatalf("file stat = %#v, want existing README file", file)
	}

	dir, err := StatPath(ctx, root, "docs", DefaultLimits())
	if err != nil {
		t.Fatalf("StatPath(dir) error = %v", err)
	}
	if dir.Path != "docs" || !dir.Exists || !dir.IsDir {
		t.Fatalf("dir stat = %#v, want existing docs dir", dir)
	}

	missing, err := StatPath(ctx, root, "missing.md", DefaultLimits())
	if err != nil {
		t.Fatalf("StatPath(missing) error = %v", err)
	}
	if missing.Path != "missing.md" || missing.Exists {
		t.Fatalf("missing stat = %#v, want missing workspace path", missing)
	}
}

func TestFileTreeIsBoundedAndSkipsProtectedPaths(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), "# Test\n")
	writeFile(t, filepath.Join(root, "docs", "guide.md"), "# Guide\n")
	writeFile(t, filepath.Join(root, "src", "main.go"), "package main\n")
	writeFile(t, filepath.Join(root, ".env"), "SECRET=value\n")
	writeFile(t, filepath.Join(root, "private.key"), "secret\n")

	limits := DefaultLimits()
	limits.MaxSearchResults = 3
	tree, err := FileTree(ctx, root, ".", limits)
	if err != nil {
		t.Fatalf("FileTree() error = %v", err)
	}
	if len(tree.Entries) != 3 {
		t.Fatalf("entries len = %d, want 3: %#v", len(tree.Entries), tree.Entries)
	}
	if !tree.LimitReached {
		t.Fatalf("LimitReached = false, want true: %#v", tree)
	}
	for _, entry := range tree.Entries {
		if strings.Contains(entry.Path, ".env") || strings.Contains(entry.Path, "private.key") {
			t.Fatalf("tree included protected path: %#v", tree.Entries)
		}
	}
	if tree.Skipped == 0 {
		t.Fatalf("Skipped = 0, want protected/limit skips: %#v", tree)
	}
}

func TestBuildQuestionContextIncludesSummaryAndSources(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), "# Yemaka\nLocal-first agent.\n")
	writeFile(t, filepath.Join(root, "go.mod"), "module yemaka\n")

	qctx, err := BuildQuestionContext(ctx, root, "Explain this project", DefaultLimits())
	if err != nil {
		t.Fatalf("BuildQuestionContext() error = %v", err)
	}
	if !strings.Contains(qctx.Text, "WORKSPACE SUMMARY") {
		t.Fatalf("context missing summary: %s", qctx.Text)
	}
	if !strings.Contains(qctx.Text, "README.md") {
		t.Fatalf("context missing README source: %s", qctx.Text)
	}
	if len(qctx.Sources) == 0 {
		t.Fatal("Sources is empty")
	}
}

func TestBuildQuestionContextIncludesStructureGuideForWorkspaceStructureQuestions(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), "# Project\n")
	writeFile(t, filepath.Join(root, "go.mod"), "module example.test/project\n")
	writeFile(t, filepath.Join(root, "cmd/app/main.go"), "package main\n")
	writeFile(t, filepath.Join(root, "internal/agent/service.go"), "package agent\n")
	writeFile(t, filepath.Join(root, "frontend/package.json"), `{"scripts":{"build":"vite build"}}`)
	writeFile(t, filepath.Join(root, "docs/blueprint.md"), "# Blueprint\n")

	qctx, err := BuildQuestionContext(ctx, root, "Explain this workspace structure and tell me what files matter first.", DefaultLimits())
	if err != nil {
		t.Fatalf("BuildQuestionContext() error = %v", err)
	}
	for _, want := range []string{
		"WORKSPACE STRUCTURE GUIDE",
		"Top-level folders",
		"cmd/",
		"internal/",
		"frontend/",
		"Root/key files",
		"README.md",
		"go.mod",
		"Mention only files and folders listed here",
	} {
		if !strings.Contains(qctx.Text, want) {
			t.Fatalf("context missing %q:\n%s", want, qctx.Text)
		}
	}
}

func TestBuildQuestionContextPrioritizesLocalDocumentsForDocumentQuestions(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), "# Project\nGeneral overview.\n")
	writeFile(t, filepath.Join(root, "go.mod"), "module example.test/project\n")
	writeFile(t, filepath.Join(root, "wails.json"), `{"name":"Example"}`)
	writeFile(t, filepath.Join(root, "frontend/package.json"), `{"scripts":{"build":"vite build"}}`)
	writeFile(t, filepath.Join(root, "frontend/vite.config.ts"), "export default {}\n")
	writeFile(t, filepath.Join(root, "docs/blueprint.md"), "# Blueprint\nArchitecture notes.\n")
	writeFile(t, filepath.Join(root, "docs/company.md"), "The launch codename is Stone Lantern.\nThe support email is local-only@example.test.\n")
	writeFile(t, filepath.Join(root, "docs/roadmap.md"), "# Roadmap\nMilestones.\n")

	limits := DefaultLimits()
	limits.MaxContextFiles = 4
	qctx, err := BuildQuestionContext(ctx, root, "Using my local documents, what is the launch codename and support email?", limits)
	if err != nil {
		t.Fatalf("BuildQuestionContext() error = %v", err)
	}
	if !strings.Contains(qctx.Text, "LOCAL DOCUMENT REQUEST") {
		t.Fatalf("context missing local document guidance:\n%s", qctx.Text)
	}
	if !contains(qctx.Sources, "docs/company.md") {
		t.Fatalf("Sources = %#v, want docs/company.md", qctx.Sources)
	}
	if !strings.Contains(qctx.Text, "Stone Lantern") || !strings.Contains(qctx.Text, "local-only@example.test") {
		t.Fatalf("context missing company facts:\n%s", qctx.Text)
	}
}

func TestBuildQuestionContextPrioritizesPythonFilesForPythonTestTask(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), "# Go project overview\n")
	writeFile(t, filepath.Join(root, "go.mod"), "module example.test/project\n")
	writeFile(t, filepath.Join(root, "wails.json"), `{"name":"Example"}`)
	writeFile(t, filepath.Join(root, "test-files", "calculator.py"), "def add(a, b):\n    return a - b\n")
	writeFile(t, filepath.Join(root, "test-files", "test_calculator.py"), "from calculator import add\n\ndef test_add():\n    assert add(2, 3) == 5\n")

	limits := DefaultLimits()
	limits.MaxContextFiles = 2
	qctx, err := BuildQuestionContext(ctx, root, "Inspect the Python files in test-files, run the tests if safe, find the bug, propose a fix, and only apply it after I approve.", limits)
	if err != nil {
		t.Fatalf("BuildQuestionContext() error = %v", err)
	}
	if !contains(qctx.Sources, "test-files/calculator.py") {
		t.Fatalf("Sources = %#v, want calculator.py", qctx.Sources)
	}
	if !contains(qctx.Sources, "test-files/test_calculator.py") {
		t.Fatalf("Sources = %#v, want test_calculator.py", qctx.Sources)
	}
	if contains(qctx.Sources, "go.mod") {
		t.Fatalf("Sources = %#v, did not expect go.mod to outrank Python task files", qctx.Sources)
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
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
