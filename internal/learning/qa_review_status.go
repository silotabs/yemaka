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

const QARegressionReviewSchema = "yemaka.qa_review_regression_review.v1"

const (
	QARegressionReviewApproved  = "approved"
	QARegressionReviewDismissed = "dismissed"
	QARegressionReviewCovered   = "covered"
)

type QARegressionReviewRequest struct {
	TraceID        string `json:"traceId"`
	SuggestionName string `json:"suggestionName,omitempty"`
	Status         string `json:"status"`
	ReviewedBy     string `json:"reviewedBy,omitempty"`
	ReviewNote     string `json:"reviewNote,omitempty"`
}

type QARegressionReviewRecord struct {
	Schema              string                   `json:"schema"`
	Name                string                   `json:"name"`
	Kind                string                   `json:"kind"`
	TraceID             string                   `json:"traceId"`
	SuggestionName      string                   `json:"suggestionName,omitempty"`
	Category            string                   `json:"category,omitempty"`
	Fingerprint         string                   `json:"fingerprint,omitempty"`
	Status              string                   `json:"status"`
	ReviewedBy          string                   `json:"reviewedBy,omitempty"`
	ReviewNote          string                   `json:"reviewNote,omitempty"`
	ReviewedAt          string                   `json:"reviewedAt"`
	CoveredByEvalOrTest bool                     `json:"coveredByEvalOrTest"`
	Promotion           RouteRegressionPromotion `json:"promotion,omitempty"`
	Path                string                   `json:"path,omitempty"`
}

func ReviewQARegressionSuggestion(outputDir string, traces replay.Store, latestEval *QAEvalSummary, request QARegressionReviewRequest) (QARegressionReviewRecord, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return QARegressionReviewRecord{}, fmt.Errorf("QA regression review directory is required")
	}
	if regressionArtifactPathLooksRemote(outputDir) {
		return QARegressionReviewRecord{}, fmt.Errorf("QA regression review directory must be a local filesystem path: %s", outputDir)
	}
	traceID := strings.TrimSpace(request.TraceID)
	if traceID == "" {
		return QARegressionReviewRecord{}, fmt.Errorf("replay trace id is required")
	}
	status := normalizeQARegressionReviewStatus(request.Status)
	if status == "" {
		return QARegressionReviewRecord{}, fmt.Errorf("QA regression review status must be approved, dismissed, or covered")
	}
	trace, err := traces.Load(traceID)
	if err != nil {
		return QARegressionReviewRecord{}, err
	}
	trace = trace.Normalize()
	classification := ApplyRouteGovernanceCoverage(trace, ClassifyRouteFailure(trace), latestEval)
	suggestions := regressionSuggestionsForTrace(trace, classification)
	if len(suggestions) == 0 {
		return QARegressionReviewRecord{}, fmt.Errorf("no regression suggestion is available for replay trace %s", traceID)
	}
	suggestion := chooseQARegressionSuggestion(suggestions, request.SuggestionName)
	if strings.TrimSpace(suggestion.Name) == "" {
		return QARegressionReviewRecord{}, fmt.Errorf("regression suggestion is missing a name")
	}
	artifact := qaRegressionPromotionArtifact(trace, classification, suggestion)
	record := QARegressionReviewRecord{
		Schema:              QARegressionReviewSchema,
		Name:                artifact.Name,
		Kind:                artifact.Kind,
		TraceID:             artifact.TraceID,
		SuggestionName:      sanitizeRegressionName(firstNonEmpty(suggestion.Name, suggestion.Kind)),
		Category:            artifact.Category,
		Fingerprint:         artifact.Fingerprint,
		Status:              status,
		ReviewedBy:          sanitizeRegressionText(request.ReviewedBy, 80),
		ReviewNote:          sanitizeRegressionText(request.ReviewNote, 240),
		ReviewedAt:          time.Now().UTC().Format(time.RFC3339Nano),
		CoveredByEvalOrTest: artifact.CoveredByEvalOrTest,
		Promotion:           artifact.Promotion,
	}

	base := filepath.Clean(outputDir)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return QARegressionReviewRecord{}, fmt.Errorf("create QA regression review directory: %w", err)
	}
	info, err := os.Stat(base)
	if err != nil {
		return QARegressionReviewRecord{}, fmt.Errorf("stat QA regression review directory: %w", err)
	}
	if !info.IsDir() {
		return QARegressionReviewRecord{}, fmt.Errorf("QA regression review path is not a directory: %s", outputDir)
	}
	path, err := regressionArtifactPath(base, qaRegressionReviewFilename(record))
	if err != nil {
		return QARegressionReviewRecord{}, err
	}
	record.Path = path
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return QARegressionReviewRecord{}, fmt.Errorf("encode QA regression review record: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return QARegressionReviewRecord{}, fmt.Errorf("write QA regression review record: %w", err)
	}
	return record, nil
}

