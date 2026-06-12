package learning

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const QARegressionSourcePatchSchema = "yemaka.qa_review_source_patch_draft.v1"

type QARegressionSourcePatchRequest struct {
	TraceID        string `json:"traceId"`
	SuggestionName string `json:"suggestionName,omitempty"`
	DraftName      string `json:"draftName,omitempty"`
	ReviewedBy     string `json:"reviewedBy,omitempty"`
	ReviewNote     string `json:"reviewNote,omitempty"`
}

type QARegressionSourcePatchRecord struct {
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
	DraftedAt              string                   `json:"draftedAt"`
	Path                   string                   `json:"path,omitempty"`
	ApprovedRegressionPath string                   `json:"approvedRegressionPath,omitempty"`
	TestDraftPath          string                   `json:"testDraftPath,omitempty"`
	SuggestedTestFile      string                   `json:"suggestedTestFile"`
	SuggestedTestName      string                   `json:"suggestedTestName"`
	AutomaticTestWritten   bool                     `json:"automaticTestWritten"`
	RequiresManualApply    bool                     `json:"requiresManualApply"`
	TestSnippet            string                   `json:"testSnippet"`
	PatchPreview           string                   `json:"patchPreview"`
	Preview                string                   `json:"preview"`
	Promotion              RouteRegressionPromotion `json:"promotion"`
}

func GenerateQARegressionSourcePatchDraft(outputDir string, approvals []QARegressionApprovedRecord, request QARegressionSourcePatchRequest) (QARegressionSourcePatchRecord, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return QARegressionSourcePatchRecord{}, fmt.Errorf("QA regression source patch directory is required")
	}
	if regressionArtifactPathLooksRemote(outputDir) {
		return QARegressionSourcePatchRecord{}, fmt.Errorf("QA regression source patch directory must be a local filesystem path: %s", outputDir)
	}
	approval, ok := findQARegressionApproval(approvals, request)
	if !ok {
		return QARegressionSourcePatchRecord{}, fmt.Errorf("approved regression must exist before a source patch draft can be generated")
	}
	if strings.TrimSpace(approval.Path) != "" {
		if _, err := os.Stat(approval.Path); err != nil {
			return QARegressionSourcePatchRecord{}, fmt.Errorf("approved regression artifact is not readable: %w", err)
		}
	}
	if err := validateSuggestedSourceTestFile(approval.SuggestedTestFile); err != nil {
		return QARegressionSourcePatchRecord{}, err
	}
	record := qaRegressionSourcePatchRecord(approval, request)
	record.TestSnippet = qaRegressionSourcePatchSnippet(record)
	record.PatchPreview = qaRegressionSourcePatchPreview(record.SuggestedTestFile, record.TestSnippet)
	record.Preview = summarizeText(FormatQARegressionSourcePatchYAMLish(record), 1800)

	base := filepath.Clean(outputDir)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return QARegressionSourcePatchRecord{}, fmt.Errorf("create QA regression source patch directory: %w", err)
	}
	info, err := os.Stat(base)
	if err != nil {
		return QARegressionSourcePatchRecord{}, fmt.Errorf("stat QA regression source patch directory: %w", err)
	}
	if !info.IsDir() {
		return QARegressionSourcePatchRecord{}, fmt.Errorf("QA regression source patch path is not a directory: %s", outputDir)
	}
	path, err := regressionArtifactPath(base, qaRegressionSourcePatchFilename(record))
	if err != nil {
		return QARegressionSourcePatchRecord{}, err
	}
	record.Path = path
	if err := writeJSONArtifact(path, record); err != nil {
		return QARegressionSourcePatchRecord{}, fmt.Errorf("write QA regression source patch draft: %w", err)
	}
	return record, nil
}

