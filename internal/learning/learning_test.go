package learning

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/memory"
	"yemaka/internal/replay"
	"yemaka/internal/routing"
)

func TestRecordWorkflowAndReport(t *testing.T) {
	ctx := context.Background()
	store := testMemoryStore(t)
	conversation := seedConversation(t, ctx, store)
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

	record, err := RecordWorkflow(ctx, store, conversation.ID)
	if err != nil {
		t.Fatalf("RecordWorkflow() error = %v", err)
	}
	if record.Kind != KindWorkflowSuccess {
		t.Fatalf("record kind = %q, want success", record.Kind)
	}
	if !strings.Contains(record.Lesson, "Create skill") && !strings.Contains(record.Lesson, "reusable skill") {
		t.Fatalf("lesson = %q, want reusable skill hint", record.Lesson)
	}
	if strings.Contains(record.Lesson, "/Users/example") {
		t.Fatalf("workflow lesson leaked user path: %q", record.Lesson)
	}

	correction, err := SaveCorrection(ctx, store, conversation.ID, "token=secret-value should be hidden")
	if err != nil {
		t.Fatalf("SaveCorrection() error = %v", err)
	}
	if strings.Contains(correction.Content, "secret-value") {
		t.Fatalf("correction leaked secret: %q", correction.Content)
	}
	report, err := BuildReport(ctx, store, nil)
	if err != nil {
		t.Fatalf("BuildReport() error = %v", err)
	}
	if report.WorkflowSuccesses != 1 || report.Corrections != 1 || report.AutomaticTraining {
		t.Fatalf("report = %+v, want one success, one correction, no training", report)
	}
}