func ListQARegressionReviews(outputDir string) ([]QARegressionReviewRecord, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return nil, fmt.Errorf("QA regression review directory is required")
	}
	if regressionArtifactPathLooksRemote(outputDir) {
		return nil, fmt.Errorf("QA regression review directory must be a local filesystem path: %s", outputDir)
	}
	entries, err := os.ReadDir(filepath.Clean(outputDir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list QA regression review records: %w", err)
	}
	var records []QARegressionReviewRecord
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		path, err := regressionArtifactPath(filepath.Clean(outputDir), entry.Name())
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read QA regression review record: %w", err)
		}
		var record QARegressionReviewRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("decode QA regression review record %s: %w", entry.Name(), err)
		}
		if err := validateQARegressionReviewRecord(record); err != nil {
			return nil, fmt.Errorf("invalid QA regression review record %s: %w", entry.Name(), err)
		}
		record.Path = path
		records = append(records, record)
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].ReviewedAt < records[j].ReviewedAt
	})
	return records, nil
}

func ApplyQARegressionReviews(review *QAReview, records []QARegressionReviewRecord) {
	if review == nil || len(records) == 0 {
		return
	}
	latest := map[string]QARegressionReviewRecord{}
	for _, record := range records {
		if err := validateQARegressionReviewRecord(record); err != nil {
			continue
		}
		for _, key := range qaRegressionReviewRecordKeys(record) {
			latest[key] = record
		}
	}
	seen := map[string]bool{}
	for i := range review.ReplayFailures {
		failure := &review.ReplayFailures[i]
		record, ok := qaRegressionReviewForFailure(*failure, latest)
		if ok {
			applyQARegressionReviewToFailure(failure, record)
			qaRegressionReviewCountSummary(&review.Summary, record, seen)
		}
		for j := range failure.RegressionSuggestions {
			suggestion := &failure.RegressionSuggestions[j]
			record, ok := qaRegressionReviewForSuggestion(*failure, *suggestion, latest)
			if !ok {
				continue
			}
			applyQARegressionReviewToSuggestion(suggestion, record)
			qaRegressionReviewCountSummary(&review.Summary, record, seen)
		}
	}
}

func normalizeQARegressionReviewStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case QARegressionReviewApproved:
		return QARegressionReviewApproved
	case QARegressionReviewDismissed:
		return QARegressionReviewDismissed
	case QARegressionReviewCovered:
		return QARegressionReviewCovered
	default:
		return ""
	}
}

func qaRegressionReviewFilename(record QARegressionReviewRecord) string {
	name := sanitizeRegressionName(firstNonEmpty(record.Name, record.SuggestionName, "qa_review_regression"))
	fingerprint := sanitizeRegressionName(strings.TrimPrefix(record.Fingerprint, "route_governance:"))
	if fingerprint != "" {
		return fmt.Sprintf("review_%s_%s.json", name, fingerprint)
	}
	return fmt.Sprintf("review_%s.json", name)
}

func validateQARegressionReviewRecord(record QARegressionReviewRecord) error {
	if record.Schema != QARegressionReviewSchema {
		return fmt.Errorf("schema = %q, want %q", record.Schema, QARegressionReviewSchema)
	}
	if strings.TrimSpace(record.TraceID) == "" {
		return fmt.Errorf("trace id is required")
	}
	if normalizeQARegressionReviewStatus(record.Status) == "" {
		return fmt.Errorf("status must be approved, dismissed, or covered")
	}
	if strings.TrimSpace(record.Name) == "" && strings.TrimSpace(record.SuggestionName) == "" {
		return fmt.Errorf("suggestion name is required")
	}
	return nil
}

