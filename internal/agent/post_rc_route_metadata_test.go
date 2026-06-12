package agent

import (
	"testing"

	promptcore "yemaka/internal/prompt"
	"yemaka/internal/replay"
	"yemaka/internal/routing"
)

func TestBuildPlanPreservesPostRCRouteMetadata(t *testing.T) {
	cases := []struct {
		name           string
		prompt         string
		wantCategory   string
		wantIntent     string
		wantDomain     string
		wantTarget     string
		wantCapability string
		wantApproval   bool
	}{
		{
			name:           "internet search",
			prompt:         "Search the web for SearXNG documentation.",
			wantCategory:   routing.RouteInternetSearch,
			wantIntent:     routing.IntentResearch,
			wantDomain:     routing.DomainGeneral,
			wantCapability: "internet",
		},
		{
			name:           "local document search",
			prompt:         "Search docs for architecture notes.",
			wantCategory:   routing.RouteRAGSearch,
			wantIntent:     routing.IntentResearch,
			wantDomain:     routing.DomainArchitecture,
			wantTarget:     "local_documents",
			wantCapability: "rag",
		},
		{
			name:           "file edit approval",
			prompt:         "Edit README.md to add a troubleshooting note.",
			wantCategory:   routing.RouteFileWrite,
			wantIntent:     routing.IntentEdit,
			wantDomain:     routing.DomainGeneral,
			wantTarget:     "README.md",
			wantCapability: "filesystem_write",
			wantApproval:   true,
		},
		{
			name:           "scheduler creation approval",
			prompt:         "Create a scheduled job that runs every hour to summarize release notes.",
			wantCategory:   routing.RouteSchedulerCreate,
			wantIntent:     routing.IntentSchedule,
			wantDomain:     routing.DomainGeneral,
			wantTarget:     "scheduler_job",
			wantCapability: "scheduler",
			wantApproval:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := PlanInput{Content: tc.prompt}
			route := RouteRequest(input)
			plan := BuildPlan(input)

			if route.RouteCategory != tc.wantCategory {
				t.Fatalf("route.RouteCategory = %q, want %q; route=%+v", route.RouteCategory, tc.wantCategory, route)
			}
			if route.Intent != tc.wantIntent {
				t.Fatalf("route.Intent = %q, want %q; route=%+v", route.Intent, tc.wantIntent, route)
			}
			if route.Domain != tc.wantDomain {
				t.Fatalf("route.Domain = %q, want %q; route=%+v", route.Domain, tc.wantDomain, route)
			}
			if route.Target != tc.wantTarget {
				t.Fatalf("route.Target = %q, want %q; route=%+v", route.Target, tc.wantTarget, route)
			}
			if route.Capability != tc.wantCapability {
				t.Fatalf("route.Capability = %q, want %q; route=%+v", route.Capability, tc.wantCapability, route)
			}
			if route.RequiresApproval != tc.wantApproval {
				t.Fatalf("route.RequiresApproval = %v, want %v; route=%+v", route.RequiresApproval, tc.wantApproval, route)
			}
			assertPlanPreservesRouteMetadata(t, plan, route)
		})
	}
}

func TestReplayTraceFromPlanCopiesRouteAndPlanSnapshot(t *testing.T) {
	input := PlanInput{Content: "Edit README.md to add a troubleshooting note."}
	plan := BuildPlan(input)
	trace := ReplayTraceFromPlan(input, plan)

	if trace.Schema != replay.TraceSchema {
		t.Fatalf("Schema = %q, want %q", trace.Schema, replay.TraceSchema)
	}
	if trace.UserRequest != input.Content {
		t.Fatalf("UserRequest = %q, want %q", trace.UserRequest, input.Content)
	}
	if trace.Route.Category != routing.RouteFileWrite {
		t.Fatalf("trace.Route.Category = %q, want %q; trace=%+v", trace.Route.Category, routing.RouteFileWrite, trace)
	}
	if trace.Route.Intent != routing.IntentEdit {
		t.Fatalf("trace.Route.Intent = %q, want %q; trace=%+v", trace.Route.Intent, routing.IntentEdit, trace)
	}
	if trace.Route.Domain != routing.DomainGeneral {
		t.Fatalf("trace.Route.Domain = %q, want %q; trace=%+v", trace.Route.Domain, routing.DomainGeneral, trace)
	}
	if trace.Route.Target != "README.md" {
		t.Fatalf("trace.Route.Target = %q, want README.md; trace=%+v", trace.Route.Target, trace)
	}
	if trace.Route.Capability != "filesystem_write" {
		t.Fatalf("trace.Route.Capability = %q, want filesystem_write; trace=%+v", trace.Route.Capability, trace)
	}
	if !trace.Route.ShouldAskApproval {
		t.Fatalf("trace.Route.ShouldAskApproval = false, want true; trace=%+v", trace)
	}
	if !trace.Route.ShouldWriteFiles {
		t.Fatalf("trace.Route.ShouldWriteFiles = false, want true; trace=%+v", trace)
	}
	if trace.Plan.Goal != plan.Goal {
		t.Fatalf("trace.Plan.Goal = %q, want %q", trace.Plan.Goal, plan.Goal)
	}
	if trace.Plan.MaxSteps != plan.MaxSteps {
		t.Fatalf("trace.Plan.MaxSteps = %d, want %d", trace.Plan.MaxSteps, plan.MaxSteps)
	}
	if !containsTool(trace.Plan.ToolsNeeded, "edit_file") {
		t.Fatalf("trace.Plan.ToolsNeeded = %v, want copied edit_file tool", trace.Plan.ToolsNeeded)
	}
	if !hasReplayToolConsideration(trace.ToolsConsidered, "edit_file", "planned") {
		t.Fatalf("trace.ToolsConsidered = %+v, want planned edit_file diagnostic", trace.ToolsConsidered)
	}
}

