package learning

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/replay"
)

func TestPromoteQARegressionSuggestionWritesReviewedRouteGovernanceArtifact(t *testing.T) {
	replayStore := replay.NewStore(filepath.Join(t.TempDir(), "replay"))
	trace := replay.NewTrace("Search ingested documents with token=super-secret-value")
	trace.ID = "trace-qa-promotion"
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
	artifact, err := PromoteQARegressionSuggestion(dir, replayStore, nil, QARegressionPromotionRequest{
		TraceID:    trace.ID,
		ReviewedBy: "qa",
		ReviewNote: "Reviewed from QA Review before landing a real regression test.",
	})
	if err != nil {
		t.Fatalf("PromoteQARegressionSuggestion() error = %v", err)
	}
	if artifact.Schema != QARegressionPromotionSchema {
		t.Fatalf("artifact schema = %q, want %q", artifact.Schema, QARegressionPromotionSchema)
	}
	if artifact.Category != RouteFailureWrongSource {
		t.Fatalf("artifact category = %q, want %q", artifact.Category, RouteFailureWrongSource)
	}
	if artifact.PromotionMode != "suggestion_only" || artifact.AutomaticTestWritten {
		t.Fatalf("artifact promotion mode = %q auto = %t, want suggestion-only/no auto test", artifact.PromotionMode, artifact.AutomaticTestWritten)
	}
	if artifact.Path == "" || !strings.HasPrefix(artifact.Path, filepath.Clean(dir)+string(os.PathSeparator)) {
		t.Fatalf("artifact path = %q, want inside %q", artifact.Path, dir)
	}
	if strings.Contains(filepath.Base(artifact.Path), "..") {
		t.Fatalf("artifact filename should be sanitized: %q", artifact.Path)
	}

	data, err := os.ReadFile(artifact.Path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	for _, want := range []string{
		`schema: "yemaka.qa_review_regression_promotion.v1"`,
		`review_status: "reviewed_suggestion"`,
		`promotion_mode: "suggestion_only"`,
		`automatic_test_written: false`,
		`source_trace_id: "trace-qa-promotion"`,
		`category: "wrong_source"`,
		`route_under_test: "internet_search"`,
		`tool_lane: "web_search_lane"`,
		`reviewed_by: "qa"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("artifact missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "super-secret-value") {
		t.Fatalf("artifact leaked secret:\n%s", text)
	}
	if !strings.Contains(text, "token=[REDACTED]") {
		t.Fatalf("artifact missing redaction marker:\n%s", text)
	}
	if artifact.Preview == "" || !strings.Contains(artifact.Preview, "suggestion_only") {
		t.Fatalf("artifact preview missing promotion summary: %+v", artifact)
	}
}

func TestPromoteQARegressionSuggestionRejectsRemoteOutputDirectory(t *testing.T) {
	if _, err := PromoteQARegressionSuggestion("https://example.com/promotions", replay.Store{}, nil, QARegressionPromotionRequest{TraceID: "trace"}); err == nil {
		t.Fatal("PromoteQARegressionSuggestion() error = nil, want remote output directory rejection")
	}
}

func TestPromoteQARegressionSuggestionRequiresTraceID(t *testing.T) {
	if _, err := PromoteQARegressionSuggestion(t.TempDir(), replay.Store{}, nil, QARegressionPromotionRequest{}); err == nil {
		t.Fatal("PromoteQARegressionSuggestion() error = nil, want trace id requirement")
	}
}
