package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/learning"
	"yemaka/internal/memory"
	"yemaka/internal/replay"
	"yemaka/internal/routing"
)

func TestRunLearnReportCorrectionAndExport(t *testing.T) {
	ctx := context.Background()
	app := newExtensionTestApp(t)
	store, err := memory.Open(ctx, app.profile.Database)
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()
	app.store = store
	conversation := seedCLILearnConversation(t, ctx, store)
	if _, err := learning.RecordWorkflow(ctx, store, conversation.ID); err != nil {
		t.Fatalf("RecordWorkflow() error = %v", err)
	}

	var out bytes.Buffer
	if err := runLearn(ctx, app, []string{"correction", "--conversation", conversation.ID, "token=secret-value", "prefer", "tests"}, &out); err != nil {
		t.Fatalf("runLearn(correction) error = %v", err)
	}
	if !strings.Contains(out.String(), `"Kind": "correction"`) {
		t.Fatalf("correction output = %q, want correction memory", out.String())
	}

	out.Reset()
	if err := runLearn(ctx, app, []string{"route-correction", "add", "--conversation", conversation.ID, "--pattern", "email address in notebook", "--route", routing.RouteRAGSearch, "--task", routing.TaskRAG, "--tool", "rag_search", "--forbid", "internet_search", "--tag", "source:local_documents", "--original-prompt", "search @gmail.com from indexed document", "--correction", "No, use local documents."}, &out); err != nil {
		t.Fatalf("runLearn(route-correction add) error = %v", err)
	}
	var routeCorrection routing.RouteCorrection
	if err := json.Unmarshal(out.Bytes(), &routeCorrection); err != nil {
		t.Fatalf("decode route correction: %v", err)
	}
	if routeCorrection.ApprovalStatus != routing.RouteCorrectionStatusPending {
		t.Fatalf("route correction status = %q, want pending", routeCorrection.ApprovalStatus)
	}
	if !strings.Contains(out.String(), `"tags":`) || !strings.Contains(out.String(), `"originalPrompt": "search @gmail.com from indexed document"`) {
		t.Fatalf("route correction add output = %q, want tags and original prompt", out.String())
	}
	out.Reset()
	if err := runLearn(ctx, app, []string{"route-correction", "approve", routeCorrection.ID}, &out); err != nil {
		t.Fatalf("runLearn(route-correction approve) error = %v", err)
	}
	if !strings.Contains(out.String(), `"approvalStatus": "approved"`) {
		t.Fatalf("route correction approve output = %q", out.String())
	}
	out.Reset()
	if err := runLearn(ctx, app, []string{"route-correction", "list"}, &out); err != nil {
		t.Fatalf("runLearn(route-correction list) error = %v", err)
	}
	if !strings.Contains(out.String(), routeCorrection.ID) {
		t.Fatalf("route correction list output = %q, want %s", out.String(), routeCorrection.ID)
	}

	out.Reset()
	if err := runLearn(ctx, app, []string{"report"}, &out); err != nil {
		t.Fatalf("runLearn(report) error = %v", err)
	}
	if !strings.Contains(out.String(), `"workflowSuccesses": 1`) || !strings.Contains(out.String(), `"automaticTraining": false`) {
		t.Fatalf("report output = %q", out.String())
	}

	exportPath := filepath.Join(t.TempDir(), "trajectory.json")
	out.Reset()
	if err := runLearn(ctx, app, []string{"export", "--conversation", conversation.ID, "--out", exportPath}, &out); err != nil {
		t.Fatalf("runLearn(export) error = %v", err)
	}
	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if !strings.Contains(string(data), `"schema": "yemaka.trajectory.v1"`) || strings.Contains(string(data), "secret-value") {
		t.Fatalf("export = %s", string(data))
	}
}

