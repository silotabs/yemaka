package learning

import (
	"strings"
	"testing"

	"yemaka/internal/replay"
	"yemaka/internal/routing"
)

func TestClassifyRouteFailureCategories(t *testing.T) {
	cases := []struct {
		name  string
		trace replay.Trace
		want  string
	}{
		{
			name: "wrong source",
			trace: replay.Trace{
				UserRequest: "Search my indexed documents for this answer",
				Route:       replay.RouteSnapshot{Category: routing.RouteInternetSearch, ShouldUseInternet: true},
				Errors: []replay.TraceError{
					{Stage: "routing", Code: "wrong_source", Subject: "internet_search", Message: "local documents should have been used", ExpectedRoute: routing.RouteRAGSearch},
				},
				FinalResult: replay.ResultSnapshot{Status: "failed"},
			},
			want: RouteFailureWrongSource,
		},
		{
			name: "wrong tool lane",
			trace: replay.Trace{
				UserRequest: "Answer from local documents",
				Route:       replay.RouteSnapshot{Category: routing.RouteRAGSearch},
				ToolsCalled: []replay.ToolCall{
					{Name: "internet_search", Status: "completed"},
				},
				Verification: replay.VerificationResult{Status: "failed", Notes: []string{"tool lane mismatch"}},
			},
			want: RouteFailureWrongToolLane,
		},
		{
			name: "rag route read file permission failure is wrong lane before permission",
			trace: replay.Trace{
				UserRequest: "Answer from the indexed documents",
				Route:       replay.RouteSnapshot{Category: routing.RouteRAGSearch},
				ToolsCalled: []replay.ToolCall{
					{Name: "read_file", Status: "failed", Error: "operation not permitted"},
				},
			},
			want: RouteFailureWrongToolLane,
		},
		{
			name: "rag route considered read file is wrong lane",
			trace: replay.Trace{
				UserRequest: "Search local indexed files",
				Route:       replay.RouteSnapshot{Category: routing.RouteRAGSearch},
				ToolsConsidered: []replay.ToolConsideration{
					{Name: "read_file", Status: "blocked", Reason: "model requested workspace file read"},
				},
			},
			want: RouteFailureWrongToolLane,
		},
		{
			name: "stale answer",
			trace: replay.Trace{
				UserRequest:  "What are the current updates?",
				Route:        replay.RouteSnapshot{Category: routing.RouteChatExplanation},
				Attributes:   []replay.Attribute{{Key: "route_preflight_source_of_truth", Value: routing.PreflightSourceInternet}, {Key: "route_preflight_freshness_risk", Value: routing.PreflightFreshnessHigh}},
				Verification: replay.VerificationResult{Status: "failed", Notes: []string{"current public fact needs fresh source required"}},
			},
			want: RouteFailureStaleAnswer,
		},
		{
			name: "missing approval",
			trace: replay.Trace{
				UserRequest: "Create a recurring job",
				Route:       replay.RouteSnapshot{Category: routing.RouteSchedulerCreate},
				PermissionsRequested: []replay.PermissionRequest{
					{ToolName: "scheduler_create", Status: "denied", Reason: "approval required before job creation"},
				},
			},
			want: RouteFailureMissingApproval,
		},
		{
			name: "missing evidence",
			trace: replay.Trace{
				UserRequest:  "Answer from my local notes",
				Route:        replay.RouteSnapshot{Category: routing.RouteRAGSearch},
				Verification: replay.VerificationResult{Status: "failed", Notes: []string{"no local document evidence"}},
			},
			want: RouteFailureMissingEvidence,
		},
		{
			name: "rag no matches is missing evidence",
			trace: replay.Trace{
				UserRequest: "Answer from my local notes",
				Route:       replay.RouteSnapshot{Category: routing.RouteRAGSearch},
				ToolsCalled: []replay.ToolCall{
					{Name: "rag_search", Status: "completed", OutputSummary: "No RAG matches."},
				},
				Verification: replay.VerificationResult{Status: "needs_follow_up", Notes: []string{"no retrieved local document context"}},
			},
			want: RouteFailureMissingEvidence,
		},
		{
			name: "rag citation verifier is missing evidence",
			trace: replay.Trace{
				UserRequest: "Answer from retrieved local documents",
				Route:       replay.RouteSnapshot{Category: routing.RouteRAGSearch},
				RAGDocsUsed: []replay.DocumentRef{{Title: "notes.md", Source: "rag"}},
				Verification: replay.VerificationResult{
					Status: "needs_follow_up",
					Checks: []string{"response_present", "source_citation_checked"},
					Notes:  []string{"RAG answer should cite or name retrieved local sources"},
				},
			},
			want: RouteFailureMissingEvidence,
		},
		{
			name: "general knowledge optional evidence contract mismatch",
			trace: replay.Trace{
				UserRequest: "Explain a general technical concept.",
				Route:       replay.RouteSnapshot{Category: routing.RouteChatExplanation},
				Attributes: []replay.Attribute{
					{Key: "evidence_policy", Value: "general_knowledge_allowed"},
					{Key: "response_contract", Value: "general model knowledge is allowed; use provided local context only when it is relevant"},
				},
				Verification: replay.VerificationResult{
					Status: "needs_follow_up",
					Notes:  []string{"general knowledge route refused because optional local/workspace evidence was absent"},
				},
				FinalResult: replay.ResultSnapshot{
					Status:        "completed",
					OutputSummary: "The requested concept is not present in the available local context or workspace files, so I cannot provide an explanation based on available evidence.",
				},
			},
			want: RouteFailureEvidenceContract,
		},
		{
			name: "bad clarification",
			trace: replay.Trace{
				UserRequest:  "Do that one instead",
				Route:        replay.RouteSnapshot{Category: routing.RouteChatExplanation, ShouldAskClarification: true},
				Attributes:   []replay.Attribute{{Key: "route_preflight_missing_slots", Value: "target"}},
				Verification: replay.VerificationResult{Status: "failed", Notes: []string{"should ask clarification before proceeding"}},
			},
			want: RouteFailureBadClarification,
		},
		{
			name: "provider config issue",
			trace: replay.Trace{
				UserRequest: "Search current public news",
				Route:       replay.RouteSnapshot{Category: routing.RouteInternetSearch, ShouldUseInternet: true},
				ToolsCalled: []replay.ToolCall{
					{Name: "internet_search", Status: "failed", Error: "SearXNG provider not configured"},
				},
			},
			want: RouteFailureProviderConfigIssue,
		},
		{
			name: "internet provider failure in document lane is wrong tool lane",
			trace: replay.Trace{
				UserRequest: "Answer from local documents",
				Route:       replay.RouteSnapshot{Category: routing.RouteRAGSearch},
				ToolsCalled: []replay.ToolCall{
					{Name: "internet_search", Status: "failed", Error: "search provider not configured"},
				},
			},
			want: RouteFailureWrongToolLane,
		},
		{
			name: "local embedding runtime provider issue",
			trace: replay.Trace{
				UserRequest: "Search my local documents for this information",
				Route:       replay.RouteSnapshot{Category: routing.RouteRAGSearch},
				ToolsCalled: []replay.ToolCall{
					{Name: "rag_search", Status: "failed", Error: `ollama embed request failed: Post "http://localhost:11434/api/embed": context deadline exceeded`},
				},
				FinalResult: replay.ResultSnapshot{Status: "failed"},
			},
			want: RouteFailureProviderConfigIssue,
		},
		{
			name: "local chat runtime provider issue",
			trace: replay.Trace{
				UserRequest: "Explain the previous answer",
				Route:       replay.RouteSnapshot{Category: routing.RouteChatExplanation},
				Errors: []replay.TraceError{
					{Stage: "model", Code: "runtime_unavailable", Message: `ollama chat request failed: Post "http://localhost:11434/api/chat": dial tcp 127.0.0.1:11434: connect: connection refused`},
				},
				FinalResult: replay.ResultSnapshot{Status: "failed"},
			},
			want: RouteFailureProviderConfigIssue,
		},
		{
			name: "permission missing",
			trace: replay.Trace{
				UserRequest: "Read a workspace file",
				Route:       replay.RouteSnapshot{Category: routing.RouteWorkspaceRead},
				ToolsCalled: []replay.ToolCall{
					{Name: "read_file", Status: "failed", Error: "operation not permitted"},
				},
			},
			want: RouteFailurePermissionMissing,
		},
		{
			name: "missing capability",
			trace: replay.Trace{
				UserRequest: "Build a reusable capability",
				Route:       replay.RouteSnapshot{Category: routing.RouteExtensionGenerate},
				Errors: []replay.TraceError{
					{Stage: "capability", Code: "missing_capability", Message: "no matching safe executor is available"},
				},
			},
			want: RouteFailureMissingCapability,
		},
		{
			name: "wrong route",
			trace: replay.Trace{
				UserRequest: "Search current public news",
				Route:       replay.RouteSnapshot{Category: routing.RouteChatExplanation},
				Errors: []replay.TraceError{
					{Stage: "routing", Code: "route_mismatch", Message: "expected route did not match task frame", ExpectedRoute: routing.RouteInternetSearch},
				},
			},
			want: RouteFailureWrongRoute,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyRouteFailure(tc.trace)
			if got.Category != tc.want {
				t.Fatalf("Category = %q, want %q; classification = %+v", got.Category, tc.want, got)
			}
			if strings.TrimSpace(got.Reason) == "" || strings.TrimSpace(got.SuggestedRegression) == "" {
				t.Fatalf("classification missing reason/suggestion: %+v", got)
			}
			if got.RegressionPromotion.SuggestedTestName == "" || got.RegressionPromotion.ExpectedOutcome == "" {
				t.Fatalf("promotion missing test metadata: %+v", got.RegressionPromotion)
			}
		})
	}
}

