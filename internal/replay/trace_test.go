package replay

import "testing"

func TestTraceNormalizeKeepsDeterministicGenericShape(t *testing.T) {
	trace := NewTrace("  Explain website monitoring  ")
	trace.ID = " trace-1 "
	trace.Schema = ""
	trace.Route = RouteSnapshot{
		Category:   " scheduler_create ",
		RiskLevel:  " medium ",
		Confidence: 140,
		Reasons:    []string{" action phrase ", "", " action phrase "},
	}
	trace.Plan = PlanSnapshot{
		Goal:        " monitor site ",
		Steps:       []string{" create job ", " create job ", ""},
		ToolsNeeded: []string{" scheduler_create ", ""},
		MaxSteps:    -1,
	}
	trace.Attributes = []Attribute{
		{Key: "z", Value: "last"},
		{Key: "a", Value: "first"},
		{Key: "a", Value: "first"},
		{Key: "", Value: "ignored"},
	}
	trace.MemoryUsed = []DocumentRef{{}, {ID: " mem-1 ", Source: " memory ", Snippet: " note "}}
	trace.ToolsConsidered = []ToolConsideration{
		{},
		{Name: " internet_search ", Source: " plan ", Status: " planned ", Reason: " route selected search "},
		{Name: "internet_search", Source: "plan", Status: "planned", Reason: "route selected search"},
	}
	trace.Context = &ContextSnapshot{
		LowMemory:       true,
		ItemsConsidered: -1,
		BySource: []ContextSourceUsage{
			{Source: " rag ", Bytes: -3, Tokens: 10},
			{Source: "rag", Bytes: 99, Tokens: 99},
		},
		Included: []ContextItem{{ID: " retrieved_context ", Source: " rag ", Reason: " selected "}},
		Dropped:  []ContextItem{{ID: " task_memory ", Source: " memory ", Reason: " budget_exhausted "}},
	}

	got := trace.Normalize()
	if got.Schema != TraceSchema {
		t.Fatalf("Schema = %q, want %q", got.Schema, TraceSchema)
	}
	if got.UserRequest != "Explain website monitoring" || got.ID != "trace-1" {
		t.Fatalf("normalized identifiers = %#v", got)
	}
	if got.Route.Confidence != 100 || len(got.Route.Reasons) != 1 || got.Route.Reasons[0] != "action phrase" {
		t.Fatalf("Route = %+v, want clamped confidence and unique reasons", got.Route)
	}
	if got.Plan.MaxSteps != 0 || len(got.Plan.Steps) != 1 || got.Plan.ToolsNeeded[0] != "scheduler_create" {
		t.Fatalf("Plan = %+v, want compact plan", got.Plan)
	}
	if len(got.Attributes) != 2 || got.Attributes[0].Key != "a" || got.Attributes[1].Key != "z" {
		t.Fatalf("Attributes = %+v, want deterministic sorted attributes", got.Attributes)
	}
	if len(got.MemoryUsed) != 1 || got.MemoryUsed[0].ID != "mem-1" {
		t.Fatalf("MemoryUsed = %+v, want empty refs dropped", got.MemoryUsed)
	}
	if len(got.ToolsConsidered) != 1 || got.ToolsConsidered[0].Name != "internet_search" {
		t.Fatalf("ToolsConsidered = %+v, want normalized unique consideration", got.ToolsConsidered)
	}
	if got.Context == nil || !got.Context.LowMemory || got.Context.ItemsConsidered != 0 {
		t.Fatalf("Context = %+v, want normalized context snapshot", got.Context)
	}
	if len(got.Context.BySource) != 1 || got.Context.BySource[0].Source != "rag" || got.Context.BySource[0].Bytes != 0 {
		t.Fatalf("Context.BySource = %+v, want compact source usage", got.Context.BySource)
	}
	if len(got.Context.Included) != 1 || got.Context.Included[0].ID != "retrieved_context" {
		t.Fatalf("Context.Included = %+v, want normalized context item", got.Context.Included)
	}
}

