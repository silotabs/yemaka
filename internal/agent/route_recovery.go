package agent

import (
	"strings"

	"yemaka/internal/routing"
)

func RouteRecoveryForExecution(plan Plan, decision ExecutionDecision, result *ExecutionResult, err error) routing.RouteRecoveryDecision {
	input := routing.RouteRecoveryInput{
		UserPrompt:        plan.Goal,
		SelectedRoute:     plan.RouteCategory,
		SelectedLane:      plan.RouteLane,
		SelectedRiskLevel: plan.RiskLevel,
		SelectedSource:    firstNonEmptyString(plan.EvidenceSource, routePreflightSource(plan)),
		ToolName:          decision.ToolName,
		ToolStatus:        decision.Status,
		ErrorText:         firstNonEmptyString(routeRecoveryErrorString(err), decision.Reason),
		ProposedRoute:     "",
		Continuation:      routeContinuation(plan),
	}
	if result != nil {
		input.ToolStatus = firstNonEmptyString(result.Status, input.ToolStatus)
		input.ResultSourceKind = result.SourceKind
		input.ResultSummary = firstNonEmptyString(result.Context, strings.Join(result.Sources, ", "))
		if input.ErrorText == "" && result.Status == "failed" {
			input.ErrorText = result.Context
		}
	}
	return routing.RecoverRoute(input)
}

func routePreflightSource(plan Plan) string {
	if plan.RoutePreflight == nil {
		return ""
	}
	return plan.RoutePreflight.SourceOfTruth
}

func routeContinuation(plan Plan) routing.ContinuationFrame {
	if plan.RouteContinuation == nil {
		return routing.ContinuationFrame{}
	}
	return *plan.RouteContinuation
}

func routeRecoveryErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func routeRecoverySessionReason(failure string, recovery routing.RouteRecoveryDecision) string {
	failure = strings.TrimSpace(failure)
	if recovery.IsZero() {
		return failure
	}
	parts := []string{
		failure,
		"route_recovery_trigger: " + recovery.Trigger,
		"route_recovery_action: " + recovery.FinalAction,
	}
	if recovery.Reason != "" {
		parts = append(parts, "route_recovery_reason: "+recovery.Reason)
	}
	return strings.Join(nonEmptyRouteRecoveryParts(parts), "\n")
}

func nonEmptyRouteRecoveryParts(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