func TestFormatRouteGovernanceRegressionYAMLishIncludesPromotionMetadata(t *testing.T) {
	trace := replay.Trace{
		ID:          "trace_123",
		UserRequest: "Answer from local documents",
		Route:       replay.RouteSnapshot{Category: routing.RouteRAGSearch},
		Attributes: []replay.Attribute{
			{Key: "route_lane", Value: routing.ToolLaneDocument},
			{Key: "route_preflight_source_of_truth", Value: routing.PreflightSourceLocalDocuments},
			{Key: "continuation_mode", Value: routing.ContinuationModeContinueSameTask},
		},
		Verification: replay.VerificationResult{Status: "failed", Notes: []string{"no local document evidence"}},
	}
	classification := ClassifyRouteFailure(trace)
	preview := FormatRouteGovernanceRegressionYAMLish("", trace, classification)
	for _, want := range []string{
		RouteGovernanceRegressionSchema,
		"missing_evidence",
		"route_under_test",
		"observed_route",
		"expected_route",
		routing.RouteRAGSearch,
		routing.ToolLaneDocument,
		routing.ContinuationModeContinueSameTask,
	} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview missing %q:\n%s", want, preview)
		}
	}
}

func TestClassifyRouteFailureIncludesRouteRecoveryDecision(t *testing.T) {
	trace := replay.Trace{
		UserRequest: "Search current public news",
		Route:       replay.RouteSnapshot{Category: routing.RouteChatExplanation},
		Errors: []replay.TraceError{
			{Stage: "routing", Code: "route_mismatch", Message: "expected route did not match task frame", ExpectedRoute: routing.RouteInternetSearch},
		},
	}
	got := ClassifyRouteFailure(trace)
	if got.RouteRecovery.Trigger != routing.RouteRecoveryTriggerQAReviewRouteMismatch {
		t.Fatalf("RouteRecovery.Trigger = %q, want qa_review_route_mismatch; classification=%+v", got.RouteRecovery.Trigger, got)
	}
	if got.RouteRecovery.ProposedRoute != routing.RouteInternetSearch {
		t.Fatalf("RouteRecovery.ProposedRoute = %q, want internet_search; recovery=%+v", got.RouteRecovery.ProposedRoute, got.RouteRecovery)
	}
}

