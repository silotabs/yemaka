package learning

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const QARegressionSourcePatchApplyPlanSchema = "yemaka.qa_review_source_patch_apply_plan.v1"

type QARegressionSourcePatchApplyPlanRequest struct {
	TraceID         string `json:"traceId"`
	SuggestionName  string `json:"suggestionName,omitempty"`
	DraftName       string `json:"draftName,omitempty"`
	SourcePatchName string `json:"sourcePatchName,omitempty"`
	Approved        bool   `json:"approved"`
	ReviewedBy      string `json:"reviewedBy,omitempty"`
	ReviewNote      string `json:"reviewNote,omitempty"`
}

type QARegressionSourcePatchApplyPlanRecord struct {
	Schema                 string                   `json:"schema"`
	Name                   string                   `json:"name"`
	Kind                   string                   `json:"kind"`
	TraceID                string                   `json:"traceId"`
	SuggestionName         string                   `json:"suggestionName,omitempty"`
	Category               string                   `json:"category,omitempty"`
	Fingerprint            string                   `json:"fingerprint,omitempty"`
	Status                 string                   `json:"status"`
	ReviewedBy             string                   `json:"reviewedBy,omitempty"`
	ReviewNote             string                   `json:"reviewNote,omitempty"`
	ApprovedAt             string                   `json:"approvedAt"`
	Path                   string                   `json:"path,omitempty"`
	SourcePatchPath        string                   `json:"sourcePatchPath,omitempty"`
	ApprovedRegressionPath string                   `json:"approvedRegressionPath,omitempty"`
	TestDraftPath          string                   `json:"testDraftPath,omitempty"`
	SuggestedTestFile      string                   `json:"suggestedTestFile"`
	SuggestedTestName      string                   `json:"suggestedTestName"`
	AutomaticTestWritten   bool                     `json:"automaticTestWritten"`
	SourceTestWritten      bool                     `json:"sourceTestWritten"`
	RequiresManualApply    bool                     `json:"requiresManualApply"`
	PatchPreview           string                   `json:"patchPreview"`
	ManualApplyChecklist   []string                 `json:"manualApplyChecklist"`
	Preview                string                   `json:"preview"`
	Promotion              RouteRegressionPromotion `json:"promotion"`
}

func GenerateQARegressionSourcePatchApplyPlan(outputDir string, patches []QARegressionSourcePatchRecord, request QARegressionSourcePatchApplyPlanRequest) (QARegressionSourcePatchApplyPlanRecord, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return QARegressionSourcePatchApplyPlanRecord{}, fmt.Errorf("QA regression source patch apply-plan directory is required")
	}
	if regressionArtifactPathLooksRemote(outputDir) {
		return QARegressionSourcePatchApplyPlanRecord{}, fmt.Errorf("QA regression source patch apply-plan directory must be a local filesystem path: %s", outputDir)
	}
	if !request.Approved {
		return QARegressionSourcePatchApplyPlanRecord{}, fmt.Errorf("source patch apply plan requires explicit approval")
	}
	patch, ok := findQARegressionSourcePatch(patches, request)
	if !ok {
		return QARegressionSourcePatchApplyPlanRecord{}, fmt.Errorf("source patch draft must exist before an apply plan can be approved")
	}
	if strings.TrimSpace(patch.Path) != "" {
		if _, err := os.Stat(patch.Path); err != nil {
			return QARegressionSourcePatchApplyPlanRecord{}, fmt.Errorf("source patch draft artifact is not readable: %w", err)
		}
	}
	if err := validateQARegressionSourcePatchRecord(patch); err != nil {
		return QARegressionSourcePatchApplyPlanRecord{}, err
	}
	record := qaRegressionSourcePatchApplyPlanRecord(patch, request)
	record.Preview = summarizeText(FormatQARegressionSourcePatchApplyPlanYAMLish(record), 1800)

	base := filepath.Clean(outputDir)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return QARegressionSourcePatchApplyPlanRecord{}, fmt.Errorf("create QA regression source patch apply-plan directory: %w", err)
	}
	info, err := os.Stat(base)
	if err != nil {
		return QARegressionSourcePatchApplyPlanRecord{}, fmt.Errorf("stat QA regression source patch apply-plan directory: %w", err)
	}
	if !info.IsDir() {
		return QARegressionSourcePatchApplyPlanRecord{}, fmt.Errorf("QA regression source patch apply-plan path is not a directory: %s", outputDir)
	}
	path, err := regressionArtifactPath(base, qaRegressionSourcePatchApplyPlanFilename(record))
	if err != nil {
		return QARegressionSourcePatchApplyPlanRecord{}, err
	}
	record.Path = path
	if err := writeJSONArtifact(path, record); err != nil {
		return QARegressionSourcePatchApplyPlanRecord{}, fmt.Errorf("write QA regression source patch apply plan: %w", err)
	}
	return record, nil
}

