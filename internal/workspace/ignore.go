package workspace

import (
	"path/filepath"
	"strings"

	"yemaka/internal/safety"
)

var ignoredDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	"target":       true,
	".cache":       true,
	".svelte-kit":  true,
	"wailsjs":      true,
}

var supportedTextExts = map[string]string{
	".go":     "Go",
	".js":     "JavaScript",
	".jsx":    "JavaScript",
	".ts":     "TypeScript",
	".tsx":    "TypeScript",
	".svelte": "Svelte",
	".py":     "Python",
	".md":     "Markdown",
	".txt":    "Text",
	".json":   "JSON",
	".yaml":   "YAML",
	".yml":    "YAML",
	".toml":   "TOML",
	".mod":    "Go",
	".sum":    "Go",
	".html":   "HTML",
	".css":    "CSS",
}

func shouldSkipDir(rel string, name string, limits Limits) (bool, string) {
	if ignoredDirs[name] {
		return true, "ignored directory"
	}
	if !limits.IncludeHidden && strings.HasPrefix(name, ".") {
		return true, "hidden directory"
	}
	if safety.IsProtectedPath(rel) {
		return true, "protected path"
	}
	return false, ""
}

func shouldSkipFile(rel string, name string, limits Limits) (bool, string) {
	if !limits.IncludeHidden && safety.IsHiddenPath(rel) {
		return true, "hidden file"
	}
	if safety.IsProtectedPath(rel) {
		return true, "protected path"
	}
	if name == ".DS_Store" {
		return true, "system file"
	}
	if strings.HasSuffix(name, ".lock") {
		return true, "lock file"
	}
	return false, ""
}

func languageForPath(path string) string {
	base := filepath.Base(path)
	if strings.EqualFold(base, "README") || strings.EqualFold(base, "LICENSE") {
		return "Text"
	}
	ext := strings.ToLower(filepath.Ext(path))
	if language, ok := supportedTextExts[ext]; ok {
		return language
	}
	return ""
}

func LanguageForPath(path string) string {
	return languageForPath(path)
}

func isDocumentationPath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return base == "readme.md" ||
		base == "readme.txt" ||
		strings.HasPrefix(filepath.ToSlash(path), "docs/") ||
		strings.HasSuffix(base, ".md")
}

func importantScore(path string) int {
	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "readme.md", "readme.txt":
		return 100
	case "go.mod", "package.json", "wails.json", "vite.config.ts", "svelte.config.js":
		return 90
	case "main.go", "app.go":
		return 70
	}
	if strings.HasPrefix(filepath.ToSlash(path), "docs/") && strings.HasSuffix(base, ".md") {
		return 60
	}
	return 0
}