func ListQARegressionSourcePatchDrafts(outputDir string) ([]QARegressionSourcePatchRecord, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return nil, fmt.Errorf("QA regression source patch directory is required")
	}
	if regressionArtifactPathLooksRemote(outputDir) {
		return nil, fmt.Errorf("QA regression source patch directory must be a local filesystem path: %s", outputDir)
	}
	entries, err := os.ReadDir(filepath.Clean(outputDir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list QA regression source patches: %w", err)
	}
	var records []QARegressionSourcePatchRecord
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		path, err := regressionArtifactPath(filepath.Clean(outputDir), entry.Name())
		if err != nil {
			return nil, err
		}
		var record QARegressionSourcePatchRecord
		if err := readJSONArtifact(path, &record); err != nil {
			return nil, fmt.Errorf("decode QA regression source patch %s: %w", entry.Name(), err)
		}
		if err := validateQARegressionSourcePatchRecord(record); err != nil {
			return nil, fmt.Errorf("invalid QA regression source patch %s: %w", entry.Name(), err)
		}
		record.Path = path
		records = append(records, record)
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].DraftedAt < records[j].DraftedAt
	})
	return records, nil
}

func ApplyQARegressionSourcePatches(review *QAReview, patches []QARegressionSourcePatchRecord) {
	if review == nil {
		return
	}
	patchIndex := map[string]QARegressionSourcePatchRecord{}
	for _, patch := range patches {
		if err := validateQARegressionSourcePatchRecord(patch); err != nil {
			continue
		}
		for _, key := range qaRegressionDraftKeys(patch.TraceID, patch.Fingerprint, patch.SuggestionName, patch.Name, patch.Kind) {
			patchIndex[key] = patch
		}
	}
	seen := map[string]bool{}
	for i := range review.ReplayFailures {
		failure := &review.ReplayFailures[i]
		if patch, ok := qaRegressionSourcePatchForFailure(*failure, patchIndex); ok {
			applyQARegressionSourcePatchToFailure(failure, patch)
			countQARegressionSourcePatch(&review.Summary, patch, seen)
		}
		for j := range failure.RegressionSuggestions {
			suggestion := &failure.RegressionSuggestions[j]
			if patch, ok := qaRegressionSourcePatchForSuggestion(*failure, *suggestion, patchIndex); ok {
				applyQARegressionSourcePatchToSuggestion(suggestion, patch)
				countQARegressionSourcePatch(&review.Summary, patch, seen)
			}
		}
	}
}

func FormatQARegressionSourcePatchYAMLish(record QARegressionSourcePatchRecord) string {
	var out strings.Builder
	fmt.Fprintf(&out, "schema: %s\n", yamlString(QARegressionSourcePatchSchema))
	fmt.Fprintf(&out, "status: %s\n", yamlString(record.Status))
	fmt.Fprintf(&out, "automatic_test_written: %t\n", record.AutomaticTestWritten)
	fmt.Fprintf(&out, "requires_manual_apply: %t\n", record.RequiresManualApply)
	fmt.Fprintf(&out, "name: %s\n", yamlString(record.Name))
	fmt.Fprintf(&out, "kind: %s\n", yamlString(record.Kind))
	fmt.Fprintf(&out, "source_trace_id: %s\n", yamlString(record.TraceID))
	fmt.Fprintf(&out, "category: %s\n", yamlString(record.Category))
	fmt.Fprintf(&out, "fingerprint: %s\n", yamlString(record.Fingerprint))
	fmt.Fprintf(&out, "suggested_test_file: %s\n", yamlString(record.SuggestedTestFile))
	fmt.Fprintf(&out, "suggested_test_name: %s\n", yamlString(record.SuggestedTestName))
	writeYAMLStringList(&out, "", "manual_assertion_checklist", qaRegressionSourcePatchAssertionChecklist(record))
	out.WriteString("patch_preview: |\n")
	writeIndentedLines(&out, record.PatchPreview, "  ")
	return strings.TrimRight(out.String(), "\n")
}

func validateQARegressionSourcePatchRecord(record QARegressionSourcePatchRecord) error {
	if record.Schema != QARegressionSourcePatchSchema {
		return fmt.Errorf("schema = %q, want %q", record.Schema, QARegressionSourcePatchSchema)
	}
	if strings.TrimSpace(record.TraceID) == "" || strings.TrimSpace(record.Name) == "" {
		return fmt.Errorf("source patch trace id and name are required")
	}
	if record.Status != "requires_manual_apply" {
		return fmt.Errorf("source patch status = %q, want requires_manual_apply", record.Status)
	}
	if record.AutomaticTestWritten {
		return fmt.Errorf("source patch must not mark automatic test writing as true")
	}
	if !record.RequiresManualApply {
		return fmt.Errorf("source patch must require manual apply")
	}
	return validateSuggestedSourceTestFile(record.SuggestedTestFile)
}

