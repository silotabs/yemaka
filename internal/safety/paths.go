package safety

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PathPolicy struct {
	WorkspaceRoot  string
	FollowSymlinks bool
	IncludeHidden  bool
}

func ResolveWorkspace(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		path = "."
	}
	path = normalizeUserSuppliedPath(path)
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve workspace path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("stat workspace path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace path is not a directory: %s", path)
	}
	return filepath.Clean(abs), nil
}

func ResolveWorkspaceTarget(path string) (string, bool, error) {
	if strings.TrimSpace(path) == "" {
		path = "."
	}
	path = normalizeUserSuppliedPath(path)
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false, fmt.Errorf("resolve workspace path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", false, fmt.Errorf("stat workspace path: %w", err)
	}
	return filepath.Clean(abs), info.IsDir(), nil
}

func ResolveWorkspaceGrantRoot(path string) (string, error) {
	abs, isDir, err := ResolveWorkspaceTarget(path)
	if err != nil {
		return "", err
	}
	if isDir {
		return abs, nil
	}
	return filepath.Dir(abs), nil
}

func ResolveReadPath(policy PathPolicy, target string) (string, string, error) {
	root, err := ResolveWorkspace(policy.WorkspaceRoot)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(target) == "" {
		return "", "", fmt.Errorf("path is empty")
	}
	target = normalizeUserSuppliedPath(target)

	var abs string
	if filepath.IsAbs(target) {
		abs = filepath.Clean(target)
	} else {
		abs = filepath.Clean(filepath.Join(root, target))
	}

	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", "", fmt.Errorf("resolve path relative to workspace: %w", err)
	}
	if rel == "." {
		return "", "", fmt.Errorf("path points to workspace root, not a file")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("path is outside workspace: %s", target)
	}
	rel = filepath.ToSlash(rel)

	if IsProtectedPath(rel) {
		return "", "", fmt.Errorf("protected path is blocked: %s", rel)
	}
	if !policy.IncludeHidden && IsHiddenPath(rel) {
		return "", "", fmt.Errorf("hidden paths are disabled: %s", rel)
	}
	if !policy.FollowSymlinks {
		if err := rejectSymlinkPath(root, abs); err != nil {
			return "", "", err
		}
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", "", fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return "", "", fmt.Errorf("path is a directory: %s", rel)
	}

	return abs, rel, nil
}

func ResolveDirectoryPath(policy PathPolicy, target string) (string, string, error) {
	root, err := ResolveWorkspace(policy.WorkspaceRoot)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(target) == "" {
		target = "."
	}
	target = normalizeUserSuppliedPath(target)

	var abs string
	if filepath.IsAbs(target) {
		abs = filepath.Clean(target)
	} else {
		abs = filepath.Clean(filepath.Join(root, target))
	}

	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", "", fmt.Errorf("resolve path relative to workspace: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("path is outside workspace: %s", target)
	}
	rel = filepath.ToSlash(rel)

	if rel != "." {
		if IsProtectedPath(rel) {
			return "", "", fmt.Errorf("protected path is blocked: %s", rel)
		}
		if !policy.IncludeHidden && IsHiddenPath(rel) {
			return "", "", fmt.Errorf("hidden paths are disabled: %s", rel)
		}
	}
	if !policy.FollowSymlinks {
		if err := rejectSymlinkPath(root, abs); err != nil {
			return "", "", err
		}
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", "", fmt.Errorf("stat directory: %w", err)
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("path is not a directory: %s", rel)
	}

	return abs, rel, nil
}

func ResolveWritePath(policy PathPolicy, target string) (string, string, bool, error) {
	root, err := ResolveWorkspace(policy.WorkspaceRoot)
	if err != nil {
		return "", "", false, err
	}
	if strings.TrimSpace(target) == "" {
		return "", "", false, fmt.Errorf("path is empty")
	}
	target = normalizeUserSuppliedPath(target)

	var abs string
	if filepath.IsAbs(target) {
		abs = filepath.Clean(target)
	} else {
		abs = filepath.Clean(filepath.Join(root, target))
	}

	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", "", false, fmt.Errorf("resolve path relative to workspace: %w", err)
	}
	if rel == "." {
		return "", "", false, fmt.Errorf("path points to workspace root, not a file")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", false, fmt.Errorf("path is outside workspace: %s", target)
	}
	rel = filepath.ToSlash(rel)

	if IsProtectedPath(rel) {
		return "", "", false, fmt.Errorf("protected path is blocked: %s", rel)
	}
	if !policy.IncludeHidden && IsHiddenPath(rel) {
		return "", "", false, fmt.Errorf("hidden paths are disabled: %s", rel)
	}
	if !policy.FollowSymlinks {
		if err := rejectSymlinkPath(root, filepath.Dir(abs)); err != nil {
			return "", "", false, err
		}
		if info, err := os.Lstat(abs); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", "", false, fmt.Errorf("symlink paths are disabled: %s", abs)
		}
	}

	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return abs, rel, false, nil
		}
		return "", "", false, fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return "", "", false, fmt.Errorf("path is a directory: %s", rel)
	}
	return abs, rel, true, nil
}

