package rag

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"yemaka/internal/safety"
	"yemaka/internal/workspace"
)

const defaultPathSuggestionLimit = 50

type PathSuggestion struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Directory string `json:"directory"`
	Kind      string `json:"kind"`
	SizeBytes int64  `json:"sizeBytes"`
}

func SuggestDocumentPaths(path string, limits workspace.Limits, limit int) ([]PathSuggestion, error) {
	limits = applyRAGWorkspaceLimits(DefaultConfig(), limits)
	if limit <= 0 {
		limit = defaultPathSuggestionLimit
	}
	dirAbs, typedDir, prefix, absoluteInput, err := resolveSuggestionDirectory(path)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dirAbs)
	if err != nil {
		return nil, fmt.Errorf("read document folder: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name()) })

	capacity := limit
	if len(entries) < capacity {
		capacity = len(entries)
	}
	out := make([]PathSuggestion, 0, capacity)
	for _, entry := range entries {
		if len(out) >= limit {
			break
		}
		name := entry.Name()
		if entry.IsDir() {
			continue
		}
		if prefix != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			continue
		}
		candidateAbs := filepath.Join(dirAbs, name)
		if entry.Type()&os.ModeSymlink != 0 && !limits.FollowSymlinks {
			continue
		}
		if safety.IsProtectedPath(name) || !limits.IncludeHidden && safety.IsHiddenPath(name) {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.IsDir() {
			continue
		}
		if limits.MaxFileBytes > 0 && info.Size() > limits.MaxFileBytes {
			continue
		}
		kind := workspace.LanguageForPath(name)
		if kind == "" && isExtractableDocument(name) {
			kind = strings.TrimPrefix(strings.ToUpper(filepath.Ext(name)), ".")
		}
		if kind == "" {
			continue
		}
		out = append(out, PathSuggestion{
			Path:      suggestionPath(candidateAbs, typedDir, name, absoluteInput),
			Name:      name,
			Directory: displayDirectory(dirAbs, typedDir, absoluteInput),
			Kind:      kind,
			SizeBytes: info.Size(),
		})
	}
	return out, nil
}

func SuggestDocumentAccessPath(path string) (string, error) {
	dirAbs, _, _, _, err := resolveSuggestionDirectory(path)
	return dirAbs, err
}

func resolveSuggestionDirectory(path string) (dirAbs string, typedDir string, prefix string, absoluteInput bool, err error) {
	typed := strings.TrimSpace(path)
	if typed == "" {
		typed = "."
	}
	typed = safety.NormalizeUserSuppliedPath(typed)
	absoluteInput = filepath.IsAbs(typed)
	abs, err := filepath.Abs(typed)
	if err != nil {
		return "", "", "", false, fmt.Errorf("resolve document path: %w", err)
	}
	info, err := os.Stat(abs)
	if err == nil {
		if info.IsDir() {
			return filepath.Clean(abs), filepath.Clean(typed), "", absoluteInput, nil
		}
		return filepath.Clean(filepath.Dir(abs)), filepath.Clean(filepath.Dir(typed)), filepath.Base(typed), absoluteInput, nil
	}
	if !os.IsNotExist(err) {
		return "", "", "", false, fmt.Errorf("stat document path: %w", err)
	}
	parentAbs := filepath.Clean(filepath.Dir(abs))
	parentInfo, parentErr := os.Stat(parentAbs)
	if parentErr != nil {
		return "", "", "", false, fmt.Errorf("stat document folder: %w", parentErr)
	}
	if !parentInfo.IsDir() {
		return "", "", "", false, fmt.Errorf("document folder is not a directory: %s", filepath.Dir(typed))
	}
	return parentAbs, filepath.Clean(filepath.Dir(typed)), filepath.Base(typed), absoluteInput, nil
}

func suggestionPath(candidateAbs string, typedDir string, name string, absoluteInput bool) string {
	if absoluteInput {
		return filepath.Clean(candidateAbs)
	}
	if typedDir == "." || typedDir == "" {
		return filepath.ToSlash(name)
	}
	return filepath.ToSlash(filepath.Clean(filepath.Join(typedDir, name)))
}

func displayDirectory(dirAbs string, typedDir string, absoluteInput bool) string {
	if absoluteInput {
		return filepath.Clean(dirAbs)
	}
	if typedDir == "" {
		return "."
	}
	return filepath.ToSlash(filepath.Clean(typedDir))
}
