package learning

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/replay"
)

func TestGenerateQARegressionTestDraftRequiresApprovedReview(t *testing.T) {
	replayStore, trace := qaRegressionDraftTestReplayStore(t)
	dir := t.TempDir()
	if _, err := GenerateQARegressionTestDraft(dir, nil, replayStore, nil, QARegressionTestDraftRequest{TraceID: trace.ID}); err == nil {
		t.Fatal("GenerateQARegressionTestDraft() error = nil, want approved review requirement")
	}

	reviewRecord, err := ReviewQARegressionSuggestion(t.TempDir(), replayStore, nil, QARegressionReviewRequest{
		TraceID: trace.ID,
		Status:  QARegressionReviewApproved,
	})
	if err != nil {
		t.Fatalf("ReviewQARegressionSuggestion() error = %v", err)
	}
	draft, err := GenerateQARegressionTestDraft(dir, []QARegressionReviewRecord{reviewRecord}, replayStore, nil, QARegressionTestDraftRequest{
		TraceID:    trace.ID,
		ReviewedBy: "qa",
		ReviewNote: "Draft this after token=super-secret-value is redacted.",
	})
	if err != nil {
		t.Fatalf("GenerateQARegressionTestDraft() error = %v", err)
	}
	if draft.Schema != QARegressionTestDraftSchema || draft.Status != "drafted" || draft.AutomaticTestWritten {
		t.Fatalf("draft = %+v, want drafted/no auto test", draft)
	}
	if draft.Path == "" || !strings.HasPrefix(draft.Path, filepath.Clean(dir)+string(os.PathSeparator)) {
		t.Fatalf("draft path = %q, want inside %q", draft.Path, dir)
	}
	data, err := os.ReadFile(draft.Path)
	if err != nil {
		t.Fatalf("ReadFile() draft error = %v", err)
	}
	text := string(data)
	for _, want := range []string{
		QARegressionTestDraftSchema,
		`"status": "drafted"`,
		`"automaticTestWritten": false`,
		`"suggestedTestFile": "internal/routing/routing_test.go"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("draft missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "super-secret-value") || strings.Contains(draft.Preview, "super-secret-value") {
		t.Fatalf("draft leaked secret:\n%s\npreview=%s", text, draft.Preview)
	}
}

func TestApproveQARegressionTestDraftRequiresDraftAndDoesNotWriteTest(t *testing.T) {
	replayStore, trace := qaRegressionDraftTestReplayStore(t)
	reviewRecord, err := ReviewQARegressionSuggestion(t.TempDir(), replayStore, nil, QARegressionReviewRequest{
		TraceID: trace.ID,
		Status:  QARegressionReviewApproved,
	})
	if err != nil {
		t.Fatalf("ReviewQARegressionSuggestion() error = %v", err)
	}
	if _, err := ApproveQARegressionTestDraft(t.TempDir(), nil, QARegressionApprovalRequest{TraceID: trace.ID}); err == nil {
		t.Fatal("ApproveQARegressionTestDraft() error = nil, want draft requirement")
	}
	draft, err := GenerateQARegressionTestDraft(t.TempDir(), []QARegressionReviewRecord{reviewRecord}, replayStore, nil, QARegressionTestDraftRequest{TraceID: trace.ID})
	if err != nil {
		t.Fatalf("GenerateQARegressionTestDraft() error = %v", err)
	}
	approvalDir := t.TempDir()
	approval, err := ApproveQARegressionTestDraft(approvalDir, []QARegressionTestDraftRecord{draft}, QARegressionApprovalRequest{
		TraceID:    trace.ID,
		ReviewedBy: "qa",
	})
	if err != nil {
		t.Fatalf("ApproveQARegressionTestDraft() error = %v", err)
	}
	if approval.Schema != QARegressionApprovedSchema || approval.Status != "approved_regression" || approval.AutomaticTestWritten {
		t.Fatalf("approval = %+v, want approved_regression/no auto test", approval)
	}
	if approval.TestDraftPath != draft.Path {
		t.Fatalf("approval draft path = %q, want %q", approval.TestDraftPath, draft.Path)
	}
	reviews, err := ListQARegressionReviews(filepath.Dir(reviewRecord.Path))
	if err != nil {
		t.Fatalf("ListQARegressionReviews() error = %v", err)
	}
	drafts, err := ListQARegressionTestDrafts(filepath.Dir(draft.Path))
	if err != nil {
		t.Fatalf("ListQARegressionTestDrafts() error = %v", err)
	}
	approvals, err := ListQARegressionApprovedRecords(approvalDir)
	if err != nil {
		t.Fatalf("ListQARegressionApprovedRecords() error = %v", err)
	}
	if err := ValidateQARegressionReviewFlow(reviews, drafts, approvals); err != nil {
		t.Fatalf("ValidateQARegressionReviewFlow() error = %v", err)
	}
}

func TestValidateQARegressionReviewFlowRejectsUngatedDrafts(t *testing.T) {
	draft := QARegressionTestDraftRecord{
		Schema:               QARegressionTestDraftSchema,
		Name:                 "draft_without_review",
		Kind:                 "route_governance_wrong_route",
		TraceID:              "trace",
		Status:               "drafted",
		AutomaticTestWritten: false,
	}
	if err := ValidateQARegressionReviewFlow(nil, []QARegressionTestDraftRecord{draft}, nil); err == nil {
		t.Fatal("ValidateQARegressionReviewFlow() error = nil, want missing approved review rejection")
	}
}

func qaRegressionDraftTestReplayStore(t *testing.T) (replay.Store, replay.Trace) {
	t.Helper()
	replayStore := replay.NewStore(filepath.Join(t.TempDir(), "replay"))
	trace := replay.NewTrace("Search local documents with token=super-secret-value")
	trace.ID = "trace-qa-regression-draft"
	trace.Route = replay.RouteSnapshot{
		Category:          "internet_search",
		RiskLevel:         "medium",
		ShouldUseInternet: true,
	}
	trace.Errors = []replay.TraceError{{
		Stage:         "routing",
		Code:          "wrong_source",
		Message:       "wrong source: local documents should have been used, not internet",
		ExpectedRoute: "rag_search",
	}}
	if _, err := replayStore.Save(trace); err != nil {
		t.Fatalf("Save() trace error = %v", err)
	}
	return replayStore, trace
}
