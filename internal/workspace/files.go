package workspace

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"yemaka/internal/safety"
)

func ReadFile(ctx context.Context, root string, path string, limits Limits) (ReadResult, error) {
	limits = applyLimitDefaults(limits)
	abs, rel, err := safety.ResolveReadPath(safety.PathPolicy{
		WorkspaceRoot:  root,
		FollowSymlinks: limits.FollowSymlinks,
		IncludeHidden:  limits.IncludeHidden,
	}, path)
	if err != nil {
		return ReadResult{}, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return ReadResult{}, fmt.Errorf("stat file: %w", err)
	}
	if info.Size() > limits.MaxFileBytes {
		return ReadResult{}, fmt.Errorf("file exceeds max read size: %s", rel)
	}
	if languageForPath(rel) == "" {
		return ReadResult{}, fmt.Errorf("unsupported text file type: %s", rel)
	}

	content, err := readText(ctx, abs, limits.MaxFileBytes)
	if err != nil {
		return ReadResult{}, err
	}
	return ReadResult{Path: rel, Content: content, Size: info.Size()}, nil
}

func SearchFiles(ctx context.Context, root string, query string, limits Limits) ([]SearchMatch, error) {
	limits = applyLimitDefaults(limits)
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("search query is empty")
	}

	scan, err := Scan(root, limits)
	if err != nil {
		return nil, err
	}

	needle := strings.ToLower(query)
	var matches []SearchMatch
	for _, file := range scan.Files {
		if len(matches) >= limits.MaxSearchResults {
			break
		}
		content, err := ReadFile(ctx, scan.Root, file.Path, limits)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(strings.NewReader(content.Content))
		scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			if strings.Contains(strings.ToLower(line), needle) {
				matches = append(matches, SearchMatch{
					Path: file.Path,
					Line: lineNo,
					Text: strings.TrimSpace(line),
				})
				if len(matches) >= limits.MaxSearchResults {
					break
				}
			}
		}
	}
	return matches, nil
}

func ListFiles(ctx context.Context, root string, path string, limits Limits) (ListResult, error) {
	limits = applyLimitDefaults(limits)
	abs, rel, err := safety.ResolveDirectoryPath(safety.PathPolicy{
		WorkspaceRoot:  root,
		FollowSymlinks: limits.FollowSymlinks,
		IncludeHidden:  limits.IncludeHidden,
	}, path)
	if err != nil {
		return ListResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return ListResult{}, err
	}

	dirEntries, err := os.ReadDir(abs)
	if err != nil {
		return ListResult{}, fmt.Errorf("read directory: %w", err)
	}
	limit := limits.MaxSearchResults
	if limit <= 0 {
		limit = DefaultLimits().MaxSearchResults
	}
	if limit < 1 {
		limit = 1
	}

	result := ListResult{Path: rel}
	for _, entry := range dirEntries {
		if err := ctx.Err(); err != nil {
			return ListResult{}, err
		}
		name := entry.Name()
		childRel := name
		if rel != "." {
			childRel = filepath.ToSlash(filepath.Join(rel, name))
		}
		if (!limits.IncludeHidden && safety.IsHiddenPath(childRel)) || safety.IsProtectedPath(childRel) {
			result.Skipped++
			continue
		}
		info, err := entry.Info()
		if err != nil {
			result.Skipped++
			continue
		}
		result.Entries = append(result.Entries, ListEntry{
			Path:  childRel,
			Name:  name,
			IsDir: entry.IsDir(),
			Size:  info.Size(),
		})
		if len(result.Entries) >= limit {
			result.Skipped += len(dirEntries) - len(result.Entries) - result.Skipped
			break
		}
	}
	sort.SliceStable(result.Entries, func(i, j int) bool {
		if result.Entries[i].IsDir != result.Entries[j].IsDir {
			return result.Entries[i].IsDir
		}
		return strings.ToLower(result.Entries[i].Name) < strings.ToLower(result.Entries[j].Name)
	})
	return result, nil
}

func StatPath(ctx context.Context, root string, path string, limits Limits) (StatResult, error) {
	limits = applyLimitDefaults(limits)
	abs, rel, exists, err := resolveStatPath(safety.PathPolicy{
		WorkspaceRoot:  root,
		FollowSymlinks: limits.FollowSymlinks,
		IncludeHidden:  limits.IncludeHidden,
	}, path)
	if err != nil {
		return StatResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return StatResult{}, err
	}
	if !exists {
		return StatResult{Path: rel, Exists: false}, nil
	}
	info, err := os.Stat(abs)
	if err != nil {
		return StatResult{}, fmt.Errorf("stat path: %w", err)
	}
	return StatResult{
		Path:    rel,
		Exists:  true,
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}, nil
}

