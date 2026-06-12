package routing

import "testing"

func TestRecoverRouteRequestsConfigurationForSearchProviderFailure(t *testing.T) {
	got := RecoverRoute(RouteRecoveryInput{
		SelectedRoute: RouteInternetSearch,
		ToolName:      "internet_search",
		ErrorText:     "could not reach SearXNG search provider: connection refused",
	})
	if got.Trigger != RouteRecoveryTriggerProviderUnavailable {
		t.Fatalf("Trigger = %q, want provider_unavailable; got=%+v", got.Trigger, got)
	}
	if got.FinalAction != RouteRecoveryActionRequestConfiguration || !got.RequiresConfiguration {
		t.Fatalf("action/config = %q/%t, want request_configuration/true; got=%+v", got.FinalAction, got.RequiresConfiguration, got)
	}
}

func TestRecoverRouteReportsSearchDuringWrongRoute(t *testing.T) {
	got := RecoverRoute(RouteRecoveryInput{
		SelectedRoute: RouteRAGSearch,
		ToolName:      "internet_search",
		ErrorText:     "provider not configured",
	})
	if got.Trigger != RouteRecoveryTriggerToolResultContradictsRoute {
		t.Fatalf("Trigger = %q, want tool_result_contradicts_route; got=%+v", got.Trigger, got)
	}
	if got.FinalAction != RouteRecoveryActionAskClarification || !got.RequiresClarification {
		t.Fatalf("action/clarify = %q/%t, want ask_clarification/true; got=%+v", got.FinalAction, got.RequiresClarification, got)
	}
}

func TestRecoverRouteDetectsWrongSourceType(t *testing.T) {
	got := RecoverRoute(RouteRecoveryInput{
		SelectedRoute:    RouteRAGSearch,
		SelectedSource:   PreflightSourceLocalDocuments,
		ToolName:         "rag_search",
		ResultSourceKind: "internet",
		ResultSummary:    "external web evidence",
	})
	if got.Trigger != RouteRecoveryTriggerWrongSourceType {
		t.Fatalf("Trigger = %q, want wrong_source_type; got=%+v", got.Trigger, got)
	}
	if got.FinalAction != RouteRecoveryActionAskClarification || !got.RequiresClarification {
		t.Fatalf("action/clarify = %q/%t, want ask_clarification/true; got=%+v", got.FinalAction, got.RequiresClarification, got)
	}
	if got.ProposedRoute != RouteRAGSearch {
		t.Fatalf("ProposedRoute = %q, want same selected route; got=%+v", got.ProposedRoute, got)
	}
}

func TestRecoverRouteRequestsWorkspaceGrantForFilePermission(t *testing.T) {
	got := RecoverRoute(RouteRecoveryInput{
		SelectedRoute: RouteFileWrite,
		ToolName:      "edit_file",
		ErrorText:     "open /Users/example/Documents/report.md: operation not permitted",
	})
	if got.Trigger != RouteRecoveryTriggerMissingPermission {
		t.Fatalf("Trigger = %q, want missing_permission; got=%+v", got.Trigger, got)
	}
	if got.FinalAction != RouteRecoveryActionRequestConfiguration || !got.RequiresConfiguration {
		t.Fatalf("action/config = %q/%t, want request_configuration/true; got=%+v", got.FinalAction, got.RequiresConfiguration, got)
	}
}

func TestRecoverRouteExplainsUploadedCopyVersusGrantPath(t *testing.T) {
	got := RecoverRoute(RouteRecoveryInput{
		SelectedRoute:    RouteFileRead,
		SelectedSource:   "attachment",
		ToolName:         "read_file",
		ResultSourceKind: "attachment",
		ErrorText:        "uploaded copy cannot be opened as a filesystem path: operation not permitted",
	})
	if got.Trigger != RouteRecoveryTriggerMissingPermission {
		t.Fatalf("Trigger = %q, want missing_permission; got=%+v", got.Trigger, got)
	}
	if !containsControlPlaneText(got.Reason, "conversation copy") || !containsControlPlaneText(got.Reason, "grant") {
		t.Fatalf("Reason = %q, want upload-copy vs grant-path guidance", got.Reason)
	}
}

func TestRecoverRouteHandlesTargetCorrection(t *testing.T) {
	got := RecoverRoute(RouteRecoveryInput{
		SelectedRoute: RouteFileRead,
		Continuation:  BuildContinuationFrame(ContinuationInput{Content: "no, I meant the other file"}),
	})
	if got.Trigger != RouteRecoveryTriggerUserCorrection {
		t.Fatalf("Trigger = %q, want user_correction; got=%+v", got.Trigger, got)
	}
	if got.FinalAction != RouteRecoveryActionContinueSameRoute || got.ProposedRoute != RouteFileRead || !got.SafeToContinue {
		t.Fatalf("decision = %+v, want safe same-route target correction", got)
	}
}

func TestRecoverRouteQAReviewMismatchRecordsDecision(t *testing.T) {
	got := RecoverRoute(RouteRecoveryInput{
		SelectedRoute:    RouteChatExplanation,
		ExpectedRoute:    RouteInternetSearch,
		QAReviewCategory: "wrong_route",
	})
	if got.Trigger != RouteRecoveryTriggerQAReviewRouteMismatch {
		t.Fatalf("Trigger = %q, want qa_review_route_mismatch; got=%+v", got.Trigger, got)
	}
	if got.ProposedRoute != RouteInternetSearch || got.FinalAction != RouteRecoveryActionAskClarification {
		t.Fatalf("decision = %+v, want expected route and clarification", got)
	}
}

func TestRecoverRouteDoesNotSilentlySwitchToRiskierRoute(t *testing.T) {
	got := RecoverRoute(RouteRecoveryInput{
		SelectedRoute: RouteChatExplanation,
		ProposedRoute: RouteFileWrite,
		Continuation:  BuildContinuationFrame(ContinuationInput{Content: "no, I meant edit the file"}),
	})
	if got.Trigger != RouteRecoveryTriggerUserCorrection {
		t.Fatalf("Trigger = %q, want user_correction; got=%+v", got.Trigger, got)
	}
	if got.FinalAction != RouteRecoveryActionRequestApproval || !got.RequiresApproval || got.SafeToContinue {
		t.Fatalf("decision = %+v, want approval before riskier route switch", got)
	}
}