func TestRunLearnRegressionArtifactsFromTrace(t *testing.T) {
	app := newExtensionTestApp(t)
	trace := replay.NewTrace("Search docs with token=secret-value")
	trace.AppendRouteSnapshot(replay.RouteSnapshot{Category: "internet_search", RiskLevel: "medium"})
	trace.AppendToolCall(replay.ToolCall{Name: "internet_search", Status: "failed", Error: "provider not configured"})
	tracePath := filepath.Join(t.TempDir(), "trace.json")
	data, err := json.Marshal(trace)
	if err != nil {
		t.Fatalf("marshal trace: %v", err)
	}
	if err := os.WriteFile(tracePath, data, 0o644); err != nil {
		t.Fatalf("write trace: %v", err)
	}

	outDir := filepath.Join(t.TempDir(), "regressions")
	var out bytes.Buffer
	if err := runLearn(context.Background(), app, []string{"regression-artifacts", "--trace", tracePath, "--out", outDir}, &out); err != nil {
		t.Fatalf("runLearn(regression-artifacts) error = %v", err)
	}
	if !strings.Contains(out.String(), `"path"`) || !strings.Contains(out.String(), `"internet_search"`) {
		t.Fatalf("regression-artifacts output = %q, want artifact path and kind", out.String())
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("artifact count = %d, want 1", len(entries))
	}
	content, err := os.ReadFile(filepath.Join(outDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if strings.Contains(string(content), "secret-value") || !strings.Contains(string(content), "[REDACTED]") {
		t.Fatalf("artifact redaction content = %s", string(content))
	}
}

func TestRunLearnRegressionArtifactsFromSavedReplayTraceStableLocalAndRedacted(t *testing.T) {
	app := newExtensionTestApp(t)
	store, err := replayStore(app)
	if err != nil {
		t.Fatalf("replayStore() error = %v", err)
	}
	trace := replay.NewTrace("Search docs with token=saved-secret-value")
	trace.ID = "saved regression trace"
	trace.AppendRouteSnapshot(replay.RouteSnapshot{
		Category:          "internet_search",
		RiskLevel:         "medium",
		ShouldUseTool:     true,
		ShouldUseInternet: true,
		ShouldAskApproval: true,
	})
	trace.AppendToolCall(replay.ToolCall{
		Name:   "internet_search",
		Status: "failed",
		Error:  "provider rejected token=saved-secret-value",
	})
	saved, err := store.Save(trace)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var firstName, firstText string
	for i := 0; i < 2; i++ {
		outDir := filepath.Join(t.TempDir(), "regressions")
		var out bytes.Buffer
		if err := runLearn(context.Background(), app, []string{"regression-artifacts", "--trace", saved.Path, "--out", outDir}, &out); err != nil {
			t.Fatalf("runLearn(regression-artifacts) error = %v", err)
		}
		if strings.Contains(out.String(), "saved-secret-value") {
			t.Fatalf("regression-artifacts output leaked secret: %s", out.String())
		}

		var artifacts []learning.RegressionArtifact
		if err := json.Unmarshal(out.Bytes(), &artifacts); err != nil {
			t.Fatalf("decode regression-artifacts output: %v\n%s", err, out.String())
		}
		if len(artifacts) != 1 {
			t.Fatalf("artifact count = %d, want 1: %+v", len(artifacts), artifacts)
		}
		artifact := artifacts[0]
		if artifact.Case.Kind != learning.FailureKindInternetSearch {
			t.Fatalf("artifact kind = %q, want %q", artifact.Case.Kind, learning.FailureKindInternetSearch)
		}
		if !strings.HasPrefix(artifact.Path, filepath.Clean(outDir)+string(os.PathSeparator)) {
			t.Fatalf("artifact path %q is outside output dir %q", artifact.Path, outDir)
		}

		textBytes, err := os.ReadFile(artifact.Path)
		if err != nil {
			t.Fatalf("read artifact: %v", err)
		}
		text := string(textBytes)
		if strings.Contains(text, "saved-secret-value") || !strings.Contains(text, "token=[REDACTED]") {
			t.Fatalf("artifact redaction content = %s", text)
		}
		if !strings.HasSuffix(text, "\n") {
			t.Fatalf("artifact should end with newline: %q", text)
		}

		name := filepath.Base(artifact.Path)
		if i == 0 {
			firstName = name
			firstText = text
			continue
		}
		if name != firstName {
			t.Fatalf("artifact filename changed: %q vs %q", name, firstName)
		}
		if text != firstText {
			t.Fatalf("artifact content changed between writes:\nfirst:\n%s\nsecond:\n%s", firstText, text)
		}
	}
}

func seedCLILearnConversation(t *testing.T, ctx context.Context, store *memory.Store) memory.Conversation {
	t.Helper()
	conversation, err := store.CreateConversation(ctx, "learn cli")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{ConversationID: conversation.ID, Role: "user", Content: "Run tests", Model: "test"}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{ConversationID: conversation.ID, Role: "assistant", Content: "Tests passed. token=secret-value", Model: "test"}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}
	if _, err := store.SaveToolRun(ctx, memory.ToolRun{
		ConversationID: conversation.ID,
		ToolName:       "run_tests",
		Input:          map[string]any{"command": "go test ./..."},
		Output:         map[string]any{"ok": true},
		Status:         "completed",
		RiskLevel:      "low",
	}); err != nil {
		t.Fatalf("SaveToolRun() error = %v", err)
	}
	return conversation
}