func TestTraceAppendHelpersNormalizeSnapshots(t *testing.T) {
	trace := NewTrace("  Fetch current docs  ")
	trace.Schema = ""
	trace.ID = " trace-live-1 "

	trace.AppendRouteSnapshot(RouteSnapshot{
		Category:          " internet_search ",
		RiskLevel:         " medium ",
		Confidence:        120,
		Reasons:           []string{" needs current data ", "needs current data", ""},
		ShouldUseTool:     true,
		ShouldUseInternet: true,
	})
	trace.AppendPlanSnapshot(PlanSnapshot{
		Goal:        " fetch docs ",
		Steps:       []string{" search ", " search ", ""},
		ToolsNeeded: []string{" internet_search ", ""},
		MaxSteps:    -3,
	})
	trace.AppendToolCall(ToolCall{})
	trace.AppendToolConsideration(ToolConsideration{
		Name:      " internet_search ",
		Source:    " execution_decision ",
		Status:    " blocked ",
		Reason:    " internet disabled ",
		RiskLevel: " medium ",
	})
	trace.AppendToolCall(ToolCall{
		Name:   " internet_search ",
		Status: " failed ",
		Error:  " provider not configured ",
		Attributes: []Attribute{
			{Key: "provider", Value: " searxng "},
			{Key: "provider", Value: "searxng"},
		},
	})
	trace.AppendPermissionRequest(PermissionRequest{
		ID:                " permission-1 ",
		ToolName:          " internet_search ",
		Status:            " denied ",
		Reason:            " internet disabled ",
		PolicyExplanation: " ask_each_time requires approval ",
	})
	trace.AppendError(TraceError{
		Stage:         " routing ",
		Code:          " internet_disabled ",
		Subject:       " internet_search ",
		Message:       " provider not configured ",
		ExpectedRoute: " internet_search ",
		Attributes:    []Attribute{{Key: "z", Value: "last"}, {Key: "a", Value: "first"}},
	})
	trace.AppendFinalResult(ResultSnapshot{
		Status:  " failed ",
		Summary: " could not search ",
	})
	trace.AppendVerification(VerificationResult{
		Status: " failed ",
		Checks: []string{" search failed ", " search failed "},
		Notes:  []string{" internet provider missing ", ""},
	})

	if trace.Schema != TraceSchema || trace.ID != "trace-live-1" || trace.UserRequest != "Fetch current docs" {
		t.Fatalf("trace identifiers = %+v, want normalized identifiers", trace)
	}
	if trace.Route.Confidence != 100 || len(trace.Route.Reasons) != 1 || !trace.Route.ShouldUseInternet {
		t.Fatalf("Route = %+v, want normalized internet route", trace.Route)
	}
	if trace.Plan.MaxSteps != 0 || len(trace.Plan.Steps) != 1 || trace.Plan.ToolsNeeded[0] != "internet_search" {
		t.Fatalf("Plan = %+v, want normalized plan", trace.Plan)
	}
	if len(trace.ToolsConsidered) != 1 || trace.ToolsConsidered[0].Status != "blocked" {
		t.Fatalf("ToolsConsidered = %+v, want normalized tool consideration", trace.ToolsConsidered)
	}
	if len(trace.ToolsCalled) != 1 || trace.ToolsCalled[0].Name != "internet_search" || trace.ToolsCalled[0].Attributes[0].Value != "searxng" {
		t.Fatalf("ToolsCalled = %+v, want normalized non-empty tool call", trace.ToolsCalled)
	}
	if len(trace.PermissionsRequested) != 1 || trace.PermissionsRequested[0].Status != "denied" {
		t.Fatalf("PermissionsRequested = %+v, want normalized permission", trace.PermissionsRequested)
	}
	if len(trace.Errors) != 1 || trace.Errors[0].Stage != "routing" || trace.Errors[0].Attributes[0].Key != "a" {
		t.Fatalf("Errors = %+v, want normalized sorted error attributes", trace.Errors)
	}
	if trace.FinalResult.Status != "failed" || trace.Verification.Status != "failed" || len(trace.Verification.Checks) != 1 {
		t.Fatalf("final/verification = %+v / %+v, want normalized failure snapshots", trace.FinalResult, trace.Verification)
	}
}

func TestTraceDetectsPrimaryFailures(t *testing.T) {
	trace := NewTrace("Fetch current docs")
	trace.PermissionsRequested = []PermissionRequest{{
		ToolName:          "internet_search",
		Status:            "denied",
		Reason:            "internet disabled",
		PolicyExplanation: "internet requires approval",
	}}
	trace.ToolsCalled = []ToolCall{{
		Name:   "internet_search",
		Status: "failed",
		Error:  "provider not configured",
	}}

	if !trace.HasFailures() {
		t.Fatal("HasFailures() = false, want true")
	}
	failure, ok := trace.PrimaryFailure()
	if !ok {
		t.Fatal("PrimaryFailure() ok = false, want true")
	}
	if failure.Stage != "permission" || failure.Subject != "internet_search" || failure.Code != "permission_denied" {
		t.Fatalf("PrimaryFailure() = %+v, want permission failure first", failure)
	}
}

func TestTraceUsesExplicitErrorAsPrimaryFailure(t *testing.T) {
	trace := NewTrace("Explain website monitoring")
	trace.Errors = []TraceError{{
		Stage:         "routing",
		Code:          "false_positive_action",
		Message:       "created scheduler job",
		ExpectedRoute: "chat_explanation",
	}}
	trace.ToolsCalled = []ToolCall{{Name: "scheduler_create", Status: "completed"}}

	failure, ok := trace.PrimaryFailure()
	if !ok {
		t.Fatal("PrimaryFailure() ok = false, want true")
	}
	if failure.Stage != "routing" || failure.ExpectedRoute != "chat_explanation" {
		t.Fatalf("PrimaryFailure() = %+v, want explicit routing error", failure)
	}
}

func TestStatusHelpers(t *testing.T) {
	if !StatusFailed("needs-follow-up") || !StatusFailed("timed out") {
		t.Fatal("StatusFailed() did not recognize common failure spellings")
	}
	if !StatusSuccessful("completed") || StatusSuccessful("blocked") {
		t.Fatal("StatusSuccessful() did not recognize common success/failure statuses")
	}
}
