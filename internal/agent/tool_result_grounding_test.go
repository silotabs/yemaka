package agent

import (
	"fmt"
	"strings"
	"testing"

	"yemaka/internal/routing"
)

func TestGroundFailedRunTestsResponseDisclosesMissingPytest(t *testing.T) {
	response := GroundFailedToolResponse(
		"Should I run the tests now?",
		ExecutionDecision{ToolName: "run_tests"},
		&ExecutionResult{
			Status:  "failed",
			Context: "COMMAND RESULT:\nCOMMAND: python3 -m pytest\nEXIT_CODE: 1\n\nSTDERR:\npython3: No module named pytest",
		},
	)
	if !strings.Contains(response, "missing `pytest`") {
		t.Fatalf("response = %q, want missing pytest disclosure", response)
	}
	if !strings.Contains(response, "I did not apply any file changes") {
		t.Fatalf("response = %q, want no file changes disclosure", response)
	}
}

func TestGroundFailedToolResponseDoesNotDuplicateDisclosure(t *testing.T) {
	input := "I could not run the tests because pytest is not installed."
	response := GroundFailedToolResponse(
		input,
		ExecutionDecision{ToolName: "run_tests"},
		&ExecutionResult{Status: "failed", Context: "No module named pytest"},
	)
	if response != input {
		t.Fatalf("response = %q, want original disclosure", response)
	}
}

func TestFriendlyToolFailureResponseDisclosesMissingPytest(t *testing.T) {
	response := FriendlyToolFailureResponse(
		ExecutionDecision{ToolName: "run_tests"},
		fmt.Errorf("/usr/bin/python3: No module named pytest"),
	)
	if !strings.Contains(response, "missing `pytest`") {
		t.Fatalf("response = %q, want missing pytest disclosure", response)
	}
	if strings.Contains(response, "local tool stopped") {
		t.Fatalf("response = %q, want specific pytest failure instead of generic tool stop", response)
	}
	if !strings.Contains(response, "I did not change any files") {
		t.Fatalf("response = %q, want no file changes disclosure", response)
	}
}

func TestFriendlyToolFailureResponseDisclosesMissingPythonTests(t *testing.T) {
	response := FriendlyToolFailureResponse(
		ExecutionDecision{ToolName: "run_tests"},
		fmt.Errorf("python tests were requested, but no Python tests or pytest config were detected"),
	)
	if !strings.Contains(response, "could not find Python tests") {
		t.Fatalf("response = %q, want missing Python tests disclosure", response)
	}
	if strings.Contains(response, "local tool stopped") {
		t.Fatalf("response = %q, want specific missing-tests failure instead of generic tool stop", response)
	}
}

func TestFriendlyToolFailureResponseDisclosesMissingFile(t *testing.T) {
	response := FriendlyToolFailureResponse(
		ExecutionDecision{ToolName: "read_file", Command: []string{"read_file", "test-files/permission_smoke.md"}},
		fmt.Errorf("stat file: open test-files/permission_smoke.md: no such file or directory"),
	)
	if !strings.Contains(response, "not found in the current workspace") {
		t.Fatalf("response = %q, want missing file disclosure", response)
	}
	if !strings.Contains(response, "test-files/permission_smoke.md") {
		t.Fatalf("response = %q, want missing file path", response)
	}
	if strings.Contains(response, "local tool stopped") {
		t.Fatalf("response = %q, want specific missing-file response instead of generic tool stop", response)
	}
}

func TestFriendlyToolFailureResponseNamesSelectedSearchProviderAndEnv(t *testing.T) {
	response := FriendlyToolFailureResponse(
		ExecutionDecision{ToolName: "internet_search"},
		fmt.Errorf("Firecrawl search API key environment variable FIRECRAWL_API_KEY is not set or not visible to Yemaka"),
	)
	if !strings.Contains(response, "Firecrawl is selected") || !strings.Contains(response, "FIRECRAWL_API_KEY") {
		t.Fatalf("response = %q, want provider-specific env guidance", response)
	}
	if strings.Contains(response, "SearXNG needs") || strings.Contains(response, "Brave needs") {
		t.Fatalf("response = %q, should not use generic legacy provider guidance", response)
	}
}

