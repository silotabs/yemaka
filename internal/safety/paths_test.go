package safety

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveReadPathRejectsOutsideWorkspace(t *testing.T) {
	root := t.TempDir()

	_, _, err := ResolveReadPath(PathPolicy{WorkspaceRoot: root}, "../outside.txt")
	if err == nil {
		t.Fatal("expected outside workspace path to be rejected")
	}
	if !strings.Contains(err.Error(), "outside workspace") {
		t.Fatalf("error = %q, want outside workspace", err)
	}
}

func TestResolveReadPathRejectsHiddenAndProtectedFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".hidden.md"), "hidden")
	writeFile(t, filepath.Join(root, ".env"), "secret")

	_, _, err := ResolveReadPath(PathPolicy{WorkspaceRoot: root}, ".hidden.md")
	if err == nil || !strings.Contains(err.Error(), "hidden") {
		t.Fatalf("hidden file error = %v", err)
	}

	_, _, err = ResolveReadPath(PathPolicy{WorkspaceRoot: root, IncludeHidden: true}, ".env")
	if err == nil || !strings.Contains(err.Error(), "protected") {
		t.Fatalf("protected file error = %v", err)
	}
}

func TestNormalizeUserSuppliedPathTreatsUsersPrefixAsAbsolute(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "Users", "example")
	t.Setenv("HOME", home)

	got := normalizeUserSuppliedPath("users/example/downloads/yemaka/docs/company.md")
	want := filepath.Join(home, "Downloads", "yemaka", "docs", "company.md")
	if got != want {
		t.Fatalf("normalizeUserSuppliedPath() = %q, want %q", got, want)
	}

	relative := normalizeUserSuppliedPath("docs/company.md")
	if relative != "docs/company.md" {
		t.Fatalf("normalizeUserSuppliedPath(relative) = %q, want unchanged", relative)
	}
}

func TestNormalizeUserSuppliedPathCanonicalizesAbsoluteUsersPrefix(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "Users", "example")
	t.Setenv("HOME", home)

	got := normalizeUserSuppliedPath("/users/example/documents/email")
	want := filepath.Join(home, "Documents", "email")
	if got != want {
		t.Fatalf("normalizeUserSuppliedPath(abs /users) = %q, want %q", got, want)
	}
}

func TestNormalizeUserSuppliedPathExpandsHomePrefix(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "Users", "example")
	t.Setenv("HOME", home)

	got := normalizeUserSuppliedPath("~/downloads/yemaka/docs/company.md")
	want := filepath.Join(home, "Downloads", "yemaka", "docs", "company.md")
	if got != want {
		t.Fatalf("normalizeUserSuppliedPath(~) = %q, want %q", got, want)
	}
}

func TestNormalizeUserSuppliedPathTreatsHomeFolderAliasAsAbsolute(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "Users", "example")
	t.Setenv("HOME", home)

	got := normalizeUserSuppliedPath("downloads/yemaka/docs/company.md")
	want := filepath.Join(home, "Downloads", "yemaka", "docs", "company.md")
	if got != want {
		t.Fatalf("normalizeUserSuppliedPath(downloads alias) = %q, want %q", got, want)
	}

	relative := normalizeUserSuppliedPath("docs/company.md")
	if relative != "docs/company.md" {
		t.Fatalf("normalizeUserSuppliedPath(relative) = %q, want unchanged", relative)
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