func findQARegressionApproval(approvals []QARegressionApprovedRecord, request QARegressionSourcePatchRequest) (QARegressionApprovedRecord, bool) {
	index := map[string]QARegressionApprovedRecord{}
	for _, approval := range approvals {
		if err := validateQARegressionApprovedRecord(approval); err != nil {
			continue
		}
		for _, key := range qaRegressionDraftKeys(approval.TraceID, approval.Fingerprint, approval.SuggestionName, approval.Name, approval.Kind) {
			index[key] = approval
		}
	}
	for _, key := range qaRegressionDraftKeys(
		request.TraceID,
		"",
		sanitizeRegressionName(request.SuggestionName),
		sanitizeRegressionName(request.DraftName),
		request.SuggestionName,
	) {
		if approval, ok := index[key]; ok {
			return approval, true
		}
	}
	return QARegressionApprovedRecord{}, false
}

func qaRegressionSourcePatchRecord(approval QARegressionApprovedRecord, request QARegressionSourcePatchRequest) QARegressionSourcePatchRecord {
	return QARegressionSourcePatchRecord{
		Schema:                 QARegressionSourcePatchSchema,
		Name:                   sanitizeRegressionName(firstNonEmpty(approval.Name, approval.SuggestionName, approval.SuggestedTestName, "qa_review_source_patch")),
		Kind:                   approval.Kind,
		TraceID:                approval.TraceID,
		SuggestionName:         approval.SuggestionName,
		Category:               approval.Category,
		Fingerprint:            approval.Fingerprint,
		Status:                 "requires_manual_apply",
		ReviewedBy:             sanitizeRegressionText(request.ReviewedBy, 80),
		ReviewNote:             sanitizeRegressionText(request.ReviewNote, 240),
		DraftedAt:              time.Now().UTC().Format(time.RFC3339Nano),
		ApprovedRegressionPath: approval.Path,
		TestDraftPath:          approval.TestDraftPath,
		SuggestedTestFile:      filepath.ToSlash(filepath.Clean(approval.SuggestedTestFile)),
		SuggestedTestName:      sanitizeGoTestName(approval.SuggestedTestName),
		AutomaticTestWritten:   false,
		RequiresManualApply:    true,
		Promotion:              approval.Promotion,
	}
}

func qaRegressionSourcePatchSnippet(record QARegressionSourcePatchRecord) string {
	testName := strings.TrimSpace(record.SuggestedTestName)
	if testName == "" {
		testName = "TestQAReviewSourcePatchRegression"
	}
	target := filepath.ToSlash(filepath.Clean(record.SuggestedTestFile))
	var out strings.Builder
	fmt.Fprintf(&out, "func %s(t *testing.T) {\n", testName)
	out.WriteString("\tt.Helper()\n\n")
	out.WriteString("\t// QAReview source patch template.\n")
	out.WriteString("\t// Replace TODO: minimal public fixture prompt with a small non-private prompt.\n")
	out.WriteString("\t// Do not copy the observed failed route into an assertion until reviewed.\n")
	writeSourcePatchMetadataComments(&out, record)
	switch target {
	case "internal/routing/routing_test.go":
		out.WriteString("\t// Routing harness:\n")
		out.WriteString("\t// got := Classify(Request{Content: \"TODO: minimal public fixture prompt\"})\n")
		out.WriteString("\t// TODO_EXPECTED: assert got.RouteCategory / got.TaskType / source flags / required or forbidden tools.\n")
	case "internal/agent/routing_test.go":
		out.WriteString("\t// Agent harness:\n")
		out.WriteString("\t// plan := BuildPlan(\"TODO: minimal public fixture prompt\")\n")
		out.WriteString("\t// decision := DecideExecution(plan.Route, nil)\n")
		out.WriteString("\t// TODO_EXPECTED: assert route, risk, approval, evidence, and selected tool decision.\n")
	case "internal/learning/route_failure_classifier_test.go":
		out.WriteString("\t// Classifier harness:\n")
		out.WriteString("\t// got := ClassifyRouteFailure(replay.Trace{UserRequest: \"TODO: minimal public fixture prompt\"})\n")
		out.WriteString("\t// TODO_EXPECTED: assert category, reason, and promotion metadata.\n")
	default:
		out.WriteString("\t// Target harness:\n")
		out.WriteString("\t// TODO_EXPECTED: replace with deterministic assertions for this package.\n")
	}
	out.WriteString("\tt.Fatalf(\"source patch draft only: replace with deterministic regression assertions before committing\")\n")
	out.WriteString("}\n")
	return out.String()
}

