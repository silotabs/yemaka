package agent

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"yemaka/internal/memory"
	"yemaka/internal/routing"
)

func (s *Service) attachRoutingSessionContract(ctx context.Context, conversationID string, input *PlanInput) error {
	if s == nil || s.Memory == nil || input == nil || strings.TrimSpace(conversationID) == "" {
		return nil
	}
	if !input.SessionContract.IsZero() {
		return nil
	}
	stored, ok, err := s.Memory.GetConversationRouteState(ctx, conversationID)
	if err != nil || !ok {
		return err
	}
	var contract routing.SessionContract
	if err := json.Unmarshal([]byte(stored.StateJSON), &contract); err != nil {
		return nil
	}
	if strings.TrimSpace(contract.UpdatedAt) == "" {
		contract.UpdatedAt = stored.UpdatedAt
	}
	contract = routing.SanitizeSessionContract(contract, time.Now())
	if contract.IsZero() {
		_ = s.Memory.DeleteConversationRouteState(ctx, conversationID)
		return nil
	}
	input.SessionContract = contract
	return nil
}

func (s *Service) saveRoutingSessionContract(ctx context.Context, conversationID string, previous routing.SessionContract, plan Plan, decision ExecutionDecision, result *ExecutionResult, verification VerificationResult) error {
	if s == nil || s.Memory == nil || strings.TrimSpace(conversationID) == "" {
		return nil
	}
	routeDecision := routingDecisionFromPlan(plan)
	outcome, failure := routingOutcome(decision, result, verification)
	recovery := RouteRecoveryForExecution(plan, decision, result, nil)
	if !recovery.IsZero() {
		failure = routeRecoverySessionReason(failure, recovery)
	}
	update := routing.SessionUpdate{
		Goal:            plan.Goal,
		LastPlan:        strings.Join(plan.Steps, "\n"),
		CompletedStep:   completedStepForSession(plan, outcome),
		PendingStep:     pendingStepForSession(plan, decision),
		PendingApproval: pendingApprovalForSession(decision),
		ContextSources:  contextSourcesForSession(plan, result),
		AmbiguityFlags:  append([]string{}, plan.AmbiguityFlags...),
		LastOutcome:     outcome,
		FailureReason:   failure,
		LastRecovery:    recovery,
	}
	next := routing.UpdateSessionContract(previous, routeDecision, update)
	if next.TaskStatus == routing.TaskStatusStopped {
		return s.Memory.DeleteConversationRouteState(ctx, conversationID)
	}
	data, err := json.Marshal(next)
	if err != nil {
		return err
	}
	_, err = s.Memory.SaveConversationRouteState(ctx, memory.ConversationRouteState{
		ConversationID: conversationID,
		StateJSON:      string(data),
	})
	return err
}

func routingDecisionFromPlan(plan Plan) routing.Decision {
	decision := routing.Decision{
		TaskType:              plan.TaskType,
		Confidence:            plan.RouteConfidence,
		Reasons:               append([]string{}, plan.RouteReasons...),
		NeedsClarification:    plan.NeedsClarification,
		ClarificationQuestion: plan.ClarificationQuestion,
		Tools:                 append([]string{}, plan.ToolsNeeded...),
		Files:                 append([]string{}, plan.FilesNeeded...),
		RiskLevel:             plan.RiskLevel,
		UseWorkspace:          plan.RouteUsesWorkspace,
		UseRAG:                plan.RouteUsesRAG,
		UseInternet:           plan.RouteUsesInternet,
		RouteCategory:         plan.RouteCategory,
		Intent:                plan.RouteIntent,
		Domain:                plan.RouteDomain,
		Target:                plan.RouteTarget,
		Capability:            plan.RouteCapability,
		RequiresApproval:      plan.RouteRequiresApproval,
		ReadsFiles:            plan.RouteReadsFiles,
		WritesFiles:           plan.RouteWritesFiles,
		GeneratesExtension:    plan.RouteGeneratesExtension,
		CreatesSchedulerJob:   plan.RouteCreatesSchedulerJob,
		ConnectorAction:       plan.RouteConnectorAction,
		CrawlerTask:           plan.RouteCrawlerTask,
		RunsShell:             plan.RouteRunsShell,
		ToolLane:              plan.RouteLane,
		AllowedToolset:        append([]string{}, plan.RouteAllowedTools...),
		BlockedToolset:        append([]string{}, plan.RouteBlockedTools...),
		RouteCandidates:       append([]routing.RouteCandidate{}, plan.RouteCandidates...),
		ContinuationMode:      plan.ContinuationMode,
		AmbiguityFlags:        append([]string{}, plan.AmbiguityFlags...),
	}
	if plan.RoutePreflight != nil {
		decision.Preflight = *plan.RoutePreflight
	}
	if plan.RouteContinuation != nil {
		decision.Continuation = *plan.RouteContinuation
	}
	return decision
}

func routingOutcome(decision ExecutionDecision, result *ExecutionResult, verification VerificationResult) (string, string) {
	if strings.TrimSpace(verification.Status) == "rollback_required" {
		return routing.LastOutcomeFailed, strings.Join(verification.Reasons, "; ")
	}
	if result != nil {
		switch strings.TrimSpace(result.Status) {
		case "failed":
			return routing.LastOutcomeFailed, strings.TrimSpace(result.Context)
		case "blocked", "empty":
			return routing.LastOutcomeBlocked, strings.TrimSpace(result.Context)
		}
	}
	switch strings.TrimSpace(decision.Status) {
	case ExecutionBlocked:
		return routing.LastOutcomeBlocked, strings.TrimSpace(decision.Reason)
	case ExecutionNeedsConfirmation:
		return routing.LastOutcomeCompleted, ""
	default:
		return routing.LastOutcomeCompleted, ""
	}
}

func completedStepForSession(plan Plan, outcome string) string {
	if outcome == "" {
		return ""
	}
	if plan.RouteCategory == "" {
		return outcome
	}
	return plan.RouteCategory + ":" + outcome
}

func pendingStepForSession(plan Plan, decision ExecutionDecision) string {
	if plan.NeedsClarification {
		return plan.ClarificationQuestion
	}
	if decision.Status == ExecutionNeedsConfirmation {
		return decision.ToolName
	}
	return ""
}

func pendingApprovalForSession(decision ExecutionDecision) string {
	if decision.Status != ExecutionNeedsConfirmation {
		return ""
	}
	return firstNonEmptyString(decision.ToolName, decision.Reason)
}

func contextSourcesForSession(plan Plan, result *ExecutionResult) []string {
	sources := []string{}
	if plan.RoutePreflight != nil && strings.TrimSpace(plan.RoutePreflight.SourceOfTruth) != "" {
		sources = append(sources, plan.RoutePreflight.SourceOfTruth)
	}
	if result != nil {
		sources = append(sources, result.Sources...)
	}
	return sources
}
