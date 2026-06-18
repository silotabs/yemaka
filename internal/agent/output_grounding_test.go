package agent

import (
	"strings"
	"testing"
)

func TestPrepareAssistantPresentationStripsInternalModelAndToolArtifacts(t *testing.T) {
	content := strings.Join([]string{
		"<think>private route reasoning</think>",
		"Here is the answer.",
		"```json",
		"YEMAKA_TOOL_REQUEST",
		"{\"tool_name\":\"read_file\",\"path\":\"README.md\"}",
		"```",
		"TOOL RESULT:",
		"source_kind: workspace",
		"status: completed",
		"sources: README.md",
		"Status: readable assistant heading should stay.",
	}, "\n")

	response := PrepareAssistantPresentation(content, ExecutionDecision{ToolName: "read_file"}, &ExecutionResult{Status: "completed"})

	for _, forbidden := range []string{
		"<think>",
		"private route reasoning",
		"YEMAKA_TOOL_REQUEST",
		"TOOL RESULT",
		"source_kind:",
		"status: completed",
		"sources: README.md",
	} {
		if strings.Contains(response, forbidden) {
			t.Fatalf("PrepareAssistantPresentation() leaked %q:\n%s", forbidden, response)
		}
	}
	for _, want := range []string{
		"Here is the answer.",
		"Status: readable assistant heading should stay.",
	} {
		if !strings.Contains(response, want) {
			t.Fatalf("PrepareAssistantPresentation() missing %q:\n%s", want, response)
		}
	}
}

func TestPrepareAssistantPresentationRemovesRawToolFence(t *testing.T) {
	content := strings.Join([]string{
		"Before.",
		"```",
		"TOOL RESULT:",
		"status: completed",
		"sources: README.md",
		"```",
		"After.",
	}, "\n")

	response := PrepareAssistantPresentation(content, ExecutionDecision{ToolName: "read_file"}, &ExecutionResult{Status: "completed"})

	if strings.Contains(response, "TOOL RESULT") || strings.Contains(response, "status: completed") || strings.Contains(response, "sources: README.md") {
		t.Fatalf("PrepareAssistantPresentation() leaked fenced raw tool output:\n%s", response)
	}
	if !strings.Contains(response, "Before.") || !strings.Contains(response, "After.") {
		t.Fatalf("PrepareAssistantPresentation() removed visible prose:\n%s", response)
	}
}

func TestPrepareAssistantPresentationFallsBackToGroundedIngestSummary(t *testing.T) {
	result := ExecutionResult{
		Status: "completed",
		Context: strings.Join([]string{
			"DOCUMENT INGEST RESULT:",
			"target: /Users/example/Documents/email/Rocky_emails",
			"files_indexed: 0",
			"files_unchanged: 0",
			"files_skipped: 2",
			"chunks_created: 0",
			"bytes_indexed: 0",
		}, "\n"),
	}

	response := PrepareAssistantPresentation(result.Context, ExecutionDecision{ToolName: "ingest_documents"}, &result)

	for _, want := range []string{
		"Document ingest completed.",
		"Path: `/Users/example/Documents/email/Rocky_emails`",
		"Result: 0 indexed, 0 unchanged, 2 skipped, 0 chunks, 0 bytes.",
		"Skipped file names were not returned by the ingest tool.",
	} {
		if !strings.Contains(response, want) {
			t.Fatalf("PrepareAssistantPresentation() missing %q:\n%s", want, response)
		}
	}
	for _, forbidden := range []string{
		"DOCUMENT INGEST RESULT",
		"files_indexed:",
		"likely due to",
		"unsupported formats",
	} {
		if strings.Contains(response, forbidden) {
			t.Fatalf("PrepareAssistantPresentation() leaked/guessed %q:\n%s", forbidden, response)
		}
	}
}

func TestPrepareAssistantPresentationGroundsCompletedFetchInsteadOfFutureConfirmation(t *testing.T) {
	response := PrepareAssistantPresentation(
		strings.Join([]string{
			"**Step 1 - Fetch the target site**",
			"I will now run the *internet_fetch* tool against `https://example.com` to obtain a fresh HTTP response.",
			"",
			"> **Confirmation**: Please confirm if you want me to proceed with the fetch before I execute.",
		}, "\n"),
		ExecutionDecision{ToolName: "internet_fetch"},
		&ExecutionResult{
			Context: strings.Join([]string{
				"INTERNET RESULT:",
				"url: https://example.com",
				"final_url: https://www.example.com/",
				"method: GET",
				"status: 200",
			}, "\n"),
			Sources:    []string{"https://www.example.com/"},
			SourceKind: "internet",
			Status:     "completed",
		},
	)
	lower := strings.ToLower(response)
	for _, unwanted := range []string{"will now run", "please confirm", "before i execute"} {
		if strings.Contains(lower, unwanted) {
			t.Fatalf("PrepareAssistantPresentation() leaked completed-tool future claim %q:\n%s", unwanted, response)
		}
	}
	if !strings.Contains(response, "I checked `https://www.example.com/`") || !strings.Contains(response, "HTTP 200") {
		t.Fatalf("PrepareAssistantPresentation() did not produce grounded fetch summary:\n%s", response)
	}
}

