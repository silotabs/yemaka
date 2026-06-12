package workspace

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"yemaka/internal/safety"
)

func Scan(rootPath string, limits Limits) (ScanResult, error) {
	limits = applyLimitDefaults(limits)
	root, err := safety.ResolveWorkspace(rootPath)
	if err != nil {
		return ScanResult{}, err
	}

	result := ScanResult{
		Root:           root,
		LanguageCounts: map[string]int{},
	}
	topDirs := map[string]bool{}
	important := map[string]bool{}
	docs := map[string]bool{}

	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			rel := safeRel(root, path)
			result.Skipped = append(result.Skipped, SkipInfo{Path: rel, Reason: walkErr.Error()})
			return nil
		}
		if path == root {
			return nil
		}

		rel := safeRel(root, path)
		name := entry.Name()
		if entry.IsDir() {
			if skip, reason := shouldSkipDir(rel, name, limits); skip {
				result.Skipped = append(result.Skipped, SkipInfo{Path: rel, Reason: reason})
				return filepath.SkipDir
			}
			if first := firstPathPart(rel); first != "" {
				topDirs[first] = true
			}
			return nil
		}

		if entry.Type()&fs.ModeSymlink != 0 && !limits.FollowSymlinks {
			result.Skipped = append(result.Skipped, SkipInfo{Path: rel, Reason: "symlink"})
			return nil
		}
		if skip, reason := shouldSkipFile(rel, name, limits); skip {
			result.Skipped = append(result.Skipped, SkipInfo{Path: rel, Reason: reason})
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			result.Skipped = append(result.Skipped, SkipInfo{Path: rel, Reason: err.Error()})
			return nil
		}
		if info.Size() > limits.MaxFileBytes {
			result.Skipped = append(result.Skipped, SkipInfo{Path: rel, Reason: "file too large"})
			return nil
		}

		language := languageForPath(rel)
		if language == "" {
			result.Skipped = append(result.Skipped, SkipInfo{Path: rel, Reason: "unsupported file type"})
			return nil
		}

		if len(result.Files) >= limits.MaxFilesScanned {
			result.LimitReached = true
			return filepath.SkipAll
		}
		if result.TotalIndexedSize+info.Size() > limits.MaxTotalScanBytes {
			result.LimitReached = true
			return filepath.SkipAll
		}

		result.Files = append(result.Files, FileInfo{
			Path:     rel,
			Size:     info.Size(),
			Language: language,
			Kind:     fileKind(rel),
		})
		result.LanguageCounts[language]++
		result.TotalIndexedSize += info.Size()
		if first := firstPathPart(rel); first != "" && first != filepath.Base(rel) {
			topDirs[first] = true
		}
		if importantScore(rel) > 0 {
			important[rel] = true
		}
		if isDocumentationPath(rel) {
			docs[rel] = true
		}
		return nil
	})
	if err != nil {
		return ScanResult{}, fmt.Errorf("scan workspace: %w", err)
	}

	sort.Slice(result.Files, func(i, j int) bool { return result.Files[i].Path < result.Files[j].Path })
	result.TopLevelDirs = sortedKeys(topDirs)
	result.ImportantFiles = sortedKeysByScore(important)
	result.Documentation = sortedKeys(docs)

	return result, nil
}

func Summary(result ScanResult) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Workspace: %s\n", result.Root)
	fmt.Fprintf(&builder, "- Files indexed: %d\n", len(result.Files))
	fmt.Fprintf(&builder, "- Files skipped: %d\n", len(result.Skipped))
	fmt.Fprintf(&builder, "- Indexed bytes: %d\n", result.TotalIndexedSize)
	if result.LimitReached {
		fmt.Fprintf(&builder, "- Scan limit reached: true\n")
	}
	if len(result.LanguageCounts) > 0 {
		fmt.Fprintf(&builder, "- Detected languages: %s\n", formatLanguageCounts(result.LanguageCounts))
	}
	if len(result.TopLevelDirs) > 0 {
		fmt.Fprintf(&builder, "- Top-level folders: %s\n", strings.Join(limitStrings(result.TopLevelDirs, 12), ", "))
	}
	if len(result.ImportantFiles) > 0 {
		fmt.Fprintf(&builder, "- Key files: %s\n", strings.Join(limitStrings(result.ImportantFiles, 12), ", "))
	}
	if len(result.Documentation) > 0 {
		fmt.Fprintf(&builder, "- Docs found: %s\n", strings.Join(limitStrings(result.Documentation, 12), ", "))
	}
	return strings.TrimRight(builder.String(), "\n")
}

func applyLimitDefaults(limits Limits) Limits {
	defaults := DefaultLimits()
	if limits.MaxFilesScanned == 0 {
		limits.MaxFilesScanned = defaults.MaxFilesScanned
	}
	if limits.MaxFileBytes == 0 {
		limits.MaxFileBytes = defaults.MaxFileBytes
	}
	if limits.MaxTotalScanBytes == 0 {
		limits.MaxTotalScanBytes = defaults.MaxTotalScanBytes
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

func safeRel(root string, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func firstPathPart(rel string) string {
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysByScore(values map[string]bool) []string {
	keys := sortedKeys(values)
	sort.SliceStable(keys, func(i, j int) bool {
		left := importantScore(keys[i])
		right := importantScore(keys[j])
		if left == right {
			return keys[i] < keys[j]
		}
		return left > right
	})
	return keys
}

func formatLanguageCounts(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", key, counts[key]))
	}
	return strings.Join(parts, ", ")
}

func limitStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	limited := append([]string{}, values[:limit]...)
	limited = append(limited, fmt.Sprintf("+%d more", len(values)-limit))
	return limited
}

func fileKind(path string) string {
	if isDocumentationPath(path) {
		return "doc"
	}
	return "code"
}
