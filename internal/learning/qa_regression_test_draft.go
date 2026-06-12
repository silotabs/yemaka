package learning

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"yemaka/internal/replay"
)

const QARegressionTestDraftSchema = "yemaka.qa_review_regression_test_draft.v1"
const QARegressionApprovedSchema = "yemaka.qa_review_approved_regression.v1"

type QARegressionTestDraftRequest struct {
	TraceID        string `json:"traceId"`
	SuggestionName string `json:"suggestionName,omitempty"`
	ReviewedBy     string `json:"reviewedBy,omitempty"`
	ReviewNote     string `json:"reviewNote,omitempty"`
}

type QARegressionTestDraftRecord struct {
	Schema               string                   `json:"schema"`
	Name                 string                   `json:"name"`
	Kind                 string                   `json:"kind"`
	TraceID              string                   `json:"traceId"`
	SuggestionName       string                   `json:"suggestionName,omitempty"`
	Category             string                   `json:"category,omitempty"`
	Fingerprint          string                   `json:"fingerprint,omitempty"`
	Status               string                   `json:"status"`
	ReviewedBy           string                   `json:"reviewedBy,omitempty"`
	ReviewNote           string                   `json:"reviewNote,omitempty"`
	DraftedAt            string                   `json:"draftedAt"`
	Path                 string                   `json:"path,omitempty"`
	Preview              string                   `json:"preview"`
	SuggestedTestFile    string                   `json:"suggestedTestFile"`
	SuggestedTestName    string                   `json:"suggestedTestName"`
	AutomaticTestWritten bool                     `json:"automaticTestWritten"`
	Promotion            RouteRegressionPromotion `json:"promotion"`
}

type QARegressionApprovalRequest struct {
	TraceID        string `json:"traceId"`
	SuggestionName string `json:"suggestionName,omitempty"`
	DraftName      string `json:"draftName,omitempty"`
	ReviewedBy     string `json:"reviewedBy,omitempty"`
	ReviewNote     string `json:"reviewNote,omitempty"`
}

type QARegressionApprovedRecord struct {
	Schema               string                   `json:"schema"`
	Name                 string                   `json:"name"`
	Kind                 string                   `json:"kind"`
	TraceID              string                   `json:"traceId"`
	SuggestionName       string                   `json:"suggestionName,omitempty"`
	Category             string                   `json:"category,omitempty"`
	Fingerprint          string                   `json:"fingerprint,omitempty"`
	Status               string                   `json:"status"`
	ReviewedBy           string                   `json:"reviewedBy,omitempty"`
	ReviewNote           string                   `json:"reviewNote,omitempty"`
	ApprovedAt           string                   `json:"approvedAt"`
	Path                 string                   `json:"path,omitempty"`
	TestDraftPath        string                   `json:"testDraftPath,omitempty"`
	SuggestedTestFile    string                   `json:"suggestedTestFile"`
	SuggestedTestName    string                   `json:"suggestedTestName"`
	AutomaticTestWritten bool                     `json:"automaticTestWritten"`
	Promotion            RouteRegressionPromotion `json:"promotion"`
}