func qaRegressionReviewRecordKeys(record QARegressionReviewRecord) []string {
	fingerprint := strings.TrimSpace(record.Fingerprint)
	traceID := strings.TrimSpace(record.TraceID)
	suggestionName := strings.TrimSpace(record.SuggestionName)
	name := strings.TrimSpace(record.Name)
	kind := strings.TrimSpace(record.Kind)
	keys := []string{}
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

func qaRegressionReviewForFailure(failure QAReplayFailure, records map[string]QARegressionReviewRecord) (QARegressionReviewRecord, bool) {
	for _, key := range []string{
		qaRegressionReviewKey("fingerprint", failure.PromotionFingerprint),
		qaRegressionReviewKey("trace", failure.TraceID),
	} {
		if key == "" {
			continue
		}
		if record, ok := records[key]; ok {
			return record, true
		}
	}
	return QARegressionReviewRecord{}, false
}

func qaRegressionReviewForSuggestion(failure QAReplayFailure, suggestion QARegressionSuggestion, records map[string]QARegressionReviewRecord) (QARegressionReviewRecord, bool) {
	fingerprint := firstNonEmpty(suggestion.PromotionFingerprint, failure.PromotionFingerprint)
	for _, key := range []string{
		qaRegressionReviewKey("fingerprint", fingerprint, "suggestion", suggestion.Name),
		qaRegressionReviewKey("fingerprint", fingerprint, "name", suggestion.Name),
		qaRegressionReviewKey("fingerprint", fingerprint, "kind", suggestion.Kind),
		qaRegressionReviewKey("trace", failure.TraceID, "suggestion", suggestion.Name),
		qaRegressionReviewKey("trace", failure.TraceID, "name", suggestion.Name),
		qaRegressionReviewKey("trace", failure.TraceID, "kind", suggestion.Kind),
		qaRegressionReviewKey("fingerprint", fingerprint),
		qaRegressionReviewKey("trace", failure.TraceID),
	} {
		if key == "" {
			continue
		}
		if record, ok := records[key]; ok {
			return record, true
		}
	}
	return QARegressionReviewRecord{}, false
}

func qaRegressionReviewKey(parts ...string) string {
	if len(parts) < 2 || len(parts)%2 != 0 {
		return ""
	}
	for i := 1; i < len(parts); i += 2 {
		if strings.TrimSpace(parts[i]) == "" {
			return ""
		}
	}
	return strings.Join(cleanPromotionFingerprintParts(parts), "|")
}

func applyQARegressionReviewToFailure(failure *QAReplayFailure, record QARegressionReviewRecord) {
	failure.ReviewStatus = record.Status
	failure.ReviewNote = record.ReviewNote
	failure.ReviewedAt = record.ReviewedAt
	failure.CoverageStatus = qaRegressionCoverageStatus(record.Status, failure.CoverageStatus)
	failure.CoverageSources = appendUniqueStrings(failure.CoverageSources, qaRegressionReviewCoverageSource(record))
}

func applyQARegressionReviewToSuggestion(suggestion *QARegressionSuggestion, record QARegressionReviewRecord) {
	suggestion.ReviewStatus = record.Status
	suggestion.ReviewNote = record.ReviewNote
	suggestion.ReviewedAt = record.ReviewedAt
	suggestion.CoverageStatus = qaRegressionCoverageStatus(record.Status, suggestion.CoverageStatus)
	suggestion.CoverageSources = appendUniqueStrings(suggestion.CoverageSources, qaRegressionReviewCoverageSource(record))
}

func qaRegressionCoverageStatus(status string, fallback string) string {
	switch normalizeQARegressionReviewStatus(status) {
	case QARegressionReviewApproved:
		return "approved_for_test"
	case QARegressionReviewDismissed:
		return "dismissed"
	case QARegressionReviewCovered:
		return "marked_covered"
	default:
		return fallback
	}
}

func qaRegressionReviewCoverageSource(record QARegressionReviewRecord) string {
	return "qa_review:" + normalizeQARegressionReviewStatus(record.Status)
}

func qaRegressionReviewCountSummary(summary *QAReviewSummary, record QARegressionReviewRecord, seen map[string]bool) {
	key := firstNonEmpty(record.Fingerprint, record.TraceID) + ":" + record.Status
	if key == ":" || seen[key] {
		return
	}
	seen[key] = true
	switch normalizeQARegressionReviewStatus(record.Status) {
	case QARegressionReviewApproved:
		summary.ApprovedSuggestions++
	case QARegressionReviewDismissed:
		summary.DismissedSuggestions++
	case QARegressionReviewCovered:
		summary.CoveredSuggestions++
	}
}
