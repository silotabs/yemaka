package learning

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const QARegressionSourceWriteSchema = "yemaka.qa_review_source_test_write.v1"

type QARegressionSourceWriteRequest struct {
	TraceID         string `json:"traceId"`
	SuggestionName  string `json:"suggestionName,omitempty"`
	SourcePatchName string `json:"sourcePatchName,omitempty"`
	Content         string `json:"content"`
	Approved        bool   `json:"approved"`
	ReviewedBy      string `json:"reviewedBy,omitempty"`
	ReviewNote      string `json:"reviewNote,omitempty"`
}

type QARegressionSourceWriteRecord struct {
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
	WrittenAt            string                   `json:"writtenAt"`
	Path                 string                   `json:"path,omitempty"`
	ApplyPlanPath        string                   `json:"applyPlanPath,omitempty"`
	SourcePatchPath      string                   `json:"sourcePatchPath,omitempty"`
	SuggestedTestFile    string                   `json:"suggestedTestFile"`
	SuggestedTestName    string                   `json:"suggestedTestName"`
	AutomaticTestWritten bool                     `json:"automaticTestWritten"`
	SourceTestWritten    bool                     `json:"sourceTestWritten"`
	RequiresManualApply  bool                     `json:"requiresManualApply"`
	SnapshotID           string                   `json:"snapshotId,omitempty"`
	VerificationStatus   string                   `json:"verificationStatus,omitempty"`
	Changed              bool                     `json:"changed"`
	TargetBytes          int                      `json:"targetBytes"`
	Diff                 string                   `json:"diff,omitempty"`
	Preview              string                   `json:"preview,omitempty"`
	Promotion            RouteRegressionPromotion `json:"promotion"`
}

type QARegressionSourceWriteOutcome struct {
	SnapshotID         string
	VerificationStatus string
	Changed            bool
	TargetBytes        int
	Diff               string
}

func ValidateQARegressionSourceWrite(plans []QARegressionSourcePatchApplyPlanRecord, request QARegressionSourceWriteRequest) (QARegressionSourcePatchApplyPlanRecord, error) {
	plan, ok := findQARegressionSourcePatchApplyPlan(plans, request)
	if !ok {
		return QARegressionSourcePatchApplyPlanRecord{}, fmt.Errorf("approved source patch apply plan must exist before source tests can be written")
	}
	if err := validateQARegressionSourcePatchApplyPlanRecord(plan); err != nil {
		return QARegressionSourcePatchApplyPlanRecord{}, err
	}
	if err := validateSuggestedSourceTestFile(plan.SuggestedTestFile); err != nil {
		return QARegressionSourcePatchApplyPlanRecord{}, err
	}
	if err := validateQARegressionSourceWriteContent(plan, request.Content); err != nil {
		return QARegressionSourcePatchApplyPlanRecord{}, err
	}
	return plan, nil
}

func RecordQARegressionSourceWrite(outputDir string, plan QARegressionSourcePatchApplyPlanRecord, request QARegressionSourceWriteRequest, outcome QARegressionSourceWriteOutcome) (QARegressionSourceWriteRecord, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return QARegressionSourceWriteRecord{}, fmt.Errorf("QA regression source write directory is required")
	}
	if regressionArtifactPathLooksRemote(outputDir) {
		return QARegressionSourceWriteRecord{}, fmt.Errorf("QA regression source write directory must be a local filesystem path: %s", outputDir)
	}
	if !request.Approved {
		return QARegressionSourceWriteRecord{}, fmt.Errorf("source test write requires explicit approval")
	}
	if err := validateQARegressionSourcePatchApplyPlanRecord(plan); err != nil {
		return QARegressionSourceWriteRecord{}, err
	}
	if err := validateQARegressionSourceWriteContent(plan, request.Content); err != nil {
		return QARegressionSourceWriteRecord{}, err
	}
	record := qaRegressionSourceWriteRecord(plan, request, outcome)
	record.Preview = summarizeText(FormatQARegressionSourceWriteYAMLish(record), 1800)

	base := filepath.Clean(outputDir)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return QARegressionSourceWriteRecord{}, fmt.Errorf("create QA regression source write directory: %w", err)
	}
	info, err := os.Stat(base)
	if err != nil {
		return QARegressionSourceWriteRecord{}, fmt.Errorf("stat QA regression source write directory: %w", err)
	}
	if !info.IsDir() {
		return QARegressionSourceWriteRecord{}, fmt.Errorf("QA regression source write path is not a directory: %s", outputDir)
	}
	path, err := regressionArtifactPath(base, qaRegressionSourceWriteFilename(record))
	if err != nil {
		return QARegressionSourceWriteRecord{}, err
	}
	record.Path = path
	if err := writeJSONArtifact(path, record); err != nil {
		return QARegressionSourceWriteRecord{}, fmt.Errorf("write QA regression source write record: %w", err)
	}
	return record, nil
}

