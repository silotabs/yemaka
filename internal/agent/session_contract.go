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
	if strings.TrimSpace(contract.PendingOperationID) != "" {
		ok, err := s.storedPendingOperationMatches(ctx, conversationID, contract)
		if err != nil {
			return err
		}
		if !ok {
			clearPendingOperation(&contract)
			contract = routing.SanitizeSessionContract(contract, time.Now())
			if contract.IsZero() {
				_ = s.Memory.DeleteConversationRouteState(ctx, conversationID)
				return nil
			}
		}
	}
	input.SessionContract = contract
	return nil
}

func (s *Service) storedPendingOperationMatches(ctx context.Context, conversationID string, contract routing.SessionContract) (bool, error) {
	if s == nil || s.Memory == nil {
		return false, nil
	}
	requestID := strings.TrimSpace(contract.PendingOperationID)
	if requestID == "" {
		return false, nil
	}
	runs, err := s.Memory.ListToolRunsForConversation(ctx, conversationID, 200)
	if err != nil {
		return false, err
	}
	for _, run := range runs {
		if normalizeToolName(run.ToolName) == "permission_decision" && permissionRunHasRequestID(run.Input, requestID) {
			return false, nil
		}
		if normalizeToolName(run.ToolName) == normalizeToolName(contract.PendingOperationType) && permissionRunHasRequestID(run.Input, requestID) {
			return false, nil
		}
	}
	for _, run := range runs {
		if normalizeToolName(run.ToolName) != "permission_request" {
			continue
		}
		request, ok := permissionRequestFromStoredValue(run.Output)
		if !ok || strings.TrimSpace(request.RequestID) != requestID {
			continue
		}
		if !pendingOperationRequestMatchesContract(request, contract) {
			continue
		}
		return true, nil
	}
	return false, nil
}

func pendingOperationRequestMatchesContract(request PermissionRequest, contract routing.SessionContract) bool {
	if normalizeToolName(contract.PendingOperationType) != "" &&
		normalizeToolName(request.ToolName) != normalizeToolName(contract.PendingOperationType) {
		return false
	}
	target := strings.TrimSpace(contract.PendingOperationTarget)
	if target == "" {
		return true
	}
	for _, part := range request.Command {
		if strings.TrimSpace(part) == target {
			return true
		}
	}
	if normalizeToolName(request.ToolName) == "edit_file" {
		return len(request.Command) == 0
	}
	return false
}

func clearPendingOperation(contract *routing.SessionContract) {
	if contract == nil {
		return
	}
	contract.PendingStep = ""
	contract.PendingOperationID = ""
	contract.PendingOperationType = ""
	contract.PendingOperationTarget = ""
	contract.PendingOperationStatus = ""
	contract.RouteLockStrength = ""
	if strings.TrimSpace(contract.TaskStatus) == routing.TaskStatusAwaitingApproval {
		contract.PendingApproval = ""
		switch strings.TrimSpace(contract.LastOutcome) {
		case routing.LastOutcomeBlocked, routing.LastOutcomeFailed:
			contract.TaskStatus = routing.TaskStatusBlocked
		default:
			contract.TaskStatus = routing.TaskStatusActive
		}
	}
}

func ClearPendingOperationState(ctx context.Context, store *memory.Store, conversationID string, requestID string) error {
	if store == nil || strings.TrimSpace(conversationID) == "" {
		return nil
	}
	stored, ok, err := store.GetConversationRouteState(ctx, conversationID)
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
		return store.DeleteConversationRouteState(ctx, conversationID)
	}
	if requestID != "" && strings.TrimSpace(contract.PendingOperationID) != strings.TrimSpace(requestID) {
		return nil
	}
	clearPendingOperation(&contract)
	contract.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if contract.IsZero() {
		return store.DeleteConversationRouteState(ctx, conversationID)
	}
	data, err := json.Marshal(contract)
	if err != nil {
		return err
	}
	_, err = store.SaveConversationRouteState(ctx, memory.ConversationRouteState{
		ConversationID: conversationID,
		StateJSON:      string(data),
	})
	return err
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
		Goal:                   plan.Goal,
		LastPlan:               strings.Join(plan.Steps, "\n"),
		CompletedStep:          completedStepForSession(plan, outcome),
		PendingStep:            pendingStepForSession(plan, decision),
		PendingApproval:        pendingApprovalForSession(decision),
		ContextSources:         contextSourcesForSession(plan, result),
		AmbiguityFlags:         append([]string{}, plan.AmbiguityFlags...),
		LastOutcome:            outcome,
		FailureReason:          failure,
		LastRecovery:           recovery,
		PendingOperationID:     pendingOperationIDForSession(decision),
		PendingOperationType:   pendingOperationTypeForSession(decision),
		PendingOperationTarget: pendingOperationTargetForSession(plan, decision),
		PendingOperationStatus: pendingOperationStatusForSession(decision),
		RouteLockStrength:      routeLockStrengthForSession(plan, decision),
	}
	next := routing.UpdateSessionContract(previous, routeDecision, update)
	if plan.ContinuationMode == routing.ContinuationModeApprovePendingAction && result != nil {
		clearPendingOperation(&next)
	}
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

