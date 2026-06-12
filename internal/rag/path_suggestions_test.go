package rag

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"yemaka/internal/workspace"
)

func TestSuggestDocumentPathsListsSupportedFilesOnly(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	writeFile(t, filepath.Join(docs, "company.md"), "# Company\nYemaka")
	writeFile(t, filepath.Join(docs, "notes.txt"), "notes")
	writeFile(t, filepath.Join(docs, "handout.pdf"), "%PDF-1.4\n")
	writeFile(t, filepath.Join(docs, "archive.bin"), "\x00\x01")
	writeFile(t, filepath.Join(docs, ".hidden.md"), "hidden")
	writeFile(t, filepath.Join(docs, "too-large.md"), strings.Repeat("large", 30))

	limits := workspace.DefaultLimits()
	limits.MaxFileBytes = 64

	suggestions, err := SuggestDocumentPaths(docs, limits, 20)
	if err != nil {
		t.Fatalf("SuggestDocumentPaths() error = %v", err)
	}

	names := suggestionNames(suggestions)
	for _, name := range []string{"company.md", "handout.pdf", "notes.txt"} {
		if !slices.Contains(names, name) {
			t.Fatalf("names = %#v, want %s", names, name)
		}
	}
	for _, name := range []string{".hidden.md", "archive.bin", "too-large.md"} {
		if slices.Contains(names, name) {
			t.Fatalf("names = %#v, should not include %s", names, name)
		}
	}
}

func TestSuggestDocumentPathsUsesTypedFilePrefix(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	writeFile(t, filepath.Join(docs, "company.md"), "# Company")
	writeFile(t, filepath.Join(docs, "contracts.md"), "# Contracts")
	writeFile(t, filepath.Join(docs, "notes.txt"), "notes")

	suggestions, err := SuggestDocumentPaths(filepath.Join(docs, "co"), workspace.DefaultLimits(), 20)
	if err != nil {
		t.Fatalf("SuggestDocumentPaths(prefix) error = %v", err)
	}

	names := suggestionNames(suggestions)
	if !slices.Equal(names, []string{"company.md", "contracts.md"}) {
		t.Fatalf("names = %#v, want company.md/contracts.md", names)
	}
}

func TestSuggestDocumentPathsCanonicalizesAbsoluteUsersPrefix(t *testing.T) {
	home := filepath.Join(t.TempDir(), "example")
	email := filepath.Join(home, "Documents", "email")
	t.Setenv("HOME", home)
	writeFile(t, filepath.Join(email, "company.md"), "# Company")

	suggestions, err := SuggestDocumentPaths("/users/example/documents/email", workspace.DefaultLimits(), 20)
	if err != nil {
		t.Fatalf("SuggestDocumentPaths(/users prefix) error = %v", err)
	}

	names := suggestionNames(suggestions)
	if !slices.Equal(names, []string{"company.md"}) {
		t.Fatalf("names = %#v, want company.md", names)
	}
}

func TestSuggestDocumentPathsSkipsSymlinksByDefault(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	writeFile(t, filepath.Join(docs, "company.md"), "# Company")
	if err := os.Symlink(filepath.Join(docs, "company.md"), filepath.Join(docs, "linked.md")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	suggestions, err := SuggestDocumentPaths(docs, workspace.DefaultLimits(), 20)
	if err != nil {
		t.Fatalf("SuggestDocumentPaths() error = %v", err)
	}

	names := suggestionNames(suggestions)
	if slices.Contains(names, "linked.md") {
		t.Fatalf("names = %#v, should not include symlink", names)
	}
}

func suggestionNames(suggestions []PathSuggestion) []string {
	names := make([]string, 0, len(suggestions))
	for _, suggestion := range suggestions {
		names = append(names, suggestion.Name)
	}
	return names
}
