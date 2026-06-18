package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const yemakaHomeEnv = "YEMAKA_HOME"

// DefaultDir returns the built-in default skill directory.
func DefaultDir() (string, error) {
	seen := map[string]struct{}{}
	for _, candidate := range defaultDirCandidates() {
		if dir, ok := findDefaultDir(candidate, seen); ok {
			return dir, nil
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "unknown"
	}
	return "", fmt.Errorf("default skills not found; checked %s/skills/default, executable-relative skills/default, and cwd parents from %s", yemakaHomeEnv, cwd)
}

func defaultDirCandidates() []string {
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
	if _, file, _, ok := runtime.Caller(0); ok {
		add(filepath.Dir(file))
	}
	if cwd, err := os.Getwd(); err == nil {
		add(cwd)
	}
	return candidates
}

func findDefaultDir(start string, seen map[string]struct{}) (string, bool) {
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
		defaultDir := filepath.Join(dir, "skills", "default")
		if info, err := os.Stat(defaultDir); err == nil && info.IsDir() {
			return defaultDir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