func TestRouteCorrectionApprovalControlsActivation(t *testing.T) {
	ctx := context.Background()
	store := testMemoryStore(t)
	conversation := seedConversation(t, ctx, store)

	pending, err := SaveRouteCorrection(ctx, store, routing.RouteCorrection{
		SourceConversationID:  conversation.ID,
		Pattern:               "email address in notebook",
		IntendedRouteCategory: routing.RouteRAGSearch,
		IntendedTaskType:      routing.TaskRAG,
		RequiredTools:         []string{"rag_search"},
		ForbiddenTools:        []string{"internet_search"},
	})
	if err != nil {
		t.Fatalf("SaveRouteCorrection() error = %v", err)
	}
	if pending.ApprovalStatus != routing.RouteCorrectionStatusPending {
		t.Fatalf("pending status = %q", pending.ApprovalStatus)
	}
	active, err := ApprovedRouteCorrections(ctx, store, "email address in notebook", 10)
	if err != nil {
		t.Fatalf("ApprovedRouteCorrections() error = %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("pending correction was active: %+v", active)
	}

	approved, err := ApproveRouteCorrection(ctx, store, pending.ID)
	if err != nil {
		t.Fatalf("ApproveRouteCorrection() error = %v", err)
	}
	if approved.ApprovalStatus != routing.RouteCorrectionStatusApproved {
		t.Fatalf("approved status = %q", approved.ApprovalStatus)
	}
	active, err = ApprovedRouteCorrections(ctx, store, "please find the email address in notebook", 10)
	if err != nil {
		t.Fatalf("ApprovedRouteCorrections() after approval error = %v", err)
	}
	if len(active) != 1 || active[0].ID != pending.ID {
		t.Fatalf("active corrections = %+v, want approved %s", active, pending.ID)
	}

	disabled, err := DisableRouteCorrection(ctx, store, pending.ID)
	if err != nil {
		t.Fatalf("DisableRouteCorrection() error = %v", err)
	}
	if !disabled.Disabled || disabled.ApprovalStatus != routing.RouteCorrectionStatusRejected {
		t.Fatalf("disabled correction = %+v", disabled)
	}
	active, err = ApprovedRouteCorrections(ctx, store, "email address in notebook", 10)
	if err != nil {
		t.Fatalf("ApprovedRouteCorrections() after disable error = %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("disabled correction was active: %+v", active)
	}
}

func TestRouteCorrectionRejectsUnsafeTools(t *testing.T) {
	ctx := context.Background()
	store := testMemoryStore(t)
	_, err := SaveRouteCorrection(ctx, store, routing.RouteCorrection{
		Pattern:               "edit config next time",
		IntendedRouteCategory: routing.RouteRAGSearch,
		RequiredTools:         []string{"edit_file"},
		ApprovalStatus:        routing.RouteCorrectionStatusApproved,
	})
	if err == nil {
		t.Fatal("SaveRouteCorrection() error = nil, want unsafe tool rejection")
	}
}

func TestRecordWorkflowFailure(t *testing.T) {
	ctx := context.Background()
	store := testMemoryStore(t)
	conversation := seedConversation(t, ctx, store)
	if _, err := store.SaveToolRun(ctx, memory.ToolRun{
		ConversationID: conversation.ID,
		ToolName:       "agent_verifier",
		Input:          map[string]any{},
		Output:         map[string]any{"status": "needs_follow_up"},
		Status:         "needs_follow_up",
		RiskLevel:      "medium",
	}); err != nil {
		t.Fatalf("SaveToolRun() error = %v", err)
	}
	record, err := RecordWorkflow(ctx, store, conversation.ID)
	if err != nil {
		t.Fatalf("RecordWorkflow() error = %v", err)
	}
	if record.Kind != KindWorkflowFailure {
		t.Fatalf("record kind = %q, want failure", record.Kind)
	}
}

func TestRecordWorkflowSkipsApprovalPendingToolRun(t *testing.T) {
	ctx := context.Background()
	store := testMemoryStore(t)
	conversation := seedConversation(t, ctx, store)
	if _, err := store.SaveToolRun(ctx, memory.ToolRun{
		ConversationID: conversation.ID,
		ToolName:       "write_file",
		Input:          map[string]any{"path": "/Users/example/project/main.go"},
		Output:         map[string]any{"status": "needs_confirmation"},
		Status:         "needs_confirmation",
		RiskLevel:      "medium",
	}); err != nil {
		t.Fatalf("SaveToolRun() error = %v", err)
	}

	record, err := RecordWorkflow(ctx, store, conversation.ID)
	if err != nil {
		t.Fatalf("RecordWorkflow() error = %v", err)
	}
	if record.Kind != KindWorkflowSkipped {
		t.Fatalf("record kind = %q, want skipped", record.Kind)
	}
	if record.Status != "skipped" {
		t.Fatalf("record status = %q, want skipped", record.Status)
	}
	report, err := BuildReport(ctx, store, nil)
	if err != nil {
		t.Fatalf("BuildReport() error = %v", err)
	}
	if report.WorkflowSuccesses != 0 || report.WorkflowFailures != 0 {
		t.Fatalf("report = %+v, want no workflow success or failure for pending approval", report)
	}
}

func TestRecordWorkflowSkipsReadyAgentExecutorWithoutToolResult(t *testing.T) {
	ctx := context.Background()
	store := testMemoryStore(t)
	conversation := seedConversation(t, ctx, store)
	if _, err := store.SaveToolRun(ctx, memory.ToolRun{
		ConversationID: conversation.ID,
		ToolName:       "agent_executor",
		Input:          map[string]any{"tool_name": "run_tests", "command": []any{"go", "test", "./internal/learning"}},
		Output:         map[string]any{"status": "ready"},
		Status:         "ready",
		RiskLevel:      "low",
	}); err != nil {
		t.Fatalf("SaveToolRun() error = %v", err)
	}

	record, err := RecordWorkflow(ctx, store, conversation.ID)
	if err != nil {
		t.Fatalf("RecordWorkflow() error = %v", err)
	}
	if record.Kind != KindWorkflowSkipped {
		t.Fatalf("record kind = %q, want skipped", record.Kind)
	}
	report, err := BuildReport(ctx, store, nil)
	if err != nil {
		t.Fatalf("BuildReport() error = %v", err)
	}
	if report.WorkflowSuccesses != 0 || report.WorkflowFailures != 0 {
		t.Fatalf("report = %+v, want no workflow success or failure for incomplete executor", report)
	}
}

func TestRecordWorkflowRecordsReadyAgentExecutorWithCompletedToolResult(t *testing.T) {
	ctx := context.Background()
	store := testMemoryStore(t)
	conversation := seedConversation(t, ctx, store)
	if _, err := store.SaveToolRun(ctx, memory.ToolRun{
		ConversationID: conversation.ID,
		ToolName:       "agent_executor",
		Input:          map[string]any{"tool_name": "run_tests", "command": []any{"go", "test", "./internal/learning"}},
		Output:         map[string]any{"status": "ready"},
		Status:         "ready",
		RiskLevel:      "low",
	}); err != nil {
		t.Fatalf("SaveToolRun(agent_executor) error = %v", err)
	}
	if _, err := store.SaveToolRun(ctx, memory.ToolRun{
		ConversationID: conversation.ID,
		ToolName:       "run_tests",
		Input:          map[string]any{"command": "go test ./internal/learning"},
		Output:         map[string]any{"ok": true},
		Status:         "completed",
		RiskLevel:      "low",
	}); err != nil {
		t.Fatalf("SaveToolRun(run_tests) error = %v", err)
	}

	record, err := RecordWorkflow(ctx, store, conversation.ID)
	if err != nil {
		t.Fatalf("RecordWorkflow() error = %v", err)
	}
	if record.Kind != KindWorkflowSuccess {
		t.Fatalf("record kind = %q, want success", record.Kind)
	}
	if strings.Contains(record.Lesson, "agent_executor") {
		t.Fatalf("lesson = %q, should not use executor bookkeeping as the learned tool", record.Lesson)
	}
}

func TestRecordWorkflowRecordsMeaningfulSuccessfulToolRun(t *testing.T) {
	ctx := context.Background()
	store := testMemoryStore(t)
	conversation := seedConversation(t, ctx, store)
	if _, err := store.SaveToolRun(ctx, memory.ToolRun{
		ConversationID: conversation.ID,
		ToolName:       "run_tests",
		Input:          map[string]any{"command": "go test ./internal/learning"},
		Output:         map[string]any{"ok": true},
		Status:         "completed",
		RiskLevel:      "low",
	}); err != nil {
		t.Fatalf("SaveToolRun() error = %v", err)
	}

	record, err := RecordWorkflow(ctx, store, conversation.ID)
	if err != nil {
		t.Fatalf("RecordWorkflow() error = %v", err)
	}
	if record.Kind != KindWorkflowSuccess {
		t.Fatalf("record kind = %q, want success", record.Kind)
	}
	report, err := BuildReport(ctx, store, nil)
	if err != nil {
		t.Fatalf("BuildReport() error = %v", err)
	}
	if report.WorkflowSuccesses != 1 || report.WorkflowFailures != 0 {
		t.Fatalf("report = %+v, want one workflow success and no failure", report)
	}
}

func TestRecordWorkflowSkipsVerifierOnlyChat(t *testing.T) {
	ctx := context.Background()
	store := testMemoryStore(t)
	conversation, err := store.CreateConversation(ctx, "hello")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "hello",
		Model:          "test-model",
	}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "Hello.",
		Model:          "test-model",
	}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}
	if _, err := store.SaveToolRun(ctx, memory.ToolRun{
		ConversationID: conversation.ID,
		ToolName:       "agent_verifier",
		Input:          map[string]any{},
		Output:         map[string]any{"status": "pass"},
		Status:         "pass",
		RiskLevel:      "low",
	}); err != nil {
		t.Fatalf("SaveToolRun() error = %v", err)
	}

	record, err := RecordWorkflow(ctx, store, conversation.ID)
	if err != nil {
		t.Fatalf("RecordWorkflow() error = %v", err)
	}
	if record.Kind != KindWorkflowSkipped {
		t.Fatalf("record kind = %q, want skipped", record.Kind)
	}
	report, err := BuildReport(ctx, store, nil)
	if err != nil {
		t.Fatalf("BuildReport() error = %v", err)
	}
	if report.WorkflowSuccesses != 0 || report.WorkflowFailures != 0 {
		t.Fatalf("report = %+v, want no workflow memory for verifier-only chat", report)
	}
}