func GenerateQARegressionTestDraft(outputDir string, reviews []QARegressionReviewRecord, traces replay.Store, latestEval *QAEvalSummary, request QARegressionTestDraftRequest) (QARegressionTestDraftRecord, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return QARegressionTestDraftRecord{}, fmt.Errorf("QA regression test draft directory is required")
	}
	if regressionArtifactPathLooksRemote(outputDir) {
		return QARegressionTestDraftRecord{}, fmt.Errorf("QA regression test draft directory must be a local filesystem path: %s", outputDir)
	}
	trace, classification, suggestion, err := qaRegressionSuggestionContext(traces, latestEval, request.TraceID, request.SuggestionName)
	if err != nil {
		return QARegressionTestDraftRecord{}, err
	}
	artifact := qaRegressionPromotionArtifact(trace, classification, suggestion)
	review, ok := approvedReviewForRegressionSuggestion(reviews, trace, artifact, suggestion)
	if !ok || review.Status != QARegressionReviewApproved {
		return QARegressionTestDraftRecord{}, fmt.Errorf("QA regression suggestion must be approved before a test draft can be generated")
	}
	record := qaRegressionTestDraftRecord(trace, classification, suggestion, artifact, request)
	content := FormatQARegressionTestDraftYAMLish(record, trace, classification) + "\n"
	record.Preview = summarizeText(content, 1600)

	base := filepath.Clean(outputDir)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return QARegressionTestDraftRecord{}, fmt.Errorf("create QA regression test draft directory: %w", err)
	}
	info, err := os.Stat(base)
	if err != nil {
		return QARegressionTestDraftRecord{}, fmt.Errorf("stat QA regression test draft directory: %w", err)
	}
	if !info.IsDir() {
		return QARegressionTestDraftRecord{}, fmt.Errorf("QA regression test draft path is not a directory: %s", outputDir)
	}
	path, err := regressionArtifactPath(base, qaRegressionTestDraftFilename(record))
	if err != nil {
		return QARegressionTestDraftRecord{}, err
	}
	record.Path = path
	if err := writeJSONArtifact(path, record); err != nil {
		return QARegressionTestDraftRecord{}, fmt.Errorf("write QA regression test draft: %w", err)
	}
	return record, nil
}

func ApproveQARegressionTestDraft(outputDir string, drafts []QARegressionTestDraftRecord, request QARegressionApprovalRequest) (QARegressionApprovedRecord, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return QARegressionApprovedRecord{}, fmt.Errorf("QA regression approval directory is required")
	}
	if regressionArtifactPathLooksRemote(outputDir) {
		return QARegressionApprovedRecord{}, fmt.Errorf("QA regression approval directory must be a local filesystem path: %s", outputDir)
	}
	draft, ok := findQARegressionTestDraft(drafts, request)
	if !ok {
		return QARegressionApprovedRecord{}, fmt.Errorf("QA regression test draft must exist before regression approval")
	}
	if strings.TrimSpace(draft.Path) != "" {
		if _, err := os.Stat(draft.Path); err != nil {
			return QARegressionApprovedRecord{}, fmt.Errorf("QA regression test draft artifact is not readable: %w", err)
		}
	}
	record := QARegressionApprovedRecord{
		Schema:               QARegressionApprovedSchema,
		Name:                 draft.Name,
		Kind:                 draft.Kind,
		TraceID:              draft.TraceID,
		SuggestionName:       draft.SuggestionName,
		Category:             draft.Category,
		Fingerprint:          draft.Fingerprint,
		Status:               "approved_regression",
		ReviewedBy:           sanitizeRegressionText(request.ReviewedBy, 80),
		ReviewNote:           sanitizeRegressionText(request.ReviewNote, 240),
		ApprovedAt:           time.Now().UTC().Format(time.RFC3339Nano),
		TestDraftPath:        draft.Path,
		SuggestedTestFile:    draft.SuggestedTestFile,
		SuggestedTestName:    draft.SuggestedTestName,
		AutomaticTestWritten: false,
		Promotion:            draft.Promotion,
	}
	base := filepath.Clean(outputDir)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return QARegressionApprovedRecord{}, fmt.Errorf("create QA regression approval directory: %w", err)
	}
	info, err := os.Stat(base)
	if err != nil {
		return QARegressionApprovedRecord{}, fmt.Errorf("stat QA regression approval directory: %w", err)
	}
	if !info.IsDir() {
		return QARegressionApprovedRecord{}, fmt.Errorf("QA regression approval path is not a directory: %s", outputDir)
	}
	path, err := regressionArtifactPath(base, qaRegressionApprovalFilename(record))
	if err != nil {
		return QARegressionApprovedRecord{}, err
	}
	record.Path = path
	if err := writeJSONArtifact(path, record); err != nil {
		return QARegressionApprovedRecord{}, fmt.Errorf("write QA regression approval record: %w", err)
	}
	return record, nil
}