func TestRouteAwareToolFailureResponseExplainsBlockedLane(t *testing.T) {
	response := RouteAwareToolFailureResponse(
		Plan{
			RouteCategory:     routing.RouteRAGSearch,
			RouteLane:         routing.ToolLaneDocument,
			RouteBlockedTools: []string{"internet_search"},
		},
		ExecutionDecision{ToolName: "internet_search"},
		fmt.Errorf("search provider not ready"),
	)
	if !strings.Contains(response, "outside the selected route lane") || !strings.Contains(response, routing.ToolLaneDocument) {
		t.Fatalf("response = %q, want blocked lane explanation", response)
	}
}

func TestRouteAwareToolFailureResponseExplainsSearchProviderRoute(t *testing.T) {
	response := RouteAwareToolFailureResponse(
		Plan{RouteCategory: routing.RouteInternetSearch, RouteLane: routing.ToolLaneWebSearch, RouteUsesInternet: true},
		ExecutionDecision{ToolName: "internet_search"},
		fmt.Errorf("could not reach SearXNG search provider: connection refused"),
	)
	if !strings.Contains(response, "web-search route") || !strings.Contains(response, "SearXNG") {
		t.Fatalf("response = %q, want web-search provider guidance", response)
	}
}

func TestRouteAwareToolFailureResponseExplainsWorkspacePermission(t *testing.T) {
	response := RouteAwareToolFailureResponse(
		Plan{RouteCategory: routing.RouteFileRead, RouteLane: routing.ToolLaneResearch},
		ExecutionDecision{ToolName: "read_file"},
		fmt.Errorf("open /Users/example/Documents/report.md: operation not permitted"),
	)
	if !strings.Contains(response, "Grant the exact folder") || !strings.Contains(response, "did not use another source") {
		t.Fatalf("response = %q, want workspace grant guidance", response)
	}
}

func TestRouteAwareToolFailureResponseExplainsUploadedCopyVsGrantPath(t *testing.T) {
	response := RouteAwareToolFailureResponse(
		Plan{RouteCategory: routing.RouteFileRead, RouteLane: routing.ToolLaneResearch, EvidenceSource: "attachment"},
		ExecutionDecision{ToolName: "read_file"},
		fmt.Errorf("uploaded copy cannot be opened as a filesystem path: operation not permitted"),
	)
	if !strings.Contains(response, "conversation context") || !strings.Contains(response, "grant/select the original folder") {
		t.Fatalf("response = %q, want upload-copy versus granted-path guidance", response)
	}
}

func TestRouteAwareToolFailureResponseExplainsLocalDocumentNoEvidence(t *testing.T) {
	response := RouteAwareToolFailureResponse(
		Plan{RouteCategory: routing.RouteRAGSearch, RouteLane: routing.ToolLaneDocument, RouteUsesRAG: true},
		ExecutionDecision{ToolName: "rag_search"},
		fmt.Errorf("no retrieved local document context"),
	)
	if !strings.Contains(response, "local-document evidence") || !strings.Contains(response, "did not answer from stale memory or web search") {
		t.Fatalf("response = %q, want no-evidence route guidance", response)
	}
}

func TestRouteAwareToolFailureResponseExplainsLocalDocumentRuntime(t *testing.T) {
	response := RouteAwareToolFailureResponse(
		Plan{RouteCategory: routing.RouteRAGSearch, RouteLane: routing.ToolLaneDocument, RouteUsesRAG: true},
		ExecutionDecision{ToolName: "rag_search"},
		fmt.Errorf("ollama embed request failed: context deadline exceeded"),
	)
	if !strings.Contains(response, "local embedding/runtime") || !strings.Contains(response, "did not use web search") {
		t.Fatalf("response = %q, want local embedding runtime guidance without web fallback", response)
	}
}