func normalizeUserSuppliedPath(path string) string {
	return NormalizeUserSuppliedPath(path)
}

func NormalizeUserSuppliedPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return path
	}
	slash := filepath.ToSlash(path)
	if filepath.IsAbs(path) {
		trimmed := strings.Trim(slash, "/")
		lower := strings.ToLower(trimmed)
		if lower == "users" {
			return normalizeUsersPrefixPath("")
		}
		if strings.HasPrefix(lower, "users/") {
			return normalizeUsersPrefixPath(trimmed[len("users/"):])
		}
		return path
	}
	if slash == "~" || strings.HasPrefix(slash, "~/") {
		if home, ok := cleanUserHomeDir(); ok {
			if slash == "~" {
				return home
			}
			rest := canonicalizeHomeRelativeRest(strings.TrimPrefix(slash, "~/"))
			return filepath.Join(home, filepath.FromSlash(rest))
		}
	}
	lower := strings.ToLower(slash)
	if strings.HasPrefix(lower, "users/") {
		return normalizeUsersPrefixPath(slash[len("users/"):])
	}
	if normalized, ok := normalizeHomeRelativeAliasPath(slash); ok {
		return normalized
	}
	return path
}

func normalizeHomeRelativeAliasPath(slash string) (string, bool) {
	trimmed := strings.Trim(filepath.ToSlash(slash), "/")
	if trimmed == "" {
		return "", false
	}
	segment, rest, hasRest := strings.Cut(trimmed, "/")
	homeSegment, ok := canonicalHomeFolderAlias(segment)
	if !ok {
		return "", false
	}
	home, ok := cleanUserHomeDir()
	if !ok {
		return "", false
	}
	if !hasRest || rest == "" {
		return filepath.Join(home, homeSegment), true
	}
	return filepath.Join(home, homeSegment, filepath.FromSlash(rest)), true
}

func canonicalHomeFolderAlias(segment string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(segment)) {
	case "desktop":
		return "Desktop", true
	case "documents":
		return "Documents", true
	case "downloads":
		return "Downloads", true
	case "library":
		return "Library", true
	case "movies":
		return "Movies", true
	case "music":
		return "Music", true
	case "pictures":
		return "Pictures", true
	case "public":
		return "Public", true
	case "desktop folder":
		return "Desktop", true
	case "documents folder":
		return "Documents", true
	case "downloads folder":
		return "Downloads", true
	case "library folder":
		return "Library", true
	case "movies folder":
		return "Movies", true
	case "music folder":
		return "Music", true
	case "pictures folder":
		return "Pictures", true
	case "public folder":
		return "Public", true
	default:
		return "", false
	}
}

func normalizeUsersPrefixPath(userPath string) string {
	userPath = strings.Trim(filepath.ToSlash(userPath), "/")
	original := "users"
	if userPath != "" {
		original += "/" + userPath
	}
	if userPath == "" {
		if home, ok := cleanUserHomeDir(); ok {
			homeParent := filepath.Dir(home)
			if strings.EqualFold(filepath.Base(homeParent), "Users") {
				return homeParent
			}
		}
		return filepath.FromSlash(original)
	}
	userName, rest, hasRest := strings.Cut(userPath, "/")
	if home, ok := cleanUserHomeDir(); ok {
		if strings.EqualFold(filepath.Base(home), userName) {
			if !hasRest || rest == "" {
				return home
			}
			return filepath.Join(home, filepath.FromSlash(canonicalizeHomeRelativeRest(rest)))
		}
		homeParent := filepath.Dir(home)
		if strings.EqualFold(filepath.Base(homeParent), "Users") {
			if !hasRest || rest == "" {
				return filepath.Join(homeParent, userName)
			}
			return filepath.Join(homeParent, userName, filepath.FromSlash(canonicalizeHomeRelativeRest(rest)))
		}
	}
	return filepath.FromSlash(original)
}

func canonicalizeHomeRelativeRest(rest string) string {
	rest = strings.Trim(filepath.ToSlash(rest), "/")
	if rest == "" {
		return rest
	}
	segment, tail, hasTail := strings.Cut(rest, "/")
	homeSegment, ok := canonicalHomeFolderAlias(segment)
	if !ok {
		return rest
	}
	if !hasTail || tail == "" {
		return homeSegment
	}
	return homeSegment + "/" + tail
}

func cleanUserHomeDir() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", false
	}
	return filepath.Clean(home), true
}

func IsHiddenPath(rel string) bool {
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if strings.HasPrefix(part, ".") && part != "." && part != ".." {
			return true
		}
	}
	return false
}

func IsProtectedPath(rel string) bool {
	base := filepath.Base(rel)
	if base == ".env" || strings.HasPrefix(base, ".env.") {
		return true
	}
	for _, exact := range []string{"id_rsa", "id_ed25519"} {
		if base == exact {
			return true
		}
	}
	for _, suffix := range []string{".pem", ".key", ".p12", ".sqlite", ".db"} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}

func rejectSymlinkPath(root string, abs string) error {
	root = filepath.Clean(root)
	abs = filepath.Clean(abs)

	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return err
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("stat path component: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink paths are disabled: %s", current)
		}
	}
	return nil
}