func ListQARegressionTestDrafts(outputDir string) ([]QARegressionTestDraftRecord, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return nil, fmt.Errorf("QA regression test draft directory is required")
	}
	if regressionArtifactPathLooksRemote(outputDir) {
		return nil, fmt.Errorf("QA regression test draft directory must be a local filesystem path: %s", outputDir)
	}
	entries, err := os.ReadDir(filepath.Clean(outputDir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list QA regression test drafts: %w", err)
	}
	var records []QARegressionTestDraftRecord
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		path, err := regressionArtifactPath(filepath.Clean(outputDir), entry.Name())
		if err != nil {
			return nil, err
		}
		var record QARegressionTestDraftRecord
		if err := readJSONArtifact(path, &record); err != nil {
			return nil, fmt.Errorf("decode QA regression test draft %s: %w", entry.Name(), err)
		}
		if err := validateQARegressionTestDraftRecord(record); err != nil {
			return nil, fmt.Errorf("invalid QA regression test draft %s: %w", entry.Name(), err)
		}
		record.Path = path
		records = append(records, record)
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].DraftedAt < records[j].DraftedAt
	})
	return records, nil
}

func ListQARegressionApprovedRecords(outputDir string) ([]QARegressionApprovedRecord, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return nil, fmt.Errorf("QA regression approval directory is required")
	}
	if regressionArtifactPathLooksRemote(outputDir) {
		return nil, fmt.Errorf("QA regression approval directory must be a local filesystem path: %s", outputDir)
	}
	entries, err := os.ReadDir(filepath.Clean(outputDir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list QA regression approval records: %w", err)
	}
	var records []QARegressionApprovedRecord
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		path, err := regressionArtifactPath(filepath.Clean(outputDir), entry.Name())
		if err != nil {
			return nil, err
		}
		var record QARegressionApprovedRecord
		if err := readJSONArtifact(path, &record); err != nil {
			return nil, fmt.Errorf("decode QA regression approval record %s: %w", entry.Name(), err)
		}
		if err := validateQARegressionApprovedRecord(record); err != nil {
			return nil, fmt.Errorf("invalid QA regression approval record %s: %w", entry.Name(), err)
		}
		record.Path = path
		records = append(records, record)
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].ApprovedAt < records[j].ApprovedAt
	})
	return records, nil
}

func ApplyQARegressionDrafts(review *QAReview, drafts []QARegressionTestDraftRecord, approvals []QARegressionApprovedRecord) {
	if review == nil {
		return
	}
	draftIndex := map[string]QARegressionTestDraftRecord{}
	for _, draft := range drafts {
		if err := validateQARegressionTestDraftRecord(draft); err != nil {
			continue
		}
		for _, key := range qaRegressionDraftKeys(draft.TraceID, draft.Fingerprint, draft.SuggestionName, draft.Name, draft.Kind) {
			draftIndex[key] = draft
		}
	}
	approvalIndex := map[string]QARegressionApprovedRecord{}
	for _, approval := range approvals {
		if err := validateQARegressionApprovedRecord(approval); err != nil {
			continue
		}
		for _, key := range qaRegressionDraftKeys(approval.TraceID, approval.Fingerprint, approval.SuggestionName, approval.Name, approval.Kind) {
			approvalIndex[key] = approval
		}
	}
	seenDrafts := map[string]bool{}
	seenApprovals := map[string]bool{}
	for i := range review.ReplayFailures {
		failure := &review.ReplayFailures[i]
		if draft, ok := qaRegressionDraftForFailure(*failure, draftIndex); ok {
			applyQARegressionDraftToFailure(failure, draft)
			countQARegressionDraft(&review.Summary, draft, seenDrafts)
		}
		if approval, ok := qaRegressionApprovalForFailure(*failure, approvalIndex); ok {
			applyQARegressionApprovalToFailure(failure, approval)
			countQARegressionApproval(&review.Summary, approval, seenApprovals)
		}
		for j := range failure.RegressionSuggestions {
			suggestion := &failure.RegressionSuggestions[j]
			if draft, ok := qaRegressionDraftForSuggestion(*failure, *suggestion, draftIndex); ok {
				applyQARegressionDraftToSuggestion(suggestion, draft)
				countQARegressionDraft(&review.Summary, draft, seenDrafts)
			}
			if approval, ok := qaRegressionApprovalForSuggestion(*failure, *suggestion, approvalIndex); ok {
				applyQARegressionApprovalToSuggestion(suggestion, approval)
				countQARegressionApproval(&review.Summary, approval, seenApprovals)
			}
		}
	}
}