func TestRouteAwareToolFailureResponseExplainsMissingCapability(t *testing.T) {
	response := RouteAwareToolFailureResponse(
		Plan{RouteCategory: routing.RouteExtensionGenerate, RouteLane: routing.ToolLaneExtension},
		ExecutionDecision{ToolName: "unknown_tool"},
		fmt.Errorf("no matching safe executor for unknown_tool"),
	)
	if !strings.Contains(response, "capability") || !strings.Contains(response, "generated extension") {
		t.Fatalf("response = %q, want missing-capability guidance", response)
	}
}

func TestGroundFailedRunTestsResponseRemovesFutureExecutionClaim(t *testing.T) {
	input := strings.Join([]string{
		"Static inspection found the likely bug.",
		"",
		"### **Next Action**",
		"",
		"I will now execute:",
		"",
		"```bash",
		"python test-files/test_calculator.py",
		"```",
		"",
		"to confirm the failure pattern.",
	}, "\n")
	response := GroundFailedToolResponse(
		input,
		ExecutionDecision{ToolName: "run_tests"},
		&ExecutionResult{
			Status:  "failed",
			Context: "COMMAND RESULT:\nCOMMAND: python3 -m pytest\nEXIT_CODE: 1\n\nSTDERR:\npython3: No module named pytest",
		},
	)
	if strings.Contains(strings.ToLower(response), "i will now execute") {
		t.Fatalf("response = %q, want future execution claim removed", response)
	}
	if strings.Contains(response, "python test-files/test_calculator.py") {
		t.Fatalf("response = %q, want unapproved fallback command removed", response)
	}
	if !strings.Contains(response, "missing `pytest`") {
		t.Fatalf("response = %q, want missing pytest disclosure", response)
	}
	if !strings.Contains(response, "install `pytest`") {
		t.Fatalf("response = %q, want concrete next step", response)
	}
}

func TestGroundFailedRunTestsResponseRemovesMissingPytestPlanCommands(t *testing.T) {
	input := strings.Join([]string{
		"### **Step 1: Classify the Request**",
		"The test files show that add returns subtraction.",
		"",
		"### **Step 3: Proposed Action**",
		"Since pytest is missing, I will check if unittest is available.",
		"",
		"**Command**:",
		"```bash",
		"cd test-files && python3 -m unittest test_calculator.py",
		"```",
		"",
		"**Will I proceed with this command?** *(Yes, to verify test failures.)*",
	}, "\n")
	response := GroundFailedToolResponse(
		input,
		ExecutionDecision{ToolName: "run_tests"},
		&ExecutionResult{
			Status:  "failed",
			Context: "COMMAND RESULT:\nCOMMAND: python3 -m pytest\nEXIT_CODE: 1\n\nSTDERR:\npython3: No module named pytest",
		},
	)
	lower := strings.ToLower(response)
	for _, unwanted := range []string{"i will check", "python3 -m unittest", "will i proceed"} {
		if strings.Contains(lower, unwanted) {
			t.Fatalf("response = %q, want %q removed", response, unwanted)
		}
	}
	if !strings.Contains(response, "missing `pytest`") {
		t.Fatalf("response = %q, want missing pytest disclosure", response)
	}
	if strings.Contains(response, "test-files/calculator.py") {
		t.Fatalf("response = %q, did not expect fixture-specific path", response)
	}
	if strings.Contains(response, "`add(a, b)` should return `a + b`") {
		t.Fatalf("response = %q, did not expect sanitizer to invent a fixture-specific fix", response)
	}
	if !strings.Contains(response, "wait for your approval") {
		t.Fatalf("response = %q, want approval boundary", response)
	}
	if !strings.Contains(response, "snapshot-backed diff") {
		t.Fatalf("response = %q, want generic snapshot-backed next step", response)
	}
}
