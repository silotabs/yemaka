package learning

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yemaka/internal/replay"
)

type RegressionArtifact struct {
	Case RegressionTestCase `json:"case"`
	Path string             `json:"path"`
}

func SaveRegressionArtifactsFromTrace(outputDir string, trace replay.Trace) ([]RegressionArtifact, error) {
	return SaveRegressionArtifacts(outputDir, GenerateRegressionTestCases(trace))
}

func SaveRegressionArtifacts(outputDir string, cases []RegressionTestCase) ([]RegressionArtifact, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return nil, fmt.Errorf("regression artifact directory is required")
	}
	if regressionArtifactPathLooksRemote(outputDir) {
		return nil, fmt.Errorf("regression artifact directory must be a local filesystem path: %s", outputDir)
	}

	normalized := make([]RegressionTestCase, 0, len(cases))
	for _, testCase := range cases {
		testCase = normalizeRegressionCase(testCase)
		if strings.TrimSpace(testCase.Kind) == "" || strings.TrimSpace(testCase.Prompt) == "" {
			continue
		}
		normalized = append(normalized, testCase)
	}
	if len(normalized) == 0 {
		return nil, nil
	}

	base := filepath.Clean(outputDir)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return nil, fmt.Errorf("create regression artifact directory: %w", err)
	}
	info, err := os.Stat(base)
	if err != nil {
		return nil, fmt.Errorf("stat regression artifact directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("regression artifact path is not a directory: %s", outputDir)
	}

	artifacts := make([]RegressionArtifact, 0, len(normalized))
	for i, testCase := range normalized {
		filename := regressionArtifactFilename(i, testCase)
		path, err := regressionArtifactPath(base, filename)
		if err != nil {
			return nil, err
		}
		content := FormatRegressionTestCaseYAMLish(testCase) + "\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("write regression artifact %s: %w", filename, err)
		}
		artifacts = append(artifacts, RegressionArtifact{
			Case: testCase,
			Path: path,
		})
	}
	return artifacts, nil
}

func regressionArtifactFilename(index int, testCase RegressionTestCase) string {
	name := sanitizeRegressionName(testCase.Name)
	if name == "" {
		name = regressionCaseName(testCase.Kind, testCase.Prompt, testCase.SourceTraceID)
	}
	return fmt.Sprintf("%03d_%s.yml", index+1, name)
}

func regressionArtifactPath(base string, filename string) (string, error) {
	if filename == "" || filepath.IsAbs(filename) {
		return "", fmt.Errorf("unsafe regression artifact filename: %q", filename)
	}
	path := filepath.Join(base, filename)
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return "", fmt.Errorf("validate regression artifact path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("regression artifact path escapes output directory: %s", filename)
	}
	return path, nil
}

func regressionArtifactPathLooksRemote(path string) bool {
	path = strings.ToLower(strings.TrimSpace(path))
	return strings.HasPrefix(path, "git@") || strings.Contains(path, "://")
}
