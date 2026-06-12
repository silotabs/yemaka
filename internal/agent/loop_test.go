package agent

import (
	"strings"
	"testing"
	"time"

	promptcore "yemaka/internal/prompt"
	"yemaka/internal/routing"
)

func TestPromptMessagesWithOptionsBoundsLargeContext(t *testing.T) {
	input := PlanInput{
		Content:           "Explain this project",
		ProfileMemory:     "Prefer concise answers.",
		WorkspaceContext:  strings.Repeat("README.md local-first agent context.\n", 600),
		SkillName:         "project_explainer",
		SkillInstructions: "Explain the project using retrieved files.",
	}
	plan := BuildPlan(input)
	messages := PromptMessagesWithOptions(plan, input, "system", promptcore.Options{MaxChars: 2600, LowMemory: true})
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(messages))
	}
	user := messages[1].Content
	if len(user) > 2600 {
		t.Fatalf("user prompt len = %d, want <= 2600", len(user))
	}
	for _, expected := range []string{"TASK PLAN:", "PROFILE MEMORY:", "RETRIEVED CONTEXT:", "SKILL INSTRUCTIONS:", "USER REQUEST:"} {
		if !strings.Contains(user, expected) {
			t.Fatalf("prompt missing %s:\n%s", expected, user)
		}
	}
	if !strings.Contains(user, "trimmed for low-memory prompt budget") {
		t.Fatalf("prompt was not visibly trimmed:\n%s", user)
	}
}

func TestCompilePromptContextSectionsDropsOversizedLowerPriorityContext(t *testing.T) {
	input := PlanInput{
		Content:          "Explain this project architecture from retrieved context.",
		TaskMemory:       strings.Repeat("old unrelated task memory ", 500) + "irrelevant-tail",
		WorkspaceContext: strings.Repeat("README.md local-first routing context.\n", 120),
	}

	sections := compilePromptContextSections(input, promptcore.Options{MaxChars: 2200, LowMemory: true})
	rendered := renderTestSections(sections)
	if strings.Contains(rendered, "irrelevant-tail") {
		t.Fatalf("oversized lower-priority task memory reached prompt sections:\n%s", rendered)
	}
	if !strings.Contains(rendered, "RETRIEVED CONTEXT") {
		t.Fatalf("retrieved context was not preserved:\n%s", rendered)
	}
	if !strings.Contains(rendered, "truncated for context budget") {
		t.Fatalf("retrieved context was not compiled/truncated before prompt assembly:\n%s", rendered)
	}
}

func TestCompilePromptContextCompactsLongChatWithoutLosingActiveRoute(t *testing.T) {
	input := PlanInput{
		Content:       "continue",
		TaskMemory:    strings.Repeat("old small talk and duplicate logs ", 400),
		SourceKind:    "rag",
		Sources:       []string{"docs/motors/cogging-torque.md"},
		ProfileMemory: "User constraint: only use ingested documents for this study session.",
		SessionContract: routing.SessionContract{
			ActiveGoal:     "Quiz me from the selected motor design document.",
			ActiveRoute:    routing.RouteRAGSearch,
			ActiveTarget:   "docs/motors/cogging-torque.md",
			ContextSources: []string{"docs/motors/cogging-torque.md"},
			LastOutcome:    routing.LastOutcomeCompleted,
			CompletedSteps: []string{"rag_search:completed"},
			UpdatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		},
	}
	plan := BuildPlan(input)
	sections := compilePromptContext(plan, input, promptcore.Options{MaxChars: 2200, LowMemory: true}).Sections
	rendered := renderTestSections(sections)
	for _, want := range []string{"COMPACT TASK STATE", "active_route: rag_search", "docs/motors/cogging-torque.md", "only use ingested documents"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("compiled prompt missing %q:\n%s", want, rendered)
		}
	}
}

func TestCompilePromptContextPreservesPendingApprovalAndSelectedFolder(t *testing.T) {
	input := PlanInput{
		Content: "yes, approve the edit",
		SessionContract: routing.SessionContract{
			ActiveGoal:      "Update the project README.",
			ActiveRoute:     routing.RouteFileWrite,
			ActiveTarget:    "README.md",
			PendingApproval: "edit_file",
			PendingStep:     "wait for edit approval",
			ContextSources:  []string{"README.md", "docs/"},
			UpdatedAt:       time.Now().UTC().Format(time.RFC3339Nano),
		},
	}
	plan := BuildPlan(input)
	rendered := renderTestSections(compilePromptContext(plan, input, promptcore.Options{MaxChars: 2200, LowMemory: true}).Sections)
	for _, want := range []string{"pending_approval: edit_file", "README.md", "docs/", "wait for edit approval"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("compiled prompt missing %q:\n%s", want, rendered)
		}
	}
}

