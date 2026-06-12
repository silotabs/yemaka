package evaluation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yemaka/internal/profiles"
)

const (
	StatusPass = "pass"
	StatusFail = "fail"
	StatusSkip = "skip"
)

type Report struct {
	ID          string         `json:"id"`
	Mode        string         `json:"mode"`
	Model       string         `json:"model"`
	StartedAt   string         `json:"started_at"`
	CompletedAt string         `json:"completed_at"`
	DurationMS  int64          `json:"duration_ms"`
	Summary     Summary        `json:"summary"`
	Metrics     map[string]any `json:"metrics,omitempty"`
	Tasks       []TaskResult   `json:"tasks"`
	Path        string         `json:"path"`
}

type Summary struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

type TaskResult struct {
	Name       string         `json:"name"`
	Status     string         `json:"status"`
	DurationMS int64          `json:"duration_ms"`
	Details    string         `json:"details,omitempty"`
	Metrics    map[string]any `json:"metrics,omitempty"`
}

func (r *Report) add(result TaskResult) {
	r.Tasks = append(r.Tasks, result)
	switch result.Status {
	case StatusPass:
		r.Summary.Passed++
	case StatusFail:
		r.Summary.Failed++
	case StatusSkip:
		r.Summary.Skipped++
	}
}

func Save(profile *profiles.Profile, report Report) (Report, error) {
	if profile == nil {
		return Report{}, fmt.Errorf("profile is required")
	}
	root := ReportsRoot(profile)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Report{}, fmt.Errorf("create eval reports directory: %w", err)
	}
	if report.ID == "" {
		report.ID = newReportID()
	}
	report.Path = filepath.Join(root, report.ID+".json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return Report{}, fmt.Errorf("encode eval report: %w", err)
	}
	if err := os.WriteFile(report.Path, data, 0o644); err != nil {
		return Report{}, fmt.Errorf("write eval report: %w", err)
	}
	latest := LatestReportPath(profile)
	if err := os.WriteFile(latest, data, 0o644); err != nil {
		return Report{}, fmt.Errorf("write latest eval report: %w", err)
	}
	return report, nil
}

func LoadLatest(profile *profiles.Profile) (Report, error) {
	return Load(LatestReportPath(profile))
}

func Load(path string) (Report, error) {
	if strings.TrimSpace(path) == "" {
		return Report{}, fmt.Errorf("eval report path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, fmt.Errorf("read eval report: %w", err)
	}
	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		return Report{}, fmt.Errorf("parse eval report: %w", err)
	}
	if report.Path == "" {
		report.Path = path
	}
	return report, nil
}

func ReportsRoot(profile *profiles.Profile) string {
	if profile.Evals != "" {
		return profile.Evals
	}
	return filepath.Join(profile.Root, "evals")
}

func LatestReportPath(profile *profiles.Profile) string {
	return filepath.Join(ReportsRoot(profile), "latest.json")
}

func newReportID() string {
	return "eval_" + time.Now().UTC().Format("20060102T150405.000000000Z")
}