func ListQARegressionSourcePatchApplyPlans(outputDir string) ([]QARegressionSourcePatchApplyPlanRecord, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return nil, fmt.Errorf("QA regression source patch apply-plan directory is required")
	}
	if regressionArtifactPathLooksRemote(outputDir) {
		return nil, fmt.Errorf("QA regression source patch apply-plan directory must be a local filesystem path: %s", outputDir)
	}
	entries, err := os.ReadDir(filepath.Clean(outputDir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list QA regression source patch apply plans: %w", err)
	}
	var records []QARegressionSourcePatchApplyPlanRecord
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		path, err := regressionArtifactPath(filepath.Clean(outputDir), entry.Name())
		if err != nil {
			return nil, err
		}
		var record QARegressionSourcePatchApplyPlanRecord
		if err := readJSONArtifact(path, &record); err != nil {
			return nil, fmt.Errorf("decode QA regression source patch apply plan %s: %w", entry.Name(), err)
		}
		if err := validateQARegressionSourcePatchApplyPlanRecord(record); err != nil {
			return nil, fmt.Errorf("invalid QA regression source patch apply plan %s: %w", entry.Name(), err)
		}
		record.Path = path
		records = append(records, record)
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].ApprovedAt < records[j].ApprovedAt
	})
	return records, nil
}

func ApplyQARegressionSourcePatchApplyPlans(review *QAReview, plans []QARegressionSourcePatchApplyPlanRecord) {
	if review == nil {
		return
	}
	planIndex := map[string]QARegressionSourcePatchApplyPlanRecord{}
	for _, plan := range plans {
		if err := validateQARegressionSourcePatchApplyPlanRecord(plan); err != nil {
			continue
		}
		for _, key := range qaRegressionDraftKeys(plan.TraceID, plan.Fingerprint, plan.SuggestionName, plan.Name, plan.Kind) {
			planIndex[key] = plan
		}
	}
	seen := map[string]bool{}
	for i := range review.ReplayFailures {
		failure := &review.ReplayFailures[i]
		if plan, ok := qaRegressionSourcePatchApplyPlanForFailure(*failure, planIndex); ok {
			applyQARegressionSourcePatchApplyPlanToFailure(failure, plan)
			countQARegressionSourcePatchApplyPlan(&review.Summary, plan, seen)
		}
		for j := range failure.RegressionSuggestions {
			suggestion := &failure.RegressionSuggestions[j]
			if plan, ok := qaRegressionSourcePatchApplyPlanForSuggestion(*failure, *suggestion, planIndex); ok {
				applyQARegressionSourcePatchApplyPlanToSuggestion(suggestion, plan)
				countQARegressionSourcePatchApplyPlan(&review.Summary, plan, seen)
			}
		}
	}
}

func FormatQARegressionSourcePatchApplyPlanYAMLish(record QARegressionSourcePatchApplyPlanRecord) string {
	var out strings.Builder
	fmt.Fprintf(&out, "schema: %s\n", yamlString(QARegressionSourcePatchApplyPlanSchema))
	fmt.Fprintf(&out, "status: %s\n", yamlString(record.Status))
	fmt.Fprintf(&out, "automatic_test_written: %t\n", record.AutomaticTestWritten)
	fmt.Fprintf(&out, "source_test_written: %t\n", record.SourceTestWritten)
	fmt.Fprintf(&out, "requires_manual_apply: %t\n", record.RequiresManualApply)
	fmt.Fprintf(&out, "name: %s\n", yamlString(record.Name))
	fmt.Fprintf(&out, "kind: %s\n", yamlString(record.Kind))
	fmt.Fprintf(&out, "source_trace_id: %s\n", yamlString(record.TraceID))
	fmt.Fprintf(&out, "category: %s\n", yamlString(record.Category))
	fmt.Fprintf(&out, "fingerprint: %s\n", yamlString(record.Fingerprint))
	fmt.Fprintf(&out, "source_patch_path: %s\n", yamlString(record.SourcePatchPath))
	fmt.Fprintf(&out, "suggested_test_file: %s\n", yamlString(record.SuggestedTestFile))
	fmt.Fprintf(&out, "suggested_test_name: %s\n", yamlString(record.SuggestedTestName))
	writeYAMLStringList(&out, "", "manual_apply_checklist", record.ManualApplyChecklist)
	out.WriteString("patch_preview: |\n")
	writeIndentedLines(&out, record.PatchPreview, "  ")
	return strings.TrimRight(out.String(), "\n")
}