func FileTree(ctx context.Context, root string, path string, limits Limits) (TreeResult, error) {
	limits = applyLimitDefaults(limits)
	abs, rel, err := safety.ResolveDirectoryPath(safety.PathPolicy{
		WorkspaceRoot:  root,
		FollowSymlinks: limits.FollowSymlinks,
		IncludeHidden:  limits.IncludeHidden,
	}, path)
	if err != nil {
		return TreeResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return TreeResult{}, err
	}
	resolvedRoot, err := safety.ResolveWorkspace(root)
	if err != nil {
		return TreeResult{}, err
	}
	limit := limits.MaxSearchResults
	if limit <= 0 {
		limit = DefaultLimits().MaxSearchResults
	}
	if limit < 1 {
		limit = 1
	}

	result := TreeResult{Path: rel}
	err = filepath.WalkDir(abs, func(current string, entry os.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			result.Skipped++
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if current == abs {
			return nil
		}
		childRel, err := filepath.Rel(resolvedRoot, current)
		if err != nil {
			result.Skipped++
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		childRel = filepath.ToSlash(childRel)
		if (!limits.IncludeHidden && safety.IsHiddenPath(childRel)) || safety.IsProtectedPath(childRel) {
			result.Skipped++
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !limits.FollowSymlinks {
			info, err := entry.Info()
			if err != nil {
				result.Skipped++
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if info.Mode()&os.ModeSymlink != 0 {
				result.Skipped++
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if len(result.Entries) >= limit {
			result.LimitReached = true
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			result.Skipped++
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		result.Entries = append(result.Entries, TreeEntry{
			Path:  childRel,
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
			Size:  info.Size(),
			Depth: treeDepth(rel, childRel),
		})
		return nil
	})
	if err != nil {
		return TreeResult{}, err
	}
	sort.SliceStable(result.Entries, func(i, j int) bool {
		if result.Entries[i].Path == result.Entries[j].Path {
			return false
		}
		return strings.ToLower(result.Entries[i].Path) < strings.ToLower(result.Entries[j].Path)
	})
	return result, nil
}

func resolveStatPath(policy safety.PathPolicy, target string) (string, string, bool, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		target = "."
	}
	if abs, rel, err := safety.ResolveReadPath(policy, target); err == nil {
		return abs, rel, true, nil
	}
	if abs, rel, err := safety.ResolveDirectoryPath(policy, target); err == nil {
		return abs, rel, true, nil
	}
	abs, rel, exists, err := safety.ResolveWritePath(policy, target)
	if err != nil {
		return "", "", false, err
	}
	return abs, rel, exists, nil
}

func treeDepth(rootRel string, childRel string) int {
	rootRel = strings.Trim(filepath.ToSlash(rootRel), "/")
	childRel = strings.Trim(filepath.ToSlash(childRel), "/")
	if rootRel != "" && rootRel != "." {
		childRel = strings.TrimPrefix(childRel, rootRel)
		childRel = strings.Trim(childRel, "/")
	}
	if childRel == "" {
		return 0
	}
	return strings.Count(childRel, "/")
}

func BuildQuestionContext(ctx context.Context, root string, question string, limits Limits) (QuestionContext, error) {
	limits = applyLimitDefaults(limits)
	scan, err := Scan(root, limits)
	if err != nil {
		return QuestionContext{}, err
	}

	selected := selectContextFiles(scan, question, limits.MaxContextFiles)
	var builder strings.Builder
	summary := Summary(scan)
	builder.WriteString("WORKSPACE SUMMARY:\n")
	builder.WriteString(summary)
	if looksLocalDocumentQuestion(question) {
		builder.WriteString("\n\nLOCAL DOCUMENT REQUEST:\n")
		builder.WriteString("Prefer document excerpts from docs, uploaded documents, Markdown, text, PDF, or DOCX-derived content before generic project metadata. If the answer appears in a document excerpt, answer directly and cite that document. Do not use internet context for this request.\n")
	}
	if looksStructureQuestion(question) {
		builder.WriteString("\n\nWORKSPACE STRUCTURE GUIDE:\n")
		builder.WriteString(StructureGuide(scan, 12, 4))
	}
	builder.WriteString("\n\nRELEVANT FILES:\n")

	remaining := limits.MaxContextChars
	sources := make([]string, 0, len(selected))
	for _, path := range selected {
		if remaining <= 0 {
			break
		}
		read, err := ReadFile(ctx, scan.Root, path, limits)
		if err != nil {
			continue
		}
		excerpt := truncateString(read.Content, min(remaining, perFileContextBudget(limits, len(selected))))
		remaining -= len(excerpt)
		sources = append(sources, read.Path)
		fmt.Fprintf(&builder, "[%d] %s\n%s\n\n", len(sources), read.Path, excerpt)
	}

	return QuestionContext{
		Root:    scan.Root,
		Summary: summary,
		Sources: sources,
		Text:    strings.TrimSpace(builder.String()),
	}, nil
}

func StructureGuide(result ScanResult, maxDirs int, maxFilesPerDir int) string {
	if maxDirs <= 0 {
		maxDirs = 12
	}
	if maxFilesPerDir <= 0 {
		maxFilesPerDir = 4
	}
	var builder strings.Builder
	builder.WriteString("For workspace-structure answers, describe top-level folders first, then key files. Mention only files and folders listed here.\n")
	if len(result.TopLevelDirs) > 0 {
		builder.WriteString("Top-level folders:\n")
		for _, dir := range limitStrings(result.TopLevelDirs, maxDirs) {
			if strings.HasPrefix(dir, "+") {
				fmt.Fprintf(&builder, "- %s\n", dir)
				continue
			}
			files := representativeFilesForDir(result.Files, dir, maxFilesPerDir)
			if len(files) == 0 {
				fmt.Fprintf(&builder, "- %s/\n", dir)
				continue
			}
			fmt.Fprintf(&builder, "- %s/: %s\n", dir, strings.Join(files, ", "))
		}
	}
	rootFiles := representativeRootFiles(result.Files, maxFilesPerDir)
	if len(rootFiles) > 0 {
		fmt.Fprintf(&builder, "Root/key files: %s\n", strings.Join(rootFiles, ", "))
	}
	if len(result.ImportantFiles) > 0 {
		fmt.Fprintf(&builder, "Important files by signal: %s\n", strings.Join(limitStrings(result.ImportantFiles, maxDirs), ", "))
	}
	return strings.TrimRight(builder.String(), "\n")
}

func selectContextFiles(scan ScanResult, question string, limit int) []string {
	if limit <= 0 {
		limit = DefaultLimits().MaxContextFiles
	}
	terms := queryTerms(question)
	documentIntent := looksLocalDocumentQuestion(question)
	pythonIntent := looksPythonWorkspaceQuestion(question)
	testIntent := looksTestOrBugQuestion(question)
	scores := map[string]int{}
	for _, file := range scan.Files {
		score := importantScore(file.Path)
		lowerPath := strings.ToLower(file.Path)
		if documentIntent && isDocumentationPath(file.Path) {
			score += 85
		}
		if documentIntent && strings.HasPrefix(filepath.ToSlash(lowerPath), "docs/") {
			score += 30
		}
		if pythonIntent && strings.HasSuffix(lowerPath, ".py") {
			score += 150
		}
		if pythonIntent && isTestPath(lowerPath) {
			score += 25
		}
		if testIntent && isTestPath(lowerPath) {
			score += 70
		}
		for _, term := range terms {
			if strings.Contains(lowerPath, term) {
				score += 25
			}
		}
		if file.Kind == "doc" {
			score += 15
		}
		if score > 0 {
			scores[file.Path] = score
		}
	}
	if len(scores) == 0 {
		for _, file := range scan.Files {
			scores[file.Path] = 1
		}
	}

	paths := make([]string, 0, len(scores))
	for path := range scores {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool {
		if scores[paths[i]] == scores[paths[j]] {
			return paths[i] < paths[j]
		}
		return scores[paths[i]] > scores[paths[j]]
	})
	if len(paths) > limit {
		return paths[:limit]
	}
	return paths
}

func representativeFilesForDir(files []FileInfo, dir string, limit int) []string {
	prefix := filepath.ToSlash(dir) + "/"
	var paths []string
	for _, file := range files {
		path := filepath.ToSlash(file.Path)
		if strings.HasPrefix(path, prefix) {
			paths = append(paths, path)
		}
	}
	sortRepresentativePaths(paths)
	if len(paths) > limit {
		return paths[:limit]
	}
	return paths
}

func representativeRootFiles(files []FileInfo, limit int) []string {
	var paths []string
	for _, file := range files {
		path := filepath.ToSlash(file.Path)
		if !strings.Contains(path, "/") {
			paths = append(paths, path)
		}
	}
	sortRepresentativePaths(paths)
	if len(paths) > limit {
		return paths[:limit]
	}
	return paths
}

func sortRepresentativePaths(paths []string) {
	sort.SliceStable(paths, func(i, j int) bool {
		left := importantScore(paths[i])
		right := importantScore(paths[j])
		if left == right {
			return paths[i] < paths[j]
		}
		return left > right
	})
}

func looksStructureQuestion(question string) bool {
	content := strings.ToLower(strings.Join(strings.Fields(question), " "))
	terms := []string{
		"workspace structure",
		"project structure",
		"repo structure",
		"repository structure",
		"codebase structure",
		"folder structure",
		"directory structure",
		"what files matter",
		"files matter first",
		"what should i read first",
		"where should i start",
		"explain this workspace",
		"explain this project",
		"explain this repo",
		"map this workspace",
		"map this project",
		"project map",
	}
	for _, term := range terms {
		if strings.Contains(content, term) {
			return true
		}
	}
	return false
}

func looksPythonWorkspaceQuestion(question string) bool {
	content := strings.ToLower(strings.Join(strings.Fields(question), " "))
	terms := []string{
		"python project",
		"python code",
		"python files",
		"python file",
		"python directory",
		"python folder",
		"python tests",
		"python test",
		"pytest",
		".py",
	}
	for _, term := range terms {
		if strings.Contains(content, term) {
			return true
		}
	}
	return false
}

func looksTestOrBugQuestion(question string) bool {
	content := strings.ToLower(strings.Join(strings.Fields(question), " "))
	terms := []string{
		"run tests",
		"run the tests",
		"test failure",
		"failing test",
		"bug",
		"find the bug",
		"fix",
	}
	for _, term := range terms {
		if strings.Contains(content, term) {
			return true
		}
	}
	return false
}

func isTestPath(path string) bool {
	path = filepath.ToSlash(strings.ToLower(path))
	base := filepath.Base(path)
	return strings.Contains(path, "/test") ||
		strings.Contains(path, "test/") ||
		strings.Contains(path, "tests/") ||
		strings.HasPrefix(base, "test_") ||
		strings.HasSuffix(base, "_test.py") ||
		strings.HasSuffix(base, "_test.go")
}

func looksLocalDocumentQuestion(question string) bool {
	content := strings.ToLower(strings.Join(strings.Fields(question), " "))
	terms := []string{
		"my docs",
		"my documents",
		"local docs",
		"local documents",
		"using my docs",
		"using my local documents",
		"using local documents",
		"from my docs",
		"from my documents",
		"from local docs",
		"from local documents",
		"according to the docs",
		"according to the documents",
		"ingested document",
		"ingested docs",
		"retrieved document",
		"retrieved docs",
	}
	for _, term := range terms {
		if strings.Contains(content, term) {
			return true
		}
	}
	return false
}

func readText(ctx context.Context, path string, maxBytes int64) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return "", fmt.Errorf("file exceeds max read size")
	}
	if bytes.Contains(data, []byte{0}) {
		return "", fmt.Errorf("binary file is blocked")
	}
	return string(data), nil
}

func queryTerms(input string) []string {
	raw := strings.FieldsFunc(strings.ToLower(input), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-')
	})
	stop := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "do": true,
		"does": true, "explain": true, "for": true, "how": true,
		"in": true, "is": true, "it": true, "of": true, "project": true,
		"the": true, "this": true, "to": true, "what": true, "with": true,
	}
	var terms []string
	for _, term := range raw {
		term = strings.Trim(term, "-_")
		if len(term) < 3 || stop[term] {
			continue
		}
		terms = append(terms, term)
	}
	return terms
}

func truncateString(input string, maxChars int) string {
	input = strings.TrimSpace(input)
	if maxChars <= 0 || len(input) <= maxChars {
		return input
	}
	return strings.TrimSpace(input[:maxChars]) + "\n[truncated]"
}

func perFileContextBudget(limits Limits, selected int) int {
	if selected <= 0 {
		return limits.MaxContextChars
	}
	budget := limits.MaxContextChars / selected
	if budget < 1200 {
		return 1200
	}
	if budget > 3000 {
		return 3000
	}
	return budget
}

func min(left int, right int) int {
	if left < right {
		return left
	}
	return right
}
