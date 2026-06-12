package learning

import (
	"strings"
	"testing"

	"yemaka/internal/replay"
)

func TestGenerateRegressionCaseForRoutingFalsePositive(t *testing.T) {
	trace := replay.NewTrace("Explain website monitoring")
	trace.ID = "trace-route-1"
	trace.Route = replay.RouteSnapshot{
		Category:                 "scheduler_create",
		RiskLevel:                "medium",
		ShouldUseTool:            true,
		ShouldCreateSchedulerJob: true,
	}
	trace.Plan = replay.PlanSnapshot{ToolsNeeded: []string{"scheduler_create"}}
	trace.ToolsCalled = []replay.ToolCall{{Name: "scheduler_create", Status: "completed"}}

	testCase, ok := GenerateRegressionTestCase(trace)
	if !ok {
		t.Fatal("GenerateRegressionTestCase() ok = false, want true")
	}
	if testCase.Kind != FailureKindRouting {
		t.Fatalf("Kind = %q, want routing failure", testCase.Kind)
	}
	if testCase.Expected.Route != "chat_explanation" || testCase.Expected.ShouldCreateSchedulerJob {
		t.Fatalf("Expected = %+v, want chat route without scheduler job", testCase.Expected)
	}
	if !containsString(testCase.Expected.ForbiddenTools, "scheduler_create") {
		t.Fatalf("ForbiddenTools = %v, want scheduler_create", testCase.Expected.ForbiddenTools)
	}

	text := FormatRegressionTestCaseYAMLish(testCase)
	for _, want := range []string{
		`kind: "routing_failure"`,
		`prompt: "Explain website monitoring"`,
		`route: "chat_explanation"`,
		`should_create_scheduler_job: false`,
		`  - "scheduler_create"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("YAML-ish text missing %q:\n%s", want, text)
		}
	}
}

func TestGenerateRegressionCaseSanitizesSecrets(t *testing.T) {
	trace := replay.NewTrace("Search docs with token=super-secret-value")
	trace.ID = "trace-secret"
	trace.Route = replay.RouteSnapshot{Category: "internet_search", RiskLevel: "medium"}
	trace.PermissionsRequested = []replay.PermissionRequest{{
		ToolName:          "internet_search",
		Status:            "denied",
		RiskLevel:         "medium",
		Reason:            "internet disabled for token=super-secret-value",
		PolicyExplanation: "internet requires approval",
	}}

	testCase, ok := GenerateRegressionTestCase(trace)
	if !ok {
		t.Fatal("GenerateRegressionTestCase() ok = false, want true")
	}
	if testCase.Kind != FailureKindPermission {
		t.Fatalf("Kind = %q, want permission failure", testCase.Kind)
	}
	if !testCase.Expected.ShouldAskApproval || !testCase.Expected.ShouldUseInternet {
		t.Fatalf("Expected = %+v, want internet approval gate", testCase.Expected)
	}
	text := FormatRegressionTestCaseYAMLish(testCase)
	if strings.Contains(text, "super-secret-value") {
		t.Fatalf("YAML-ish text leaked secret:\n%s", text)
	}
	if !strings.Contains(text, `token=[REDACTED]`) {
		t.Fatalf("YAML-ish text = %q, want redaction marker", text)
	}
}

func TestGenerateRegressionCasesClassifiesCommonFailures(t *testing.T) {
	tests := []struct {
		name  string
		trace replay.Trace
		kind  string
	}{
		{
			name: "internet",
			trace: traceWithToolFailure("Find current docs", "internet_search", "provider not configured", replay.RouteSnapshot{
				Category:  "internet_search",
				RiskLevel: "medium",
			}),
			kind: FailureKindInternetSearch,
		},
		{
			name: "extension",
			trace: traceWithToolFailure("Generate an extension", "extension_generate", "manifest validation failed", replay.RouteSnapshot{
				Category:  "extension_generate",
				RiskLevel: "medium",
			}),
			kind: FailureKindExtensionGeneration,
		},
		{
			name: "document",
			trace: traceWithToolFailure("Ingest this PDF", "pdf_extract", "unsupported PDF encoding", replay.RouteSnapshot{
				Category: "rag_search",
			}),
			kind: FailureKindDocumentIngestion,
		},
		{
			name: "rag",
			trace: traceWithToolFailure("Use project docs", "rag_search", "no retrieval results", replay.RouteSnapshot{
				Category: "rag_search",
			}),
			kind: FailureKindRAGRetrieval,
		},
		{
			name: "ui",
			trace: traceWithError("Retry response", replay.TraceError{
				Stage:   "ui",
				Code:    "stream_render_failed",
				Message: "streaming output stalled",
			}),
			kind: FailureKindUIFlow,
		},
		{
			name: "generic tool",
			trace: traceWithToolFailure("Run tests", "run_tests", "exit status 1", replay.RouteSnapshot{
				Category:  "shell_tool",
				RiskLevel: "low",
			}),
			kind: FailureKindToolCall,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cases := GenerateRegressionTestCases(tt.trace)
			if !hasCaseKind(cases, tt.kind) {
				t.Fatalf("GenerateRegressionTestCases() = %+v, want kind %q", cases, tt.kind)
			}
		})
	}
}

func TestGenerateRegressionCasesFromLiveTraceWithRoutingToolAndInternetFailures(t *testing.T) {
	trace := replay.NewTrace("Explain the release checklist and include current upstream notes")
	trace.ID = "trace-live-regression-1"
	trace.AppendRouteSnapshot(replay.RouteSnapshot{
		Category:          "internet_search",
		RiskLevel:         "medium",
		ShouldUseTool:     true,
		ShouldUseInternet: true,
	})
	trace.AppendPlanSnapshot(replay.PlanSnapshot{
		Goal:        "answer with current notes",
		ToolsNeeded: []string{"internet_search", "run_tests"},
	})
	trace.AppendError(replay.TraceError{
		Stage:             "routing",
		Code:              "should_have_stayed_local",
		Subject:           "internet_search",
		Message:           "explanation request routed to an internet action path",
		ExpectedRoute:     "chat_explanation",
		ExpectedRiskLevel: "low",
	})
	trace.AppendToolCall(replay.ToolCall{
		Name:   "internet_search",
		Status: "failed",
		Error:  "SearXNG provider not configured",
	})
	trace.AppendToolCall(replay.ToolCall{
		Name:   "run_tests",
		Status: "failed",
		Error:  "exit status 1",
	})
	trace.AppendFinalResult(replay.ResultSnapshot{
		Status:  "failed",
		Summary: "routing and tool failures captured",
	})

	cases := GenerateRegressionTestCases(trace)
	for _, want := range []string{FailureKindRouting, FailureKindInternetSearch, FailureKindToolCall} {
		if !hasCaseKind(cases, want) {
			t.Fatalf("GenerateRegressionTestCases() = %+v, want kind %q", cases, want)
		}
	}

	routing := caseByKind(cases, FailureKindRouting)
	if routing.Expected.Route != "chat_explanation" || routing.Expected.ShouldUseInternet {
		t.Fatalf("routing Expected = %+v, want local chat explanation without internet", routing.Expected)
	}
	internet := caseByKind(cases, FailureKindInternetSearch)
	if !internet.Expected.ShouldUseInternet || !internet.Expected.ShouldAskApproval || !containsString(internet.Expected.RequiredTools, "internet_search") {
		t.Fatalf("internet Expected = %+v, want gated internet_search", internet.Expected)
	}
	tool := caseByKind(cases, FailureKindToolCall)
	if !tool.Expected.ShouldUseTool || !containsString(tool.Expected.RequiredTools, "run_tests") {
		t.Fatalf("tool Expected = %+v, want failed generic tool captured", tool.Expected)
	}
}

func TestFormatRegressionTestCaseYAMLishIsDeterministic(t *testing.T) {
	trace := traceWithToolFailure("Run tests", "run_tests", "exit status 1", replay.RouteSnapshot{
		Category:  "shell_tool",
		RiskLevel: "low",
	})
	testCase, ok := GenerateRegressionTestCase(trace)
	if !ok {
		t.Fatal("GenerateRegressionTestCase() ok = false, want true")
	}
	first := FormatRegressionTestCaseYAMLish(testCase)
	second := FormatRegressionTestCaseYAMLish(testCase)
	if first != second {
		t.Fatalf("YAML-ish output changed between calls:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func traceWithToolFailure(prompt string, tool string, message string, route replay.RouteSnapshot) replay.Trace {
	trace := replay.NewTrace(prompt)
	trace.Route = route
	trace.ToolsCalled = []replay.ToolCall{{
		Name:   tool,
		Status: "failed",
		Error:  message,
	}}
	return trace
}

func traceWithError(prompt string, err replay.TraceError) replay.Trace {
	trace := replay.NewTrace(prompt)
	trace.Errors = []replay.TraceError{err}
	return trace
}

func hasCaseKind(cases []RegressionTestCase, kind string) bool {
	for _, testCase := range cases {
		if testCase.Kind == kind {
			return true
		}
	}
	return false
}

func caseByKind(cases []RegressionTestCase, kind string) RegressionTestCase {
	for _, testCase := range cases {
		if testCase.Kind == kind {
			return testCase
		}
	}
	return RegressionTestCase{}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