func FormatQARegressionTestDraftYAMLish(record QARegressionTestDraftRecord, trace replay.Trace, classification RouteFailureClassification) string {
	var out strings.Builder
	fmt.Fprintf(&out, "schema: %s\n", yamlString(QARegressionTestDraftSchema))
	fmt.Fprintf(&out, "status: %s\n", yamlString(record.Status))
	fmt.Fprintf(&out, "automatic_test_written: %t\n", record.AutomaticTestWritten)
	fmt.Fprintf(&out, "name: %s\n", yamlString(record.Name))
	fmt.Fprintf(&out, "kind: %s\n", yamlString(record.Kind))
	fmt.Fprintf(&out, "source_trace_id: %s\n", yamlString(record.TraceID))
	fmt.Fprintf(&out, "category: %s\n", yamlString(record.Category))
	fmt.Fprintf(&out, "fingerprint: %s\n", yamlString(record.Fingerprint))
	fmt.Fprintf(&out, "prompt: %s\n", yamlString(sanitizeRegressionText(trace.UserRequest, 240)))
	fmt.Fprintf(&out, "reason: %s\n", yamlString(sanitizeRegressionText(classification.Reason, 240)))
	fmt.Fprintf(&out, "suggested_test_file: %s\n", yamlString(record.SuggestedTestFile))
	fmt.Fprintf(&out, "suggested_test_name: %s\n", yamlString(record.SuggestedTestName))
	out.WriteString("expected:\n")
	fmt.Fprintf(&out, "  route_under_test: %s\n", yamlString(record.Promotion.RouteUnderTest))
	fmt.Fprintf(&out, "  continuation_mode: %s\n", yamlString(record.Promotion.ContinuationMode))
	fmt.Fprintf(&out, "  source: %s\n", yamlString(record.Promotion.ExpectedSource))
	fmt.Fprintf(&out, "  tool_lane: %s\n", yamlString(record.Promotion.ExpectedToolLane))
	fmt.Fprintf(&out, "  outcome: %s\n", yamlString(record.Promotion.ExpectedOutcome))
	out.WriteString("go_test_draft: |\n")
	fmt.Fprintf(&out, "  func %s(t *testing.T) {\n", sanitizeRegressionText(record.SuggestedTestName, 120))
	fmt.Fprintf(&out, "    // Draft only. Add a deterministic fixture for category %q.\n", sanitizeRegressionText(record.Category, 80))
	fmt.Fprintf(&out, "    // Assert route=%q, source=%q, lane=%q, outcome=%q.\n", sanitizeRegressionText(record.Promotion.RouteUnderTest, 80), sanitizeRegressionText(record.Promotion.ExpectedSource, 80), sanitizeRegressionText(record.Promotion.ExpectedToolLane, 80), sanitizeRegressionText(record.Promotion.ExpectedOutcome, 120))
	fmt.Fprintf(&out, "    t.Fatal(\"draft regression is not implemented yet\")\n")
	fmt.Fprintf(&out, "  }\n")
	return strings.TrimRight(out.String(), "\n")
}