func writeSourcePatchMetadataComments(out *strings.Builder, record QARegressionSourcePatchRecord) {
	fmt.Fprintf(out, "\t// Category: %s\n", sanitizeRegressionText(record.Category, 80))
	fmt.Fprintf(out, "\t// Observed route: %s\n", sanitizeRegressionText(record.Promotion.ObservedRoute, 120))
	fmt.Fprintf(out, "\t// Expected route: %s\n", sanitizeRegressionText(firstNonEmpty(record.Promotion.ExpectedRoute, record.Promotion.RouteUnderTest), 120))
	fmt.Fprintf(out, "\t// Continuation: %s\n", sanitizeRegressionText(record.Promotion.ContinuationMode, 80))
	fmt.Fprintf(out, "\t// Source: %s\n", sanitizeRegressionText(record.Promotion.ExpectedSource, 80))
	fmt.Fprintf(out, "\t// Tool lane: %s\n", sanitizeRegressionText(record.Promotion.ExpectedToolLane, 80))
	fmt.Fprintf(out, "\t// Outcome: %s\n", sanitizeRegressionText(record.Promotion.ExpectedOutcome, 160))
	if len(record.Promotion.RequiredTools) > 0 {
		fmt.Fprintf(out, "\t// Required tools: %s\n", strings.Join(sanitizeRegressionTextList(record.Promotion.RequiredTools, 80), ", "))
	}
	if len(record.Promotion.ForbiddenTools) > 0 {
		fmt.Fprintf(out, "\t// Forbidden tools: %s\n", strings.Join(sanitizeRegressionTextList(record.Promotion.ForbiddenTools, 80), ", "))
	}
}

func qaRegressionSourcePatchAssertionChecklist(record QARegressionSourcePatchRecord) []string {
	target := filepath.ToSlash(filepath.Clean(record.SuggestedTestFile))
	checklist := []string{
		"Use a minimal public fixture prompt; do not paste private replay text.",
		"Assert the expected route/source/tool lane from the reviewed regression, not the observed bad route.",
		"Keep the test deterministic and avoid live model, internet, connector, or filesystem dependencies.",
	}
	switch target {
	case "internal/routing/routing_test.go":
		checklist = append(checklist, "Use the routing Classify(Request{...}) harness and assert route/source flags.")
	case "internal/agent/routing_test.go":
		checklist = append(checklist, "Use the agent plan/execution harness and assert approval/tool gate behavior.")
	case "internal/learning/route_failure_classifier_test.go":
		checklist = append(checklist, "Use a synthetic replay.Trace and assert failure category plus promotion metadata.")
	default:
		checklist = append(checklist, "Use the existing package test harness for the suggested target file.")
	}
	checklist = append(checklist, "Run the focused package test before approving the source write.")
	return checklist
}

func qaRegressionSourcePatchPreview(testFile string, snippet string) string {
	testFile = filepath.ToSlash(filepath.Clean(testFile))
	var out strings.Builder
	fmt.Fprintf(&out, "diff --git a/%s b/%s\n", testFile, testFile)
	fmt.Fprintf(&out, "--- a/%s\n", testFile)
	fmt.Fprintf(&out, "+++ b/%s\n", testFile)
	out.WriteString("@@\n")
	out.WriteString("+\n")
	for _, line := range strings.Split(strings.TrimRight(snippet, "\n"), "\n") {
		fmt.Fprintf(&out, "+%s\n", line)
	}
	return out.String()
}