func TestRouteFailurePromotionSeparatesObservedAndExpectedRoute(t *testing.T) {
	trace := replay.Trace{
		ID:          "trace_wrong_route",
		UserRequest: "Use current public sources for this.",
		Route:       replay.RouteSnapshot{Category: routing.RouteChatExplanation},
		Errors: []replay.TraceError{{
			Stage:             "routing",
			Code:              "route_mismatch",
			Message:           "expected route did not match task frame",
			ExpectedRoute:     routing.RouteInternetSearch,
			ExpectedRiskLevel: routing.RiskMedium,
		}},
		Plan: replay.PlanSnapshot{ToolsNeeded: []string{"internet_search"}},
		ToolsCalled: []replay.ToolCall{
			{Name: "read_file", Status: "blocked"},
		},
	}
	classification := ClassifyRouteFailure(trace)
	promotion := classification.RegressionPromotion
	if promotion.ObservedRoute != routing.RouteChatExplanation {
		t.Fatalf("ObservedRoute = %q, want %q", promotion.ObservedRoute, routing.RouteChatExplanation)
	}
	if promotion.ExpectedRoute != routing.RouteInternetSearch || promotion.RouteUnderTest != routing.RouteInternetSearch {
		t.Fatalf("expected route metadata = %+v, want internet search", promotion)
	}
	if promotion.ExpectedRiskLevel != routing.RiskMedium {
		t.Fatalf("ExpectedRiskLevel = %q, want %q", promotion.ExpectedRiskLevel, routing.RiskMedium)
	}
	if !containsString(promotion.RequiredTools, "internet_search") {
		t.Fatalf("RequiredTools = %+v, want internet_search", promotion.RequiredTools)
	}
	if !containsString(promotion.ForbiddenTools, "read_file") {
		t.Fatalf("ForbiddenTools = %+v, want read_file", promotion.ForbiddenTools)
	}
}

func TestApplyRouteGovernanceCoverageUsesStaticAndEvalSources(t *testing.T) {
	trace := replay.Trace{
		UserRequest: "Search current public updates",
		Route:       replay.RouteSnapshot{Category: routing.RouteChatExplanation},
		Errors:      []replay.TraceError{{Stage: "routing", Code: "route_mismatch", Message: "expected route did not match task frame", ExpectedRoute: routing.RouteInternetSearch}},
	}
	classification := ClassifyRouteFailure(trace)
	if classification.CoveredByEvalOrTest {
		t.Fatalf("ClassifyRouteFailure covered = true before QA/eval coverage is applied: %+v", classification)
	}
	latestEval := &QAEvalSummary{
		Status: "pass",
		Tasks:  []QAEvalTaskSummary{{Name: "route_governance_classifier", Status: "pass"}},
	}
	classification = ApplyRouteGovernanceCoverage(trace, classification, latestEval)
	if !classification.CoveredByEvalOrTest {
		t.Fatalf("covered = false, want true: %+v", classification)
	}
	for _, want := range []string{
		"go_test:internal/learning/TestClassifyRouteFailureCategories",
		"eval:route_governance_classifier",
	} {
		if !containsString(classification.CoverageSources, want) {
			t.Fatalf("coverage sources = %+v, missing %q", classification.CoverageSources, want)
		}
	}
}
