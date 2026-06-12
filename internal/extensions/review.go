package extensions

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	maxReviewFileBytes  = 32 * 1024
	maxReviewTotalBytes = 96 * 1024
)

type Review struct {
	Detail                Detail         `json:"detail"`
	Files                 []ReviewFile   `json:"files"`
	SuggestedActions      []string       `json:"suggestedActions"`
	ReuseCommand          string         `json:"reuseCommand"`
	SampleInput           map[string]any `json:"sampleInput,omitempty"`
	SampleInputJSON       string         `json:"sampleInputJson,omitempty"`
	CanRun                bool           `json:"canRun"`
	CanRegister           bool           `json:"canRegister"`
	RunBlockedReason      string         `json:"runBlockedReason,omitempty"`
	RegisterBlockedReason string         `json:"registerBlockedReason,omitempty"`
}

type ReviewFile struct {
	Path          string `json:"path"`
	Size          int64  `json:"size"`
	Executable    bool   `json:"executable"`
	Content       string `json:"content,omitempty"`
	Truncated     bool   `json:"truncated,omitempty"`
	Binary        bool   `json:"binary,omitempty"`
	OmittedReason string `json:"omittedReason,omitempty"`
}

func (s *Store) Review(name string) (Review, error) {
	detail, err := s.Show(name)
	if err != nil {
		return Review{}, err
	}
	sampleInput := SampleInputForManifest(detail.Manifest)
	sampleInputJSON := SampleInputJSONForManifest(detail.Manifest)
	canRegister, registerBlockedReason := s.reviewRegisterReadiness(detail)
	runBlockedReason := ""
	if !detail.Status.Callable {
		runBlockedReason = strings.TrimSpace(detail.Status.RunBlockedReason)
	}
	review := Review{
		Detail:                detail,
		Files:                 s.reviewFiles(detail),
		SuggestedActions:      suggestedReviewActions(detail),
		ReuseCommand:          fmt.Sprintf("yemaka extension run %s '%s'", detail.Status.Name, sampleInputJSON),
		SampleInput:           sampleInput,
		SampleInputJSON:       sampleInputJSON,
		CanRun:                detail.Status.Callable,
		CanRegister:           canRegister,
		RunBlockedReason:      runBlockedReason,
		RegisterBlockedReason: registerBlockedReason,
	}
	return review, nil
}

func (s *Store) reviewRegisterReadiness(detail Detail) (bool, string) {
	if !detail.Status.Valid {
		return false, strings.TrimSpace(detail.Status.ValidationError)
	}
	if err := validateRunnableManifestWithPolicyMode(detail.Manifest, s.PolicyMode); err != nil {
		return false, err.Error()
	}
	return true, ""
}

func (s *Store) reviewFiles(detail Detail) []ReviewFile {
	dir := detail.Status.Dir
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	files := append([]PackageFile(nil), detail.Inspection.Files...)
	slices.SortFunc(files, func(a PackageFile, b PackageFile) int {
		return reviewFileRank(a.Path) - reviewFileRank(b.Path)
	})

	reviewFiles := make([]ReviewFile, 0, len(files))
	var totalBytes int64
	for _, file := range files {
		item := ReviewFile{
			Path:       file.Path,
			Size:       file.Size,
			Executable: file.Executable,
		}
		if unsafePackagePath(file.Path) {
			item.OmittedReason = "unsafe generated package path"
			reviewFiles = append(reviewFiles, item)
			continue
		}
		if totalBytes >= maxReviewTotalBytes {
			item.OmittedReason = "review preview byte budget reached"
			reviewFiles = append(reviewFiles, item)
			continue
		}
		content, truncated, binary, err := readReviewPreview(filepath.Join(dir, filepath.FromSlash(file.Path)))
		if err != nil {
			item.OmittedReason = err.Error()
		} else if binary {
			item.Binary = true
			item.OmittedReason = "binary or non-UTF-8 file"
		} else {
			item.Content = content
			item.Truncated = truncated
			totalBytes += int64(len(content))
		}
		reviewFiles = append(reviewFiles, item)
	}
	slices.SortFunc(reviewFiles, func(a ReviewFile, b ReviewFile) int {
		aRank := reviewFileRank(a.Path)
		bRank := reviewFileRank(b.Path)
		if aRank != bRank {
			return aRank - bRank
		}
		return strings.Compare(a.Path, b.Path)
	})
	return reviewFiles
}

func readReviewPreview(path string) (content string, truncated bool, binary bool, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", false, false, fmt.Errorf("read review file: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxReviewFileBytes+1))
	if err != nil {
		return "", false, false, fmt.Errorf("read review file: %w", err)
	}
	if len(data) > maxReviewFileBytes {
		data = data[:maxReviewFileBytes]
		truncated = true
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return "", truncated, true, nil
	}
	return string(data), truncated, false, nil
}

func reviewFileRank(path string) int {
	switch path {
	case ManifestFile:
		return 0
	case "README.md":
		return 1
	case "main.go", "main.py", "main.js", "index.js":
		return 2
	case "main_test.go", "test_main.py":
		return 3
	case "go.mod", "package.json":
		return 4
	default:
		if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, ".test.js") {
			return 5
		}
		return 20
	}
}

func suggestedReviewActions(detail Detail) []string {
	status := detail.Status
	actions := []string{"Review extension.yaml, entrypoint source, tests, and package inspection before reuse."}
	if len(detail.Inspection.Errors) > 0 {
		actions = append(actions, "Fix hardening errors before testing or registration.")
	}
	if len(detail.Inspection.Warnings) > 0 {
		actions = append(actions, "Check hardening warnings; generated code should stay small and easy to inspect.")
	}
	if !status.Valid {
		actions = append(actions, "Run validation after fixing the manifest or package.")
		return actions
	}
	if !status.Registered {
		actions = append(actions, "Run tests and register only if the package still matches the requested capability.")
		return actions
	}
	if !status.Enabled {
		actions = append(actions, "Enable the extension before trying to reuse it from chat, CLI, web, desktop, or TUI.")
		return actions
	}
	if status.Callable {
		actions = append(actions, "Reuse the existing extension with typed JSON input instead of generating a duplicate.")
	}
	return actions
}