func validateQARegressionSourcePatchApplyPlanRecord(record QARegressionSourcePatchApplyPlanRecord) error {
	if record.Schema != QARegressionSourcePatchApplyPlanSchema {
		return fmt.Errorf("schema = %q, want %q", record.Schema, QARegressionSourcePatchApplyPlanSchema)
	}
	if strings.TrimSpace(record.TraceID) == "" || strings.TrimSpace(record.Name) == "" {
		return fmt.Errorf("source patch apply plan trace id and name are required")
	}
	if record.Status != "approved_for_manual_apply" {
		return fmt.Errorf("source patch apply plan status = %q, want approved_for_manual_apply", record.Status)
	}
	if record.AutomaticTestWritten {
		return fmt.Errorf("source patch apply plan must not mark automatic test writing as true")
	}
	if record.SourceTestWritten {
		return fmt.Errorf("source patch apply plan must not mark source test writing as true")
	}
	if !record.RequiresManualApply {
		return fmt.Errorf("source patch apply plan must require manual apply")
	}
	return validateSuggestedSourceTestFile(record.SuggestedTestFile)
}

func findQARegressionSourcePatch(patches []QARegressionSourcePatchRecord, request QARegressionSourcePatchApplyPlanRequest) (QARegressionSourcePatchRecord, bool) {
	index := map[string]QARegressionSourcePatchRecord{}
	for _, patch := range patches {
		if err := validateQARegressionSourcePatchRecord(patch); err != nil {
			continue
		}
		for _, key := range qaRegressionDraftKeys(patch.TraceID, patch.Fingerprint, patch.SuggestionName, patch.Name, patch.Kind) {
			index[key] = patch
		}
	}
	for _, key := range qaRegressionDraftKeys(
		request.TraceID,
		"",
		sanitizeRegressionName(request.SuggestionName),
		sanitizeRegressionName(firstNonEmpty(request.SourcePatchName, request.DraftName)),
		request.SuggestionName,
	) {
		if patch, ok := index[key]; ok {
			return patch, true
		}
	}
	return QARegressionSourcePatchRecord{}, false
}

func qaRegressionSourcePatchApplyPlanRecord(patch QARegressionSourcePatchRecord, request QARegressionSourcePatchApplyPlanRequest) QARegressionSourcePatchApplyPlanRecord {
	return QARegressionSourcePatchApplyPlanRecord{
		Schema:                 QARegressionSourcePatchApplyPlanSchema,
		Name:                   sanitizeRegressionName(firstNonEmpty(patch.Name, patch.SuggestionName, patch.SuggestedTestName, "qa_review_source_patch_apply_plan")),
		Kind:                   patch.Kind,
		TraceID:                patch.TraceID,
		SuggestionName:         patch.SuggestionName,
		Category:               patch.Category,
		Fingerprint:            patch.Fingerprint,
		Status:                 "approved_for_manual_apply",
		ReviewedBy:             sanitizeRegressionText(request.ReviewedBy, 80),
		ReviewNote:             sanitizeRegressionText(request.ReviewNote, 240),
		ApprovedAt:             time.Now().UTC().Format(time.RFC3339Nano),
		SourcePatchPath:        patch.Path,
		ApprovedRegressionPath: patch.ApprovedRegressionPath,
		TestDraftPath:          patch.TestDraftPath,
		SuggestedTestFile:      patch.SuggestedTestFile,
		SuggestedTestName:      patch.SuggestedTestName,
		AutomaticTestWritten:   false,
		SourceTestWritten:      false,
		RequiresManualApply:    true,
		PatchPreview:           patch.PatchPreview,
		ManualApplyChecklist:   qaRegressionSourcePatchManualApplyChecklist(patch),
		Promotion:              patch.Promotion,
	}
}

