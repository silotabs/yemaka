package learning

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"yemaka/internal/memory"
	"yemaka/internal/replay"
	"yemaka/internal/routing"
)

type QAReview struct {
	GeneratedAt      string                    `json:"generatedAt"`
	Summary          QAReviewSummary           `json:"summary"`
	ReplayFailures   []QAReplayFailure         `json:"replayFailures"`
	RouteCorrections []routing.RouteCorrection `json:"routeCorrections"`
	LatestEval       *QAEvalSummary            `json:"latestEval,omitempty"`
}

type QAReviewSummary struct {
	ReplayFailures         int            `json:"replayFailures"`
	RouteCorrections       int            `json:"routeCorrections"`
	PendingCorrections     int            `json:"pendingCorrections"`
	RegressionSuggestions  int            `json:"regressionSuggestions"`
	RepeatedSuggestions    int            `json:"repeatedSuggestions,omitempty"`
	ApprovedSuggestions    int            `json:"approvedSuggestions,omitempty"`
	DismissedSuggestions   int            `json:"dismissedSuggestions,omitempty"`
	CoveredSuggestions     int            `json:"coveredSuggestions,omitempty"`
	TestDrafts             int            `json:"testDrafts,omitempty"`
	ApprovedRegressions    int            `json:"approvedRegressions,omitempty"`
	SourcePatches          int            `json:"sourcePatches,omitempty"`
	SourcePatchApplyPlans  int            `json:"sourcePatchApplyPlans,omitempty"`
	SourceTestWrites       int            `json:"sourceTestWrites,omitempty"`
	RouteFailureCategories map[string]int `json:"routeFailureCategories,omitempty"`
	EvalStatus             string         `json:"evalStatus,omitempty"`
}

type QAReplayFailure struct {
	TraceID                        string                        `json:"traceId"`
	RequestSummary                 string                        `json:"requestSummary"`
	RouteCategory                  string                        `json:"routeCategory,omitempty"`
	RouteIntent                    string                        `json:"routeIntent,omitempty"`
	RouteTarget                    string                        `json:"routeTarget,omitempty"`
	RiskLevel                      string                        `json:"riskLevel,omitempty"`
	FailureStage                   string                        `json:"failureStage,omitempty"`
	FailureCode                    string                        `json:"failureCode,omitempty"`
	FailureSubject                 string                        `json:"failureSubject,omitempty"`
	FailureMessage                 string                        `json:"failureMessage,omitempty"`
	RouteFailureCategory           string                        `json:"routeFailureCategory,omitempty"`
	RouteFailureReason             string                        `json:"routeFailureReason,omitempty"`
	RouteRecovery                  routing.RouteRecoveryDecision `json:"routeRecovery,omitempty"`
	SuggestedRegression            string                        `json:"suggestedGenericRegression,omitempty"`
	PromotionFingerprint           string                        `json:"promotionFingerprint,omitempty"`
	RepeatCount                    int                           `json:"repeatCount,omitempty"`
	PromotionPriority              string                        `json:"promotionPriority,omitempty"`
	CoveredByEvalOrTest            bool                          `json:"coveredByEvalOrTest"`
	CoverageSources                []string                      `json:"coverageSources,omitempty"`
	ReviewStatus                   string                        `json:"reviewStatus,omitempty"`
	ReviewNote                     string                        `json:"reviewNote,omitempty"`
	ReviewedAt                     string                        `json:"reviewedAt,omitempty"`
	TestDraftStatus                string                        `json:"testDraftStatus,omitempty"`
	TestDraftPath                  string                        `json:"testDraftPath,omitempty"`
	ApprovedRegression             string                        `json:"approvedRegression,omitempty"`
	ApprovedRegressionAt           string                        `json:"approvedRegressionAt,omitempty"`
	ApprovedRegressionPath         string                        `json:"approvedRegressionPath,omitempty"`
	SourcePatchStatus              string                        `json:"sourcePatchStatus,omitempty"`
	SourcePatchPath                string                        `json:"sourcePatchPath,omitempty"`
	SourcePatchRequiresManualApply bool                          `json:"sourcePatchRequiresManualApply,omitempty"`
	SourcePatchApplyStatus         string                        `json:"sourcePatchApplyStatus,omitempty"`
	SourcePatchApplyPath           string                        `json:"sourcePatchApplyPath,omitempty"`
	SourceTestWriteStatus          string                        `json:"sourceTestWriteStatus,omitempty"`
	SourceTestWritePath            string                        `json:"sourceTestWritePath,omitempty"`
	SourceTestWriteSnapshotID      string                        `json:"sourceTestWriteSnapshotId,omitempty"`
	RegressionPromotion            RouteRegressionPromotion      `json:"regressionPromotion,omitempty"`
	ResultStatus                   string                        `json:"resultStatus,omitempty"`
	VerificationStatus             string                        `json:"verificationStatus,omitempty"`
	CoverageStatus                 string                        `json:"coverageStatus"`
	RegressionSuggestions          []QARegressionSuggestion      `json:"regressionSuggestions"`
	MatchingCorrectionIDs          []string                      `json:"matchingCorrectionIds,omitempty"`
}