func TestCompilePromptContextPreservesPendingClarificationAndRedactsSecrets(t *testing.T) {
	input := PlanInput{
		Content: "status",
		SessionContract: routing.SessionContract{
			ActiveGoal:           "Check the selected website.",
			ActiveRoute:          routing.RouteInternetHead,
			ActiveTarget:         "https://example.com",
			PendingClarification: "Do you mean status, content, design, or security headers?",
			PendingStep:          "waiting for website-check clarification",
			ContextSources:       []string{"https://example.com"},
			FailureReason:        "provider retry included api_key=sk-abcdefghijklmnopqrstuvwxyz",
			UpdatedAt:            time.Now().UTC().Format(time.RFC3339Nano),
		},
	}
	plan := BuildPlan(input)
	rendered := renderTestSections(compilePromptContext(plan, input, promptcore.Options{MaxChars: 2400, LowMemory: true}).Sections)
	for _, want := range []string{
		"pending_clarification: Do you mean status, content, design, or security headers?",
		"https://example.com",
		"waiting for website-check clarification",
		"api_key=[REDACTED]",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("compiled prompt missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "sk-abcdefghijklmnopqrstuvwxyz") {
		t.Fatalf("compiled prompt leaked secret-like value:\n%s", rendered)
	}
}

func TestCompilePromptContextFreshNewsKeepsStaleMemoryAsBackground(t *testing.T) {
	input := PlanInput{
		Content:       "Search current US-China news.",
		TaskMemory:    "Memory: old US-China news from last month said tariffs changed.",
		MemorySources: []string{"memory:old-us-china-note"},
	}
	plan := BuildPlan(input)
	rendered := renderTestSections(compilePromptContext(plan, input, promptcore.Options{MaxChars: 2400, LowMemory: true}).Sections)
	if !strings.Contains(rendered, "freshness_guard") || !strings.Contains(rendered, "must not be treated as current facts") {
		t.Fatalf("compiled prompt missing freshness guard:\n%s", rendered)
	}
	if !strings.Contains(rendered, "active_route: internet_search") {
		t.Fatalf("compiled prompt missing current-news route:\n%s", rendered)
	}
}

func TestBuildPlanRoutesPoliteSearchAsInternetSearch(t *testing.T) {
	plan := BuildPlan(PlanInput{Content: "Okay now search SearXNG and tell me what it is for"})
	if plan.TaskType != TaskTool {
		t.Fatalf("TaskType = %q, want %q", plan.TaskType, TaskTool)
	}
	if !containsTool(plan.ToolsNeeded, "internet_search") {
		t.Fatalf("ToolsNeeded = %#v, want internet_search", plan.ToolsNeeded)
	}
}

func TestGeneralExplanationEvidenceContractAllowsModelKnowledge(t *testing.T) {
	input := PlanInput{Content: "Explain the concept of cogging torque."}
	plan := BuildPlan(input)
	if plan.TaskType != TaskReasoning {
		t.Fatalf("TaskType = %q, want %q; plan=%+v", plan.TaskType, TaskReasoning, plan)
	}
	if plan.EvidencePolicy != EvidenceGeneralKnowledgeAllowed {
		t.Fatalf("EvidencePolicy = %q, want %q; plan=%+v", plan.EvidencePolicy, EvidenceGeneralKnowledgeAllowed, plan)
	}
	if shouldRetrieveLocalContext(input) {
		t.Fatal("generic explanation unexpectedly requires local retrieval")
	}
	messages := PromptMessages(plan, input, "system "+AnswerContractGuidance(plan))
	userPrompt := messages[1].Content
	for _, want := range []string{
		"general model knowledge is allowed",
		"do not refuse only because workspace files or local documents are absent",
		"Internal plan: do not quote",
	} {
		if !strings.Contains(userPrompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, userPrompt)
		}
	}
	if strings.Contains(userPrompt, "Use retrieved context before answering") ||
		strings.Contains(userPrompt, "answer stays within available context") ||
		strings.Contains(userPrompt, "State assumptions") {
		t.Fatalf("generic explanation prompt still contains over-grounding instruction:\n%s", userPrompt)
	}
}

