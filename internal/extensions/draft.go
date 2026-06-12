package extensions

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// PackageDraft is the narrow acceptance format for task-specific generated
// extension code. The trusted core validates it before any file is written.
type PackageDraft struct {
	Files  []DraftFile `json:"files"`
	Origin string      `json:"origin,omitempty"`
}

type DraftFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type normalizedDraft struct {
	Files    map[string]string
	Manifest Manifest
}

func (s *Store) writeDraftGeneratedPackage(dir string, draft *PackageDraft, normalized normalizedDraft) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create generated extension package: %w", err)
	}
	files := normalized.Files
	origin := strings.TrimSpace(draft.Origin)
	if origin == "" {
		origin = "generated_by=yemaka\nmilestone=4.17\nsource=draft_gate\n"
	}
	files[".yemaka_origin"] = strings.TrimRight(origin, "\n") + "\n"

	names := make([]string, 0, len(files))
	for filename := range files {
		names = append(names, filename)
	}
	sort.Strings(names)
	for _, filename := range names {
		path := filepath.Join(dir, filename)
		if err := ensureChild(dir, path); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create generated extension directory for %s: %w", filename, err)
		}
		if err := os.WriteFile(path, []byte(files[filename]), fileMode(filename)); err != nil {
			return fmt.Errorf("write generated extension file %s: %w", filename, err)
		}
	}
	return nil
}

func (s *Store) normalizePackageDraft(name string, draft *PackageDraft) (normalizedDraft, error) {
	if draft == nil {
		return normalizedDraft{}, fmt.Errorf("extension draft is required")
	}
	if len(draft.Files) == 0 {
		return normalizedDraft{}, fmt.Errorf("extension draft must include files")
	}
	if len(draft.Files) > maxPackageFiles {
		return normalizedDraft{}, fmt.Errorf("extension draft has too many files: %d > %d", len(draft.Files), maxPackageFiles)
	}

	files := map[string]string{}
	var totalBytes int64
	for _, file := range draft.Files {
		clean, err := cleanDraftPath(file.Path)
		if err != nil {
			return normalizedDraft{}, err
		}
		if _, exists := files[clean]; exists {
			return normalizedDraft{}, fmt.Errorf("extension draft contains duplicate file: %s", clean)
		}
		size := int64(len(file.Content))
		if size > maxFileBytes {
			return normalizedDraft{}, fmt.Errorf("extension draft file exceeds %d bytes: %s", maxFileBytes, clean)
		}
		totalBytes += size
		if totalBytes > maxPackageBytes {
			return normalizedDraft{}, fmt.Errorf("extension draft exceeds %d bytes", maxPackageBytes)
		}
		files[clean] = file.Content
	}

	for _, required := range requiredDraftFiles() {
		if _, ok := files[required]; !ok {
			return normalizedDraft{}, fmt.Errorf("extension draft missing required file: %s", required)
		}
	}

	var manifest Manifest
	if err := yaml.Unmarshal([]byte(files[ManifestFile]), &manifest); err != nil {
		return normalizedDraft{}, fmt.Errorf("decode extension draft manifest: %w", err)
	}
	if manifest.Name != name {
		return normalizedDraft{}, fmt.Errorf("extension draft manifest name %q does not match requested name %q", manifest.Name, name)
	}
	if err := ValidateManifestWithPolicyMode(manifest, s.PolicyMode); err != nil {
		return normalizedDraft{}, err
	}
	if err := validateRunnableManifestWithPolicyMode(manifest, s.PolicyMode); err != nil {
		return normalizedDraft{}, err
	}
	if len(manifest.Tests) == 0 {
		return normalizedDraft{}, fmt.Errorf("extension draft manifest must declare at least one test command")
	}
	for _, testCommand := range manifest.Tests {
		if err := validateTestCommand(testCommand); err != nil {
			return normalizedDraft{}, err
		}
	}
	if err := validateDraftEntrypointFile(manifest, files); err != nil {
		return normalizedDraft{}, err
	}

	return normalizedDraft{Files: files, Manifest: manifest}, nil
}

func validateDraftEntrypointFile(manifest Manifest, files map[string]string) error {
	entrypoint, hasPath, err := manifestEntrypointPath(manifest)
	if err != nil {
		return err
	}
	if !hasPath {
		return nil
	}
	if strings.HasPrefix(entrypoint, "./") {
		entrypoint = strings.TrimPrefix(entrypoint, "./")
	}
	if _, ok := files[entrypoint]; ok {
		return nil
	}
	if _, ok := files["main.go"]; ok && manifest.Entrypoint.Command == "./"+manifest.Name {
		return nil
	}
	return fmt.Errorf("extension draft entrypoint source is missing for %s", entrypoint)
}

func cleanDraftPath(path string) (string, error) {
	raw := strings.TrimSpace(filepath.ToSlash(path))
	if raw == "" {
		return "", fmt.Errorf("extension draft file path is required")
	}
	if filepath.IsAbs(raw) || strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("extension draft file path must be relative: %s", raw)
	}
	if unsafePackagePath(raw) {
		return "", fmt.Errorf("unsafe extension draft path: %s", raw)
	}
	for _, r := range raw {
		if r == 0 || r == '\n' || r == '\r' || unicode.IsControl(r) {
			return "", fmt.Errorf("unsafe extension draft path: %s", raw)
		}
	}
	parts := strings.Split(raw, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("unsafe extension draft path: %s", raw)
		}
		if blockedPackageDirs[part] {
			return "", fmt.Errorf("blocked generated extension directory in draft: %s", raw)
		}
		if strings.HasPrefix(part, ".") && !allowedHiddenFiles[part] {
			return "", fmt.Errorf("hidden draft file is not allowed: %s", raw)
		}
	}
	clean := filepath.ToSlash(filepath.Clean(raw))
	if clean != raw {
		return "", fmt.Errorf("extension draft path must already be normalized: %s", raw)
	}
	return clean, nil
}

func generatedSnapshotFilesFromDraft(name string, draftFiles map[string]string) []string {
	files := []string{}
	for clean := range draftFiles {
		files = append(files, filepath.Join(name, clean))
	}
	files = append(files, filepath.Join(name, ".yemaka_origin"))
	sort.Strings(files)
	return files
}

func requiredDraftFiles() []string {
	return []string{
		ManifestFile,
		"README.md",
		"go.mod",
		"main.go",
		"main_test.go",
	}
}