func ValidateQARegressionReviewFlow(reviews []QARegressionReviewRecord, drafts []QARegressionTestDraftRecord, approvals []QARegressionApprovedRecord) error {
	reviewIndex := map[string]QARegressionReviewRecord{}
	for _, review := range reviews {
		if err := validateQARegressionReviewRecord(review); err != nil {
			return err
		}
		for _, key := range qaRegressionReviewRecordKeys(review) {
			reviewIndex[key] = review
		}
	}
	draftIndex := map[string]QARegressionTestDraftRecord{}
	for _, draft := range drafts {
		if err := validateQARegressionTestDraftRecord(draft); err != nil {
			return err
		}
		if _, ok := approvedReviewForDraft(draft, reviewIndex); !ok {
			return fmt.Errorf("test draft %s has no approved QA review", draft.Name)
		}
		for _, key := range qaRegressionDraftKeys(draft.TraceID, draft.Fingerprint, draft.SuggestionName, draft.Name, draft.Kind) {
			draftIndex[key] = draft
		}
	}
	for _, approval := range approvals {
		if err := validateQARegressionApprovedRecord(approval); err != nil {
			return err
		}
		if _, ok := qaRegressionDraftForApproval(approval, draftIndex); !ok {
			return fmt.Errorf("approved regression %s has no test draft", approval.Name)
		}
	}
	return nil
}

func qaRegressionSuggestionContext(traces replay.Store, latestEval *QAEvalSummary, traceID string, suggestionName string) (replay.Trace, RouteFailureClassification, QARegressionSuggestion, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return replay.Trace{}, RouteFailureClassification{}, QARegressionSuggestion{}, fmt.Errorf("replay trace id is required")
	}
	trace, err := traces.Load(traceID)
	if err != nil {
		return replay.Trace{}, RouteFailureClassification{}, QARegressionSuggestion{}, err
	}
	trace = trace.Normalize()
	classification := ApplyRouteGovernanceCoverage(trace, ClassifyRouteFailure(trace), latestEval)
	suggestions := regressionSuggestionsForTrace(trace, classification)
	if len(suggestions) == 0 {
		return replay.Trace{}, RouteFailureClassification{}, QARegressionSuggestion{}, fmt.Errorf("no regression suggestion is available for replay trace %s", traceID)
	}
	suggestion := chooseQARegressionSuggestion(suggestions, suggestionName)
	if strings.TrimSpace(suggestion.Name) == "" {
		return replay.Trace{}, RouteFailureClassification{}, QARegressionSuggestion{}, fmt.Errorf("regression suggestion is missing a name")
	}
	return trace, classification, suggestion, nil
}

func approvedReviewForRegressionSuggestion(reviews []QARegressionReviewRecord, trace replay.Trace, artifact QARegressionPromotionArtifact, suggestion QARegressionSuggestion) (QARegressionReviewRecord, bool) {
	index := map[string]QARegressionReviewRecord{}
	for _, review := range reviews {
		if err := validateQARegressionReviewRecord(review); err != nil {
			continue
		}
		for _, key := range qaRegressionReviewRecordKeys(review) {
			index[key] = review
		}
	}
	failure := QAReplayFailure{TraceID: trace.ID, PromotionFingerprint: artifact.Fingerprint}
	suggestion.PromotionFingerprint = firstNonEmpty(suggestion.PromotionFingerprint, artifact.Fingerprint)
	record, ok := qaRegressionReviewForSuggestion(failure, suggestion, index)
	if !ok {
		record, ok = qaRegressionReviewForFailure(failure, index)
	}
	if !ok || record.Status != QARegressionReviewApproved {
		return QARegressionReviewRecord{}, false
	}
	return record, true
}

func approvedReviewForDraft(draft QARegressionTestDraftRecord, reviews map[string]QARegressionReviewRecord) (QARegressionReviewRecord, bool) {
	for _, key := range qaRegressionDraftKeys(draft.TraceID, draft.Fingerprint, draft.SuggestionName, draft.Name, draft.Kind) {
		if review, ok := reviews[key]; ok && review.Status == QARegressionReviewApproved {
			return review, true
		}
	}
	return QARegressionReviewRecord{}, false
}

