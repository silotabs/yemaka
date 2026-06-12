package agent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/learning"
	"yemaka/internal/routing"
)

func TestReplayFailureHelperWritesRedactedRegressionArtifact(t *testing.T) {
	input := PlanInput{Content: "Search current docs with token=super-secret-value"}
	plan := BuildPlan(input)
	decision := ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "internet_search",
		Command:   []string{"internet_search", "current docs token=super-secret-value"},
		RiskLevel: RiskMedium,
		Reason:    "internet search requested by route",
	}
	result := &ExecutionResult{
		Context:    "provider not configured; password=hunter2",
		SourceKind: "tool",
		Status:     "failed",
	}

	request, ok := RegressionArtifactRequestFromFailure(ReplayFailureInput{
		Input:             input,
		Plan:              plan,
		Decision:          decision,
		Result:            result,
		Err:               errors.New("SearXNG provider failed with api_key=abc123"),
		ExpectedRoute:     routing.RouteInternetSearch,
		ExpectedRiskLevel: RiskMedium,
	})
	if !ok {
		t.Fatal("RegressionArtifactRequestFromFailure() ok = false, want true")
	}
	if request.Trace.Route.Category != routing.RouteInternetSearch {
		t.Fatalf("Trace.Route.Category = %q, want %q", request.Trace.Route.Category, routing.RouteInternetSearch)
	}
	if len(request.Trace.ToolsCalled) != 1 || request.Trace.ToolsCalled[0].Name != "internet_search" {
		t.Fatalf("Trace.ToolsCalled = %+v, want internet_search failure", request.Trace.ToolsCalled)
	}
	traceText := request.Trace.UserRequest + " " + request.Trace.ToolsCalled[0].InputSummary + " " + request.Trace.ToolsCalled[0].OutputSummary + " " + request.Trace.ToolsCalled[0].Error
	for _, leaked := range []string{"super-secret-value", "hunter2", "abc123"} {
		if strings.Contains(traceText, leaked) {
			t.Fatalf("trace leaked %q: %+v", leaked, request.Trace)
		}
	}
	if len(request.Cases) == 0 || request.Cases[0].Kind != learning.FailureKindInternetSearch {
		t.Fatalf("Cases = %+v, want internet search regression case", request.Cases)
	}

	artifacts, err := SaveRegressionArtifactsFromFailure(t.TempDir(), ReplayFailureInput{
		Input:             input,
		Plan:              plan,
		Decision:          decision,
		Result:            result,
		Err:               errors.New("SearXNG provider failed with api_key=abc123"),
		ExpectedRoute:     routing.RouteInternetSearch,
		ExpectedRiskLevel: RiskMedium,
	})
	if err != nil {
		t.Fatalf("SaveRegressionArtifactsFromFailure() error = %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("SaveRegressionArtifactsFromFailure() wrote %d artifacts, want 1", len(artifacts))
	}
	content, err := os.ReadFile(filepath.Clean(artifacts[0].Path))
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	text := string(content)
	for _, leaked := range []string{"super-secret-value", "hunter2", "abc123"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("artifact leaked %q:\n%s", leaked, text)
		}
	}
	if !strings.Contains(text, "[REDACTED]") {
		t.Fatalf("artifact did not include redaction marker:\n%s", text)
	}
}

func TestRegressionArtifactRequestFromFailureSkipsSuccessfulTrace(t *testing.T) {
	input := PlanInput{Content: "What time is it?"}
	plan := BuildPlan(input)
	_, ok := RegressionArtifactRequestFromFailure(ReplayFailureInput{
		Input: input,
		Plan:  plan,
		Decision: ExecutionDecision{
			Status:    ExecutionReady,
			ToolName:  "local_time",
			RiskLevel: RiskLow,
		},
		Result: &ExecutionResult{Status: "completed", Context: "12:00", SourceKind: "local_time"},
	})
	if ok {
		t.Fatal("RegressionArtifactRequestFromFailure() ok = true, want false for successful trace")
	}
}