type QARegressionSuggestion struct {
	Name                           string                   `json:"name"`
	Kind                           string                   `json:"kind"`
	Prompt                         string                   `json:"prompt"`
	SourceTraceID                  string                   `json:"sourceTraceId,omitempty"`
	PromotionFingerprint           string                   `json:"promotionFingerprint,omitempty"`
	RepeatCount                    int                      `json:"repeatCount,omitempty"`
	PromotionPriority              string                   `json:"promotionPriority,omitempty"`
	CoverageStatus                 string                   `json:"coverageStatus"`
	CoveredByTest                  bool                     `json:"coveredByTest"`
	CoverageSources                []string                 `json:"coverageSources,omitempty"`
	ReviewStatus                   string                   `json:"reviewStatus,omitempty"`
	ReviewNote                     string                   `json:"reviewNote,omitempty"`
	ReviewedAt                     string                   `json:"reviewedAt,omitempty"`
	TestDraftStatus                string                   `json:"testDraftStatus,omitempty"`
	TestDraftPath                  string                   `json:"testDraftPath,omitempty"`
	ApprovedRegression             string                   `json:"approvedRegression,omitempty"`
	ApprovedRegressionAt           string                   `json:"approvedRegressionAt,omitempty"`
	ApprovedRegressionPath         string                   `json:"approvedRegressionPath,omitempty"`
	SourcePatchStatus              string                   `json:"sourcePatchStatus,omitempty"`
	SourcePatchPath                string                   `json:"sourcePatchPath,omitempty"`
	SourcePatchRequiresManualApply bool                     `json:"sourcePatchRequiresManualApply,omitempty"`
	SourcePatchApplyStatus         string                   `json:"sourcePatchApplyStatus,omitempty"`
	SourcePatchApplyPath           string                   `json:"sourcePatchApplyPath,omitempty"`
	SourceTestWriteStatus          string                   `json:"sourceTestWriteStatus,omitempty"`
	SourceTestWritePath            string                   `json:"sourceTestWritePath,omitempty"`
	SourceTestWriteSnapshotID      string                   `json:"sourceTestWriteSnapshotId,omitempty"`
	Tags                           []string                 `json:"tags,omitempty"`
	Preview                        string                   `json:"preview,omitempty"`
	Promotion                      RouteRegressionPromotion `json:"promotion,omitempty"`
}

type QAEvalSummary struct {
	ID          string              `json:"id,omitempty"`
	Mode        string              `json:"mode,omitempty"`
	Model       string              `json:"model,omitempty"`
	StartedAt   string              `json:"startedAt,omitempty"`
	CompletedAt string              `json:"completedAt,omitempty"`
	Passed      int                 `json:"passed"`
	Failed      int                 `json:"failed"`
	Skipped     int                 `json:"skipped"`
	Status      string              `json:"status"`
	Metrics     map[string]any      `json:"metrics,omitempty"`
	Tasks       []QAEvalTaskSummary `json:"tasks,omitempty"`
}

type QAEvalTaskSummary struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Details string `json:"details,omitempty"`
}

func BuildQAReview(ctx context.Context, memories *memory.Store, traces replay.Store, latestEval *QAEvalSummary, limit int) (QAReview, error) {
	if limit <= 0 {
		limit = 30
	}
	review := QAReview{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		LatestEval:  latestEval,
	}
	review.Summary.RouteFailureCategories = map[string]int{}
	if latestEval != nil {
		review.Summary.EvalStatus = latestEval.Status
	}

	corrections, err := listRouteCorrectionsIfAvailable(ctx, memories, limit*4)
	if err != nil {
		return QAReview{}, err
	}
	review.RouteCorrections = corrections
	review.Summary.RouteCorrections = len(corrections)
	for _, correction := range corrections {
		if correction.ApprovalStatus == routing.RouteCorrectionStatusPending && !correction.Disabled {
			review.Summary.PendingCorrections++
		}
	}

	files, err := traces.List()
	if err != nil {
		return QAReview{}, err
	}
	for _, file := range latestTraceFiles(files, limit*3) {
		trace, err := traces.Load(file.ID)
		if err != nil {
			continue
		}
		trace = trace.Normalize()
		if !trace.HasFailures() {
			continue
		}
		failure := replayFailureFromTrace(trace, corrections, latestEval)
		if failure.RouteFailureCategory != "" {
			review.Summary.RouteFailureCategories[failure.RouteFailureCategory]++
		}
		review.Summary.RegressionSuggestions += len(failure.RegressionSuggestions)
		review.ReplayFailures = append(review.ReplayFailures, failure)
		if len(review.ReplayFailures) >= limit {
			break
		}
	}
	review.Summary.ReplayFailures = len(review.ReplayFailures)
	applyPromotionRepeatMetadata(&review)
	return review, nil
}