func TestExportTrajectorySanitizesSecretsAndPaths(t *testing.T) {
	ctx := context.Background()
	store := testMemoryStore(t)
	conversation := seedConversation(t, ctx, store)
	if _, err := store.SaveToolRun(ctx, memory.ToolRun{
		ConversationID: conversation.ID,
		ToolName:       "read_file",
		Input:          map[string]any{"path": "/Users/example/project/.env"},
		Output:         map[string]any{"content": "api_key=sk-thisshouldberemoved"},
		Status:         "completed",
		RiskLevel:      "low",
	}); err != nil {
		t.Fatalf("SaveToolRun() error = %v", err)
	}
	trajectory, err := ExportTrajectory(ctx, store, conversation.ID)
	if err != nil {
		t.Fatalf("ExportTrajectory() error = %v", err)
	}
	if trajectory.Schema != TrajectorySchema || trajectory.Training.AutomaticTraining {
		t.Fatalf("trajectory metadata = %+v", trajectory)
	}
	if !trajectory.PrivacyFilter.SecretsRedacted || !trajectory.PrivacyFilter.PathsRedacted {
		t.Fatalf("privacy filter = %+v, want secrets and paths redacted", trajectory.PrivacyFilter)
	}
	output := trajectory.ToolRuns[0].Output.(map[string]any)["content"].(string)
	if strings.Contains(output, "sk-this") {
		t.Fatalf("output was not redacted: %q", output)
	}
}

