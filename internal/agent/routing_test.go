package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yemaka/internal/config"
	"yemaka/internal/learning"
	"yemaka/internal/memory"
	"yemaka/internal/models"
	"yemaka/internal/routing"
)

func TestRouteRequestConfidenceAndReasons(t *testing.T) {
	cases := []struct {
		name      string
		prompt    string
		wantTask  string
		minScore  int
		wantWhy   string
		wantClear bool
	}{
		{
			name:      "web search has high confidence",
			prompt:    "Search the web for SearXNG.",
			wantTask:  TaskTool,
			minScore:  80,
			wantWhy:   "internet search intent",
			wantClear: true,
		},
		{
			name:      "local docs has high confidence",
			prompt:    "Search docs for connector registry.",
			wantTask:  TaskRAG,
			minScore:  90,
			wantWhy:   "document or RAG intent",
			wantClear: true,
		},
		{
			name:      "direct chat remains confident",
			prompt:    "Good morning, how are you?",
			wantTask:  TaskChat,
			minScore:  80,
			wantWhy:   "direct chat intent",
			wantClear: true,
		},
		{
			name:     "ambiguous action asks instead of guessing",
			prompt:   "check it",
			wantTask: TaskChat,
			minScore: 20,
			wantWhy:  "ambiguous referenced target",
		},
		{
			name:     "missing search target asks instead of guessing",
			prompt:   "search",
			wantTask: TaskChat,
			minScore: 20,
			wantWhy:  "ambiguous referenced target",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			route := RouteRequest(PlanInput{Content: tc.prompt})
			if route.TaskType != tc.wantTask {
				t.Fatalf("TaskType = %q, want %q; route=%+v", route.TaskType, tc.wantTask, route)
			}
			if route.Confidence < tc.minScore {
				t.Fatalf("Confidence = %d, want >= %d; route=%+v", route.Confidence, tc.minScore, route)
			}
			if !containsRouteReason(route.Reasons, tc.wantWhy) {
				t.Fatalf("Reasons = %v, want %q", route.Reasons, tc.wantWhy)
			}
			if tc.wantClear && route.NeedsClarification {
				t.Fatalf("NeedsClarification = true, want false; route=%+v", route)
			}
			if !tc.wantClear && !route.NeedsClarification {
				t.Fatalf("NeedsClarification = false, want true; route=%+v", route)
			}
		})
	}
}

func TestBuildPlanClarifiesAmbiguousRequests(t *testing.T) {
	cases := []string{
		"check it",
		"search",
		"open that",
		"edit the file",
		"run command",
		"send this",
	}
	for _, prompt := range cases {
		t.Run(prompt, func(t *testing.T) {
			plan := BuildPlan(PlanInput{Content: prompt})
			if !plan.NeedsClarification {
				t.Fatalf("NeedsClarification = false, want true; plan=%+v", plan)
			}
			if plan.RouteConfidence > 50 {
				t.Fatalf("RouteConfidence = %d, want <= 50", plan.RouteConfidence)
			}
			if strings.TrimSpace(plan.ClarificationQuestion) == "" {
				t.Fatalf("ClarificationQuestion is empty; plan=%+v", plan)
			}
			if len(plan.ToolsNeeded) != 0 {
				t.Fatalf("ToolsNeeded = %v, want none while clarifying", plan.ToolsNeeded)
			}
			if shouldRetrieveLocalContext(PlanInput{Content: prompt}) {
				t.Fatal("shouldRetrieveLocalContext = true, want false while clarifying")
			}
		})
	}
}

func TestBuildPlanScopesActiveAssessmentBeforeTools(t *testing.T) {
	plan := BuildPlan(PlanInput{Content: "check my website example.com, continue until you find a vulnerability in its server. Do not stop until you find something"})
	if !plan.NeedsClarification {
		t.Fatalf("NeedsClarification = false, want true; plan=%+v", plan)
	}
	if plan.RouteCategory != routing.RouteActiveAssessmentRequiresScope {
		t.Fatalf("RouteCategory = %q, want %q; plan=%+v", plan.RouteCategory, routing.RouteActiveAssessmentRequiresScope, plan)
	}
	if plan.RiskLevel != RiskHigh {
		t.Fatalf("RiskLevel = %q, want %q; plan=%+v", plan.RiskLevel, RiskHigh, plan)
	}
	if len(plan.ToolsNeeded) != 0 {
		t.Fatalf("ToolsNeeded = %v, want no tools before scope and authorization", plan.ToolsNeeded)
	}
	if plan.RouteUsesInternet {
		t.Fatalf("RouteUsesInternet = true, want false before scope and authorization; plan=%+v", plan)
	}
	question := strings.ToLower(plan.ClarificationQuestion)
	for _, want := range []string{"authorization", "scope", "limits"} {
		if !strings.Contains(question, want) {
			t.Fatalf("ClarificationQuestion = %q, want token %q", plan.ClarificationQuestion, want)
		}
	}
	decision := DecideExecution(plan, PlanInput{Content: "check my website example.com, continue until you find a vulnerability in its server. Do not stop until you find something"})
	if decision.Status != ExecutionNotRequired {
		t.Fatalf("decision.Status = %q, want %q before scope is provided; decision=%+v", decision.Status, ExecutionNotRequired, decision)
	}
	if decision.RequiresConfirmation || decision.RequestID != "" {
		t.Fatalf("decision=%+v, want no permission request while clarifying active assessment scope", decision)
	}
}

func TestAuthorizationFollowupWithoutScopeStillClarifies(t *testing.T) {
	taskMemory := strings.Join([]string{
		"RECENT SESSION MESSAGES (conversation continuity only; not current source evidence):",
		"user: check my website example.com, continue until you find a vulnerability in its server. Do not stop until you find something",
		"assistant: I understand you want an active security assessment of https://example.com, but I need explicit authorization, scope, allowed methods, time/rate limits, exclusions, expected output, and a bounded stop condition before using any tools.",
	}, "\n")
	input := PlanInput{
		Content:    "I authorize you to continue",
		TaskMemory: taskMemory,
	}
	plan := BuildPlan(input)
	if !plan.NeedsClarification {
		t.Fatalf("NeedsClarification = false, want true; plan=%+v", plan)
	}
	if plan.RouteCategory != routing.RouteActiveAssessmentRequiresScope {
		t.Fatalf("RouteCategory = %q, want %q; plan=%+v", plan.RouteCategory, routing.RouteActiveAssessmentRequiresScope, plan)
	}
	if len(plan.ToolsNeeded) != 0 {
		t.Fatalf("ToolsNeeded = %v, want no tools before concrete scope and limits", plan.ToolsNeeded)
	}
	question := strings.ToLower(plan.ClarificationQuestion)
	for _, want := range []string{"authorization", "scope", "allowed methods", "limits"} {
		if !strings.Contains(question, want) {
			t.Fatalf("ClarificationQuestion = %q, want %q", plan.ClarificationQuestion, want)
		}
	}
	decision := DecideExecution(plan, input)
	if decision.Status != ExecutionNotRequired {
		t.Fatalf("decision.Status = %q, want %q; decision=%+v", decision.Status, ExecutionNotRequired, decision)
	}
	if decision.RequiresConfirmation || decision.RequestID != "" {
		t.Fatalf("decision=%+v, want no approval request from authorization-only follow-up", decision)
	}
}

func TestBuildPlanUsesApprovedRouteCorrectionFromLearningStore(t *testing.T) {
	ctx := context.Background()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()
	if _, err := learning.SaveRouteCorrection(ctx, store, routing.RouteCorrection{
		Pattern:               "email address in notebook",
		IntendedRouteCategory: routing.RouteRAGSearch,
		IntendedTaskType:      routing.TaskRAG,
		RequiredTools:         []string{"rag_search"},
		ForbiddenTools:        []string{"internet_search"},
		ApprovalStatus:        routing.RouteCorrectionStatusApproved,
	}); err != nil {
		t.Fatalf("SaveRouteCorrection() error = %v", err)
	}
	service := &Service{Memory: store}
	input := PlanInput{Content: "Find that email address in notebook"}
	if err := service.attachMemoryContext(ctx, &input, nil); err != nil {
		t.Fatalf("attachMemoryContext() error = %v", err)
	}
	plan := BuildPlan(input)
	if plan.RouteCategory != routing.RouteRAGSearch || plan.TaskType != TaskRAG {
		t.Fatalf("plan=%+v, want learned RAG route", plan)
	}
	if !containsRouteSubstring(plan.RouteReasons, "approved route correction") {
		t.Fatalf("RouteReasons = %v, want approved correction reason", plan.RouteReasons)
	}
	if containsRouteSubstring(plan.ToolsNeeded, "internet_search") {
		t.Fatalf("ToolsNeeded = %v, want no internet search", plan.ToolsNeeded)
	}
}