func TestLocalDocumentEvidenceContractStillRequiresRAG(t *testing.T) {
	input := PlanInput{Content: "Answer from my indexed documents about cogging torque."}
	plan := BuildPlan(input)
	if plan.TaskType != TaskRAG {
		t.Fatalf("TaskType = %q, want %q; plan=%+v", plan.TaskType, TaskRAG, plan)
	}
	if plan.EvidencePolicy != EvidenceLocalDocumentsRequired {
		t.Fatalf("EvidencePolicy = %q, want %q; plan=%+v", plan.EvidencePolicy, EvidenceLocalDocumentsRequired, plan)
	}
	if !containsTool(plan.ToolsNeeded, "rag_search") {
		t.Fatalf("ToolsNeeded = %#v, want rag_search", plan.ToolsNeeded)
	}
	if !strings.Contains(plan.ResponseContract, "answer only from retrieved local document") {
		t.Fatalf("ResponseContract = %q, want local document requirement", plan.ResponseContract)
	}
}

func TestVerifyResponseFlagsGeneralKnowledgeLocalEvidenceRefusal(t *testing.T) {
	plan := BuildPlan(PlanInput{Content: "Explain the process of cellular respiration in plants."})
	result := VerifyResponse(plan, "The process of cellular respiration in plants is not present in the available local context or workspace files.")
	if result.Status != "needs_follow_up" {
		t.Fatalf("Status = %q, want needs_follow_up; result=%+v", result.Status, result)
	}
	if !containsString(result.Reasons, "general knowledge route refused because optional local/workspace evidence was absent") {
		t.Fatalf("Reasons = %#v, want general-knowledge refusal reason", result.Reasons)
	}
}

func TestVerifyResponseFlagsInternalScaffoldLeak(t *testing.T) {
	plan := BuildPlan(PlanInput{Content: "Explain entropy in thermodynamics."})
	result := VerifyResponse(plan, "**Assumptions:**\n- The task is a reasoning task.\n\n**Reasoning step-by-step:**\n- Hidden chain.\n\nEntropy is disorder.")
	if result.Status != "needs_follow_up" {
		t.Fatalf("Status = %q, want needs_follow_up; result=%+v", result.Status, result)
	}
	if !containsString(result.Reasons, "internal task plan or reasoning scaffold leaked into the final answer") {
		t.Fatalf("Reasons = %#v, want internal scaffold reason", result.Reasons)
	}
}

func TestContextGroundingGuidanceIncludesCurrentDateForSearchFreshness(t *testing.T) {
	guidance := ContextGroundingGuidance()
	if !strings.Contains(guidance, time.Now().Format("2006-01-02")) {
		t.Fatalf("ContextGroundingGuidance() = %q, want current local date", guidance)
	}
	if !strings.Contains(guidance, "do not reuse older search results") {
		t.Fatalf("ContextGroundingGuidance() = %q, want stale search-result guard", guidance)
	}
}

func TestBuildPlanKeepsPoliteLocalSearchLocal(t *testing.T) {
	plan := BuildPlan(PlanInput{Content: "Please search files for config"})
	if containsTool(plan.ToolsNeeded, "internet_search") {
		t.Fatalf("ToolsNeeded = %#v, did not expect internet_search", plan.ToolsNeeded)
	}
	if !containsTool(plan.ToolsNeeded, "search_files") {
		t.Fatalf("ToolsNeeded = %#v, want search_files", plan.ToolsNeeded)
	}
}

func TestPlanEventRouteDataIncludesContinuationAndPreflight(t *testing.T) {
	input := PlanInput{
		Content: "try again",
		Continuation: routing.BuildContinuationFrame(routing.ContinuationInput{
			Content: "try again",
			Prior: routing.ContinuationPriorState{
				RouteCategory: routing.RouteInternetSearch,
				ToolName:      "internet_search",
				Target:        "prior public fact target",
				SourceOfTruth: routing.PreflightSourceInternet,
				FailureStatus: ExecutionBlocked,
				FailureReason: "internet access is disabled for this profile",
			},
		}),
	}
	plan := BuildPlan(input)
	data := planEventRouteData(plan)

	for key, want := range map[string]string{
		"route_source_of_truth":      routing.PreflightSourceInternet,
		"route_required_tool":        "internet_search",
		"route_blocked_requirement":  "internet access is disabled for this profile",
		"continuation_kind":          routing.ContinuationKindRetry,
		"continuation_prior_target":  "prior public fact target",
		"continuation_prior_failure": ExecutionBlocked,
		"continuation_target_source": routing.ContinuationTargetPriorToolRun,
	} {
		if got := data[key]; got != want {
			t.Fatalf("data[%q] = %q, want %q; data=%+v", key, got, want, data)
		}
	}
}

func renderTestSections(sections []promptcore.Section) string {
	var builder strings.Builder
	for _, section := range sections {
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(section.Name)
		builder.WriteString("\n")
		builder.WriteString(section.Content)
	}
	return builder.String()
}
