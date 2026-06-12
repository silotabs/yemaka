package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/replay"
)

func TestRunReplayListShowAndSaveArtifactsRedactsSecrets(t *testing.T) {
	app := newExtensionTestApp(t)
	store, err := replayStore(app)
	if err != nil {
		t.Fatalf("replayStore() error = %v", err)
	}
	trace := replay.NewTrace("Search docs with token=cli-secret-value")
	trace.ID = "trace secret"
	trace.Route = replay.RouteSnapshot{
		Category:          "internet_search",
		RiskLevel:         "medium",
		ShouldUseTool:     true,
		ShouldUseInternet: true,
		ShouldAskApproval: true,
	}
	trace.PermissionsRequested = []replay.PermissionRequest{{
		ToolName:          "internet_search",
		Status:            "denied",
		RiskLevel:         "medium",
		Reason:            "internet disabled for token=cli-secret-value",
		PolicyExplanation: "internet access requires explicit approval",
	}}
	trace.ToolsCalled = []replay.ToolCall{{
		Name:   "internet_search",
		Status: "failed",
		Error:  "provider rejected token=cli-secret-value",
	}}
	trace.AppendToolConsideration(replay.ToolConsideration{
		Name:      "internet_search",
		Source:    "plan",
		Status:    "planned",
		Reason:    "search provider needed for token=cli-secret-value",
		RiskLevel: "medium",
	})
	if _, err := store.Save(trace); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var out bytes.Buffer
	if err := runReplay(app, []string{"list"}, &out); err != nil {
		t.Fatalf("runReplay(list) error = %v", err)
	}
	listOutput := out.String()
	if !strings.Contains(listOutput, `"id": "trace secret"`) || !strings.Contains(listOutput, `"filename": "trace-secret.json"`) {
		t.Fatalf("list output = %q, want saved replay trace", listOutput)
	}
	if strings.Contains(listOutput, "cli-secret-value") {
		t.Fatalf("list output leaked secret: %s", listOutput)
	}

	out.Reset()
	if err := runReplay(app, []string{"show", "trace secret"}, &out); err != nil {
		t.Fatalf("runReplay(show) error = %v", err)
	}
	showOutput := out.String()
	if strings.Contains(showOutput, "cli-secret-value") {
		t.Fatalf("show output leaked secret: %s", showOutput)
	}
	if !strings.Contains(showOutput, "[redacted]") {
		t.Fatalf("show output = %q, want redaction marker", showOutput)
	}

	out.Reset()
	if err := runReplay(app, []string{"explain", "trace secret"}, &out); err != nil {
		t.Fatalf("runReplay(explain) error = %v", err)
	}
	explainOutput := out.String()
	if strings.Contains(explainOutput, "cli-secret-value") {
		t.Fatalf("explain output leaked secret: %s", explainOutput)
	}
	for _, want := range []string{`"toolsConsidered"`, `"toolsUsed"`, `"permissions"`, `"diagnosticWarnings"`} {
		if !strings.Contains(explainOutput, want) {
			t.Fatalf("explain output = %q, want %s", explainOutput, want)
		}
	}

	out.Reset()
	outDir := filepath.Join(t.TempDir(), "regressions")
	if err := runReplay(app, []string{"save-artifacts", "trace secret", "--out", outDir}, &out); err != nil {
		t.Fatalf("runReplay(save-artifacts) error = %v", err)
	}
	artifactOutput := out.String()
	if strings.Contains(artifactOutput, "cli-secret-value") {
		t.Fatalf("artifact output leaked secret: %s", artifactOutput)
	}
	if !strings.Contains(artifactOutput, `"kind": "permission_failure"`) {
		t.Fatalf("artifact output = %q, want permission regression artifact", artifactOutput)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("ReadDir(outDir) error = %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("artifact count = 0, want at least one artifact")
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(outDir, entry.Name()))
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", entry.Name(), err)
		}
		text := string(data)
		if strings.Contains(text, "cli-secret-value") {
			t.Fatalf("artifact %s leaked secret:\n%s", entry.Name(), text)
		}
		if strings.Contains(text, "token=") && !strings.Contains(text, "token=[REDACTED]") {
			t.Fatalf("artifact %s has unredacted token assignment:\n%s", entry.Name(), text)
		}
	}
}