func ListQARegressionSourceWrites(outputDir string) ([]QARegressionSourceWriteRecord, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return nil, fmt.Errorf("QA regression source write directory is required")
	}
	if regressionArtifactPathLooksRemote(outputDir) {
		return nil, fmt.Errorf("QA regression source write directory must be a local filesystem path: %s", outputDir)
	}
	entries, err := os.ReadDir(filepath.Clean(outputDir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list QA regression source writes: %w", err)
	}
	var records []QARegressionSourceWriteRecord
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		path, err := regressionArtifactPath(filepath.Clean(outputDir), entry.Name())
		if err != nil {
			return nil, err
		}
		var record QARegressionSourceWriteRecord
		if err := readJSONArtifact(path, &record); err != nil {
			return nil, fmt.Errorf("decode QA regression source write %s: %w", entry.Name(), err)
		}
		if err := validateQARegressionSourceWriteRecord(record); err != nil {
			return nil, fmt.Errorf("invalid QA regression source write %s: %w", entry.Name(), err)
		}
		record.Path = path
		records = append(records, record)
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].WrittenAt < records[j].WrittenAt
	})
	return records, nil
}

func ApplyQARegressionSourceWrites(review *QAReview, writes []QARegressionSourceWriteRecord) {
	if review == nil {
		return
	}
	index := map[string]QARegressionSourceWriteRecord{}
	for _, write := range writes {
		if err := validateQARegressionSourceWriteRecord(write); err != nil {
			continue
		}
		for _, key := range qaRegressionDraftKeys(write.TraceID, write.Fingerprint, write.SuggestionName, write.Name, write.Kind) {
			index[key] = write
		}
	}
	seen := map[string]bool{}
	for i := range review.ReplayFailures {
		failure := &review.ReplayFailures[i]
		if write, ok := qaRegressionSourceWriteForFailure(*failure, index); ok {
			applyQARegressionSourceWriteToFailure(failure, write)
			countQARegressionSourceWrite(&review.Summary, write, seen)
		}
		for j := range failure.RegressionSuggestions {
			suggestion := &failure.RegressionSuggestions[j]
			if write, ok := qaRegressionSourceWriteForSuggestion(*failure, *suggestion, index); ok {
				applyQARegressionSourceWriteToSuggestion(suggestion, write)
				countQARegressionSourceWrite(&review.Summary, write, seen)
			}
		}
	}
}

func FormatQARegressionSourceWriteYAMLish(record QARegressionSourceWriteRecord) string {
	var out strings.Builder
	fmt.Fprintf(&out, "schema: %s\n", yamlString(QARegressionSourceWriteSchema))
	fmt.Fprintf(&out, "status: %s\n", yamlString(record.Status))
	fmt.Fprintf(&out, "automatic_test_written: %t\n", record.AutomaticTestWritten)
	fmt.Fprintf(&out, "source_test_written: %t\n", record.SourceTestWritten)
	fmt.Fprintf(&out, "requires_manual_apply: %t\n", record.RequiresManualApply)
	fmt.Fprintf(&out, "name: %s\n", yamlString(record.Name))
	fmt.Fprintf(&out, "kind: %s\n", yamlString(record.Kind))
	fmt.Fprintf(&out, "source_trace_id: %s\n", yamlString(record.TraceID))
	fmt.Fprintf(&out, "category: %s\n", yamlString(record.Category))
	fmt.Fprintf(&out, "fingerprint: %s\n", yamlString(record.Fingerprint))
	fmt.Fprintf(&out, "apply_plan_path: %s\n", yamlString(record.ApplyPlanPath))
	fmt.Fprintf(&out, "source_patch_path: %s\n", yamlString(record.SourcePatchPath))
	fmt.Fprintf(&out, "suggested_test_file: %s\n", yamlString(record.SuggestedTestFile))
	fmt.Fprintf(&out, "suggested_test_name: %s\n", yamlString(record.SuggestedTestName))
	fmt.Fprintf(&out, "snapshot_id: %s\n", yamlString(record.SnapshotID))
	fmt.Fprintf(&out, "verification_status: %s\n", yamlString(record.VerificationStatus))
	return strings.TrimRight(out.String(), "\n")
}