func listRouteCorrectionsIfAvailable(ctx context.Context, memories *memory.Store, limit int) ([]routing.RouteCorrection, error) {
	if memories == nil {
		return nil, nil
	}
	return ListRouteCorrections(ctx, memories, limit)
}

func replayFailureFromTrace(trace replay.Trace, corrections []routing.RouteCorrection, latestEval *QAEvalSummary) QAReplayFailure {
	primary, _ := trace.PrimaryFailure()
	classification := ClassifyRouteFailure(trace)
	classification = ApplyRouteGovernanceCoverage(trace, classification, latestEval)
	suggestions := regressionSuggestionsForTrace(trace, classification)
	coverage := "needs_review"
	if len(suggestions) > 0 {
		coverage = "artifact_suggested"
	}
	return QAReplayFailure{
		TraceID:               trace.ID,
		RequestSummary:        summarizeText(trace.UserRequest, 180),
		RouteCategory:         trace.Route.Category,
		RouteIntent:           trace.Route.Intent,
		RouteTarget:           trace.Route.Target,
		RiskLevel:             trace.Route.RiskLevel,
		FailureStage:          primary.Stage,
		FailureCode:           primary.Code,
		FailureSubject:        primary.Subject,
		FailureMessage:        summarizeText(primary.Message, 220),
		RouteFailureCategory:  classification.Category,
		RouteFailureReason:    classification.Reason,
		RouteRecovery:         classification.RouteRecovery,
		SuggestedRegression:   classification.SuggestedRegression,
		PromotionFingerprint:  promotionFingerprint(classification),
		RepeatCount:           1,
		PromotionPriority:     promotionPriority(1, classification.CoveredByEvalOrTest),
		CoveredByEvalOrTest:   classification.CoveredByEvalOrTest,
		CoverageSources:       classification.CoverageSources,
		RegressionPromotion:   classification.RegressionPromotion,
		ResultStatus:          trace.FinalResult.Status,
		VerificationStatus:    trace.Verification.Status,
		CoverageStatus:        coverage,
		RegressionSuggestions: suggestions,
		MatchingCorrectionIDs: matchingRouteCorrectionIDs(trace, corrections),
	}
}

func regressionSuggestionsForTrace(trace replay.Trace, classification RouteFailureClassification) []QARegressionSuggestion {
	cases := GenerateRegressionTestCases(trace)
	out := make([]QARegressionSuggestion, 0, len(cases)+1)
	if classification.Category != "" {
		name := classification.RegressionPromotion.SuggestedTestName
		if name == "" {
			name = regressionCaseName("route_governance_"+classification.Category, trace.UserRequest, trace.ID)
		}
		out = append(out, QARegressionSuggestion{
			Name:                 name,
			Kind:                 "route_governance_" + classification.Category,
			Prompt:               summarizeText(trace.UserRequest, 220),
			SourceTraceID:        trace.ID,
			PromotionFingerprint: promotionFingerprint(classification),
			RepeatCount:          1,
			PromotionPriority:    promotionPriority(1, classification.CoveredByEvalOrTest),
			CoverageStatus:       "artifact_suggested",
			CoveredByTest:        classification.CoveredByEvalOrTest,
			CoverageSources:      classification.CoverageSources,
			Tags:                 []string{"replay", "generated_regression", "route_governance", classification.Category},
			Preview:              summarizeText(FormatRouteGovernanceRegressionYAMLish(name, trace, classification), 700),
			Promotion:            classification.RegressionPromotion,
		})
	}
	for _, testCase := range cases {
		out = append(out, QARegressionSuggestion{
			Name:                 testCase.Name,
			Kind:                 testCase.Kind,
			Prompt:               summarizeText(testCase.Prompt, 220),
			SourceTraceID:        testCase.SourceTraceID,
			PromotionFingerprint: promotionFingerprint(classification),
			RepeatCount:          1,
			PromotionPriority:    promotionPriority(1, classification.CoveredByEvalOrTest),
			CoverageStatus:       "artifact_suggested",
			CoveredByTest:        false,
			CoverageSources:      classification.CoverageSources,
			Tags:                 testCase.Tags,
			Preview:              summarizeText(FormatRegressionTestCaseYAMLish(testCase), 700),
			Promotion:            regressionPromotionForTestCase(testCase, classification),
		})
	}
	return out
}

