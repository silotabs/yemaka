package extensions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFailuresDetectsRepeatedExtensionFailures(t *testing.T) {
	root := t.TempDir()
	logs := filepath.Join(root, "logs")
	store := NewStore(filepath.Join(root, "extensions", "generated"), logs)
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	appendJSONLine(t, filepath.Join(logs, runsFile), RunResult{
		Extension:   "website_monitor",
		Status:      "failed",
		Error:       "bad response",
		CompletedAt: "2026-05-08T10:00:00Z",
	})
	appendJSONLine(t, filepath.Join(logs, testsFile), TestResult{
		Name:        "website_monitor",
		Status:      "failed",
		Output:      "test failed",
		CompletedAt: "2026-05-08T10:01:00Z",
	})
	failures, err := store.Failures(10)
	if err != nil {
		t.Fatalf("Failures() error = %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("failure count = %d, want 1", len(failures))
	}
	if failures[0].Name != "website_monitor" || failures[0].Failures != 2 || !failures[0].SuggestReview {
		t.Fatalf("failure = %+v, want repeated website_monitor review suggestion", failures[0])
	}
}

func appendJSONLine(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
