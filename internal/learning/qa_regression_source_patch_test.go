package learning

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateQARegressionSourcePatchDraftRequiresApprovedRegression(t *testing.T) {
	if _, err := GenerateQARegressionSourcePatchDraft(t.TempDir(), nil, QARegressionSourcePatchRequest{TraceID: "trace"}); err == nil {
		t.Fatal("GenerateQARegressionSourcePatchDraft() error = nil, want approved regression requirement")
	}
}

func TestGenerateQARegressionSourcePatchDraftWritesManualPatchArtifact(t *testing.T) {
	approval := qaRegressionSourcePatchTestApproval()
	dir := t.TempDir()
	record, err := GenerateQARegressionSourcePatchDraft(dir, []QARegressionApprovedRecord{approval}, QARegressionSourcePatchRequest{
		TraceID:    approval.TraceID,
		ReviewedBy: "qa",
		ReviewNote: "Create source patch after token=super-secret-value is reviewed.",
	})
	if err != nil {
		t.Fatalf("GenerateQARegressionSourcePatchDraft() error = %v", err)
	}
	if record.Schema != QARegressionSourcePatchSchema || record.Status != "requires_manual_apply" {
		t.Fatalf("record schema/status = %q/%q", record.Schema, record.Status)
	}
	if record.AutomaticTestWritten || !record.RequiresManualApply {
		t.Fatalf("record auto/manual = %t/%t, want no auto/manual required", record.AutomaticTestWritten, record.RequiresManualApply)
	}
	if record.Path == "" || !strings.HasPrefix(record.Path, filepath.Clean(dir)+string(os.PathSeparator)) {
		t.Fatalf("record path = %q, want inside %q", record.Path, dir)
	}
	if !strings.Contains(record.PatchPreview, "diff --git a/internal/routing/routing_test.go b/internal/routing/routing_test.go") ||
		!strings.Contains(record.PatchPreview, "+func TestQAReviewSourcePatchWrongSource(t *testing.T)") {
		t.Fatalf("patch preview missing diff/test snippet:\n%s", record.PatchPreview)
	}
	for _, want := range []string{
		"QAReview source patch template",
		"TODO: minimal public fixture prompt",
		"Routing harness",
		"Do not copy the observed failed route",
	} {
		if !strings.Contains(record.TestSnippet, want) {
			t.Fatalf("test snippet missing %q:\n%s", want, record.TestSnippet)
		}
	}
	data, err := os.ReadFile(record.Path)
	if err != nil {
		t.Fatalf("ReadFile() source patch error = %v", err)
	}
	text := string(data)
	for _, want := range []string{
		QARegressionSourcePatchSchema,
		`"status": "requires_manual_apply"`,
		`"automaticTestWritten": false`,
		`"requiresManualApply": true`,
		`"suggestedTestFile": "internal/routing/routing_test.go"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("source patch missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "super-secret-value") || strings.Contains(record.Preview, "super-secret-value") {
		t.Fatalf("source patch leaked secret:\n%s\npreview=%s", text, record.Preview)
	}
}

func TestQARegressionSourcePatchSnippetForAgentTargetIncludesPlanDecisionGuidance(t *testing.T) {
	approval := qaRegressionSourcePatchTestApproval()
	approval.SuggestedTestFile = "internal/agent/routing_test.go"
	approval.SuggestedTestName = "TestQAReviewSourcePatchAgentGate"
	record, err := GenerateQARegressionSourcePatchDraft(t.TempDir(), []QARegressionApprovedRecord{approval}, QARegressionSourcePatchRequest{TraceID: approval.TraceID})
	if err != nil {
		t.Fatalf("GenerateQARegressionSourcePatchDraft() error = %v", err)
	}
	for _, want := range []string{"Agent harness", "BuildPlan", "DecideExecution", "approval"} {
		if !strings.Contains(record.TestSnippet, want) {
			t.Fatalf("agent snippet missing %q:\n%s", want, record.TestSnippet)
		}
	}
}

func TestQARegressionSourcePatchSnippetForClassifierTargetIncludesTraceGuidance(t *testing.T) {
	approval := qaRegressionSourcePatchTestApproval()
	approval.SuggestedTestFile = "internal/learning/route_failure_classifier_test.go"
	approval.SuggestedTestName = "TestQAReviewSourcePatchClassifier"
	record, err := GenerateQARegressionSourcePatchDraft(t.TempDir(), []QARegressionApprovedRecord{approval}, QARegressionSourcePatchRequest{TraceID: approval.TraceID})
	if err != nil {
		t.Fatalf("GenerateQARegressionSourcePatchDraft() error = %v", err)
	}
	for _, want := range []string{"Classifier harness", "ClassifyRouteFailure", "replay.Trace", "promotion metadata"} {
		if !strings.Contains(record.TestSnippet, want) {
			t.Fatalf("classifier snippet missing %q:\n%s", want, record.TestSnippet)
		}
	}
}

func TestListAndApplyQARegressionSourcePatches(t *testing.T) {
	approval := qaRegressionSourcePatchTestApproval()
	record, err := GenerateQARegressionSourcePatchDraft(t.TempDir(), []QARegressionApprovedRecord{approval}, QARegressionSourcePatchRequest{
		TraceID: approval.TraceID,
	})
	if err != nil {
		t.Fatalf("GenerateQARegressionSourcePatchDraft() error = %v", err)
	}
	records, err := ListQARegressionSourcePatchDrafts(filepath.Dir(record.Path))
	if err != nil {
		t.Fatalf("ListQARegressionSourcePatchDrafts() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records len = %d, want 1", len(records))
	}
	review := QAReview{
		ReplayFailures: []QAReplayFailure{{
			TraceID:              approval.TraceID,
			PromotionFingerprint: approval.Fingerprint,
			RegressionSuggestions: []QARegressionSuggestion{{
				Name:                 approval.Name,
				PromotionFingerprint: approval.Fingerprint,
			}},
		}},
	}
	ApplyQARegressionSourcePatches(&review, records)
	if review.Summary.SourcePatches != 1 {
		t.Fatalf("source patch count = %d, want 1", review.Summary.SourcePatches)
	}
	if review.ReplayFailures[0].SourcePatchStatus != "requires_manual_apply" || !review.ReplayFailures[0].SourcePatchRequiresManualApply {
		t.Fatalf("failure source patch status/manual = %q/%t", review.ReplayFailures[0].SourcePatchStatus, review.ReplayFailures[0].SourcePatchRequiresManualApply)
	}
	if review.ReplayFailures[0].RegressionSuggestions[0].SourcePatchPath == "" {
		t.Fatalf("suggestion source patch path missing: %+v", review.ReplayFailures[0].RegressionSuggestions[0])
	}
}

func TestGenerateQARegressionSourcePatchRejectsUnsafeTarget(t *testing.T) {
	approval := qaRegressionSourcePatchTestApproval()
	approval.SuggestedTestFile = "../internal/routing/routing_test.go"
	if _, err := GenerateQARegressionSourcePatchDraft(t.TempDir(), []QARegressionApprovedRecord{approval}, QARegressionSourcePatchRequest{TraceID: approval.TraceID}); err == nil {
		t.Fatal("GenerateQARegressionSourcePatchDraft() error = nil, want unsafe target rejection")
	}
}

func TestGenerateQARegressionSourcePatchApplyPlanRequiresExplicitApproval(t *testing.T) {
	patch := qaRegressionSourcePatchTestRecord(t)
	if _, err := GenerateQARegressionSourcePatchApplyPlan(t.TempDir(), []QARegressionSourcePatchRecord{patch}, QARegressionSourcePatchApplyPlanRequest{
		TraceID: patch.TraceID,
	}); err == nil {
		t.Fatal("GenerateQARegressionSourcePatchApplyPlan() error = nil, want explicit approval requirement")
	}
}

func TestGenerateQARegressionSourcePatchApplyPlanWritesManualPlanArtifact(t *testing.T) {
	patch := qaRegressionSourcePatchTestRecord(t)
	dir := t.TempDir()
	record, err := GenerateQARegressionSourcePatchApplyPlan(dir, []QARegressionSourcePatchRecord{patch}, QARegressionSourcePatchApplyPlanRequest{
		TraceID:    patch.TraceID,
		Approved:   true,
		ReviewedBy: "qa",
		ReviewNote: "Approve manual source apply after token=super-secret-value is removed.",
	})
	if err != nil {
		t.Fatalf("GenerateQARegressionSourcePatchApplyPlan() error = %v", err)
	}
	if record.Schema != QARegressionSourcePatchApplyPlanSchema || record.Status != "approved_for_manual_apply" {
		t.Fatalf("record schema/status = %q/%q", record.Schema, record.Status)
	}
	if record.AutomaticTestWritten || record.SourceTestWritten || !record.RequiresManualApply {
		t.Fatalf("record flags auto/source/manual = %t/%t/%t, want no writes/manual required", record.AutomaticTestWritten, record.SourceTestWritten, record.RequiresManualApply)
	}
	if record.Path == "" || !strings.HasPrefix(record.Path, filepath.Clean(dir)+string(os.PathSeparator)) {
		t.Fatalf("record path = %q, want inside %q", record.Path, dir)
	}
	if record.SourcePatchPath != patch.Path || !strings.Contains(record.PatchPreview, "diff --git") || len(record.ManualApplyChecklist) == 0 {
		t.Fatalf("record missing patch linkage/checklist: %+v", record)
	}
	if !strings.Contains(strings.Join(record.ManualApplyChecklist, "\n"), "focused package test") {
		t.Fatalf("manual checklist missing focused package guidance: %+v", record.ManualApplyChecklist)
	}
	data, err := os.ReadFile(record.Path)
	if err != nil {
		t.Fatalf("ReadFile() apply plan error = %v", err)
	}
	text := string(data)
	for _, want := range []string{
		QARegressionSourcePatchApplyPlanSchema,
		`"status": "approved_for_manual_apply"`,
		`"automaticTestWritten": false`,
		`"sourceTestWritten": false`,
		`"requiresManualApply": true`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("apply plan missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "super-secret-value") || strings.Contains(record.Preview, "super-secret-value") {
		t.Fatalf("apply plan leaked secret:\n%s\npreview=%s", text, record.Preview)
	}
}

func TestListAndApplyQARegressionSourcePatchApplyPlans(t *testing.T) {
	patch := qaRegressionSourcePatchTestRecord(t)
	record, err := GenerateQARegressionSourcePatchApplyPlan(t.TempDir(), []QARegressionSourcePatchRecord{patch}, QARegressionSourcePatchApplyPlanRequest{
		TraceID:  patch.TraceID,
		Approved: true,
	})
	if err != nil {
		t.Fatalf("GenerateQARegressionSourcePatchApplyPlan() error = %v", err)
	}
	records, err := ListQARegressionSourcePatchApplyPlans(filepath.Dir(record.Path))
	if err != nil {
		t.Fatalf("ListQARegressionSourcePatchApplyPlans() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records len = %d, want 1", len(records))
	}
	review := QAReview{
		ReplayFailures: []QAReplayFailure{{
			TraceID:              patch.TraceID,
			PromotionFingerprint: patch.Fingerprint,
			RegressionSuggestions: []QARegressionSuggestion{{
				Name:                 patch.Name,
				PromotionFingerprint: patch.Fingerprint,
			}},
		}},
	}
	ApplyQARegressionSourcePatchApplyPlans(&review, records)
	if review.Summary.SourcePatchApplyPlans != 1 {
		t.Fatalf("source patch apply plan count = %d, want 1", review.Summary.SourcePatchApplyPlans)
	}
	if review.ReplayFailures[0].SourcePatchApplyStatus != "approved_for_manual_apply" || !review.ReplayFailures[0].SourcePatchRequiresManualApply {
		t.Fatalf("failure source patch apply status/manual = %q/%t", review.ReplayFailures[0].SourcePatchApplyStatus, review.ReplayFailures[0].SourcePatchRequiresManualApply)
	}
	if review.ReplayFailures[0].RegressionSuggestions[0].SourcePatchApplyPath == "" {
		t.Fatalf("suggestion source patch apply path missing: %+v", review.ReplayFailures[0].RegressionSuggestions[0])
	}
}

func qaRegressionSourcePatchTestRecord(t *testing.T) QARegressionSourcePatchRecord {
	t.Helper()
	approval := qaRegressionSourcePatchTestApproval()
	record, err := GenerateQARegressionSourcePatchDraft(t.TempDir(), []QARegressionApprovedRecord{approval}, QARegressionSourcePatchRequest{
		TraceID: approval.TraceID,
	})
	if err != nil {
		t.Fatalf("GenerateQARegressionSourcePatchDraft() error = %v", err)
	}
	return record
}

func qaRegressionSourcePatchTestApproval() QARegressionApprovedRecord {
	return QARegressionApprovedRecord{
		Schema:               QARegressionApprovedSchema,
		Name:                 "qa_review_source_patch_wrong_source",
		Kind:                 "route_governance_wrong_source",
		TraceID:              "trace-qa-source-patch",
		SuggestionName:       "qa_review_source_patch_wrong_source",
		Category:             RouteFailureWrongSource,
		Fingerprint:          "route_governance:source_patch",
		Status:               "approved_regression",
		SuggestedTestFile:    "internal/routing/routing_test.go",
		SuggestedTestName:    "TestQAReviewSourcePatchWrongSource",
		AutomaticTestWritten: false,
		Promotion: RouteRegressionPromotion{
			SuggestedTestName: "TestQAReviewSourcePatchWrongSource",
			RouteUnderTest:    "internet_search",
			ExpectedSource:    "local_documents",
			ExpectedToolLane:  "document_lane",
			ExpectedOutcome:   "ask_clarification_or_route_to_local_docs",
		},
	}
}