func TestSanitizeValueKeepsScalarValues(t *testing.T) {
	clean, report := SanitizeValue(map[string]any{"ok": true, "count": 3, "path": "/Users/example/project"})
	values, ok := clean.(map[string]any)
	if !ok {
		t.Fatalf("clean = %T, want map", clean)
	}
	if values["ok"] != true || values["count"] != 3 {
		t.Fatalf("clean scalars = %#v, want preserved bool and number", values)
	}
	if !report.PathsRedacted {
		t.Fatalf("report = %+v, want path redaction", report)
	}
}

func TestBuildQAReviewLinksReplayFailuresCorrectionsAndRegressionSuggestions(t *testing.T) {
	ctx := context.Background()
	store := testMemoryStore(t)
	traces := replay.NewStore(t.TempDir())

	correction, err := SaveRouteCorrection(ctx, store, routing.RouteCorrection{
		Pattern:               "indexed notes",
		OriginalPrompt:        "search indexed notes for the answer",
		CorrectionText:        "Use local documents for similar prompts.",
		IntendedRouteCategory: routing.RouteRAGSearch,
		IntendedTaskType:      routing.TaskRAG,
		RequiredTools:         []string{"rag_search"},
		ForbiddenTools:        []string{"internet_search"},
	})
	if err != nil {
		t.Fatalf("SaveRouteCorrection() error = %v", err)
	}

	if _, err := traces.Save(replay.Trace{
		ID:          "trace_routing_failure",
		UserRequest: "search indexed notes for the answer",
		Route: replay.RouteSnapshot{
			Category:          routing.RouteInternetSearch,
			RiskLevel:         routing.RiskMedium,
			ShouldUseInternet: true,
		},
		Errors: []replay.TraceError{
			{Stage: "routing", Code: "wrong_source", Subject: "internet_search", Message: "local documents should have been used", ExpectedRoute: routing.RouteRAGSearch},
		},
		FinalResult:  replay.ResultSnapshot{Status: "failed"},
		Verification: replay.VerificationResult{Status: "failed"},
	}); err != nil {
		t.Fatalf("Save(trace) error = %v", err)
	}

	review, err := BuildQAReview(ctx, store, traces, &QAEvalSummary{
		ID:     "eval_1",
		Status: "pass",
		Passed: 1,
		Tasks:  []QAEvalTaskSummary{{Name: "route_governance_classifier", Status: "pass"}},
	}, 10)
	if err != nil {
		t.Fatalf("BuildQAReview() error = %v", err)
	}
	if review.Summary.RouteCorrections != 1 || review.Summary.PendingCorrections != 1 {
		t.Fatalf("review summary corrections = %+v", review.Summary)
	}
	if review.LatestEval == nil || review.LatestEval.ID != "eval_1" {
		t.Fatalf("latest eval = %+v", review.LatestEval)
	}
	if len(review.ReplayFailures) != 1 {
		t.Fatalf("replay failures = %d, want 1", len(review.ReplayFailures))
	}
	failure := review.ReplayFailures[0]
	if failure.CoverageStatus != "artifact_suggested" || len(failure.RegressionSuggestions) == 0 {
		t.Fatalf("failure coverage = %+v", failure)
	}
	if failure.RouteFailureCategory != RouteFailureWrongSource {
		t.Fatalf("route failure category = %q, want %q", failure.RouteFailureCategory, RouteFailureWrongSource)
	}
	if failure.RegressionPromotion.SuggestedTestName == "" || failure.SuggestedRegression == "" {
		t.Fatalf("route governance metadata missing: %+v", failure)
	}
	for _, want := range []string{"go_test:internal/learning/TestClassifyRouteFailureCategories", "eval:route_governance_classifier"} {
		if !containsString(failure.CoverageSources, want) {
			t.Fatalf("coverage sources = %+v, missing %q", failure.CoverageSources, want)
		}
	}
	if review.Summary.RouteFailureCategories[RouteFailureWrongSource] != 1 {
		t.Fatalf("route failure categories = %+v, want wrong_source count", review.Summary.RouteFailureCategories)
	}
	if len(failure.MatchingCorrectionIDs) != 1 || failure.MatchingCorrectionIDs[0] != correction.ID {
		t.Fatalf("matching corrections = %+v, want %s", failure.MatchingCorrectionIDs, correction.ID)
	}
}