func qaRegressionSourcePatchManualApplyChecklist(patch QARegressionSourcePatchRecord) []string {
	target := firstNonEmpty(patch.SuggestedTestFile, "the suggested Go test file")
	name := firstNonEmpty(patch.SuggestedTestName, "the suggested regression test")
	return []string{
		"Open " + target + " and review the source patch draft before editing.",
		"Replace the draft scaffold for " + name + " with deterministic assertions that do not depend on private replay data.",
		"Run the focused package test for " + target + ", then run go test ./... before committing.",
		"Keep the reviewed artifact path linked in the commit or release note so QAReview can trace why the regression exists.",
	}
}

func qaRegressionSourcePatchApplyPlanFilename(record QARegressionSourcePatchApplyPlanRecord) string {
	name := sanitizeRegressionName(firstNonEmpty(record.Name, record.SuggestionName, "qa_review_source_patch_apply_plan"))
	fingerprint := sanitizeRegressionName(strings.TrimPrefix(record.Fingerprint, "route_governance:"))
	if fingerprint != "" {
		return fmt.Sprintf("apply_plan_%s_%s.json", name, fingerprint)
	}
	return fmt.Sprintf("apply_plan_%s.json", name)
}

func qaRegressionSourcePatchApplyPlanForFailure(failure QAReplayFailure, plans map[string]QARegressionSourcePatchApplyPlanRecord) (QARegressionSourcePatchApplyPlanRecord, bool) {
	for _, key := range qaRegressionDraftKeys(failure.TraceID, failure.PromotionFingerprint, "", "", "") {
		if plan, ok := plans[key]; ok {
			return plan, true
		}
	}
	return QARegressionSourcePatchApplyPlanRecord{}, false
}

func qaRegressionSourcePatchApplyPlanForSuggestion(failure QAReplayFailure, suggestion QARegressionSuggestion, plans map[string]QARegressionSourcePatchApplyPlanRecord) (QARegressionSourcePatchApplyPlanRecord, bool) {
	for _, key := range qaRegressionDraftKeys(failure.TraceID, firstNonEmpty(suggestion.PromotionFingerprint, failure.PromotionFingerprint), suggestion.Name, suggestion.Name, suggestion.Kind) {
		if plan, ok := plans[key]; ok {
			return plan, true
		}
	}
	return QARegressionSourcePatchApplyPlanRecord{}, false
}

func applyQARegressionSourcePatchApplyPlanToFailure(failure *QAReplayFailure, plan QARegressionSourcePatchApplyPlanRecord) {
	failure.SourcePatchApplyStatus = plan.Status
	failure.SourcePatchApplyPath = plan.Path
	failure.SourcePatchRequiresManualApply = plan.RequiresManualApply
	failure.CoverageStatus = "source_patch_apply_plan_approved"
	failure.CoverageSources = appendUniqueStrings(failure.CoverageSources, "qa_review:source_patch_apply_plan")
}

func applyQARegressionSourcePatchApplyPlanToSuggestion(suggestion *QARegressionSuggestion, plan QARegressionSourcePatchApplyPlanRecord) {
	suggestion.SourcePatchApplyStatus = plan.Status
	suggestion.SourcePatchApplyPath = plan.Path
	suggestion.SourcePatchRequiresManualApply = plan.RequiresManualApply
	suggestion.CoverageStatus = "source_patch_apply_plan_approved"
	suggestion.CoverageSources = appendUniqueStrings(suggestion.CoverageSources, "qa_review:source_patch_apply_plan")
}

func countQARegressionSourcePatchApplyPlan(summary *QAReviewSummary, plan QARegressionSourcePatchApplyPlanRecord, seen map[string]bool) {
	key := firstNonEmpty(plan.Fingerprint, plan.TraceID, plan.Name)
	if key == "" || seen[key] {
		return
	}
	seen[key] = true
	summary.SourcePatchApplyPlans++
}
