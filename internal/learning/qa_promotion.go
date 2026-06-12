package learning

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yemaka/internal/replay"
)

const QARegressionPromotionSchema = "yemaka.qa_review_regression_promotion.v1"

type QARegressionPromotionRequest struct {
	TraceID        string `json:"traceId"`
	SuggestionName string `json:"suggestionName,omitempty"`
	ReviewedBy     string `json:"reviewedBy,omitempty"`
	ReviewNote     string `json:"reviewNote,omitempty"`
}

type QARegressionPromotionArtifact struct {
	Schema               string                   `json:"schema"`
	Name                 string                   `json:"name"`
	Kind                 string                   `json:"kind"`
	TraceID              string                   `json:"traceId"`
	Category             string                   `json:"category,omitempty"`
	Fingerprint          string                   `json:"fingerprint,omitempty"`
	Path                 string                   `json:"path"`
	Preview              string                   `json:"preview"`
	PromotionMode        string                   `json:"promotionMode"`
	AutomaticTestWritten bool                     `json:"automaticTestWritten"`
	CoveredByEvalOrTest  bool                     `json:"coveredByEvalOrTest"`
	Promotion            RouteRegressionPromotion `json:"promotion"`
}

func PromoteQARegressionSuggestion(outputDir string, traces replay.Store, latestEval *QAEvalSummary, request QARegressionPromotionRequest) (QARegressionPromotionArtifact, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return QARegressionPromotionArtifact{}, fmt.Errorf("QA regression promotion directory is required")
	}
	if regressionArtifactPathLooksRemote(outputDir) {
		return QARegressionPromotionArtifact{}, fmt.Errorf("QA regression promotion directory must be a local filesystem path: %s", outputDir)
	}
	traceID := strings.TrimSpace(request.TraceID)
	if traceID == "" {
		return QARegressionPromotionArtifact{}, fmt.Errorf("replay trace id is required")
	}
	trace, err := traces.Load(traceID)
	if err != nil {
		return QARegressionPromotionArtifact{}, err
	}
	trace = trace.Normalize()
	classification := ApplyRouteGovernanceCoverage(trace, ClassifyRouteFailure(trace), latestEval)
	suggestions := regressionSuggestionsForTrace(trace, classification)
	if len(suggestions) == 0 {
		return QARegressionPromotionArtifact{}, fmt.Errorf("no regression suggestion is available for replay trace %s", traceID)
	}
	suggestion := chooseQARegressionSuggestion(suggestions, request.SuggestionName)
	if strings.TrimSpace(suggestion.Name) == "" {
		return QARegressionPromotionArtifact{}, fmt.Errorf("regression suggestion is missing a name")
	}
	artifact := qaRegressionPromotionArtifact(trace, classification, suggestion)
	content := FormatQARegressionPromotionArtifactYAMLish(artifact, trace, classification, request) + "\n"

	base := filepath.Clean(outputDir)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return QARegressionPromotionArtifact{}, fmt.Errorf("create QA regression promotion directory: %w", err)
	}
	info, err := os.Stat(base)
	if err != nil {
		return QARegressionPromotionArtifact{}, fmt.Errorf("stat QA regression promotion directory: %w", err)
	}
	if !info.IsDir() {
		return QARegressionPromotionArtifact{}, fmt.Errorf("QA regression promotion path is not a directory: %s", outputDir)
	}
	path, err := regressionArtifactPath(base, qaRegressionPromotionFilename(artifact))
	if err != nil {
		return QARegressionPromotionArtifact{}, err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return QARegressionPromotionArtifact{}, fmt.Errorf("write QA regression promotion artifact: %w", err)
	}
	artifact.Path = path
	artifact.Preview = summarizeText(content, 1200)
	return artifact, nil
}