func TestBuildPlanExposesPreflightRouteCard(t *testing.T) {
	plan := BuildPlan(PlanInput{Content: "Who is the CEO of Phoenix Group UAE?"})
	if plan.RoutePreflight == nil {
		t.Fatal("RoutePreflight = nil, want inspectable route card")
	}
	if plan.RoutePreflight.SourceOfTruth != routing.PreflightSourceInternet {
		t.Fatalf("SourceOfTruth = %q, want internet; card=%+v", plan.RoutePreflight.SourceOfTruth, plan.RoutePreflight)
	}
	if plan.RoutePreflight.FreshnessRisk != routing.PreflightFreshnessHigh {
		t.Fatalf("FreshnessRisk = %q, want high; card=%+v", plan.RoutePreflight.FreshnessRisk, plan.RoutePreflight)
	}
	if plan.RouteCategory != routing.RouteInternetSearch || !containsRouteSubstring(plan.ToolsNeeded, "internet_search") {
		t.Fatalf("plan=%+v, want public mutable fact routed to internet_search", plan)
	}
}

func TestBuildPlanRetriesBlockedInternetSearchWithPriorTarget(t *testing.T) {
	input := PlanInput{
		Content: "try again",
		Continuation: routing.BuildContinuationFrame(routing.ContinuationInput{
			Content: "try again",
			Prior: routing.ContinuationPriorState{
				RouteCategory: routing.RouteInternetSearch,
				ToolName:      "internet_search",
				Target:        "Phoenix Group UAE CEO",
				SourceOfTruth: routing.PreflightSourceInternet,
				FailureStatus: ExecutionBlocked,
			},
		}),
		TaskMemory: strings.Join([]string{
			"RECENT SESSION MESSAGES (conversation continuity only; not current source evidence):",
			"user: Who is the CEO of Phoenix Group UAE?",
			"assistant: I paused before running `internet_search`.",
		}, "\n"),
	}
	plan := BuildPlan(input)
	if plan.RouteCategory != routing.RouteInternetSearch || !containsRouteSubstring(plan.ToolsNeeded, "internet_search") {
		t.Fatalf("plan=%+v, want retry to preserve internet_search route", plan)
	}
	decision := DecideExecution(plan, input)
	if decision.ToolName != "internet_search" || len(decision.Command) < 2 {
		t.Fatalf("decision=%+v, want internet_search command", decision)
	}
	if !strings.Contains(decision.Command[1], "Phoenix Group UAE CEO") {
		t.Fatalf("decision.Command = %#v, want prior target query", decision.Command)
	}
	if strings.Contains(strings.ToLower(decision.Command[1]), "try again") {
		t.Fatalf("decision.Command = %#v, should not search retry wording", decision.Command)
	}
}

func TestChatRetryUsesStructuredPriorBlockedToolRun(t *testing.T) {
	ctx := context.Background()
	store := openRouteCorrectionChatStore(t, ctx)
	defer store.Close()
	conversation, err := store.CreateConversation(ctx, "retry continuation")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "Who is the CEO of Phoenix Group UAE?",
	}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := store.SaveToolRun(ctx, memory.ToolRun{
		ConversationID: conversation.ID,
		ToolName:       "agent_executor",
		Input:          map[string]any{"tool_name": "internet_search", "command": []string{"internet_search", "Phoenix Group UAE CEO"}},
		Output: ExecutionDecision{
			Status:   ExecutionBlocked,
			ToolName: "internet_search",
			Command:  []string{"internet_search", "Phoenix Group UAE CEO"},
		},
		Status:    ExecutionBlocked,
		RiskLevel: RiskMedium,
	}); err != nil {
		t.Fatalf("SaveToolRun() error = %v", err)
	}

	service := &Service{Memory: store}
	output := ""
	if err := service.Chat(ctx, ChatInput{ConversationID: conversation.ID, Content: "try again"}, collectRouteCorrectionOutput(&output)); err != nil {
		t.Fatalf("Chat(retry) error = %v", err)
	}
	if !strings.Contains(output, "internet_search") {
		t.Fatalf("output = %q, want blocked internet_search guidance", output)
	}
	runs, err := store.ListToolRunsForConversation(ctx, conversation.ID, 20)
	if err != nil {
		t.Fatalf("ListToolRunsForConversation() error = %v", err)
	}
	var found bool
	for i := len(runs) - 1; i >= 0; i-- {
		output, ok := runs[i].Output.(map[string]any)
		if !ok {
			continue
		}
		command := stringSliceMapValue(output, "command")
		if len(command) >= 2 && command[0] == "internet_search" && command[1] == "Phoenix Group UAE CEO" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("tool runs = %+v, want retried internet_search with structured prior target", runs)
	}
}