func qaRegressionSourcePatchFilename(record QARegressionSourcePatchRecord) string {
	name := sanitizeRegressionName(firstNonEmpty(record.Name, record.SuggestionName, "qa_review_source_patch"))
	fingerprint := sanitizeRegressionName(strings.TrimPrefix(record.Fingerprint, "route_governance:"))
	if fingerprint != "" {
		return fmt.Sprintf("source_patch_%s_%s.json", name, fingerprint)
	}
	return fmt.Sprintf("source_patch_%s.json", name)
}

func validateSuggestedSourceTestFile(file string) error {
	file = strings.TrimSpace(file)
	if file == "" {
		return fmt.Errorf("suggested source test file is required")
	}
	if regressionArtifactPathLooksRemote(file) {
		return fmt.Errorf("suggested source test file must be a local repo-relative path: %s", file)
	}
	if filepath.IsAbs(file) {
		return fmt.Errorf("suggested source test file must be repo-relative: %s", file)
	}
	clean := filepath.ToSlash(filepath.Clean(file))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return fmt.Errorf("suggested source test file escapes the repository: %s", file)
	}
	if !strings.HasSuffix(clean, "_test.go") {
		return fmt.Errorf("suggested source patch target must be a Go test file: %s", file)
	}
	return nil
}

func qaRegressionSourcePatchForFailure(failure QAReplayFailure, patches map[string]QARegressionSourcePatchRecord) (QARegressionSourcePatchRecord, bool) {
	for _, key := range qaRegressionDraftKeys(failure.TraceID, failure.PromotionFingerprint, "", "", "") {
		if patch, ok := patches[key]; ok {
			return patch, true
		}
	}
	return QARegressionSourcePatchRecord{}, false
}

func qaRegressionSourcePatchForSuggestion(failure QAReplayFailure, suggestion QARegressionSuggestion, patches map[string]QARegressionSourcePatchRecord) (QARegressionSourcePatchRecord, bool) {
	for _, key := range qaRegressionDraftKeys(failure.TraceID, firstNonEmpty(suggestion.PromotionFingerprint, failure.PromotionFingerprint), suggestion.Name, suggestion.Name, suggestion.Kind) {
		if patch, ok := patches[key]; ok {
			return patch, true
		}
	}
	return QARegressionSourcePatchRecord{}, false
}

func applyQARegressionSourcePatchToFailure(failure *QAReplayFailure, patch QARegressionSourcePatchRecord) {
	failure.SourcePatchStatus = patch.Status
	failure.SourcePatchPath = patch.Path
	failure.SourcePatchRequiresManualApply = patch.RequiresManualApply
	failure.CoverageStatus = "source_patch_drafted"
	failure.CoverageSources = appendUniqueStrings(failure.CoverageSources, "qa_review:source_patch")
}

func applyQARegressionSourcePatchToSuggestion(suggestion *QARegressionSuggestion, patch QARegressionSourcePatchRecord) {
	suggestion.SourcePatchStatus = patch.Status
	suggestion.SourcePatchPath = patch.Path
	suggestion.SourcePatchRequiresManualApply = patch.RequiresManualApply
	suggestion.CoverageStatus = "source_patch_drafted"
	suggestion.CoverageSources = appendUniqueStrings(suggestion.CoverageSources, "qa_review:source_patch")
}

func countQARegressionSourcePatch(summary *QAReviewSummary, patch QARegressionSourcePatchRecord, seen map[string]bool) {
	key := firstNonEmpty(patch.Fingerprint, patch.TraceID, patch.Name)
	if key == "" || seen[key] {
		return
	}
	seen[key] = true
	summary.SourcePatches++
}

func writeIndentedLines(out *strings.Builder, text string, prefix string) {
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		fmt.Fprintf(out, "%s%s\n", prefix, line)
	}
}