func FormatQARegressionPromotionArtifactYAMLish(artifact QARegressionPromotionArtifact, trace replay.Trace, classification RouteFailureClassification, request QARegressionPromotionRequest) string {
	var out strings.Builder
	fmt.Fprintf(&out, "schema: %s\n", yamlString(QARegressionPromotionSchema))
	fmt.Fprintf(&out, "review_status: %s\n", yamlString("reviewed_suggestion"))
	fmt.Fprintf(&out, "promotion_mode: %s\n", yamlString("suggestion_only"))
	fmt.Fprintf(&out, "automatic_test_written: %t\n", false)
	fmt.Fprintf(&out, "name: %s\n", yamlString(artifact.Name))
	fmt.Fprintf(&out, "kind: %s\n", yamlString(artifact.Kind))
	fmt.Fprintf(&out, "source_trace_id: %s\n", yamlString(artifact.TraceID))
	fmt.Fprintf(&out, "category: %s\n", yamlString(artifact.Category))
	fmt.Fprintf(&out, "fingerprint: %s\n", yamlString(artifact.Fingerprint))
	fmt.Fprintf(&out, "covered_by_eval_or_test: %t\n", artifact.CoveredByEvalOrTest)
	reviewedBy := sanitizeRegressionText(request.ReviewedBy, 80)
	if reviewedBy != "" {
		fmt.Fprintf(&out, "reviewed_by: %s\n", yamlString(reviewedBy))
	}
	reviewNote := sanitizeRegressionText(request.ReviewNote, 240)
	if reviewNote != "" {
		fmt.Fprintf(&out, "review_note: %s\n", yamlString(reviewNote))
	}
	fmt.Fprintf(&out, "prompt: %s\n", yamlString(sanitizeRegressionText(trace.UserRequest, 240)))
	fmt.Fprintf(&out, "reason: %s\n", yamlString(sanitizeRegressionText(classification.Reason, 240)))
	fmt.Fprintf(&out, "suggested_regression: %s\n", yamlString(sanitizeRegressionText(classification.SuggestedRegression, 240)))
	out.WriteString("expected:\n")
	fmt.Fprintf(&out, "  route_under_test: %s\n", yamlString(artifact.Promotion.RouteUnderTest))
	fmt.Fprintf(&out, "  continuation_mode: %s\n", yamlString(artifact.Promotion.ContinuationMode))
	fmt.Fprintf(&out, "  source: %s\n", yamlString(artifact.Promotion.ExpectedSource))
	fmt.Fprintf(&out, "  tool_lane: %s\n", yamlString(artifact.Promotion.ExpectedToolLane))
	fmt.Fprintf(&out, "  outcome: %s\n", yamlString(artifact.Promotion.ExpectedOutcome))
	writeYAMLStringList(&out, "", "coverage_sources", classification.CoverageSources)
	return strings.TrimRight(out.String(), "\n")
}

func chooseQARegressionSuggestion(suggestions []QARegressionSuggestion, name string) QARegressionSuggestion {
	name = strings.TrimSpace(name)
	if name == "" {
		return suggestions[0]
	}
	for _, suggestion := range suggestions {
		if suggestion.Name == name || suggestion.Kind == name {
			return suggestion
		}
	}
	return QARegressionSuggestion{}
}

func qaRegressionPromotionArtifact(trace replay.Trace, classification RouteFailureClassification, suggestion QARegressionSuggestion) QARegressionPromotionArtifact {
	promotion := suggestion.Promotion
	if promotion.SuggestedTestName == "" {
		promotion = classification.RegressionPromotion
	}
	name := firstNonEmpty(suggestion.Name, promotion.SuggestedTestName, regressionCaseName("qa_review_"+classification.Category, trace.UserRequest, trace.ID))
	kind := firstNonEmpty(suggestion.Kind, "route_governance_"+classification.Category, FailureKindUnknown)
	return QARegressionPromotionArtifact{
		Schema:               QARegressionPromotionSchema,
		Name:                 sanitizeRegressionName(name),
		Kind:                 sanitizeRegressionText(kind, 120),
		TraceID:              sanitizeRegressionText(trace.ID, 120),
		Category:             sanitizeRegressionText(classification.Category, 80),
		Fingerprint:          promotionFingerprint(classification),
		PromotionMode:        "suggestion_only",
		AutomaticTestWritten: false,
		CoveredByEvalOrTest:  classification.CoveredByEvalOrTest,
		Promotion:            promotion,
	}
}

func qaRegressionPromotionFilename(artifact QARegressionPromotionArtifact) string {
	name := sanitizeRegressionName(artifact.Name)
	if name == "" {
		name = "qa_review_regression_promotion"
	}
	fingerprint := sanitizeRegressionName(strings.TrimPrefix(artifact.Fingerprint, "route_governance:"))
	if fingerprint != "" {
		return fmt.Sprintf("reviewed_%s_%s.yml", name, fingerprint)
	}
	return fmt.Sprintf("reviewed_%s.yml", name)
}