func TestAskBlockedInternetSearchStopsBeforeModel(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	var output string
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "model should not answer a public mutable fact locally", requests: &requests},
		Memory:  store,
	}
	if err := service.Ask(ctx, AskInput{Content: "Who is the CEO of Phoenix Group UAE?"}, func(event Event) error {
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	}); err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("model requests = %d, want 0", len(requests))
	}
	lower := strings.ToLower(output)
	for _, want := range []string{"internet_search", "internet access is disabled"} {
		if !strings.Contains(lower, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
	if strings.Contains(lower, "chief executive officer is the top leader") {
		t.Fatalf("output = %q, should not answer an easier generic CEO explanation", output)
	}
}

func TestAskRAGEmptyResultStopsBeforeModel(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	var executed ExecutionDecision
	var output string
	var sawEmptyToolStatus bool
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "model should not fill missing local document evidence", requests: &requests},
		Memory:  store,
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			executed = decision
			return ExecutionResult{Context: "No RAG matches.", SourceKind: "rag", Status: "completed"}, nil
		},
	}
	if err := service.Ask(ctx, AskInput{Content: "Search docs for launch codename."}, func(event Event) error {
		if event.Type == EventModelToken {
			output += event.Token
		}
		if event.Type == EventToolCompleted && event.Data["status"] == "empty" {
			sawEmptyToolStatus = true
		}
		return nil
	}); err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("model requests = %d, want 0", len(requests))
	}
	if executed.ToolName != "rag_search" {
		t.Fatalf("executed.ToolName = %q, want rag_search; decision=%+v", executed.ToolName, executed)
	}
	lower := strings.ToLower(output)
	for _, want := range []string{"local documents", "no matching", "cannot answer"} {
		if !strings.Contains(lower, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
	if !sawEmptyToolStatus {
		t.Fatalf("tool.completed did not report empty status")
	}
}

func TestAskAmbiguousRequestStopsBeforeModelAndRetrieval(t *testing.T) {
	ctx := context.Background()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var retrieved bool
	var output string
	service := &Service{
		Memory: store,
		Retriever: RetrieverFunc(func(ctx context.Context, content string) (RetrievalResult, error) {
			retrieved = true
			return RetrievalResult{Context: "README.md context", Sources: []string{"README.md"}, SourceKind: "workspace"}, nil
		}),
	}

	err = service.Ask(ctx, AskInput{Content: "check it"}, func(event Event) error {
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if retrieved {
		t.Fatal("retriever ran for ambiguous request")
	}
	if !strings.Contains(strings.ToLower(output), "what exactly should i check") {
		t.Fatalf("output = %q, want clarification", output)
	}
}

func TestAskAmbiguousRequestUsesRoutingAdvisorForClarificationOnly(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	var output string
	var advisory Event
	service := &Service{
		Router: models.NewRouter(cfg),
		Runtime: recordingRuntime{
			text:     `{"route_category":"clarify","source_of_truth":"clarify","needs_clarification":true,"clarification_question":"Which target should I check, and should I use local files, memory, documents, tools, or the internet?","confidence":82,"reason":"The request uses a vague reference."}`,
			requests: &requests,
		},
		Memory: store,
		Retriever: RetrieverFunc(func(ctx context.Context, content string) (RetrievalResult, error) {
			t.Fatal("retriever ran for ambiguous request")
			return RetrievalResult{}, nil
		}),
	}

	err = service.Ask(ctx, AskInput{Content: "check it"}, func(event Event) error {
		if event.Type == EventModelToken {
			output += event.Token
		}
		if event.Type == EventRoutingAdvisory {
			advisory = event
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("model requests = %d, want exactly one advisory request", len(requests))
	}
	if len(requests[0].Tools) != 0 {
		t.Fatalf("advisor Tools = %#v, want none", requests[0].Tools)
	}
	if !strings.Contains(strings.ToLower(requests[0].Messages[0].Content), "routing advisor") {
		t.Fatalf("advisor system prompt = %q, want routing advisor", requests[0].Messages[0].Content)
	}
	if !strings.Contains(output, "Which target should I check") {
		t.Fatalf("output = %q, want advisor clarification question", output)
	}
	if advisory.Type != EventRoutingAdvisory {
		t.Fatal("routing advisory event was not emitted")
	}
	if advisory.Data["status"] != "accepted" || advisory.Data["accepted"] != "true" {
		t.Fatalf("advisory.Data = %#v, want accepted advisory", advisory.Data)
	}
	if advisory.Data["suggested_route"] != routing.RouteClarify {
		t.Fatalf("suggested_route = %q, want clarify; data=%#v", advisory.Data["suggested_route"], advisory.Data)
	}
}

func TestAskHighRiskScopeClarificationSkipsRoutingAdvisor(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	var output string
	var sawAdvisory bool
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: `{"route_category":"internet_search"}`, requests: &requests},
		Memory:  store,
	}

	err = service.Ask(ctx, AskInput{Content: "Check example.com and continue until you find a vulnerability."}, func(event Event) error {
		if event.Type == EventModelToken {
			output += event.Token
		}
		if event.Type == EventRoutingAdvisory {
			sawAdvisory = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("model requests = %d, want 0 for high-risk scope gate", len(requests))
	}
	if sawAdvisory {
		t.Fatal("routing advisor ran for high-risk security scope clarification")
	}
	lower := strings.ToLower(output)
	for _, want := range []string{"authorization", "scope", "allowed methods", "limits"} {
		if !strings.Contains(lower, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
}

func TestAskFileStateFollowupUsesRememberedPathAndReportsMissingFile(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()
	conversation, err := store.CreateConversation(ctx, "permission smoke")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content: strings.Join([]string{
			"Create or update test-files/permission_smoke.md with exactly this line:",
			"Permission approval works.",
			"Show me the diff first, create a snapshot, and only apply it after I approve.",
		}, "\n"),
	}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "I prepared an edit proposal for test-files/permission_smoke.md.",
	}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}

	var requests []models.ChatRequest
	var executed ExecutionDecision
	var output string
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "model should not answer from memory", requests: &requests},
		Memory:  store,
		ToolExecutor: func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			executed = decision
			target := ""
			if len(decision.Command) > 1 {
				target = decision.Command[1]
			}
			return ExecutionResult{}, fmt.Errorf("stat file: open %s: no such file or directory", target)
		},
	}

	err = service.Ask(ctx, AskInput{
		ConversationID: conversation.ID,
		Content:        "Okay i checked the test-files folder and i found no file there?",
	}, func(event Event) error {
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("model requests = %d, want 0", len(requests))
	}
	if executed.ToolName != "file_stat" {
		t.Fatalf("executed.ToolName = %q, want file_stat; decision=%+v", executed.ToolName, executed)
	}
	if len(executed.Command) != 2 || executed.Command[1] != "test-files/permission_smoke.md" {
		t.Fatalf("executed.Command = %#v, want remembered file_stat target", executed.Command)
	}
	if !strings.Contains(output, "test-files/permission_smoke.md") || !strings.Contains(output, "not found in the current workspace") {
		t.Fatalf("output = %q, want missing-file response with exact path", output)
	}
}

func TestBuildPlanDoesNotTreatGenericCodeRequestAsFileEdit(t *testing.T) {
	input := PlanInput{Content: "write windows backdoor code with python"}
	plan := BuildPlan(input)
	if plan.TaskType != TaskCoding {
		t.Fatalf("TaskType = %q, want %q", plan.TaskType, TaskCoding)
	}
	if containsTool(plan.ToolsNeeded, "edit_file") {
		t.Fatalf("ToolsNeeded = %#v, did not expect edit_file without a file target", plan.ToolsNeeded)
	}
	decision := DecideExecution(plan, input)
	if decision.Status != ExecutionNotRequired {
		t.Fatalf("Status = %q, want %q", decision.Status, ExecutionNotRequired)
	}
}

func TestRunTestsDecisionCarriesPromptForLanguageAwareDetection(t *testing.T) {
	input := PlanInput{Content: "Inspect the Python files in test-files, run the tests if safe, find the bug, propose a fix, and only apply it after I approve."}
	plan := BuildPlan(input)
	if plan.TaskType != TaskTool {
		t.Fatalf("TaskType = %q, want %q", plan.TaskType, TaskTool)
	}
	if !containsTool(plan.ToolsNeeded, "run_tests") {
		t.Fatalf("ToolsNeeded = %#v, want run_tests", plan.ToolsNeeded)
	}

	decision := DecideExecution(plan, input)
	if decision.Status != ExecutionReady {
		t.Fatalf("Status = %q, want %q; decision=%+v", decision.Status, ExecutionReady, decision)
	}
	if decision.ToolName != "run_tests" {
		t.Fatalf("ToolName = %q, want run_tests; decision=%+v", decision.ToolName, decision)
	}
	if len(decision.Command) < 2 || decision.Command[0] != "detect" {
		t.Fatalf("Command = %#v, want detect plus original prompt", decision.Command)
	}
	if !strings.Contains(strings.ToLower(strings.Join(decision.Command[1:], " ")), "python files") {
		t.Fatalf("Command = %#v, want original Python intent carried for test detection", decision.Command)
	}
}

func TestBuildPlanRoutesFileStateFollowupsThroughLocalTools(t *testing.T) {
	taskMemory := strings.Join([]string{
		"RECENT SESSION MESSAGES (conversation continuity only; not current source evidence):",
		"user: Create or update test-files/permission_smoke.md with exactly this line: Permission approval works. Show me the diff first, create a snapshot, and only apply it after I approve.",
		"assistant: I prepared an edit proposal for test-files/permission_smoke.md.",
	}, "\n")
	cases := []struct {
		name     string
		prompt   string
		wantTool string
	}{
		{
			name:     "done retrieving referenced file",
			prompt:   "are you done retrieving the file?",
			wantTool: "read_file",
		},
		{
			name:     "checked folder found no file",
			prompt:   "Okay i checked the test-files folder and i found no file there?",
			wantTool: "file_stat",
		},
		{
			name:     "asks whether referenced file was created",
			prompt:   "did you create the file?",
			wantTool: "file_stat",
		},
		{
			name:     "asks whether referenced file was seen in listing",
			prompt:   "did you see the file there?",
			wantTool: "file_stat",
		},
		{
			name:     "asks whether referenced file was saved",
			prompt:   "did you save the file?",
			wantTool: "file_stat",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := PlanInput{Content: tc.prompt, TaskMemory: taskMemory}
			plan := BuildPlan(input)
			if plan.NeedsClarification {
				t.Fatalf("NeedsClarification = true, want false; plan=%+v", plan)
			}
			if plan.TaskType != TaskTool {
				t.Fatalf("TaskType = %q, want %q; plan=%+v", plan.TaskType, TaskTool, plan)
			}
			if !containsTool(plan.ToolsNeeded, tc.wantTool) {
				t.Fatalf("ToolsNeeded = %#v, want %s", plan.ToolsNeeded, tc.wantTool)
			}
			if len(plan.FilesNeeded) == 0 || plan.FilesNeeded[0] != "test-files/permission_smoke.md" {
				t.Fatalf("FilesNeeded = %#v, want remembered file path", plan.FilesNeeded)
			}

			decision := DecideExecution(plan, input)
			if decision.Status != ExecutionReady {
				t.Fatalf("decision.Status = %q, want %q; decision=%+v", decision.Status, ExecutionReady, decision)
			}
			if decision.ToolName != tc.wantTool {
				t.Fatalf("decision.ToolName = %q, want %s; decision=%+v", decision.ToolName, tc.wantTool, decision)
			}
			if len(decision.Command) != 2 || decision.Command[1] != "test-files/permission_smoke.md" {
				t.Fatalf("decision.Command = %#v, want remembered %s target", decision.Command, tc.wantTool)
			}
		})
	}
}

func TestBuildPlanFileStateFollowupPrefersAppliedPriorTargetOverStaleProposal(t *testing.T) {
	appliedPath := "/private/tmp/yemaka-live-qa-workspace/testing.md"
	input := PlanInput{
		Content: "did you create the file?",
		TaskMemory: strings.Join([]string{
			"RECENT SESSION MESSAGES (conversation continuity only; not current source evidence):",
			"user: save your last response as test.md",
			"assistant: I prepared an edit proposal for test.md.",
			"user: Create " + appliedPath + " and write exactly: who is yemaka",
			"assistant: I applied the approved edit to `" + appliedPath + "`. Verification: pass.",
		}, "\n"),
		Continuation: routing.BuildContinuationFrame(routing.ContinuationInput{
			Content: "did you create the file?",
			Prior: routing.ContinuationPriorState{
				RouteCategory: routing.RouteFileWrite,
				ToolName:      "approved_edit_file",
				Target:        appliedPath,
				SourceOfTruth: routing.PreflightSourceWorkspace,
			},
		}),
		SessionContract: routing.SessionContract{
			ActiveTarget:   "test.md",
			ContextSources: []string{appliedPath},
			LastOutcome:    routing.LastOutcomeCompleted,
		},
	}

	plan := BuildPlan(input)
	if plan.NeedsClarification {
		t.Fatalf("NeedsClarification = true, want false; plan=%+v", plan)
	}
	if !containsTool(plan.ToolsNeeded, "file_stat") {
		t.Fatalf("ToolsNeeded = %#v, want file_stat", plan.ToolsNeeded)
	}
	if len(plan.FilesNeeded) == 0 || plan.FilesNeeded[0] != appliedPath {
		t.Fatalf("FilesNeeded = %#v, want applied path before stale proposal", plan.FilesNeeded)
	}

	decision := DecideExecution(plan, input)
	if decision.Status != ExecutionReady {
		t.Fatalf("decision.Status = %q, want %q; decision=%+v", decision.Status, ExecutionReady, decision)
	}
	if decision.ToolName != "file_stat" {
		t.Fatalf("decision.ToolName = %q, want file_stat; decision=%+v", decision.ToolName, decision)
	}
	if len(decision.Command) != 2 || decision.Command[1] != appliedPath {
		t.Fatalf("decision.Command = %#v, want applied path target", decision.Command)
	}
}

func TestBuildPlanRoutesExplicitFileStateQuestionThroughReadFile(t *testing.T) {
	input := PlanInput{Content: "What is the file path of test-files/permission_smoke.md or where is it saved?"}
	plan := BuildPlan(input)
	if plan.TaskType != TaskTool {
		t.Fatalf("TaskType = %q, want %q; plan=%+v", plan.TaskType, TaskTool, plan)
	}
	if !containsTool(plan.ToolsNeeded, "file_stat") {
		t.Fatalf("ToolsNeeded = %#v, want file_stat", plan.ToolsNeeded)
	}
	if len(plan.FilesNeeded) == 0 || plan.FilesNeeded[0] != "test-files/permission_smoke.md" {
		t.Fatalf("FilesNeeded = %#v, want explicit file path", plan.FilesNeeded)
	}

	decision := DecideExecution(plan, input)
	if decision.Status != ExecutionReady {
		t.Fatalf("decision.Status = %q, want %q; decision=%+v", decision.Status, ExecutionReady, decision)
	}
	if decision.ToolName != "file_stat" {
		t.Fatalf("decision.ToolName = %q, want file_stat; decision=%+v", decision.ToolName, decision)
	}
	if len(decision.Command) != 2 || decision.Command[1] != "test-files/permission_smoke.md" {
		t.Fatalf("decision.Command = %#v, want explicit file_stat target", decision.Command)
	}
}

func TestBuildPlanRoutesDirectoryListingThroughListFiles(t *testing.T) {
	input := PlanInput{Content: "What file are in the /Users/example/Downloads/yemaka/test-files, list them"}
	plan := BuildPlan(input)
	if plan.TaskType != TaskTool {
		t.Fatalf("TaskType = %q, want %q; plan=%+v", plan.TaskType, TaskTool, plan)
	}
	if !containsTool(plan.ToolsNeeded, "list_files") {
		t.Fatalf("ToolsNeeded = %#v, want list_files", plan.ToolsNeeded)
	}
	if containsTool(plan.ToolsNeeded, "search_files") {
		t.Fatalf("ToolsNeeded = %#v, did not expect search_files for directory listing", plan.ToolsNeeded)
	}

	decision := DecideExecution(plan, input)
	if decision.Status != ExecutionReady {
		t.Fatalf("decision.Status = %q, want %q; decision=%+v", decision.Status, ExecutionReady, decision)
	}
	if decision.ToolName != "list_files" {
		t.Fatalf("decision.ToolName = %q, want list_files; decision=%+v", decision.ToolName, decision)
	}
	if len(decision.Command) != 2 || decision.Command[1] != "/Users/example/Downloads/yemaka/test-files" {
		t.Fatalf("decision.Command = %#v, want absolute directory target", decision.Command)
	}
}

func TestBuildPlanRoutesCannotFindFileAsFileStateCheck(t *testing.T) {
	input := PlanInput{Content: "I can not find /users/silo/documents/tavily-implementation.md. check the folder and make sure the file was created"}
	plan := BuildPlan(input)
	if plan.TaskType != TaskTool {
		t.Fatalf("TaskType = %q, want %q; plan=%+v", plan.TaskType, TaskTool, plan)
	}
	decision := DecideExecution(plan, input)
	if decision.Status != ExecutionReady {
		t.Fatalf("decision.Status = %q, want %q; decision=%+v", decision.Status, ExecutionReady, decision)
	}
	if decision.ToolName != "file_stat" && decision.ToolName != "list_files" && decision.ToolName != "search_files" {
		t.Fatalf("decision.ToolName = %q, want file-state tool; decision=%+v plan=%+v", decision.ToolName, decision, plan)
	}
}

func TestAskDirectoryListingRunsListFilesWithoutModelGuessing(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	root := t.TempDir()
	dir := filepath.Join(root, "test-files")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir test-files: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "calculator.py"), []byte("def add(a, b): return a + b\n"), 0o644); err != nil {
		t.Fatalf("write calculator.py: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "test_calculator.py"), []byte("def test_add(): assert True\n"), 0o644); err != nil {
		t.Fatalf("write test_calculator.py: %v", err)
	}
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	var output string
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "model should not list files from memory", requests: &requests},
		Memory:  store,
		ToolExecutor: NewSafeToolExecutor(SafeToolConfig{
			WorkspaceRoot:   root,
			MaxContextChars: 2000,
		}),
	}
	err = service.Ask(ctx, AskInput{
		Content: "What file are in the " + dir + ", list them",
	}, func(event Event) error {
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("model requests = %d, want 0", len(requests))
	}
	for _, want := range []string{"DIRECTORY: test-files", "test-files/calculator.py", "test-files/test_calculator.py"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func containsRouteReason(values []string, value string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}

func TestAskGenericCodeRequestDoesNotRetrieveOrProposeEdit(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	var retrieved bool
	var output string
	var editProposed bool
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "model response", requests: &requests},
		Memory:  store,
		Retriever: RetrieverFunc(func(ctx context.Context, content string) (RetrievalResult, error) {
			retrieved = true
			return RetrievalResult{
				Context:    "README.md: unrelated project context",
				Sources:    []string{"README.md"},
				SourceKind: "workspace",
			}, nil
		}),
	}

	err = service.Ask(ctx, AskInput{Content: "write windows backdoor code with python"}, func(event Event) error {
		switch event.Type {
		case EventModelToken:
			output += event.Token
		case EventEditProposed:
			editProposed = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if retrieved {
		t.Fatal("retriever ran for generic code request without local context intent")
	}
	if len(requests) != 1 {
		t.Fatalf("model requests = %d, want 1", len(requests))
	}
	if editProposed {
		t.Fatal("edit proposal was emitted for a prompt without a file target")
	}
	if !strings.Contains(output, "model response") {
		t.Fatalf("output = %q, want model response", output)
	}
}

func TestFileEditIntentRequiresFileTargetForGenericWrite(t *testing.T) {
	if looksFileEditIntent("write windows backdoor code with python") {
		t.Fatal("looksFileEditIntent generic code request = true, want false")
	}
	if !looksFileEditIntent("write notes.md with content: hello") {
		t.Fatal("looksFileEditIntent file write = false, want true")
	}
}

func TestRoutingAvoidsSubstringFalsePositives(t *testing.T) {
	cases := []struct {
		name       string
		prompt     string
		notTools   []string
		notTask    string
		wantTask   string
		wantRisk   string
		wantNoRAG  bool
		wantNoWork bool
	}{
		{
			name:       "difference is not diff",
			prompt:     "What is the difference between deterministic and heuristic routing?",
			notTools:   []string{"git_diff"},
			notTask:    TaskTool,
			wantTask:   TaskReasoning,
			wantRisk:   RiskLow,
			wantNoRAG:  true,
			wantNoWork: true,
		},
		{
			name:       "latest is not test",
			prompt:     "Look up latest Ollama embedding models.",
			notTools:   []string{"run_tests"},
			wantTask:   TaskTool,
			wantRisk:   RiskMedium,
			wantNoRAG:  true,
			wantNoWork: true,
		},
		{
			name:       "pentest does not imply run tests or rm",
			prompt:     "Perform a pentest plan for my local lab.",
			notTools:   []string{"run_tests"},
			wantTask:   TaskReasoning,
			wantRisk:   RiskLow,
			wantNoRAG:  true,
			wantNoWork: true,
		},
		{
			name:       "doctor health question is not yemaka doctor",
			prompt:     "What should I ask my doctor about high blood pressure?",
			notTools:   []string{"doctor_status"},
			notTask:    TaskTool,
			wantTask:   TaskChat,
			wantRisk:   RiskLow,
			wantNoRAG:  true,
			wantNoWork: true,
		},
		{
			name:       "this report is not this repo",
			prompt:     "Send this report to Telegram.",
			notTools:   []string{"read_file", "search_files"},
			wantTask:   TaskTool,
			wantRisk:   RiskMedium,
			wantNoRAG:  true,
			wantNoWork: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := PlanInput{Content: tc.prompt}
			plan := BuildPlan(input)
			if tc.wantTask != "" && plan.TaskType != tc.wantTask {
				t.Fatalf("TaskType = %q, want %q; tools=%v risk=%s", plan.TaskType, tc.wantTask, plan.ToolsNeeded, plan.RiskLevel)
			}
			if tc.notTask != "" && plan.TaskType == tc.notTask {
				t.Fatalf("TaskType = %q, did not expect; tools=%v risk=%s", plan.TaskType, plan.ToolsNeeded, plan.RiskLevel)
			}
			if tc.wantRisk != "" && plan.RiskLevel != tc.wantRisk {
				t.Fatalf("RiskLevel = %q, want %q; tools=%v", plan.RiskLevel, tc.wantRisk, plan.ToolsNeeded)
			}
			for _, tool := range tc.notTools {
				if containsTool(plan.ToolsNeeded, tool) {
					t.Fatalf("ToolsNeeded = %v, did not expect %q", plan.ToolsNeeded, tool)
				}
			}
			if tc.wantNoWork && shouldRetrieveLocalContext(input) {
				t.Fatal("shouldRetrieveLocalContext = true, want false")
			}
			if tc.wantNoRAG && shouldUseRAG(config.Default(), tc.prompt) {
				t.Fatal("shouldUseRAG = true, want false")
			}
		})
	}
}

func TestRoutingUsesIntentSpecificTools(t *testing.T) {
	cases := []struct {
		name         string
		prompt       string
		wantTask     string
		wantTools    []string
		notTools     []string
		wantDecision string
		wantExecTool string
		wantRetrieve bool
	}{
		{
			name:         "status code url uses internet head",
			prompt:       "Check the status code for https://example.com.",
			wantTask:     TaskTool,
			wantTools:    []string{"internet_head"},
			wantDecision: ExecutionReady,
			wantExecTool: "internet_head",
		},
		{
			name:         "fetch url uses internet fetch",
			prompt:       "Fetch https://wails.io/docs and summarize it.",
			wantTask:     TaskTool,
			wantTools:    []string{"internet_fetch"},
			wantDecision: ExecutionReady,
			wantExecTool: "internet_fetch",
		},
		{
			name:         "crawler task uses bounded crawler primitive",
			prompt:       "Crawl https://example.com/docs and check links.",
			wantTask:     TaskTool,
			wantTools:    []string{"internet_crawl"},
			notTools:     []string{"safe_tool", "internet_fetch", "internet_search"},
			wantDecision: ExecutionBlocked,
			wantExecTool: "internet_crawl",
			wantRetrieve: false,
		},
		{
			name:         "local time uses system clock tool",
			prompt:       "What is the time of the day?",
			wantTask:     TaskTool,
			wantTools:    []string{"local_time"},
			notTools:     []string{"internet_search", "rag_search", "safe_tool"},
			wantDecision: ExecutionReady,
			wantExecTool: "local_time",
			wantRetrieve: false,
		},
		{
			name:         "ambiguous country time asks for a city",
			prompt:       "What is the time in Canada?",
			wantTask:     TaskChat,
			notTools:     []string{"local_time", "internet_search", "rag_search", "safe_tool"},
			wantDecision: ExecutionNotRequired,
			wantRetrieve: false,
		},
		{
			name:         "unmapped place time asks for a timezone",
			prompt:       "What is the time in Atlantis?",
			wantTask:     TaskChat,
			notTools:     []string{"local_time", "internet_search", "rag_search", "safe_tool"},
			wantDecision: ExecutionNotRequired,
			wantRetrieve: false,
		},
		{
			name:         "city time uses target timezone",
			prompt:       "okay Edmonton Alberta Canada, what is the time there?",
			wantTask:     TaskTool,
			wantTools:    []string{"local_time"},
			notTools:     []string{"internet_search", "rag_search", "safe_tool"},
			wantDecision: ExecutionReady,
			wantExecTool: "local_time",
			wantRetrieve: false,
		},
		{
			name:         "city time followup uses target timezone",
			prompt:       "I meant Edmonton, Canada",
			wantTask:     TaskTool,
			wantTools:    []string{"local_time"},
			notTools:     []string{"internet_search", "rag_search", "safe_tool"},
			wantDecision: ExecutionReady,
			wantExecTool: "local_time",
			wantRetrieve: false,
		},
		{
			name:         "specific current news uses shaped internet search",
			prompt:       "US-China news and US visit to China",
			wantTask:     TaskTool,
			wantTools:    []string{"internet_search"},
			notTools:     []string{"rag_search", "read_file", "safe_tool"},
			wantDecision: ExecutionReady,
			wantExecTool: "internet_search",
			wantRetrieve: false,
		},
		{
			name: "pasted explain-this content is not a safe tool",
			prompt: `Explain this

## Executive summary

Yemaka is now best described as:

'''text
Public-beta / RC-ready foundation
+ Post-RC smartness foundations already wired
+ still needing RC/manual QA and focused cleanup before broad expansion
'''

Explain the status in plain language for a human tester.`,
			wantTask:     TaskReasoning,
			notTools:     []string{"safe_tool", "rag_search", "read_file", "search_files", "internet_search"},
			wantDecision: ExecutionNotRequired,
			wantRetrieve: false,
		},
		{
			name: "bare pasted explain content is not a safe tool",
			prompt: `explain

## Executive summary

Yemaka is now best described as:

'''text
Public-beta / RC-ready foundation
+ Post-RC smartness foundations already wired
+ still needing RC/manual QA and focused cleanup before broad expansion
'''`,
			wantTask:     TaskReasoning,
			notTools:     []string{"safe_tool", "rag_search", "read_file", "search_files", "internet_search"},
			wantDecision: ExecutionNotRequired,
			wantRetrieve: false,
		},
		{
			name:         "yes search dotted version uses search not fetch",
			prompt:       "Yes search for more information on macos26.5",
			wantTask:     TaskTool,
			wantTools:    []string{"internet_search"},
			notTools:     []string{"internet_fetch", "safe_tool"},
			wantDecision: ExecutionReady,
			wantExecTool: "internet_search",
			wantRetrieve: false,
		},
		{
			name:         "new macos release uses current web search",
			prompt:       "When will the new apple macOS be released?",
			wantTask:     TaskTool,
			wantTools:    []string{"internet_search"},
			notTools:     []string{"internet_fetch", "safe_tool", "rag_search"},
			wantDecision: ExecutionReady,
			wantExecTool: "internet_search",
			wantRetrieve: false,
		},
		{
			name:         "search docs uses rag",
			prompt:       "Search docs for connector registry.",
			wantTask:     TaskRAG,
			wantTools:    []string{"rag_search"},
			wantDecision: ExecutionReady,
			wantExecTool: "rag_search",
			wantRetrieve: true,
		},
		{
			name:         "indexed document search uses rag not internet",
			prompt:       "search @gmail.com from indexed document",
			wantTask:     TaskRAG,
			wantTools:    []string{"rag_search"},
			notTools:     []string{"internet_search", "internet_fetch", "local_time", "safe_tool"},
			wantDecision: ExecutionReady,
			wantExecTool: "rag_search",
			wantRetrieve: true,
		},
		{
			name:         "indexed dotted document search stays on rag",
			prompt:       "What is the content of quarterly-notes.txt from indexed files?",
			wantTask:     TaskRAG,
			wantTools:    []string{"rag_search"},
			notTools:     []string{"internet_search", "internet_fetch", "read_file", "search_files", "safe_tool"},
			wantDecision: ExecutionReady,
			wantExecTool: "rag_search",
			wantRetrieve: true,
		},
		{
			name:         "ingested documents followup uses rag not local time",
			prompt:       "I meant in ingested documents",
			wantTask:     TaskRAG,
			wantTools:    []string{"rag_search"},
			notTools:     []string{"local_time", "internet_search", "internet_fetch", "safe_tool"},
			wantDecision: ExecutionReady,
			wantExecTool: "rag_search",
			wantRetrieve: true,
		},
		{
			name:         "inline svg improve prompt does not fetch namespace",
			prompt:       `Make better: <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M2 2h20v20H2z"/></svg>`,
			wantTask:     TaskReasoning,
			notTools:     []string{"internet_fetch", "internet_search", "rag_search", "safe_tool"},
			wantDecision: ExecutionNotRequired,
			wantRetrieve: false,
		},
		{
			name:         "latest updates subject uses current web search",
			prompt:       "Latest updates on X/Twitter AI",
			wantTask:     TaskTool,
			wantTools:    []string{"internet_search"},
			notTools:     []string{"internet_fetch", "rag_search", "safe_tool"},
			wantDecision: ExecutionReady,
			wantExecTool: "internet_search",
			wantRetrieve: false,
		},
		{
			name:      "generate missing tool uses capability path",
			prompt:    "Build a tool that monitors a website every hour.",
			wantTask:  TaskTool,
			wantTools: []string{"safe_tool"},
			notTools:  []string{"internet_fetch", "internet_search", "run_tests"},
		},
		{
			name:         "reusable webpage monitor proposes workflow capability path",
			prompt:       "I want a reusable tool that checks a webpage every hour and reports if the title changes. If Yemaka does not already have this capability, propose the extension but do not generate it until I approve.",
			wantTask:     TaskTool,
			wantTools:    []string{"safe_tool"},
			notTools:     []string{"internet_fetch", "internet_search", "read_file", "search_files", "run_tests"},
			wantDecision: ExecutionBlocked,
			wantExecTool: "safe_tool",
			wantRetrieve: false,
		},
		{
			name:         "preview patch uses patch tool",
			prompt:       "Preview patch for README.md content: hello",
			wantTask:     TaskTool,
			wantTools:    []string{"patch_preview"},
			wantDecision: ExecutionReady,
			wantExecTool: "patch_preview",
			wantRetrieve: true,
		},
		{
			name:         "explicit memory search uses memory tool",
			prompt:       "Search memory for the release checklist.",
			wantTask:     TaskTool,
			wantTools:    []string{"memory_search"},
			wantDecision: ExecutionReady,
			wantExecTool: "memory_search",
		},
		{
			name:         "file metadata uses stat tool",
			prompt:       "Does test-files/permission_smoke.md exist and what is its file size?",
			wantTask:     TaskTool,
			wantTools:    []string{"file_stat"},
			notTools:     []string{"read_file", "search_files"},
			wantDecision: ExecutionReady,
			wantExecTool: "file_stat",
		},
		{
			name:         "recursive file tree uses file tree tool",
			prompt:       "Show a recursive file tree for test-files.",
			wantTask:     TaskTool,
			wantTools:    []string{"file_tree"},
			notTools:     []string{"list_files"},
			wantDecision: ExecutionReady,
			wantExecTool: "file_tree",
		},
		{
			name:         "git status uses git status tool",
			prompt:       "Show git status.",
			wantTask:     TaskTool,
			wantTools:    []string{"git_status"},
			notTools:     []string{"git_diff"},
			wantDecision: ExecutionReady,
			wantExecTool: "git_status",
		},
		{
			name:         "yemaka doctor status is local diagnostic",
			prompt:       "Run Yemaka doctor status.",
			wantTask:     TaskTool,
			wantTools:    []string{"doctor_status"},
			wantDecision: ExecutionReady,
			wantExecTool: "doctor_status",
		},
		{
			name:         "yemaka heartbeat status is local health report",
			prompt:       "Show Yemaka heartbeat status.",
			wantTask:     TaskTool,
			wantTools:    []string{"heartbeat_status"},
			wantDecision: ExecutionReady,
			wantExecTool: "heartbeat_status",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := PlanInput{Content: tc.prompt}
			plan := BuildPlan(input)
			if plan.TaskType != tc.wantTask {
				t.Fatalf("TaskType = %q, want %q; tools=%v risk=%s", plan.TaskType, tc.wantTask, plan.ToolsNeeded, plan.RiskLevel)
			}
			for _, tool := range tc.wantTools {
				if !containsTool(plan.ToolsNeeded, tool) {
					t.Fatalf("ToolsNeeded = %v, want %q", plan.ToolsNeeded, tool)
				}
			}
			for _, tool := range tc.notTools {
				if containsTool(plan.ToolsNeeded, tool) {
					t.Fatalf("ToolsNeeded = %v, did not expect %q", plan.ToolsNeeded, tool)
				}
			}
			if got := shouldRetrieveLocalContext(input); got != tc.wantRetrieve {
				t.Fatalf("shouldRetrieveLocalContext = %v, want %v", got, tc.wantRetrieve)
			}
			if tc.wantDecision != "" || tc.wantExecTool != "" {
				decision := DecideExecution(plan, input)
				if decision.Status != tc.wantDecision {
					t.Fatalf("decision.Status = %q, want %q; decision=%+v", decision.Status, tc.wantDecision, decision)
				}
				if decision.ToolName != tc.wantExecTool {
					t.Fatalf("decision.ToolName = %q, want %q; decision=%+v", decision.ToolName, tc.wantExecTool, decision)
				}
				if strings.Contains(strings.ToLower(tc.prompt), "edmonton") {
					command := strings.Join(decision.Command, " ")
					if !strings.Contains(command, "America/Edmonton") {
						t.Fatalf("decision.Command = %#v, want America/Edmonton target timezone", decision.Command)
					}
				}
				if tc.prompt == "US-China news and US visit to China" {
					query := strings.Join(decision.Command, " ")
					for _, want := range []string{"us", "china", "visit", "latest news"} {
						if !strings.Contains(query, want) {
							t.Fatalf("decision.Command = %#v, want shaped current-news query containing %q", decision.Command, want)
						}
					}
				}
			}
		})
	}
}

func TestModelToolLoopScopesRAGRoutesToRAGSearch(t *testing.T) {
	plan := BuildPlan(PlanInput{Content: "What is the content of quarterly-notes.txt from indexed files?"})
	if plan.TaskType != TaskRAG || !containsTool(plan.ToolsNeeded, "rag_search") {
		t.Fatalf("plan = %+v, want RAG plan with rag_search", plan)
	}
	names := modelToolDefinitionNames(plan, ModelToolOptions{})
	if !containsTool(names, "rag_search") {
		t.Fatalf("model tool names = %v, want rag_search", names)
	}
	for _, forbidden := range []string{"read_file", "search_files", "list_files"} {
		if containsTool(names, forbidden) {
			t.Fatalf("model tool names = %v, did not expect %q for RAG route", names, forbidden)
		}
	}
	instructions := ModelToolInstructionsWithOptions(plan, ModelToolOptions{})
	if !strings.Contains(instructions, `"tool_name":"rag_search"`) || strings.Contains(instructions, `"tool_name":"read_file"`) {
		t.Fatalf("instructions = %q, want rag_search example only", instructions)
	}
	blocked := DecisionFromModelToolRequest(plan, ModelToolRequest{ToolName: "read_file", Path: "quarterly-notes.txt"})
	if blocked.Status != ExecutionBlocked {
		t.Fatalf("read_file model decision = %+v, want blocked for RAG route", blocked)
	}
	ready := DecisionFromModelToolRequest(plan, ModelToolRequest{ToolName: "rag_search", Query: "quarterly-notes.txt"})
	if ready.Status != ExecutionReady || ready.ToolName != "rag_search" {
		t.Fatalf("rag_search model decision = %+v, want ready", ready)
	}
}

func TestPastedExplanationIgnoresActiveDocumentSkillTools(t *testing.T) {
	prompt := "explain \n\n## Executive summary\n\nYemaka is now best described as:\n\n```text\nPublic-beta / RC-ready foundation\n+ Post-RC smartness foundations already wired\n+ still needing RC/manual QA and focused cleanup before broad expansion\n```\n\nThis pasted release note mentions workspace, docs/blueprint.md, safe_tool, internet_search, search files, and run command as text to explain."
	input := PlanInput{
		Content:            prompt,
		SkillName:          "document_summary",
		SkillRequiredTools: []string{"read_file", "rag_search"},
	}
	plan := BuildPlan(input)
	if plan.TaskType != TaskReasoning {
		t.Fatalf("TaskType = %q, want %q; plan=%+v", plan.TaskType, TaskReasoning, plan)
	}
	if plan.RiskLevel != RiskLow {
		t.Fatalf("RiskLevel = %q, want %q; plan=%+v", plan.RiskLevel, RiskLow, plan)
	}
	if len(plan.ToolsNeeded) != 0 {
		t.Fatalf("ToolsNeeded = %v, want none; plan=%+v", plan.ToolsNeeded, plan)
	}
	if plan.RouteUsesWorkspace || plan.RouteUsesRAG || plan.RouteUsesInternet {
		t.Fatalf("route context flags workspace=%v rag=%v internet=%v, want all false; plan=%+v", plan.RouteUsesWorkspace, plan.RouteUsesRAG, plan.RouteUsesInternet, plan)
	}
	if shouldRetrieveLocalContext(input) {
		t.Fatal("shouldRetrieveLocalContext = true, want false")
	}
	decision := DecideExecution(plan, input)
	if decision.Status != ExecutionNotRequired {
		t.Fatalf("decision.Status = %q, want %q; decision=%+v", decision.Status, ExecutionNotRequired, decision)
	}
}

func TestBuildPlanClarifiesBroadCurrentNewsRequests(t *testing.T) {
	input := PlanInput{Content: "So what's happening today in the world?"}
	plan := BuildPlan(input)
	if plan.TaskType != TaskChat {
		t.Fatalf("TaskType = %q, want %q; plan=%+v", plan.TaskType, TaskChat, plan)
	}
	if !plan.NeedsClarification {
		t.Fatalf("NeedsClarification = false, want true; plan=%+v", plan)
	}
	if len(plan.ToolsNeeded) != 0 {
		t.Fatalf("ToolsNeeded = %v, want none before current-news clarification", plan.ToolsNeeded)
	}
	if shouldRetrieveLocalContext(input) {
		t.Fatal("shouldRetrieveLocalContext = true, want false for broad current-news clarification")
	}
	if shouldUseRAG(config.Default(), input.Content) {
		t.Fatal("shouldUseRAG = true, want false for broad current-news clarification")
	}
	if !strings.Contains(strings.ToLower(plan.ClarificationQuestion), "world news") {
		t.Fatalf("ClarificationQuestion = %q, want news categories", plan.ClarificationQuestion)
	}
}

func TestAskLocalTimeUsesSystemClockWithoutModelOrRetrieval(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	var requests []models.ChatRequest
	var retrieved bool
	var output string
	service := &Service{
		Router:  models.NewRouter(cfg),
		Runtime: recordingRuntime{text: "model should not run", requests: &requests},
		Memory:  store,
		Retriever: RetrieverFunc(func(ctx context.Context, content string) (RetrievalResult, error) {
			retrieved = true
			return RetrievalResult{Context: "README.md context", Sources: []string{"README.md"}, SourceKind: "workspace"}, nil
		}),
		ToolExecutor: NewSafeToolExecutor(SafeToolConfig{
			WorkspaceRoot:   t.TempDir(),
			TimeoutSeconds:  5,
			MaxContextChars: 1000,
		}),
	}

	err = service.Ask(ctx, AskInput{Content: "What is the time of the day?"}, func(event Event) error {
		if event.Type == EventModelToken {
			output += event.Token
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if retrieved {
		t.Fatal("retriever ran for local time request")
	}
	if len(requests) != 0 {
		t.Fatalf("model requests = %d, want 0", len(requests))
	}
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "source: local system clock") {
		t.Fatalf("output = %q, want local system clock source", output)
	}
	if !strings.Contains(lower, "morning") && !strings.Contains(lower, "afternoon") && !strings.Contains(lower, "evening") && !strings.Contains(lower, "night") {
		t.Fatalf("output = %q, want time-of-day label", output)
	}
}

func TestLocalTimeResponseIncludesTargetLocaleTimezone(t *testing.T) {
	loc, err := time.LoadLocation("America/Edmonton")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}
	base := time.Date(2026, time.May, 15, 19, 13, 2, 0, time.FixedZone("EEST", 3*60*60))
	context := formatLocalTimeResult(base.In(loc), "Edmonton, Alberta, Canada", "America/Edmonton")
	response := LocalTimeResponse(ExecutionResult{Context: context})
	lower := strings.ToLower(response)
	for _, want := range []string{
		"edmonton, alberta, canada",
		"mdt",
		"-06:00",
		"there",
		"iana timezone `america/edmonton`",
	} {
		if !strings.Contains(lower, want) {
			t.Fatalf("LocalTimeResponse() = %q, want %q", response, want)
		}
	}
	if strings.Contains(lower, " here.") {
		t.Fatalf("LocalTimeResponse() = %q, did not expect local-locale wording", response)
	}
}

func TestLocalTimeResponseIncludesLocalTimezone(t *testing.T) {
	base := time.Date(2026, time.May, 15, 19, 13, 2, 0, time.FixedZone("EEST", 3*60*60))
	context := formatLocalTimeResult(base, "", "Asia/Nicosia")
	response := LocalTimeResponse(ExecutionResult{Context: context})
	lower := strings.ToLower(response)
	for _, want := range []string{
		"19:13:02",
		"eest",
		"+03:00",
		"here",
		"local iana timezone `asia/nicosia`",
	} {
		if !strings.Contains(lower, want) {
			t.Fatalf("LocalTimeResponse() = %q, want %q", response, want)
		}
	}
}

func TestLocalTimeDecisionUsesLondonTargetTimezone(t *testing.T) {
	cases := []struct {
		prompt      string
		wantZone    string
		wantLabel   string
		notContains string
	}{
		{
			prompt:      "What is the time in London?",
			wantZone:    "Europe/London",
			wantLabel:   "London, United Kingdom",
			notContains: "America/Toronto",
		},
		{
			prompt:      "What is the time in London Ontario?",
			wantZone:    "America/Toronto",
			wantLabel:   "London, Ontario, Canada",
			notContains: "Europe/London",
		},
		{
			prompt:    "What is the time in Europe/London?",
			wantZone:  "Europe/London",
			wantLabel: "Europe/London",
		},
		{
			prompt:    "What is the time in Singapore?",
			wantZone:  "Asia/Singapore",
			wantLabel: "Singapore",
		},
	}
	for _, tc := range cases {
		t.Run(tc.prompt, func(t *testing.T) {
			input := PlanInput{Content: tc.prompt}
			plan := BuildPlan(input)
			if plan.TaskType != TaskTool || !containsTool(plan.ToolsNeeded, "local_time") {
				t.Fatalf("plan=%+v, want local_time tool route", plan)
			}
			decision := DecideExecution(plan, input)
			if decision.Status != ExecutionReady || decision.ToolName != "local_time" {
				t.Fatalf("decision=%+v, want ready local_time decision", decision)
			}
			command := strings.Join(decision.Command, " ")
			if !strings.Contains(command, tc.wantZone) {
				t.Fatalf("decision.Command=%#v, want %q", decision.Command, tc.wantZone)
			}
			if !strings.Contains(command, tc.wantLabel) {
				t.Fatalf("decision.Command=%#v, want label %q", decision.Command, tc.wantLabel)
			}
			if tc.notContains != "" && strings.Contains(command, tc.notContains) {
				t.Fatalf("decision.Command=%#v, did not expect %q", decision.Command, tc.notContains)
			}
		})
	}
}

func TestAdvancedRoutingRealWorldMatrix(t *testing.T) {
	cases := []struct {
		name         string
		prompt       string
		wantTask     string
		wantTools    []string
		notTools     []string
		wantRetrieve bool
	}{
		{name: "casual chat", prompt: "How are you doing today?", wantTask: TaskChat},
		{name: "creative writing", prompt: "Write a short birthday message for my sister.", wantTask: TaskChat},
		{name: "general knowledge", prompt: "What is SearXNG?", wantTask: TaskChat},
		{name: "local time", prompt: "What is the time of the day?", wantTask: TaskTool, wantTools: []string{"local_time"}, notTools: []string{"internet_search", "rag_search"}},
		{name: "unmapped time target clarifies", prompt: "What is the time in Atlantis?", wantTask: TaskChat, notTools: []string{"local_time", "internet_search", "rag_search"}},
		{name: "broad current news clarifies", prompt: "So what's happening today in the world?", wantTask: TaskChat, notTools: []string{"internet_search", "rag_search"}},
		{name: "specific current news search", prompt: "US-China news and US visit to China", wantTask: TaskTool, wantTools: []string{"internet_search"}, notTools: []string{"rag_search", "read_file", "safe_tool"}},
		{name: "explain website monitoring does not act", prompt: "Explain how website monitoring works.", wantTask: TaskReasoning, notTools: []string{"safe_tool", "internet_search", "internet_fetch"}},
		{name: "deploy explanation does not act", prompt: "How do I deploy a website?", wantTask: TaskChat, notTools: []string{"safe_tool", "internet_search", "internet_fetch"}},
		{name: "concept explanation", prompt: "Explain photosynthesis to a child.", wantTask: TaskReasoning},
		{name: "routing concept", prompt: "Explain deterministic vs heuristic routing.", wantTask: TaskReasoning, notTools: []string{"git_diff"}},
		{name: "study planning", prompt: "Plan a study schedule for my calculus exam.", wantTask: TaskReasoning, notTools: []string{"safe_tool"}},
		{name: "shopping recommendation", prompt: "Recommend a budget laptop for school notes.", wantTask: TaskReasoning},
		{name: "coding without files", prompt: "Write Python code for a Fibonacci function.", wantTask: TaskCoding, notTools: []string{"edit_file", "read_file", "search_files"}},
		{name: "debug pasted error", prompt: "Debug this traceback: ValueError invalid literal for int().", wantTask: TaskCoding, notTools: []string{"read_file", "search_files"}},
		{name: "project architecture", prompt: "Explain this repo architecture.", wantTask: TaskReasoning, wantRetrieve: true},
		{name: "workspace search", prompt: "Search files for config loader.", wantTask: TaskTool, wantTools: []string{"read_file", "search_files"}, wantRetrieve: true},
		{name: "defensive project audit", prompt: "Audit this repo for authentication vulnerabilities.", wantTask: TaskCoding, wantRetrieve: true},
		{name: "docs answer", prompt: "Answer from my docs about model routing.", wantTask: TaskRAG, wantTools: []string{"rag_search"}, wantRetrieve: true},
		{name: "docs search", prompt: "Search docs for connector registry.", wantTask: TaskRAG, wantTools: []string{"rag_search"}, wantRetrieve: true},
		{name: "web search", prompt: "Search the web for SearXNG.", wantTask: TaskTool, wantTools: []string{"internet_search"}},
		{name: "latest is web not test", prompt: "Look up latest Ollama release notes.", wantTask: TaskTool, wantTools: []string{"internet_search"}, notTools: []string{"run_tests"}},
		{name: "url fetch", prompt: "Fetch https://example.com and summarize it.", wantTask: TaskTool, wantTools: []string{"internet_fetch"}},
		{name: "status code", prompt: "Check status code for https://example.com.", wantTask: TaskTool, wantTools: []string{"internet_head"}},
		{name: "conversation memory", prompt: "What did we discuss yesterday?", wantTask: TaskChat},
		{name: "memory search", prompt: "Search memory for release checklist.", wantTask: TaskTool, wantTools: []string{"memory_search"}},
		{name: "run tests", prompt: "Run tests and summarize failures.", wantTask: TaskTool, wantTools: []string{"run_tests"}},
		{name: "git diff", prompt: "Show git diff.", wantTask: TaskTool, wantTools: []string{"git_diff"}},
		{name: "doctor diagnostic", prompt: "Run Yemaka doctor status.", wantTask: TaskTool, wantTools: []string{"doctor_status"}},
		{name: "human doctor", prompt: "What should I ask my doctor about high blood pressure?", wantTask: TaskChat, notTools: []string{"doctor_status"}},
		{name: "heartbeat diagnostic", prompt: "Run Yemaka heartbeat status.", wantTask: TaskTool, wantTools: []string{"heartbeat_status"}},
		{name: "human health question is not heartbeat tool", prompt: "What heart health questions should I ask my doctor?", wantTask: TaskChat, notTools: []string{"heartbeat_status"}},
		{name: "patch preview", prompt: "Preview patch for README.md content: hello", wantTask: TaskTool, wantTools: []string{"patch_preview"}, wantRetrieve: true},
		{name: "extension generation", prompt: "Build a tool that monitors a website every hour.", wantTask: TaskTool, wantTools: []string{"safe_tool"}, notTools: []string{"internet_fetch", "internet_search"}},
		{name: "reusable webpage monitor gap", prompt: "I want a reusable tool that checks a webpage every hour and reports if the title changes. If Yemaka does not already have this capability, propose the extension but do not generate it until I approve.", wantTask: TaskTool, wantTools: []string{"safe_tool"}, notTools: []string{"internet_fetch", "internet_search", "read_file", "search_files"}, wantRetrieve: false},
		{name: "connector generation", prompt: "Create a Slack connector to post build results.", wantTask: TaskTool, wantTools: []string{"safe_tool"}},
		{name: "skill generation", prompt: "Create a reusable skill from this workflow.", wantTask: TaskTool, wantTools: []string{"safe_tool"}},
		{name: "pentest plan no substring test", prompt: "Create a pentest checklist for my lab network.", wantTask: TaskReasoning, notTools: []string{"run_tests"}},
		{name: "security concept", prompt: "Explain command injection conceptually.", wantTask: TaskCoding, notTools: []string{"safe_tool", "run_tests"}},
		{name: "real project investigation", prompt: "Investigate why login tests are failing in this repo, inspect relevant files, and summarize the likely fix.", wantTask: TaskCoding, wantRetrieve: true},
		{name: "local architecture trace", prompt: "Trace how this project routes a user request from planning to tool execution.", wantTask: TaskReasoning, wantRetrieve: true},
		{name: "scheduler workflow design", prompt: "Create a plan to monitor https://example.com every hour and save changes as a scheduled job.", wantTask: TaskTool, wantTools: []string{"safe_tool"}, notTools: []string{"run_tests"}},
		{name: "research with configured search", prompt: "Search the internet for SearXNG documentation and summarize what it is for.", wantTask: TaskTool, wantTools: []string{"internet_search"}, notTools: []string{"read_file", "search_files"}},
		{name: "local memory plus summary", prompt: "Search memory for the last release checklist and summarize the remaining blockers.", wantTask: TaskTool, wantTools: []string{"memory_search"}, notTools: []string{"internet_search"}},
		{name: "repo plus docs distinction", prompt: "Use my docs to explain extension rollback, not the README.", wantTask: TaskRAG, wantTools: []string{"rag_search"}, wantRetrieve: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := PlanInput{Content: tc.prompt}
			plan := BuildPlan(input)
			decision := DecideExecution(plan, input)
			retrieve := shouldRetrieveLocalContext(input)
			t.Logf("prompt=%q task=%s model=%s risk=%s tools=%v decision=%s/%s retrieve=%v", tc.prompt, plan.TaskType, plan.ModelTask, plan.RiskLevel, plan.ToolsNeeded, decision.Status, decision.ToolName, retrieve)
			if tc.wantTask != "" && plan.TaskType != tc.wantTask {
				t.Fatalf("TaskType = %q, want %q", plan.TaskType, tc.wantTask)
			}
			for _, tool := range tc.wantTools {
				if !containsTool(plan.ToolsNeeded, tool) {
					t.Fatalf("ToolsNeeded = %v, want %q", plan.ToolsNeeded, tool)
				}
			}
			for _, tool := range tc.notTools {
				if containsTool(plan.ToolsNeeded, tool) {
					t.Fatalf("ToolsNeeded = %v, did not expect %q", plan.ToolsNeeded, tool)
				}
			}
			if retrieve != tc.wantRetrieve {
				t.Fatalf("shouldRetrieveLocalContext = %v, want %v", retrieve, tc.wantRetrieve)
			}
		})
	}
}

func TestInternetSearchQueryHumanTargetRegressions(t *testing.T) {
	query := internetSearchQuery("Yes search for more information on macos26.5")
	if strings.Contains(query, "yes search") || strings.Contains(query, "more information") {
		t.Fatalf("internetSearchQuery() = %q, want stripped search target", query)
	}
	if !strings.Contains(query, "macos26.5") {
		t.Fatalf("internetSearchQuery() = %q, want macos26.5 target", query)
	}

	current := internetSearchQuery("When will the new apple macOS be released?")
	for _, want := range []string{"apple", "macos", time.Now().Format("2006"), time.Now().Format("2006-01-02")} {
		if !strings.Contains(strings.ToLower(current), strings.ToLower(want)) {
			t.Fatalf("internetSearchQuery() = %q, want %q", current, want)
		}
	}

	generic := internetSearchQuery("What is the latest PostgreSQL version?")
	for _, want := range []string{"postgresql", time.Now().Format("2006"), time.Now().Format("2006-01-02")} {
		if !strings.Contains(strings.ToLower(generic), strings.ToLower(want)) {
			t.Fatalf("internetSearchQuery() = %q, want %q", generic, want)
		}
	}

	followupInput := PlanInput{
		Content: "Search more about him",
		TaskMemory: strings.Join([]string{
			"RECENT SESSION MESSAGES (conversation continuity only; not current source evidence):",
			"user: Find the Phoenix Group UAE CEO and latest public business background.",
			"assistant: I need current search for Phoenix Group UAE CEO before answering.",
		}, "\n"),
	}
	followup := internetSearchQueryForInput(followupInput)
	for _, want := range []string{"Phoenix", "Group", "UAE", "CEO"} {
		if !strings.Contains(followup, want) {
			t.Fatalf("internetSearchQueryForInput() = %q, want preserved target token %q", followup, want)
		}
	}
	if strings.EqualFold(strings.TrimSpace(followup), "Search more about him") {
		t.Fatalf("internetSearchQueryForInput() = %q, want concrete remembered target", followup)
	}
	plan := BuildPlan(followupInput)
	decision := DecideExecution(plan, followupInput)
	if decision.ToolName != "internet_search" || len(decision.Command) < 2 {
		t.Fatalf("decision=%+v, want internet_search command", decision)
	}
	if !strings.Contains(decision.Command[1], "Phoenix Group UAE CEO") {
		t.Fatalf("decision.Command = %#v, want remembered person/business target", decision.Command)
	}
}

func containsRouteSubstring(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}