func TestBuildQAReviewMarksRepeatedRouteGovernanceSuggestions(t *testing.T) {
	ctx := context.Background()
	store := testMemoryStore(t)
	traces := replay.NewStore(t.TempDir())

	for _, item := range []struct {
		id      string
		request string
	}{
		{id: "trace_repeat_a", request: "search indexed notes for release facts"},
		{id: "trace_repeat_b", request: "find the answer in my indexed documents"},
	} {
		if _, err := traces.Save(replay.Trace{
			ID:          item.id,
			UserRequest: item.request,
			Route: replay.RouteSnapshot{
				Category:          routing.RouteInternetSearch,
				RiskLevel:         routing.RiskMedium,
				ShouldUseInternet: true,
			},
			Errors: []replay.TraceError{
				{Stage: "routing", Code: "wrong_source", Subject: "internet_search", Message: "local documents should have been used", ExpectedRoute: routing.RouteRAGSearch},
			},
			FinalResult:  replay.ResultSnapshot{Status: "failed"},
			Verification: replay.VerificationResult{Status: "failed"},
		}); err != nil {
			t.Fatalf("Save(%s) error = %v", item.id, err)
		}
	}

	review, err := BuildQAReview(ctx, store, traces, nil, 10)
	if err != nil {
		t.Fatalf("BuildQAReview() error = %v", err)
	}
	if review.Summary.RepeatedSuggestions != 1 {
		t.Fatalf("RepeatedSuggestions = %d, want 1; review=%+v", review.Summary.RepeatedSuggestions, review.Summary)
	}
	if len(review.ReplayFailures) != 2 {
		t.Fatalf("replay failures = %d, want 2", len(review.ReplayFailures))
	}
	fingerprint := review.ReplayFailures[0].PromotionFingerprint
	if fingerprint == "" {
		t.Fatalf("promotion fingerprint is empty: %+v", review.ReplayFailures[0])
	}
	for _, failure := range review.ReplayFailures {
		if failure.PromotionFingerprint != fingerprint {
			t.Fatalf("fingerprints differ: %q vs %q", failure.PromotionFingerprint, fingerprint)
		}
		if failure.RepeatCount != 2 || failure.PromotionPriority != "medium" {
			t.Fatalf("repeat metadata = count %d priority %q, want 2/medium; failure=%+v", failure.RepeatCount, failure.PromotionPriority, failure)
		}
		if len(failure.RegressionSuggestions) == 0 || failure.RegressionSuggestions[0].RepeatCount != 2 {
			t.Fatalf("suggestion repeat metadata missing: %+v", failure.RegressionSuggestions)
		}
	}
}

func testMemoryStore(t *testing.T) *memory.Store {
	t.Helper()
	store, err := memory.Open(context.Background(), filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func seedConversation(t *testing.T, ctx context.Context, store *memory.Store) memory.Conversation {
	t.Helper()
	conversation, err := store.CreateConversation(ctx, "Fix tests")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "Fix the tests in /Users/example/project",
		Model:          "test-model",
	}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "Tests passed after using local context.",
		Model:          "test-model",
	}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}
	return conversation
}