func qaRegressionTestDraftRecord(trace replay.Trace, classification RouteFailureClassification, suggestion QARegressionSuggestion, artifact QARegressionPromotionArtifact, request QARegressionTestDraftRequest) QARegressionTestDraftRecord {
	promotion := artifact.Promotion
	name := firstNonEmpty(artifact.Name, suggestion.Name, promotion.SuggestedTestName, regressionCaseName("qa_review_"+classification.Category, trace.UserRequest, trace.ID))
	testName := sanitizeGoTestName(firstNonEmpty(promotion.SuggestedTestName, name))
	return QARegressionTestDraftRecord{
		Schema:               QARegressionTestDraftSchema,
		Name:                 sanitizeRegressionName(name),
		Kind:                 artifact.Kind,
		TraceID:              artifact.TraceID,
		SuggestionName:       sanitizeRegressionName(firstNonEmpty(suggestion.Name, suggestion.Kind)),
		Category:             artifact.Category,
		Fingerprint:          artifact.Fingerprint,
		Status:               "drafted",
		ReviewedBy:           sanitizeRegressionText(request.ReviewedBy, 80),
		ReviewNote:           sanitizeRegressionText(request.ReviewNote, 240),
		DraftedAt:            time.Now().UTC().Format(time.RFC3339Nano),
		SuggestedTestFile:    suggestedQARegressionTestFile(classification.Category, promotion.RouteUnderTest),
		SuggestedTestName:    testName,
		AutomaticTestWritten: false,
		Promotion:            promotion,
	}
}

func suggestedQARegressionTestFile(category string, route string) string {
	switch category {
	case RouteFailureWrongRoute, RouteFailureWrongSource, RouteFailureStaleAnswer, RouteFailureBadClarification:
		return "internal/routing/routing_test.go"
	case RouteFailureWrongToolLane, RouteFailureMissingApproval, RouteFailureMissingEvidence, RouteFailureEvidenceContract, RouteFailureProviderConfigIssue, RouteFailurePermissionMissing, RouteFailureMissingCapability:
		return "internal/agent/routing_test.go"
	default:
		if strings.TrimSpace(route) != "" {
			return "internal/routing/routing_test.go"
		}
		return "internal/learning/route_failure_classifier_test.go"
	}
}

func sanitizeGoTestName(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "Test") && validGoIdentifier(name) {
		return name
	}
	name = sanitizeRegressionName(name)
	if name == "" {
		name = "qa_review_regression"
	}
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == ' '
	})
	var out strings.Builder
	out.WriteString("Test")
	for _, part := range parts {
		if part == "" {
			continue
		}
		out.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			out.WriteString(part[1:])
		}
	}
	if out.String() == "Test" {
		return "TestQAReviewRegression"
	}
	return out.String()
}

func validGoIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_' {
				continue
			}
			return false
		}
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func qaRegressionTestDraftFilename(record QARegressionTestDraftRecord) string {
	name := sanitizeRegressionName(firstNonEmpty(record.Name, record.SuggestionName, "qa_review_regression_test_draft"))
	fingerprint := sanitizeRegressionName(strings.TrimPrefix(record.Fingerprint, "route_governance:"))
	if fingerprint != "" {
		return fmt.Sprintf("draft_%s_%s.json", name, fingerprint)
	}
	return fmt.Sprintf("draft_%s.json", name)
}

func qaRegressionApprovalFilename(record QARegressionApprovedRecord) string {
	name := sanitizeRegressionName(firstNonEmpty(record.Name, record.SuggestionName, "qa_review_approved_regression"))
	fingerprint := sanitizeRegressionName(strings.TrimPrefix(record.Fingerprint, "route_governance:"))
	if fingerprint != "" {
		return fmt.Sprintf("approved_%s_%s.json", name, fingerprint)
	}
	return fmt.Sprintf("approved_%s.json", name)
}