func TestReplayTraceFromPlanIncludesPreflightSummary(t *testing.T) {
	input := PlanInput{Content: "Who is the CEO of Phoenix Group UAE?"}
	plan := BuildPlan(input)
	trace := ReplayTraceFromPlan(input, plan)

	if got := replayAttributeValue(trace.Attributes, "route_preflight_source_of_truth"); got != routing.PreflightSourceInternet {
		t.Fatalf("route_preflight_source_of_truth = %q, want internet; attrs=%+v", got, trace.Attributes)
	}
	if got := replayAttributeValue(trace.Attributes, "route_preflight_freshness_risk"); got != routing.PreflightFreshnessHigh {
		t.Fatalf("route_preflight_freshness_risk = %q, want high; attrs=%+v", got, trace.Attributes)
	}
	if got := replayAttributeValue(trace.Attributes, "route_preflight_selected_route"); got != routing.RouteInternetSearch {
		t.Fatalf("route_preflight_selected_route = %q, want internet_search; attrs=%+v", got, trace.Attributes)
	}
}

func TestReplayTraceFromPlanIncludesContinuationSummary(t *testing.T) {
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
	trace := ReplayTraceFromPlan(input, plan)

	if got := replayAttributeValue(trace.Attributes, "route_continuation_kind"); got != routing.ContinuationKindRetry {
		t.Fatalf("route_continuation_kind = %q, want retry; attrs=%+v", got, trace.Attributes)
	}
	if got := replayAttributeValue(trace.Attributes, "route_continuation_prior_target"); got != "prior public fact target" {
		t.Fatalf("route_continuation_prior_target = %q, want prior target; attrs=%+v", got, trace.Attributes)
	}
	if got := replayAttributeValue(trace.Attributes, "route_preflight_blocked_requirement"); got != "internet access is disabled for this profile" {
		t.Fatalf("route_preflight_blocked_requirement = %q, want prior failure reason; attrs=%+v", got, trace.Attributes)
	}
	if got := replayAttributeValue(trace.Attributes, "route_preflight_required_tool"); got != "internet_search" {
		t.Fatalf("route_preflight_required_tool = %q, want internet_search; attrs=%+v", got, trace.Attributes)
	}
}

func TestReplayTraceFromAgentRunRecordsExecutionToolConsideration(t *testing.T) {
	input := PlanInput{Content: "Edit README.md to add a troubleshooting note."}
	plan := BuildPlan(input)
	decision := ExecutionDecision{
		Status:               ExecutionNeedsConfirmation,
		RequestID:            "perm-edit-1",
		ToolName:             "edit_file",
		RiskLevel:            RiskMedium,
		RequiresConfirmation: true,
		Reason:               "file edits require confirmation and snapshot flow",
		PolicyLevel:          2,
		PolicyExplanation:    "confirm workspace write",
	}

	trace := replayTraceFromAgentRun(agentReplayTraceInput{
		ConversationID:     "conv-approval",
		UserMessageID:      "user-approval",
		AssistantMessageID: "assistant-approval",
		Input:              input,
		Plan:               plan,
		Decision:           decision,
		AssistantContent:   "I need approval before editing.",
	})

	if !hasReplayToolConsideration(trace.ToolsConsidered, "edit_file", "planned") {
		t.Fatalf("trace.ToolsConsidered = %+v, want planned edit_file diagnostic", trace.ToolsConsidered)
	}
	if !hasReplayToolConsideration(trace.ToolsConsidered, "edit_file", "needs_confirmation") {
		t.Fatalf("trace.ToolsConsidered = %+v, want needs_confirmation edit_file diagnostic", trace.ToolsConsidered)
	}
	if len(trace.ToolsCalled) != 0 {
		t.Fatalf("trace.ToolsCalled = %+v, want no tool call before approval", trace.ToolsCalled)
	}
	if len(trace.PermissionsRequested) != 1 || trace.PermissionsRequested[0].Status != "needs_confirmation" {
		t.Fatalf("trace.PermissionsRequested = %+v, want approval diagnostic", trace.PermissionsRequested)
	}
}

