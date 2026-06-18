package workflows

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const yemakaHomeEnv = "YEMAKA_HOME"

// BuiltInTemplateRoot returns the directory that contains packs/templates.
func BuiltInTemplateRoot() (string, error) {
	seen := map[string]struct{}{}
	for _, candidate := range builtInTemplateRootCandidates() {
		if root, ok := findBuiltInTemplateRoot(candidate, seen); ok {
			return root, nil
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "unknown"
	}
	return "", fmt.Errorf("built-in domain pack templates not found; checked %s/packs/templates, executable-relative packs/templates, and cwd parents from %s", yemakaHomeEnv, cwd)
}

func builtInTemplateRootCandidates() []string {
	candidates := []string{}
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path != "" {
			candidates = append(candidates, path)
		}
	}

	add(os.Getenv(yemakaHomeEnv))
	if executable, err := os.Executable(); err == nil && executable != "" {
		executableDir := filepath.Dir(executable)
		add(executableDir)
		add(filepath.Join(executableDir, ".."))
		add(filepath.Join(executableDir, "..", "share", "yemaka"))
	}
	if cwd, err := os.Getwd(); err == nil {
		add(cwd)
	}
	return candidates
}

func findBuiltInTemplateRoot(start string, seen map[string]struct{}) (string, bool) {
	start = strings.TrimSpace(start)
	if start == "" {
		return "", false
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		if _, exists := seen[dir]; exists {
			return "", false
		}
		seen[dir] = struct{}{}
		if hasBuiltInTemplateDir(dir) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func hasBuiltInTemplateDir(root string) bool {
	info, err := os.Stat(filepath.Join(root, "packs", "templates"))
	return err == nil && info.IsDir()
}