func validateQARegressionTestDraftRecord(record QARegressionTestDraftRecord) error {
	if record.Schema != QARegressionTestDraftSchema {
		return fmt.Errorf("schema = %q, want %q", record.Schema, QARegressionTestDraftSchema)
	}
	if strings.TrimSpace(record.TraceID) == "" || strings.TrimSpace(record.Name) == "" {
		return fmt.Errorf("test draft trace id and name are required")
	}
	if record.Status != "drafted" {
		return fmt.Errorf("test draft status = %q, want drafted", record.Status)
	}
	if record.AutomaticTestWritten {
		return fmt.Errorf("test draft must not mark automatic test writing as true")
	}
	return nil
}

func validateQARegressionApprovedRecord(record QARegressionApprovedRecord) error {
	if record.Schema != QARegressionApprovedSchema {
		return fmt.Errorf("schema = %q, want %q", record.Schema, QARegressionApprovedSchema)
	}
	if strings.TrimSpace(record.TraceID) == "" || strings.TrimSpace(record.Name) == "" {
		return fmt.Errorf("approved regression trace id and name are required")
	}
	if record.Status != "approved_regression" {
		return fmt.Errorf("approved regression status = %q, want approved_regression", record.Status)
	}
	if record.AutomaticTestWritten {
		return fmt.Errorf("approved regression must not mark automatic test writing as true")
	}
	return nil
}

func findQARegressionTestDraft(drafts []QARegressionTestDraftRecord, request QARegressionApprovalRequest) (QARegressionTestDraftRecord, bool) {
	draftName := sanitizeRegressionName(request.DraftName)
	for _, draft := range drafts {
		if err := validateQARegressionTestDraftRecord(draft); err != nil {
			continue
		}
		if draftName != "" && (draft.Name == draftName || draft.SuggestionName == draftName || draft.Kind == draftName) {
			return draft, true
		}
		if strings.TrimSpace(request.TraceID) != "" && strings.TrimSpace(request.TraceID) == draft.TraceID {
			if request.SuggestionName == "" || draft.SuggestionName == sanitizeRegressionName(request.SuggestionName) || draft.Name == sanitizeRegressionName(request.SuggestionName) || draft.Kind == request.SuggestionName {
				return draft, true
			}
		}
	}
	return QARegressionTestDraftRecord{}, false
}

func qaRegressionDraftForFailure(failure QAReplayFailure, drafts map[string]QARegressionTestDraftRecord) (QARegressionTestDraftRecord, bool) {
	for _, key := range qaRegressionDraftKeys(failure.TraceID, failure.PromotionFingerprint, "", "", "") {
		if draft, ok := drafts[key]; ok {
			return draft, true
		}
	}
	return QARegressionTestDraftRecord{}, false
}

func qaRegressionDraftForSuggestion(failure QAReplayFailure, suggestion QARegressionSuggestion, drafts map[string]QARegressionTestDraftRecord) (QARegressionTestDraftRecord, bool) {
	for _, key := range qaRegressionDraftKeys(failure.TraceID, firstNonEmpty(suggestion.PromotionFingerprint, failure.PromotionFingerprint), suggestion.Name, suggestion.Name, suggestion.Kind) {
		if draft, ok := drafts[key]; ok {
			return draft, true
		}
	}
	return QARegressionTestDraftRecord{}, false
}

func qaRegressionApprovalForFailure(failure QAReplayFailure, approvals map[string]QARegressionApprovedRecord) (QARegressionApprovedRecord, bool) {
	for _, key := range qaRegressionDraftKeys(failure.TraceID, failure.PromotionFingerprint, "", "", "") {
		if approval, ok := approvals[key]; ok {
			return approval, true
		}
	}
	return QARegressionApprovedRecord{}, false
}