func regressionPromotionForTestCase(testCase RegressionTestCase, classification RouteFailureClassification) RouteRegressionPromotion {
	promotion := classification.RegressionPromotion
	if promotion.SuggestedTestName == "" {
		promotion.SuggestedTestName = testCase.Name
	}
	if promotion.RouteUnderTest == "" {
		promotion.RouteUnderTest = testCase.Expected.Route
	}
	if promotion.ExpectedOutcome == "" {
		promotion.ExpectedOutcome = "assert_expected_route_risk_tools_and_policy"
	}
	return promotion
}

func applyPromotionRepeatMetadata(review *QAReview) {
	if review == nil {
		return
	}
	counts := map[string]int{}
	for _, failure := range review.ReplayFailures {
		fingerprint := strings.TrimSpace(failure.PromotionFingerprint)
		if fingerprint == "" {
			continue
		}
		counts[fingerprint]++
	}
	repeated := 0
	for _, count := range counts {
		if count > 1 {
			repeated++
		}
	}
	review.Summary.RepeatedSuggestions = repeated
	for i := range review.ReplayFailures {
		fingerprint := strings.TrimSpace(review.ReplayFailures[i].PromotionFingerprint)
		count := counts[fingerprint]
		if count <= 0 {
			count = 1
		}
		review.ReplayFailures[i].RepeatCount = count
		review.ReplayFailures[i].PromotionPriority = promotionPriority(count, review.ReplayFailures[i].CoveredByEvalOrTest)
		for j := range review.ReplayFailures[i].RegressionSuggestions {
			review.ReplayFailures[i].RegressionSuggestions[j].RepeatCount = count
			review.ReplayFailures[i].RegressionSuggestions[j].PromotionPriority = review.ReplayFailures[i].PromotionPriority
			if strings.TrimSpace(review.ReplayFailures[i].RegressionSuggestions[j].PromotionFingerprint) == "" {
				review.ReplayFailures[i].RegressionSuggestions[j].PromotionFingerprint = fingerprint
			}
		}
	}
}

func promotionFingerprint(classification RouteFailureClassification) string {
	promotion := classification.RegressionPromotion
	parts := []string{
		classification.Category,
		promotion.RouteUnderTest,
		promotion.ContinuationMode,
		promotion.ExpectedSource,
		promotion.ExpectedToolLane,
		promotion.ExpectedOutcome,
	}
	normalized := strings.ToLower(strings.Join(cleanPromotionFingerprintParts(parts), "|"))
	if normalized == "" {
		normalized = "unknown"
	}
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(normalized))
	return fmt.Sprintf("route_governance:%x", hash.Sum64())
}

func cleanPromotionFingerprintParts(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Join(strings.Fields(strings.TrimSpace(part)), " ")
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func promotionPriority(repeatCount int, covered bool) string {
	switch {
	case repeatCount >= 3:
		return "high"
	case repeatCount >= 2:
		return "medium"
	case covered:
		return "covered"
	default:
		return "low"
	}
}

func matchingRouteCorrectionIDs(trace replay.Trace, corrections []routing.RouteCorrection) []string {
	prompt := strings.ToLower(strings.TrimSpace(trace.UserRequest))
	if prompt == "" {
		return nil
	}
	var ids []string
	for _, correction := range corrections {
		if correction.Disabled {
			continue
		}
		for _, candidate := range []string{correction.OriginalPrompt, correction.Pattern, correction.CorrectionText} {
			candidate = strings.ToLower(strings.TrimSpace(candidate))
			if len(candidate) < 8 {
				continue
			}
			if strings.Contains(prompt, candidate) || strings.Contains(candidate, prompt) {
				ids = append(ids, correction.ID)
				break
			}
		}
	}
	return ids
}

func latestTraceFiles(files []replay.TraceFile, limit int) []replay.TraceFile {
	if limit <= 0 || limit > len(files) {
		limit = len(files)
	}
	out := make([]replay.TraceFile, 0, limit)
	for i := len(files) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, files[i])
	}
	return out
}

func summarizeText(value string, max int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if max <= 0 || len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return strings.TrimSpace(value[:max-3]) + "..."
}
