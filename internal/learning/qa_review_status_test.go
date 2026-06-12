package learning

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/replay"
)

func TestReviewQARegressionSuggestionPersistsStatusAndAppliesToQAReview(t *testing.T) {
	replayStore := replay.NewStore(filepath.Join(t.TempDir(), "replay"))
	trace := replay.NewTrace("Search local documents with token=super-secret-value")
	trace.ID = "trace-qa-review-status"
	trace.Route = replay.RouteSnapshot{
		Category:          "internet_search",
		RiskLevel:         "medium",
		ShouldUseInternet: true,
	}
	trace.Errors = []replay.TraceError{{
		Stage:   "routing",
		Code:    "wrong_source",
		Message: "wrong source: local documents should have been used, not internet",
	}}
	if _, err := replayStore.Save(trace); err != nil {
		t.Fatalf("Save() trace error = %v", err)
	}

	dir := t.TempDir()
	record, err := ReviewQARegressionSuggestion(dir, replayStore, nil, QARegressionReviewRequest{
		TraceID:    trace.ID,
		Status:     QARegressionReviewApproved,
		ReviewedBy: "qa",
		ReviewNote: "Approved after checking token=super-secret-value",
	})
	if err != nil {
		t.Fatalf("ReviewQARegressionSuggestion() error = %v", err)
	}
	if record.Schema != QARegressionReviewSchema || record.Status != QARegressionReviewApproved {
		t.Fatalf("record = %+v, want schema/status", record)
	}
	if record.Path == "" || !strings.HasPrefix(record.Path, filepath.Clean(dir)+string(os.PathSeparator)) {
		t.Fatalf("record path = %q, want inside %q", record.Path, dir)
	}
	data, err := os.ReadFile(record.Path)
	if err != nil {
		t.Fatalf("ReadFile() record error = %v", err)
	}
	if strings.Contains(string(data), "super-secret-value") || !strings.Contains(string(data), "token=[REDACTED]") {
		t.Fatalf("review record did not redact review note secret:\n%s", string(data))
	}

	records, err := ListQARegressionReviews(dir)
	if err != nil {
		t.Fatalf("ListQARegressionReviews() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	review, err := BuildQAReview(context.Background(), nil, replayStore, nil, 10)
	if err != nil {
		t.Fatalf("BuildQAReview() error = %v", err)
	}
	ApplyQARegressionReviews(&review, records)
	if review.Summary.ApprovedSuggestions != 1 {
		t.Fatalf("approved suggestions = %d, want 1", review.Summary.ApprovedSuggestions)
	}
	if len(review.ReplayFailures) != 1 {
		t.Fatalf("replay failures = %d, want 1", len(review.ReplayFailures))
	}
	failure := review.ReplayFailures[0]
	if failure.ReviewStatus != QARegressionReviewApproved || failure.CoverageStatus != "approved_for_test" {
		t.Fatalf("failure review/coverage = %q/%q, want approved/approved_for_test", failure.ReviewStatus, failure.CoverageStatus)
	}
	if len(failure.RegressionSuggestions) == 0 || failure.RegressionSuggestions[0].ReviewStatus != QARegressionReviewApproved {
		t.Fatalf("suggestion review status not applied: %+v", failure.RegressionSuggestions)
	}
}

func TestReviewQARegressionSuggestionRejectsInvalidInput(t *testing.T) {
	if _, err := ReviewQARegressionSuggestion("https://example.com/reviews", replay.Store{}, nil, QARegressionReviewRequest{TraceID: "trace", Status: QARegressionReviewApproved}); err == nil {
		t.Fatal("ReviewQARegressionSuggestion() error = nil, want remote output directory rejection")
	}
	if _, err := ReviewQARegressionSuggestion(t.TempDir(), replay.Store{}, nil, QARegressionReviewRequest{Status: QARegressionReviewApproved}); err == nil {
		t.Fatal("ReviewQARegressionSuggestion() error = nil, want trace id requirement")
	}
	if _, err := ReviewQARegressionSuggestion(t.TempDir(), replay.Store{}, nil, QARegressionReviewRequest{TraceID: "trace", Status: "maybe"}); err == nil {
		t.Fatal("ReviewQARegressionSuggestion() error = nil, want invalid status rejection")
	}
}

func TestListQARegressionReviewsValidatesStoredRecords(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte(`{"schema":"wrong","traceId":"trace","status":"approved","name":"x"}`), 0o644); err != nil {
		t.Fatalf("WriteFile() bad record error = %v", err)
	}
	if _, err := ListQARegressionReviews(dir); err == nil {
		t.Fatal("ListQARegressionReviews() error = nil, want invalid schema rejection")
	}
}