func TestReplayTraceFromAgentRunRecordsRouteRecoveryDecision(t *testing.T) {
	input := PlanInput{Content: "Search current public news"}
	plan := BuildPlan(input)
	plan.RouteCategory = routing.RouteInternetSearch
	plan.RouteLane = routing.ToolLaneWebSearch
	plan.RouteUsesInternet = true
	decision := ExecutionDecision{
		Status:   ExecutionBlocked,
		ToolName: "internet_search",
		Reason:   "provider not configured",
	}
	result := &ExecutionResult{
		Status:     "failed",
		Context:    "SearXNG provider not configured",
		SourceKind: "tool",
	}

	trace := replayTraceFromAgentRun(agentReplayTraceInput{
		ConversationID:     "conv-recovery",
		UserMessageID:      "user-recovery",
		AssistantMessageID: "assistant-recovery",
		Input:              input,
		Plan:               plan,
		Decision:           decision,
		Result:             result,
		AssistantContent:   "Provider configuration is required.",
	})

	if got := replayAttributeValue(trace.Attributes, "route_recovery_trigger"); got != routing.RouteRecoveryTriggerProviderUnavailable {
		t.Fatalf("route_recovery_trigger = %q, want provider_unavailable; attrs=%+v", got, trace.Attributes)
	}
	if got := replayAttributeValue(trace.Attributes, "route_recovery_final_action"); got != routing.RouteRecoveryActionRequestConfiguration {
		t.Fatalf("route_recovery_final_action = %q, want request_configuration; attrs=%+v", got, trace.Attributes)
	}
}

func TestReplayTraceFromAgentRunAttachesContextDiagnosticsAndRefs(t *testing.T) {
	input := PlanInput{
		Content:          "Explain this project from retrieved context.",
		TaskMemory:       "Previous useful workflow note.",
		MemorySources:    []string{"workflow:abc"},
		WorkspaceContext: "README.md says Yemaka is local-first.",
		SourceKind:       "rag",
		Sources:          []string{"README.md"},
	}
	plan := BuildPlan(input)
	options := promptcore.Options{MaxChars: 2400, LowMemory: true}
	compiled := compilePromptContext(plan, input, options)

	trace := replayTraceFromAgentRun(agentReplayTraceInput{
		ConversationID:     "conv-1",
		UserMessageID:      "user-1",
		AssistantMessageID: "assistant-1",
		Input:              input,
		Plan:               plan,
		Decision:           ExecutionDecision{Status: ExecutionNotRequired},
		AssistantContent:   "Yemaka is local-first.",
		PromptContext:      compiled.Result,
		PromptOptions:      options,
	})

	if trace.Context == nil || !trace.Context.LowMemory {
		t.Fatalf("trace.Context = %+v, want low-memory context diagnostics", trace.Context)
	}
	if trace.Context.Usage.UsedBytes == 0 || len(trace.Context.Included) == 0 {
		t.Fatalf("trace.Context = %+v, want usage and included metadata", trace.Context)
	}
	if len(trace.MemoryUsed) != 1 || trace.MemoryUsed[0].Title != "workflow:abc" {
		t.Fatalf("trace.MemoryUsed = %+v, want memory source ref", trace.MemoryUsed)
	}
	if len(trace.RAGDocsUsed) != 1 || trace.RAGDocsUsed[0].Title != "README.md" {
		t.Fatalf("trace.RAGDocsUsed = %+v, want RAG source ref", trace.RAGDocsUsed)
	}
	if !hasReplayToolConsideration(trace.ToolsConsidered, "rag_search", "planned") {
		t.Fatalf("trace.ToolsConsidered = %+v, want planned rag_search diagnostic", trace.ToolsConsidered)
	}
	for _, item := range trace.Context.Included {
		if item.Reason == "" {
			t.Fatalf("included context item missing reason: %+v", item)
		}
	}
}

func assertPlanPreservesRouteMetadata(t *testing.T, plan Plan, route RouteDecision) {
	t.Helper()
	if plan.RouteCategory != route.RouteCategory {
		t.Fatalf("plan.RouteCategory = %q, want route %q; plan=%+v route=%+v", plan.RouteCategory, route.RouteCategory, plan, route)
	}
	if plan.RouteIntent != route.Intent {
		t.Fatalf("plan.RouteIntent = %q, want route %q; plan=%+v route=%+v", plan.RouteIntent, route.Intent, plan, route)
	}
	if plan.RouteDomain != route.Domain {
		t.Fatalf("plan.RouteDomain = %q, want route %q; plan=%+v route=%+v", plan.RouteDomain, route.Domain, plan, route)
	}
	if plan.RouteTarget != route.Target {
		t.Fatalf("plan.RouteTarget = %q, want route %q; plan=%+v route=%+v", plan.RouteTarget, route.Target, plan, route)
	}
	if plan.RouteCapability != route.Capability {
		t.Fatalf("plan.RouteCapability = %q, want route %q; plan=%+v route=%+v", plan.RouteCapability, route.Capability, plan, route)
	}
	if plan.RouteRequiresApproval != route.RequiresApproval {
		t.Fatalf("plan.RouteRequiresApproval = %v, want route %v; plan=%+v route=%+v", plan.RouteRequiresApproval, route.RequiresApproval, plan, route)
	}
}

func replayAttributeValue(values []replay.Attribute, key string) string {
	for _, value := range values {
		if value.Key == key {
			return value.Value
		}
	}
	return ""
}