func validateQARegressionSourceWriteRecord(record QARegressionSourceWriteRecord) error {
	if record.Schema != QARegressionSourceWriteSchema {
		return fmt.Errorf("schema = %q, want %q", record.Schema, QARegressionSourceWriteSchema)
	}
	if strings.TrimSpace(record.TraceID) == "" || strings.TrimSpace(record.Name) == "" {
		return fmt.Errorf("source write trace id and name are required")
	}
	if record.Status != "source_test_written" {
		return fmt.Errorf("source write status = %q, want source_test_written", record.Status)
	}
	if record.AutomaticTestWritten {
		return fmt.Errorf("source write must not mark automatic test writing as true")
	}
	if !record.SourceTestWritten {
		return fmt.Errorf("source write must mark source test writing as true")
	}
	return validateSuggestedSourceTestFile(record.SuggestedTestFile)
}

func validateQARegressionSourceWriteContent(plan QARegressionSourcePatchApplyPlanRecord, content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("source test content is required")
	}
	testName := strings.TrimSpace(plan.SuggestedTestName)
	if testName == "" {
		return fmt.Errorf("suggested test name is required")
	}
	if !strings.Contains(content, "func "+testName+"(") {
		return fmt.Errorf("source test content must include %s", testName)
	}
	lower := strings.ToLower(content)
	blocked := []string{
		"source patch draft only",
		"draft regression is not implemented yet",
		"qa_review source patch draft",
		"qareview source patch template",
		"todo: minimal public fixture prompt",
		"todo_expected",
		"t.skip(",
		"t.fatal(\"draft",
		"t.fatalf(\"draft",
	}
	for _, phrase := range blocked {
		if strings.Contains(lower, phrase) {
			return fmt.Errorf("source test content still contains scaffold marker: %s", phrase)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), plan.SuggestedTestFile, content, parser.ParseComments); err != nil {
		return fmt.Errorf("source test content must be parseable Go: %w", err)
	}
	return nil
}

func findQARegressionSourcePatchApplyPlan(plans []QARegressionSourcePatchApplyPlanRecord, request QARegressionSourceWriteRequest) (QARegressionSourcePatchApplyPlanRecord, bool) {
	index := map[string]QARegressionSourcePatchApplyPlanRecord{}
	for _, plan := range plans {
		if err := validateQARegressionSourcePatchApplyPlanRecord(plan); err != nil {
			continue
		}
		for _, key := range qaRegressionDraftKeys(plan.TraceID, plan.Fingerprint, plan.SuggestionName, plan.Name, plan.Kind) {
			index[key] = plan
		}
	}
	for _, key := range qaRegressionDraftKeys(
		request.TraceID,
		"",
		sanitizeRegressionName(request.SuggestionName),
		sanitizeRegressionName(request.SourcePatchName),
		request.SuggestionName,
	) {
		if plan, ok := index[key]; ok {
			return plan, true
		}
	}
	return QARegressionSourcePatchApplyPlanRecord{}, false
}