func (s *Service) markRoutingLastFinalMessage(ctx context.Context, conversationID string, assistantMessage memory.Message, plan Plan, decision ExecutionDecision, result *ExecutionResult, verification VerificationResult) error {
	if s == nil || s.Memory == nil || strings.TrimSpace(conversationID) == "" {
		return nil
	}
	if !assistantMessageIsFinalArtifactCandidate(assistantMessage, plan, decision, result, verification) {
		return nil
	}
	stored, ok, err := s.Memory.GetConversationRouteState(ctx, conversationID)
	if err != nil {
		return err
	}
	contract := routing.SessionContract{}
	if ok {
		_ = json.Unmarshal([]byte(stored.StateJSON), &contract)
		if strings.TrimSpace(contract.UpdatedAt) == "" {
			contract.UpdatedAt = stored.UpdatedAt
		}
	}
	contract = routing.SanitizeSessionContract(contract, time.Now())
	contract.LastFinalMessageID = assistantMessage.ID
	contract.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	data, err := json.Marshal(contract)
	if err != nil {
		return err
	}
	_, err = s.Memory.SaveConversationRouteState(ctx, memory.ConversationRouteState{
		ConversationID: conversationID,
		StateJSON:      string(data),
	})
	return err
}

func assistantMessageIsFinalArtifactCandidate(message memory.Message, plan Plan, decision ExecutionDecision, result *ExecutionResult, verification VerificationResult) bool {
	if strings.TrimSpace(message.ID) == "" || strings.TrimSpace(message.Role) != "assistant" {
		return false
	}
	content := strings.TrimSpace(message.Content)
	if !usablePreviousAssistantEditContent(content) {
		return false
	}
	if plan.NeedsClarification || decision.Status == ExecutionNeedsConfirmation {
		return false
	}
	if result != nil && strings.TrimSpace(result.Status) == "failed" {
		return false
	}
	return strings.TrimSpace(verification.Status) != "rollback_required"
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
		MessageFrame:          plan.MessageFrame,
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

func pendingOperationIDForSession(decision ExecutionDecision) string {
	if decision.Status != ExecutionNeedsConfirmation {
		return ""
	}
	return strings.TrimSpace(decision.RequestID)
}

func pendingOperationTypeForSession(decision ExecutionDecision) string {
	if decision.Status != ExecutionNeedsConfirmation {
		return ""
	}
	return strings.TrimSpace(decision.ToolName)
}

func pendingOperationTargetForSession(plan Plan, decision ExecutionDecision) string {
	if decision.Status != ExecutionNeedsConfirmation {
		return ""
	}
	if len(decision.Command) > 1 {
		return strings.TrimSpace(decision.Command[1])
	}
	for _, file := range plan.FilesNeeded {
		if strings.TrimSpace(file) != "" {
			return strings.TrimSpace(file)
		}
	}
	return strings.TrimSpace(plan.RouteTarget)
}

func pendingOperationStatusForSession(decision ExecutionDecision) string {
	if decision.Status != ExecutionNeedsConfirmation {
		return ""
	}
	return "awaiting_approval"
}

func routeLockStrengthForSession(plan Plan, decision ExecutionDecision) string {
	if decision.Status == ExecutionNeedsConfirmation {
		return "pending_operation"
	}
	if plan.NeedsClarification {
		return "pending_clarification"
	}
	if plan.ContinuationMode == routing.ContinuationModeApprovePendingAction {
		return "pending_operation"
	}
	return ""
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