func TestPrepareAssistantPresentationStripsNaturalLanguageReasoningScaffold(t *testing.T) {
	content := strings.Join([]string{
		"The concept is not present in the available local context or workspace files.",
		"",
		"**Assumptions:**",
		"- The request is a reasoning task.",
		"- The source of truth is the chat context, as specified in the task plan.",
		"",
		"**Reasoning step-by-step:**",
		"- The task is to explain a technical concept.",
		"",
		"**Conclusion:** Given no retrieved files, the answer cannot be generated.",
		"",
		"👉 **Final response:** Cogging torque is the uneven rotational force a motor can feel because its rotor magnets prefer certain positions relative to the stator.",
	}, "\n")

	response := PrepareAssistantPresentation(content, ExecutionDecision{}, nil)

	for _, forbidden := range []string{
		"Assumptions",
		"Reasoning step-by-step",
		"task plan",
		"Final response:",
		"not present in the available local context",
	} {
		if strings.Contains(response, forbidden) {
			t.Fatalf("PrepareAssistantPresentation() leaked %q:\n%s", forbidden, response)
		}
	}
	if !strings.Contains(response, "Cogging torque is the uneven rotational force") {
		t.Fatalf("PrepareAssistantPresentation() lost final answer:\n%s", response)
	}
}

func TestPrepareAssistantPresentationStripsTaskExecutionScaffold(t *testing.T) {
	content := strings.Join([]string{
		"**Task Execution Summary:**",
		"- **Task Type:** Internet Search",
		"- **Route Confidence:** 91%",
		"- **Tool Used:** `internet_search`",
		"",
		"Search completed. The newest public sources are shown below.",
	}, "\n")

	response := PrepareAssistantPresentation(content, ExecutionDecision{ToolName: "internet_search"}, &ExecutionResult{Status: "completed", Sources: []string{"https://example.test"}})

	for _, forbidden := range []string{"Task Execution Summary", "Route Confidence", "Tool Used", "Task Type"} {
		if strings.Contains(response, forbidden) {
			t.Fatalf("PrepareAssistantPresentation() leaked %q:\n%s", forbidden, response)
		}
	}
	if !strings.Contains(response, "Search completed") {
		t.Fatalf("PrepareAssistantPresentation() removed user-facing summary:\n%s", response)
	}
}

func TestGroundUnsupportedActionClaimDowngradesSavedClaim(t *testing.T) {
	plan := BuildPlan(PlanInput{Content: "Save your last response as trends.md"})
	response := GroundUnsupportedActionClaim("I saved the file as trends.md.", plan, ExecutionDecision{}, nil)
	lower := strings.ToLower(response)
	for _, unwanted := range []string{"i saved", "saved the file", "saved as"} {
		if strings.Contains(lower, unwanted) {
			t.Fatalf("GroundUnsupportedActionClaim() leaked unsupported claim %q:\n%s", unwanted, response)
		}
	}
	for _, want := range []string{"unverified", "trusted tool evidence", "pending"} {
		if !strings.Contains(lower, want) {
			t.Fatalf("GroundUnsupportedActionClaim() missing %q:\n%s", want, response)
		}
	}
	if result := VerifyResponse(plan, response); result.Status != "pass" {
		t.Fatalf("VerifyResponse() after downgrade = %q, want pass; result=%+v response=%q", result.Status, result, response)
	}
}

func TestGroundUnsupportedActionClaimAllowsTrustedWriteEvidence(t *testing.T) {
	plan := BuildPlan(PlanInput{Content: "Save your last response as trends.md"})
	decision := ExecutionDecision{ToolName: "write_file", Status: ExecutionReady}
	result := &ExecutionResult{Status: "completed", SourceKind: "workspace"}
	content := "I saved the file as trends.md."
	response := GroundUnsupportedActionClaim(content, plan, decision, result)
	if response != content {
		t.Fatalf("GroundUnsupportedActionClaim() = %q, want trusted success wording preserved", response)
	}
	verification := VerifyResponseWithEvidence(plan, response, decision, result)
	if verification.Status != "pass" {
		t.Fatalf("VerifyResponseWithEvidence() = %q, want pass; result=%+v", verification.Status, verification)
	}
}

func TestGroundUnsupportedActionClaimAllowsNormalFactualConfirmedWording(t *testing.T) {
	plan := BuildPlan(PlanInput{Content: "What did the report say?"})
	content := "The report confirmed the rollout timeline and summarized the risks."
	response := GroundUnsupportedActionClaim(content, plan, ExecutionDecision{}, nil)
	if response != content {
		t.Fatalf("GroundUnsupportedActionClaim() = %q, want factual confirmed wording unchanged", response)
	}
	verification := VerifyResponse(plan, response)
	if verification.Status != "pass" {
		t.Fatalf("VerifyResponse() = %q, want pass; result=%+v", verification.Status, verification)
	}
}

func TestActionRiskRoutesBufferUntilGrounding(t *testing.T) {
	plan := BuildPlan(PlanInput{Content: "Save your last response as trends.md"})
	if shouldStreamInitialModelOutput(plan, nil, nil) {
		t.Fatal("shouldStreamInitialModelOutput() = true, want action-risk routes buffered until grounding")
	}
	if !shouldEmitBufferedAfterGrounding(plan, false) {
		t.Fatal("shouldEmitBufferedAfterGrounding() = false, want post-grounding emit for buffered action-risk route")
	}
	rewrite := BuildPlan(PlanInput{Content: "make this better: hello dear sir"})
	if !shouldStreamInitialModelOutput(rewrite, nil, nil) {
		t.Fatal("shouldStreamInitialModelOutput(rewrite) = false, want normal rewrite streaming allowed")
	}
}