func qaRegressionSourceWriteRecord(plan QARegressionSourcePatchApplyPlanRecord, request QARegressionSourceWriteRequest, outcome QARegressionSourceWriteOutcome) QARegressionSourceWriteRecord {
	return QARegressionSourceWriteRecord{
		Schema:               QARegressionSourceWriteSchema,
		Name:                 sanitizeRegressionName(firstNonEmpty(plan.Name, plan.SuggestionName, plan.SuggestedTestName, "qa_review_source_test_write")),
		Kind:                 plan.Kind,
		TraceID:              plan.TraceID,
		SuggestionName:       plan.SuggestionName,
		Category:             plan.Category,
		Fingerprint:          plan.Fingerprint,
		Status:               "source_test_written",
		ReviewedBy:           sanitizeRegressionText(request.ReviewedBy, 80),
		ReviewNote:           sanitizeRegressionText(request.ReviewNote, 240),
		WrittenAt:            time.Now().UTC().Format(time.RFC3339Nano),
		ApplyPlanPath:        plan.Path,
		SourcePatchPath:      plan.SourcePatchPath,
		SuggestedTestFile:    plan.SuggestedTestFile,
		SuggestedTestName:    plan.SuggestedTestName,
		AutomaticTestWritten: false,
		SourceTestWritten:    true,
		RequiresManualApply:  false,
		SnapshotID:           outcome.SnapshotID,
		VerificationStatus:   outcome.VerificationStatus,
		Changed:              outcome.Changed,
		TargetBytes:          outcome.TargetBytes,
		Diff:                 outcome.Diff,
		Promotion:            plan.Promotion,
	}
}

func qaRegressionSourceWriteFilename(record QARegressionSourceWriteRecord) string {
	name := sanitizeRegressionName(firstNonEmpty(record.Name, record.SuggestionName, "qa_review_source_test_write"))
	fingerprint := sanitizeRegressionName(strings.TrimPrefix(record.Fingerprint, "route_governance:"))
	if fingerprint != "" {
		return fmt.Sprintf("source_write_%s_%s.json", name, fingerprint)
	}
	return fmt.Sprintf("source_write_%s.json", name)
}

func qaRegressionSourceWriteForFailure(failure QAReplayFailure, writes map[string]QARegressionSourceWriteRecord) (QARegressionSourceWriteRecord, bool) {
	for _, key := range qaRegressionDraftKeys(failure.TraceID, failure.PromotionFingerprint, "", "", "") {
		if write, ok := writes[key]; ok {
			return write, true
		}
	}
	return QARegressionSourceWriteRecord{}, false
}

func qaRegressionSourceWriteForSuggestion(failure QAReplayFailure, suggestion QARegressionSuggestion, writes map[string]QARegressionSourceWriteRecord) (QARegressionSourceWriteRecord, bool) {
	for _, key := range qaRegressionDraftKeys(failure.TraceID, firstNonEmpty(suggestion.PromotionFingerprint, failure.PromotionFingerprint), suggestion.Name, suggestion.Name, suggestion.Kind) {
		if write, ok := writes[key]; ok {
			return write, true
		}
	}
	return QARegressionSourceWriteRecord{}, false
}

func applyQARegressionSourceWriteToFailure(failure *QAReplayFailure, write QARegressionSourceWriteRecord) {
	failure.SourceTestWriteStatus = write.Status
	failure.SourceTestWritePath = write.Path
	failure.SourceTestWriteSnapshotID = write.SnapshotID
	failure.CoverageStatus = "source_test_written"
	failure.CoverageSources = appendUniqueStrings(failure.CoverageSources, "qa_review:source_test_write")
}

func applyQARegressionSourceWriteToSuggestion(suggestion *QARegressionSuggestion, write QARegressionSourceWriteRecord) {
	suggestion.SourceTestWriteStatus = write.Status
	suggestion.SourceTestWritePath = write.Path
	suggestion.SourceTestWriteSnapshotID = write.SnapshotID
	suggestion.CoverageStatus = "source_test_written"
	suggestion.CoverageSources = appendUniqueStrings(suggestion.CoverageSources, "qa_review:source_test_write")
}

func countQARegressionSourceWrite(summary *QAReviewSummary, write QARegressionSourceWriteRecord, seen map[string]bool) {
	key := firstNonEmpty(write.Fingerprint, write.TraceID, write.Name)
	if key == "" || seen[key] {
		return
	}
	seen[key] = true
	summary.SourceTestWrites++
}