func qaRegressionApprovalForSuggestion(failure QAReplayFailure, suggestion QARegressionSuggestion, approvals map[string]QARegressionApprovedRecord) (QARegressionApprovedRecord, bool) {
	for _, key := range qaRegressionDraftKeys(failure.TraceID, firstNonEmpty(suggestion.PromotionFingerprint, failure.PromotionFingerprint), suggestion.Name, suggestion.Name, suggestion.Kind) {
		if approval, ok := approvals[key]; ok {
			return approval, true
		}
	}
	return QARegressionApprovedRecord{}, false
}

func qaRegressionDraftForApproval(approval QARegressionApprovedRecord, drafts map[string]QARegressionTestDraftRecord) (QARegressionTestDraftRecord, bool) {
	for _, key := range qaRegressionDraftKeys(approval.TraceID, approval.Fingerprint, approval.SuggestionName, approval.Name, approval.Kind) {
		if draft, ok := drafts[key]; ok {
			return draft, true
		}
	}
	return QARegressionTestDraftRecord{}, false
}

func qaRegressionDraftKeys(traceID string, fingerprint string, suggestionName string, name string, kind string) []string {
	var keys []string
	add := func(parts ...string) {
		key := qaRegressionReviewKey(parts...)
		if key != "" {
			keys = append(keys, key)
		}
	}
	add("fingerprint", fingerprint)
	add("fingerprint", fingerprint, "suggestion", suggestionName)
	add("fingerprint", fingerprint, "name", name)
	add("fingerprint", fingerprint, "kind", kind)
	add("trace", traceID)
	add("trace", traceID, "suggestion", suggestionName)
	add("trace", traceID, "name", name)
	add("trace", traceID, "kind", kind)
	return uniqueStringsStable(keys)
}

func applyQARegressionDraftToFailure(failure *QAReplayFailure, draft QARegressionTestDraftRecord) {
	failure.TestDraftStatus = draft.Status
	failure.TestDraftPath = draft.Path
	failure.CoverageSources = appendUniqueStrings(failure.CoverageSources, "qa_review:test_draft")
}

func applyQARegressionDraftToSuggestion(suggestion *QARegressionSuggestion, draft QARegressionTestDraftRecord) {
	suggestion.TestDraftStatus = draft.Status
	suggestion.TestDraftPath = draft.Path
	suggestion.CoverageSources = appendUniqueStrings(suggestion.CoverageSources, "qa_review:test_draft")
}

func applyQARegressionApprovalToFailure(failure *QAReplayFailure, approval QARegressionApprovedRecord) {
	failure.ApprovedRegression = approval.Status
	failure.ApprovedRegressionAt = approval.ApprovedAt
	failure.ApprovedRegressionPath = approval.Path
	failure.CoverageStatus = "approved_regression"
	failure.CoverageSources = appendUniqueStrings(failure.CoverageSources, "qa_review:approved_regression")
}

func applyQARegressionApprovalToSuggestion(suggestion *QARegressionSuggestion, approval QARegressionApprovedRecord) {
	suggestion.ApprovedRegression = approval.Status
	suggestion.ApprovedRegressionAt = approval.ApprovedAt
	suggestion.ApprovedRegressionPath = approval.Path
	suggestion.CoverageStatus = "approved_regression"
	suggestion.CoverageSources = appendUniqueStrings(suggestion.CoverageSources, "qa_review:approved_regression")
}

func countQARegressionDraft(summary *QAReviewSummary, draft QARegressionTestDraftRecord, seen map[string]bool) {
	key := firstNonEmpty(draft.Fingerprint, draft.TraceID, draft.Name)
	if key == "" || seen[key] {
		return
	}
	seen[key] = true
	summary.TestDrafts++
}

func countQARegressionApproval(summary *QAReviewSummary, approval QARegressionApprovedRecord, seen map[string]bool) {
	key := firstNonEmpty(approval.Fingerprint, approval.TraceID, approval.Name)
	if key == "" || seen[key] {
		return
	}
	seen[key] = true
	summary.ApprovedRegressions++
}

func writeJSONArtifact(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func readJSONArtifact(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}
