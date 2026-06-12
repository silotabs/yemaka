package learning

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateQARegressionSourceWriteRejectsScaffold(t *testing.T) {
	plan := qaRegressionSourceWriteTestApplyPlan(t)
	scaffold := "package routing\n\nfunc TestQAReviewSourcePatchWrongSource(t *testing.T) {\n\tt.Fatalf(\"source patch draft only: replace with deterministic regression assertions before committing\")\n}\n"
	if _, err := ValidateQARegressionSourceWrite([]QARegressionSourcePatchApplyPlanRecord{plan}, QARegressionSourceWriteRequest{
		TraceID: plan.TraceID,
		Content: scaffold,
	}); err == nil || !strings.Contains(err.Error(), "scaffold marker") {
		t.Fatalf("ValidateQARegressionSourceWrite() error = %v, want scaffold rejection", err)
	}
}

func TestValidateQARegressionSourceWriteRejectsTemplateMarkers(t *testing.T) {
	plan := qaRegressionSourceWriteTestApplyPlan(t)
	scaffold := "package routing\n\nimport \"testing\"\n\nfunc " + plan.SuggestedTestName + "(t *testing.T) {\n\t// TODO: minimal public fixture prompt\n\t// TODO_EXPECTED: assert the reviewed route\n}\n"
	if _, err := ValidateQARegressionSourceWrite([]QARegressionSourcePatchApplyPlanRecord{plan}, QARegressionSourceWriteRequest{
		TraceID: plan.TraceID,
		Content: scaffold,
	}); err == nil || !strings.Contains(err.Error(), "scaffold marker") {
		t.Fatalf("ValidateQARegressionSourceWrite() error = %v, want template marker rejection", err)
	}
}

func TestValidateQARegressionSourceWriteRejectsNonGoSourceContent(t *testing.T) {
	plan := qaRegressionSourceWriteTestApplyPlan(t)
	content := "package routing\n\nimport \"testing\"\n\nfunc " + plan.SuggestedTestName + "(t *testing.T) {\n\tif {\n\t\tt.Fatal(\"broken\")\n\t}\n}\n"
	if _, err := ValidateQARegressionSourceWrite([]QARegressionSourcePatchApplyPlanRecord{plan}, QARegressionSourceWriteRequest{
		TraceID: plan.TraceID,
		Content: content,
	}); err == nil || !strings.Contains(err.Error(), "parseable Go") {
		t.Fatalf("ValidateQARegressionSourceWrite() error = %v, want parseable Go rejection", err)
	}
}

func TestRecordQARegressionSourceWriteRequiresApprovalAndRecordsArtifact(t *testing.T) {
	plan := qaRegressionSourceWriteTestApplyPlan(t)
	content := qaRegressionSourceWriteValidContent(plan)
	if _, err := RecordQARegressionSourceWrite(t.TempDir(), plan, QARegressionSourceWriteRequest{
		TraceID: plan.TraceID,
		Content: content,
	}, QARegressionSourceWriteOutcome{}); err == nil {
		t.Fatal("RecordQARegressionSourceWrite() error = nil, want explicit approval requirement")
	}
	record, err := RecordQARegressionSourceWrite(t.TempDir(), plan, QARegressionSourceWriteRequest{
		TraceID:    plan.TraceID,
		Content:    content,
		Approved:   true,
		ReviewedBy: "qa",
		ReviewNote: "Write source after token=super-secret-value is removed.",
	}, QARegressionSourceWriteOutcome{
		SnapshotID:         "snap-123",
		VerificationStatus: "pass",
		Changed:            true,
		TargetBytes:        len(content),
		Diff:               "diff --git a/internal/routing/routing_test.go b/internal/routing/routing_test.go\n",
	})
	if err != nil {
		t.Fatalf("RecordQARegressionSourceWrite() error = %v", err)
	}
	if record.Schema != QARegressionSourceWriteSchema || record.Status != "source_test_written" || !record.SourceTestWritten || record.AutomaticTestWritten {
		t.Fatalf("record = %+v, want source_test_written without automatic write marker", record)
	}
	if record.SnapshotID != "snap-123" || record.VerificationStatus != "pass" {
		t.Fatalf("record snapshot/verification = %q/%q, want snap-123/pass", record.SnapshotID, record.VerificationStatus)
	}
	data, err := os.ReadFile(record.Path)
	if err != nil {
		t.Fatalf("ReadFile(source write artifact) error = %v", err)
	}
	text := string(data)
	if strings.Contains(text, "super-secret-value") || strings.Contains(record.Preview, "super-secret-value") {
		t.Fatalf("source write artifact leaked secret:\n%s\npreview=%s", text, record.Preview)
	}
}

func TestListAndApplyQARegressionSourceWrites(t *testing.T) {
	plan := qaRegressionSourceWriteTestApplyPlan(t)
	content := qaRegressionSourceWriteValidContent(plan)
	record, err := RecordQARegressionSourceWrite(t.TempDir(), plan, QARegressionSourceWriteRequest{
		TraceID:  plan.TraceID,
		Content:  content,
		Approved: true,
	}, QARegressionSourceWriteOutcome{SnapshotID: "snap-123", VerificationStatus: "pass", Changed: true, TargetBytes: len(content)})
	if err != nil {
		t.Fatalf("RecordQARegressionSourceWrite() error = %v", err)
	}
	records, err := ListQARegressionSourceWrites(filepath.Dir(record.Path))
	if err != nil {
		t.Fatalf("ListQARegressionSourceWrites() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records len = %d, want 1", len(records))
	}
	review := QAReview{
		ReplayFailures: []QAReplayFailure{{
			TraceID:              plan.TraceID,
			PromotionFingerprint: plan.Fingerprint,
			RegressionSuggestions: []QARegressionSuggestion{{
				Name:                 plan.Name,
				PromotionFingerprint: plan.Fingerprint,
			}},
		}},
	}
	ApplyQARegressionSourceWrites(&review, records)
	if review.Summary.SourceTestWrites != 1 {
		t.Fatalf("source test write count = %d, want 1", review.Summary.SourceTestWrites)
	}
	if review.ReplayFailures[0].SourceTestWriteStatus != "source_test_written" || review.ReplayFailures[0].SourceTestWriteSnapshotID != "snap-123" {
		t.Fatalf("failure source write status/snapshot = %q/%q", review.ReplayFailures[0].SourceTestWriteStatus, review.ReplayFailures[0].SourceTestWriteSnapshotID)
	}
	if review.ReplayFailures[0].RegressionSuggestions[0].SourceTestWritePath == "" {
		t.Fatalf("suggestion source write path missing: %+v", review.ReplayFailures[0].RegressionSuggestions[0])
	}
}

func qaRegressionSourceWriteTestApplyPlan(t *testing.T) QARegressionSourcePatchApplyPlanRecord {
	t.Helper()
	patch := qaRegressionSourcePatchTestRecord(t)
	plan, err := GenerateQARegressionSourcePatchApplyPlan(t.TempDir(), []QARegressionSourcePatchRecord{patch}, QARegressionSourcePatchApplyPlanRequest{
		TraceID:  patch.TraceID,
		Approved: true,
	})
	if err != nil {
		t.Fatalf("GenerateQARegressionSourcePatchApplyPlan() error = %v", err)
	}
	return plan
}

func qaRegressionSourceWriteValidContent(plan QARegressionSourcePatchApplyPlanRecord) string {
	return "package routing\n\nimport \"testing\"\n\nfunc " + plan.SuggestedTestName + "(t *testing.T) {\n\tif false {\n\t\tt.Fatal(\"expected deterministic route assertion\")\n\t}\n}\n"
}
